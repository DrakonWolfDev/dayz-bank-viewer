package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in only: no corpus identities, names, balances or file contents enter fixtures.
func TestPrivateCorpusReadOnly(t *testing.T) {
	dir := os.Getenv("DAYZ_BANK_TEST_CORPUS")
	if dir == "" {
		t.Skip("set DAYZ_BANK_TEST_CORPUS to a local banking directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string][32]byte{}
	total := new(big.Int)
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal("corpus unreadable")
		}
		hashes[e.Name()] = sha256.Sum256(data)
		var raw struct {
			Balance int64 `json:"m_OwnedCurrency"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal("corpus JSON invalid")
		}
		total.Add(total, big.NewInt(raw.Balance))
	}
	snap, err := scanDirectory(context.Background(), dir, BankingReader{})
	if err != nil {
		t.Fatal("corpus scan failed")
	}
	if len(snap.Issues) != 0 || len(snap.Accounts) != len(hashes) {
		t.Fatal("corpus scan skipped accounts")
	}
	m := testManager(t, ServerConfig{"test", "Private test corpus", dir, true})
	refreshWait(t, m, "")
	d, err := m.Dashboard("")
	if err != nil || d.Total != total.String() {
		t.Fatal("corpus summary does not match independent calculation")
	}
	for file, hash := range hashes {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil || sha256.Sum256(data) != hash {
			t.Fatal("corpus was modified")
		}
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(entries) {
		t.Fatal("corpus file list changed")
	}
	t.Logf("Read %d accounts; totals matched; all source hashes unchanged", len(snap.Accounts))
}
