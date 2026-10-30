package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type instance struct {
	URL     string `json:"url"`
	Control string `json:"control"`
	Token   string `json:"token"`
	Version string `json:"version"`
}

func bindAvailable(address string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	first, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	for p := first; p <= first+20 && p <= 65535; p++ {
		listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p)))
		if err == nil {
			return listener, nil
		}
		if !errors.Is(err, portInUse) {
			return nil, fmt.Errorf("cannot listen on %s: %w", address, err)
		}
	}
	return nil, fmt.Errorf("no free port in the range %d–%d", first, min(first+20, 65535))
}
func readInstance(config string) (instance, error) {
	var state instance
	raw, err := os.ReadFile(config + ".runtime")
	if err != nil {
		raw, err = os.ReadFile(config + ".status")
	}
	if err != nil {
		return state, err
	}
	if err = json.Unmarshal(raw, &state); err != nil {
		return state, err
	}
	// Never send the private token to a destination supplied outside loopback.
	for i, value := range []string{state.Control, state.URL} {
		req, e := http.NewRequest("GET", value, nil)
		if e != nil || req.URL.Scheme != "http" || req.URL.User != nil {
			return state, errors.New("invalid instance address")
		}
		ip := net.ParseIP(req.URL.Hostname())
		if ip == nil || (i == 0 && !ip.IsLoopback()) {
			return state, errors.New("instance is not local")
		}
	}
	_, err = instanceRequest(state, "GET", "/status")
	return state, err
}
func instanceRequest(state instance, method, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, state.Control+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+state.Token)
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("control returned HTTP %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 4096))
}
func publishInstance(config, dashboard string, stop context.CancelFunc) (func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	state := instance{URL: dashboard, Control: "http://" + listener.Addr().String(), Token: newID() + newID(), Version: version}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		public := state
		public.Token = ""
		writeJSON(w, 200, public)
	})
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		body := []byte("{\"status\":\"stopping\"}\n")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write(body)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		go stop()
	})
	srv := &http.Server{ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/status" {
			mux.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+state.Token)) != 1 {
			http.Error(w, "forbidden", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})}
	raw, _ := json.Marshal(state)
	f, err := os.CreateTemp(filepath.Dir(config), ".viewer-runtime-*")
	if err != nil {
		listener.Close()
		return nil, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = secureRuntime(tmp); err != nil {
		f.Close()
		listener.Close()
		return nil, err
	}
	if _, err = f.Write(raw); err == nil {
		err = f.Close()
	} else {
		f.Close()
	}
	if err == nil {
		err = os.Rename(tmp, config+".runtime")
	}
	if err != nil {
		listener.Close()
		return nil, err
	}
	public := state
	public.Token = ""
	publicRaw, _ := json.Marshal(public)
	if err = os.WriteFile(config+".status", publicRaw, 0644); err != nil {
		listener.Close()
		os.Remove(config + ".runtime")
		return nil, err
	}
	if err = os.Chmod(config+".status", 0644); err != nil {
		listener.Close()
		os.Remove(config + ".runtime")
		os.Remove(config + ".status")
		return nil, err
	}
	go srv.Serve(listener)
	return func() { srv.Close(); os.Remove(config + ".runtime"); os.Remove(config + ".status") }, nil
}
func ensureRunning(config string) (instance, bool, error) {
	if state, err := readInstance(config); err == nil {
		return state, false, nil
	}
	if managedInstall(config) {
		return waitManaged(config)
	}
	exe, err := os.Executable()
	if err != nil {
		return instance{}, false, err
	}
	if err = os.MkdirAll(filepath.Dir(config), 0700); err != nil {
		return instance{}, false, err
	}
	var launchUnlock func()
	for i := 0; i < 100; i++ {
		launchUnlock, err = acquireLock(config + ".launch.lock")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return instance{}, false, fmt.Errorf("another launch is still in progress: %w", err)
	}
	defer launchUnlock()
	if state, e := readInstance(config); e == nil {
		return state, false, nil
	}
	logFile, err := os.OpenFile(config+".log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return instance{}, false, err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "-config", config, "-headless")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	detach(cmd)
	if err = cmd.Start(); err != nil {
		return instance{}, false, err
	}
	go cmd.Wait()
	for i := 0; i < 100; i++ {
		if state, e := readInstance(config); e == nil {
			return state, true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return instance{}, false, fmt.Errorf("viewer did not become ready; see %s.log", config)
}
func stopInstance(config string) error {
	state, err := readInstance(config)
	if err != nil {
		return fmt.Errorf("viewer is not reachable: %w", err)
	}
	if _, err = instanceRequest(state, "POST", "/stop"); err != nil {
		return err
	}
	for i := 0; i < 70; i++ {
		if _, e := readInstance(config); e != nil {
			release, e := acquireLock(config + ".lock")
			if e == nil {
				release()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("viewer did not stop in time")
}
