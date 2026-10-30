package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const repository = "DrakonWolfDev/dayz-bank-viewer"
const releasesURL = "https://github.com/" + repository + "/releases"
const forgeURL = "https://wolf-forge.co.uk/"

type UpdateResult struct {
	Installed string `json:"installed"`
	Latest    string `json:"latest,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	URL       string `json:"url,omitempty"`
}

// Stable three-component versions only; malformed and prerelease tags fail closed.
func versionParts(s string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("unexpected release version")
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return result, fmt.Errorf("unexpected release version")
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return result, fmt.Errorf("unexpected release version")
			}
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return result, err
		}
		result[i] = n
	}
	return result, nil
}
func checkUpdate(ctx context.Context, client *http.Client, endpoint, installed string) UpdateResult {
	result := UpdateResult{Installed: installed, Status: "failed", Message: "Could not check for updates. Check your connection and try again."}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DayZ-Bank-Viewer/"+installed)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := client.Do(req)
	if err != nil {
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		result.Message = fmt.Sprintf("GitHub returned HTTP %d. Try again later.", response.StatusCode)
		return result
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return result
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if json.Unmarshal(raw, &release) != nil || release.Draft || release.Prerelease {
		result.Message = "GitHub returned unexpected release information."
		return result
	}
	latest, err := versionParts(release.Tag)
	current, e := versionParts(installed)
	if err != nil || e != nil {
		result.Message = "GitHub returned an unsupported release version."
		return result
	}
	result.Latest = strings.TrimPrefix(release.Tag, "v")
	result.Status = "current"
	result.Message = "You are up to date."
	for i := range latest {
		if latest[i] > current[i] {
			result.Status = "available"
			result.Message = "Version " + result.Latest + " is available."
			result.URL = releasesURL + "/tag/" + url.PathEscape(release.Tag)
			break
		}
		if latest[i] < current[i] {
			break
		}
	}
	return result
}

var updateCheck struct {
	sync.Mutex
	at     time.Time
	result UpdateResult
}

func manualUpdate(ctx context.Context) UpdateResult {
	updateCheck.Lock()
	defer updateCheck.Unlock()
	if time.Since(updateCheck.at) < 30*time.Second {
		return updateCheck.result
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	updateCheck.result = checkUpdate(ctx, client, "https://api.github.com/repos/"+repository+"/releases/latest", version)
	updateCheck.at = time.Now()
	return updateCheck.result
}
