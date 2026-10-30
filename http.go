package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
)

//go:embed web/* LICENSE
var webFiles embed.FS

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func apiError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, out any) error {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return errors.New("send application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.New("request exceeds 1 MiB or could not be read")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		return errors.New("request must be a JSON object")
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return errors.New("invalid request JSON")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}
func handler(m *Manager, loopback bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /licence", func(w http.ResponseWriter, r *http.Request) {
		data, _ := webFiles.ReadFile("LICENSE")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"version": version}) })
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, r *http.Request) {
		d, err := m.Dashboard(r.URL.Query().Get("server"))
		if err != nil {
			apiError(w, 404, err.Error())
			return
		}
		writeJSON(w, 200, d)
	})
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, m.Config()) })
	mux.HandleFunc("PUT /api/config", func(w http.ResponseWriter, r *http.Request) {
		var c Config
		if err := decodeJSON(w, r, &c); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		if err := m.Update(c); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, m.Config())
	})
	mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ServerID string `json:"server_id"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			apiError(w, 400, err.Error())
			return
		}
		n, err := m.Refresh(body.ServerID)
		if err != nil {
			apiError(w, 404, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]int{"started": n})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { apiError(w, 404, "API endpoint not found") })
	files, _ := fs.Sub(webFiles, "web")
	static := http.FileServer(http.FS(files))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/style.css" && r.URL.Path != "/app.js" && r.URL.Path != "/favicon.svg" {
			http.NotFound(w, r)
			return
		}
		static.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Cache-Control", "no-store")
		// Protect a local administration interface against DNS rebinding.
		if loopback {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			ip := net.ParseIP(strings.Trim(host, "[]"))
			if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				apiError(w, 403, "use a localhost or loopback address")
				return
			}
		}
		origin := r.Header.Get("Origin")
		if origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
			apiError(w, 403, "cross-origin requests are not allowed")
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			apiError(w, 403, "cross-site requests are not allowed")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
