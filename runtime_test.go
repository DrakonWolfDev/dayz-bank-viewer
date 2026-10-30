package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPortFallbackAndOwnership(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	listener, err := bindAvailable(occupied.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if listener.Addr().String() == occupied.Addr().String() {
		t.Fatal("reused occupied socket")
	}
	if _, err = net.Listen("tcp", listener.Addr().String()); err == nil {
		t.Fatal("bound socket was not retained")
	}
	path := filepath.Join(t.TempDir(), "config.lock")
	release, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, e := acquireLock(path); e == nil {
		second()
		t.Fatal("duplicate ownership")
	}
	release()
	release, err = acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	release()
}
func TestPrivateControlAndStaleDiscovery(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closeControl, err := publishInstance(config, "http://127.0.0.1:20784", cancel)
	if err != nil {
		t.Fatal(err)
	}
	defer closeControl()
	state, err := readInstance(config)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(state.Control+"/stop", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("unauthenticated stop allowed")
	}
	public, err := instanceRequest(state, "GET", "/status")
	if err != nil {
		t.Fatal(err)
	}
	var result instance
	if json.Unmarshal(public, &result) != nil || result.Token != "" {
		t.Fatal("token exposed in public discovery")
	}
	if _, err = instanceRequest(state, "POST", "/stop"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("stop not delivered")
	}
	closeControl()
	if _, err = readInstance(config); err == nil {
		t.Fatal("stale instance accepted")
	}
	if err = os.WriteFile(config+".runtime", []byte(`{"control":"https://example.com","url":"http://127.0.0.1:20784","token":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readInstance(config); err == nil {
		t.Fatal("external destination accepted")
	}
}
