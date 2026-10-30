package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopStateRunningAndStopped(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.json")
	if state := readDesktopState(config); state.Running || state.URL != "" {
		t.Fatalf("missing runtime reported as running: %+v", state)
	}
	cleanup, err := publishInstance(config, "http://127.0.0.1:20784", func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	state := readDesktopState(config)
	if !state.Running || state.URL != "http://127.0.0.1:20784" {
		t.Fatalf("healthy runtime not reported: %+v", state)
	}
	raw, err := json.Marshal(state)
	if err != nil || strings.Contains(string(raw), "token") || strings.Contains(string(raw), "control") {
		t.Fatalf("desktop status must contain only public fields: %s (%v)", raw, err)
	}
	cleanup()
	// An old discovery file must not leave the UI showing a running viewer.
	if err := os.WriteFile(config+".runtime", []byte(`{"url":"http://127.0.0.1:20784","control":"invalid","token":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if stopped := readDesktopState(config); stopped.Running || stopped.URL != "" || stopped.Account != "" {
		t.Fatalf("stale discovery leaked into stopped state: %+v", stopped)
	}
}
