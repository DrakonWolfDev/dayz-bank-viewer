package main

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func testManager(t *testing.T, servers ...ServerConfig) *Manager {
	t.Helper()
	c := defaultConfig()
	c.Servers = servers
	m := newManager(c, filepath.Join(t.TempDir(), "config.json"))
	t.Cleanup(m.Close)
	return m
}
func refreshWait(t *testing.T, m *Manager, id string) {
	t.Helper()
	if _, err := m.Refresh(id); err != nil {
		t.Fatal(err)
	}
	m.wg.Wait()
}
func TestSnapshotRecoveryAndTotals(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	put(t, a, "one.json", fixture("76561198000000001", "One", 9223372036854775807))
	put(t, b, "one.json", fixture("76561198000000001", "One", 9223372036854775807))
	m := testManager(t, ServerConfig{ID: "a", Name: "Alpha", Path: a, Enabled: true}, ServerConfig{ID: "b", Name: "Bravo", Path: b, Enabled: true})
	refreshWait(t, m, "")
	d, err := m.Dashboard("")
	if err != nil {
		t.Fatal(err)
	}
	if d.Total != "18446744073709551614" || d.UniquePlayers != 1 || d.AccountCount != 2 || !d.Complete {
		t.Fatalf("incorrect cross-server totals: %+v", d)
	}
	original := m.Views()[0].Snapshot
	success := *m.Views()[0].LastSuccess
	put(t, a, "broken.json", "{bad")
	refreshWait(t, m, "a")
	v := m.Views()[0]
	if v.Status != "partial" || v.Snapshot == original || !v.LastSuccess.Equal(success) {
		t.Fatal("partial scan replaced successful timestamp or failed to publish snapshot")
	}
	if len(original.Issues) != 0 {
		t.Fatal("published snapshot mutated")
	}
	os.Remove(filepath.Join(a, "broken.json"))
	put(t, a, "one.json", "invalid")
	partial := v.Snapshot
	refreshWait(t, m, "a")
	v = m.Views()[0]
	if v.Status != "error" || v.Snapshot != partial || len(v.FailedFiles) != 1 {
		t.Fatal("failed scan did not preserve cached snapshot and diagnostics")
	}
	d, _ = m.Dashboard("")
	if d.Complete || d.AccountCount != 2 {
		t.Fatal("failed results silently reported complete or lost cache")
	}
	put(t, a, "one.json", fixture("76561198000000001", "Recovered", 12))
	refreshWait(t, m, "a")
	if m.Views()[0].Status != "ready" || m.Views()[0].Error != "" {
		t.Fatal("scan did not recover")
	}
	c := m.Config()
	c.Servers[1].Enabled = false
	if err := m.Update(c); err != nil {
		t.Fatal(err)
	}
	d, _ = m.Dashboard("")
	if d.Total != "12" || d.AccountCount != 1 {
		t.Fatal("disabled server included in network totals")
	}
	if n, err := m.Refresh("b"); err != nil || n != 0 {
		t.Fatal("disabled server refreshed")
	}
	if _, err := m.Dashboard("missing"); err == nil {
		t.Fatal("unknown server accepted")
	}
	c = m.Config()
	c.Servers = nil
	if err := m.Update(c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a, "one.json")); err != nil {
		t.Fatal("removing configuration touched banking files")
	}
}

type blockingReader struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (b *blockingReader) Read(r io.Reader) (Account, error) {
	b.once.Do(func() { close(b.started) })
	<-b.release
	return (BankingReader{}).Read(r)
}
func TestIndependentScansAndReconfiguration(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "one.json", fixture("76561198000000001", "One", 20))
	missing := filepath.Join(t.TempDir(), "missing")
	m := testManager(t, ServerConfig{ID: "a", Name: "Alpha", Path: dir, Enabled: true}, ServerConfig{ID: "b", Name: "Broken", Path: missing, Enabled: true})
	b := &blockingReader{started: make(chan struct{}), release: make(chan struct{})}
	m.reader = b
	var release sync.Once
	defer release.Do(func() { close(b.release) })
	if n, err := m.Refresh(""); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	select {
	case <-b.started:
	case <-time.After(3 * time.Second):
		t.Fatal("scan did not start")
	}
	if n, err := m.Refresh("a"); err != nil || n != 0 {
		t.Fatal("overlapping scan was allowed")
	}
	deadline := time.Now().Add(3 * time.Second)
	for m.Views()[1].Status != "error" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.Views()[1].Status != "error" {
		t.Fatal("a blocked server prevented an independent scan")
	}
	c := m.Config()
	c.Servers[0].Path = t.TempDir()
	if err := m.Update(c); err != nil {
		t.Fatal(err)
	}
	release.Do(func() { close(b.release) })
	m.wg.Wait()
	if m.Views()[0].Snapshot != nil {
		t.Fatal("old-path scan published into the replacement configuration")
	}
	refreshWait(t, m, "a")
	if s := m.Views()[0]; s.Status != "ready" || len(s.Snapshot.Accounts) != 0 {
		t.Fatal("new path was not used")
	}
}
func TestConcurrentReadersSeeAtomicSnapshots(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "a.json", fixture("76561198000000001", "One", 10))
	put(t, dir, "b.json", fixture("76561198000000002", "Two", 20))
	m := testManager(t, ServerConfig{ID: "a", Name: "Alpha", Path: dir, Enabled: true})
	refreshWait(t, m, "")
	var readers sync.WaitGroup
	for i := 0; i < 8; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 100; j++ {
				d, err := m.Dashboard("")
				if err != nil || d.Total != "30" || d.AccountCount != 2 {
					t.Error("half-published snapshot")
				}
			}
		}()
	}
	for i := 0; i < 10; i++ {
		refreshWait(t, m, "")
	}
	readers.Wait()
}
