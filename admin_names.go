package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxAdminFiles            = 60
	maxAdminFileBytes  int64 = 4 << 20
	maxAdminTotalBytes int64 = maxAdminFiles * maxAdminFileBytes
)

type NameLookup struct {
	Directories []string    `json:"directories"`
	Files       int         `json:"files"`
	Matched     int         `json:"matched"`
	Issues      []FileIssue `json:"issues"`
}

// Anchor to an ADM event, so quoted player text inside chat cannot supply names.
// Both older connection lines and newer lines with positions are supported.
var adminPlayer = regexp.MustCompile(`^[ \t]*[0-9]{2}:[0-9]{2}:[0-9]{2}[ \t]*\|[ \t]*Player "([^"\r\n]*)"[ \t]*(?:is connected[ \t]*)?\(id=([A-Za-z0-9_+=/\-]{1,128})(?:[ \t][^)]*)?\)`)

func adminLogDirectories(config ServerConfig) []string {
	if config.LogPath != "" {
		return []string{config.LogPath}
	}
	// Expansion's standard layout is profiles/ExpansionMod/ATM. Never search
	// arbitrary ancestors: an unrelated source must not borrow another server's names.
	if strings.EqualFold(filepath.Base(config.Path), "ATM") && strings.EqualFold(filepath.Base(filepath.Dir(config.Path)), "ExpansionMod") {
		profiles := filepath.Dir(filepath.Dir(config.Path))
		dirs := []string{profiles}
		dirs = append(dirs, knownDirectories(context.Background(), profiles, "log_storage")...)
		if profileDirectoryNames[strings.ToLower(filepath.Base(profiles))] {
			dirs = append(dirs, knownDirectories(context.Background(), filepath.Dir(profiles), "log_storage")...)
		}
		return dirs
	}
	return nil
}

func enrichAccountNames(ctx context.Context, snap *Snapshot, config ServerConfig) {
	wanted := map[string]bool{}
	for _, account := range snap.Accounts {
		if account.PlayerID != "" && account.Name == "" {
			wanted[account.PlayerID] = true
		}
	}
	if len(wanted) == 0 {
		return
	}
	dirs := adminLogDirectories(config)
	if len(dirs) == 0 {
		return
	}
	lookup := &NameLookup{Directories: dirs, Issues: []FileIssue{}}
	snap.NameLookup = lookup
	names := readAdminNames(ctx, dirs, wanted, lookup)
	for i := range snap.Accounts {
		account := &snap.Accounts[i]
		if name := names[account.PlayerID]; account.Name == "" && name != "" {
			account.Name, account.NameSource = name, "admin_log"
			lookup.Matched++
		}
	}
}

func readAdminNames(ctx context.Context, dirs []string, wanted map[string]bool, lookup *NameLookup) map[string]string {
	names := map[string]string{}
	type logFile struct {
		name string
		info os.FileInfo
	}
	files := []logFile{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			lookup.Issues = append(lookup.Issues, FileIssue{dir, "cannot read admin log directory: " + err.Error()})
			continue
		}
		for _, entry := range entries {
			if ctx.Err() != nil {
				return names
			}
			if !strings.EqualFold(filepath.Ext(entry.Name()), ".adm") {
				continue
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				lookup.Issues = append(lookup.Issues, FileIssue{filepath.Join(dir, entry.Name()), "admin log is unavailable or not a regular file (links are not followed)"})
				continue
			}
			files = append(files, logFile{filepath.Join(dir, entry.Name()), info})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if !files[i].info.ModTime().Equal(files[j].info.ModTime()) {
			return files[i].info.ModTime().After(files[j].info.ModTime())
		}
		return files[i].name > files[j].name
	})
	if len(files) > maxAdminFiles {
		lookup.Issues = append(lookup.Issues, FileIssue{"ADM lookup", fmt.Sprintf("name lookup limited to the %d most recently modified ADM files", maxAdminFiles)})
		files = files[:maxAdminFiles]
	}
	var used int64
	for _, file := range files {
		if ctx.Err() != nil || len(names) == len(wanted) {
			break
		}
		remaining := maxAdminTotalBytes - used
		if remaining <= 0 {
			lookup.Issues = append(lookup.Issues, FileIssue{"ADM lookup", fmt.Sprintf("name lookup reached the %d MiB read limit", maxAdminTotalBytes>>20)})
			break
		}
		limit := min(maxAdminFileBytes, remaining)
		found, count, err := readAdminFile(ctx, file.name, wanted, limit)
		used += count
		lookup.Files++
		if file.info.Size() > limit {
			lookup.Issues = append(lookup.Issues, FileIssue{file.name, "only the recent tail of this log was searched (bounded name lookup)"})
		}
		if err != nil {
			lookup.Issues = append(lookup.Issues, FileIssue{file.name, "could not read all admin log entries: " + err.Error()})
		}
		for id, name := range found {
			if names[id] == "" {
				names[id] = name
			}
		}
	}
	return names
}

func readAdminFile(ctx context.Context, path string, wanted map[string]bool, limit int64) (map[string]string, int64, error) {
	names := map[string]string{}
	info, err := os.Lstat(path)
	if err != nil {
		return names, 0, err
	}
	if !info.Mode().IsRegular() {
		return names, 0, fmt.Errorf("not a regular file (links are not followed)")
	}
	f, err := os.Open(path) // Read-only, including when the server is writing this log.
	if err != nil {
		return names, 0, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return names, 0, err
	}
	start := max(int64(0), info.Size()-limit)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return names, 0, err
	}
	reader := bufio.NewReader(io.LimitReader(f, limit))
	if start > 0 {
		// Ignore the partial first line after seeking into the tail.
		if _, err := reader.ReadString('\n'); err != nil {
			return names, min(info.Size(), limit), err
		}
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return names, min(info.Size(), limit), err
		}
		line := strings.TrimPrefix(scanner.Text(), "\ufeff")
		match := adminPlayer.FindStringSubmatch(line)
		if match == nil || !wanted[match[2]] {
			continue
		}
		name := match[1]
		if strings.TrimSpace(name) == "" || len(name) > 512 || !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			continue
		}
		// Last usable entry wins within a file; newer files win across files.
		names[match[2]] = name
	}
	return names, min(info.Size(), limit), scanner.Err()
}
