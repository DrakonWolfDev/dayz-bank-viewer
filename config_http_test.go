package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigPersistenceAndFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := loadConfig(path)
	if err != nil || c.Listen != "127.0.0.1:20784" {
		t.Fatal(c, err)
	}
	c.Servers = []ServerConfig{{Name: " Test ", Path: t.TempDir(), Enabled: true}}
	c.RefreshTimes = []string{"06:00", "18:00"}
	if err := validateConfig(&c, path); err != nil {
		t.Fatal(err)
	}
	if c.Servers[0].ID == "" || c.Servers[0].Name != "Test" {
		t.Fatal("configuration not normalised")
	}
	for i := 0; i < 2; i++ {
		if err := saveConfig(path, c); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := loadConfig(path)
	if err != nil || loaded.Servers[0].ID != c.Servers[0].ID {
		t.Fatal("configuration did not round-trip", err)
	}
	for _, body := range []string{"null", "{}{}", `{"m_Steam64ID":"76561198000000001"}`} {
		put(t, filepath.Dir(path), "config.json", body)
		if _, err := loadConfig(path); err == nil {
			t.Fatal("invalid config accepted", body)
		}
	}
	tests := []Config{
		{Listen: "bad"}, {Listen: "localhost:0"}, {Listen: "localhost:99999"},
		{Servers: []ServerConfig{{Name: "A", Path: "relative"}}},
		{Servers: []ServerConfig{{Name: "A", Path: filepath.Dir(path)}}},
		{RefreshTimes: []string{"6:00"}}, {RefreshTimes: []string{"24:00"}}, {RefreshTimes: []string{"06:00", "06:00"}},
		{Servers: []ServerConfig{{ID: "a", Name: "A", Path: t.TempDir()}, {ID: "a", Name: "B", Path: t.TempDir()}}},
	}
	for _, c := range tests {
		if err := validateConfig(&c, path); err == nil {
			t.Fatalf("invalid config accepted: %+v", c)
		}
	}
	m := testManager(t)
	m.configPath = filepath.Join(t.TempDir(), "no-parent", "config.json")
	c = defaultConfig()
	c.RefreshTimes = []string{"12:00"}
	if err := m.Update(c); err == nil {
		t.Fatal("save failure ignored")
	}
	if len(m.Config().RefreshTimes) != 0 {
		t.Fatal("in-memory configuration changed after save failure")
	}
}
func TestScheduleHostWallClock(t *testing.T) {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	fired := map[string]bool{}
	first := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC).In(loc)
	repeated := first.Add(time.Hour)
	if !scheduleDue(first, []string{"01:30"}, fired) {
		t.Fatal("schedule missed first occurrence")
	}
	if scheduleDue(repeated, []string{"01:30"}, fired) {
		t.Fatal("repeated DST minute fired twice")
	}
	if scheduleDue(first, []string{}, fired) {
		t.Fatal("disabled schedule fired")
	}
	next := time.Date(2026, 10, 26, 1, 30, 0, 0, loc)
	if !scheduleDue(next, []string{"01:30"}, fired) {
		t.Fatal("next day did not fire")
	}
}
func request(h http.Handler, method, path, body, origin, host string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Host = host
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestHTTPAPIAndLocalProtection(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "one.json", fixture("76561198000000001", "<script>alert(1)</script>", 9007199254740993))
	m := testManager(t, ServerConfig{ID: "a", Name: "Test", Path: dir, Enabled: true})
	refreshWait(t, m, "")
	h := handler(m, true)
	w := request(h, "GET", "/api/dashboard", "", "", "localhost:20784")
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"balance":"9007199254740993"`)) {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing content security policy")
	}
	for _, tc := range []struct {
		method, path, body, origin, host string
		status                           int
	}{
		{"GET", "/", "", "", "localhost:20784", 200},
		{"GET", "/app.js", "", "", "127.0.0.1:20784", 200},
		{"GET", "/corpus/secret.json", "", "", "localhost:20784", 404},
		{"GET", "/api/dashboard?server=missing", "", "", "localhost:20784", 404},
		{"GET", "/api/dashboard", "", "", "evil.example", 403},
		{"PUT", "/api/config", "{}", "https://evil.example", "localhost:20784", 403},
		{"PUT", "/api/config", "null", "", "localhost:20784", 400},
		{"PUT", "/api/config", "{}{}", "", "localhost:20784", 400},
		{"PUT", "/api/config", "{\"unknown\":true}", "", "localhost:20784", 400},
		{"POST", "/api/refresh", "{\"server_id\":\"missing\"}", "", "localhost:20784", 404},
		{"POST", "/api/refresh", "{}", "http://localhost:20784", "localhost:20784", 202},
	} {
		w := request(h, tc.method, tc.path, tc.body, tc.origin, tc.host)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d want %d: %s", tc.method, tc.path, w.Code, tc.status, w.Body.String())
		}
	}
	c := m.Config()
	c.Servers[0].Name = "Renamed"
	data, _ := json.Marshal(c)
	w = request(h, "PUT", "/api/config", string(data), "", "localhost:20784")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	loaded, err := loadConfig(m.configPath)
	if err != nil || loaded.Servers[0].Name != "Renamed" {
		t.Fatal("settings did not persist", err)
	}
	r := httptest.NewRequest("PUT", "/api/config", strings.NewReader("{}"))
	r.Host = "localhost:20784"
	r.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("non-JSON mutation accepted")
	}
	r = httptest.NewRequest("GET", "/api/config", nil)
	r.Host = "localhost:20784"
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-site read accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "one.json")); err != nil {
		t.Fatal(err)
	}
}
