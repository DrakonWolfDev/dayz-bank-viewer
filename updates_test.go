package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUpdateResponses(t *testing.T) {
	for _, test := range []struct {
		name, body string
		code       int
		status     string
	}{
		{"newer", `{"tag_name":"v1.10.0"}`, 200, "available"},
		{"same", `{"tag_name":"1.4.0"}`, 200, "current"},
		{"older", `{"tag_name":"v1.3.9"}`, 200, "current"},
		{"prerelease", `{"tag_name":"v2.0.0","prerelease":true}`, 200, "failed"},
		{"draft", `{"tag_name":"v2.0.0","draft":true}`, 200, "failed"},
		{"invalid tag", `{"tag_name":"v2.0.0<script>"}`, 200, "failed"},
		{"missing", `{}`, 200, "failed"}, {"null", `null`, 200, "failed"},
		{"bad json", `<html>`, 200, "failed"}, {"no release", `{}`, 404, "failed"},
		{"rate limited", `{}`, 403, "failed"}, {"too large", strings.Repeat("x", (1<<20)+1), 200, "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("User-Agent") == "" {
					t.Error("missing agent")
				}
				w.WriteHeader(test.code)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			result := checkUpdate(context.Background(), server.Client(), server.URL, "1.4.0")
			if result.Status != test.status {
				t.Fatalf("%+v", result)
			}
			if result.Status == "available" && result.URL != releasesURL+"/tag/v1.10.0" {
				t.Fatal(result.URL)
			}
			if result.Status != "available" && result.URL != "" {
				t.Fatal("unexpected download link")
			}
		})
	}
}
func TestUpdateTimeoutAndRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if result := checkUpdate(ctx, server.Client(), server.URL, "1.4.0"); result.Status != "failed" {
		t.Fatal(result)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.invalid/", 302) }))
	defer redirect.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if result := checkUpdate(context.Background(), client, redirect.URL, "1.4.0"); result.Status != "failed" {
		t.Fatal(result)
	}
}
