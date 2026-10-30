package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const nameTestID = "synthetic-DayZ-hashed-identity="

func TestAdminNamesPreferRecentLogsAndLastEntry(t *testing.T) {
	dir := t.TempDir()
	old := put(t, dir, "old.ADM", `12:00:00 | Player "Old name" is connected (id=`+nameTestID+`)`)
	if err := os.Chtimes(old, time.Unix(100, 0), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	text := "\ufeff12:00:00 | Player \"First name\" is connected (id=" + nameTestID + ")\r\n" +
		`12:01:00 | Player "Latest <name>"(id=` + nameTestID + ` pos=<1,2,3>) is connected` + "\r\n" +
		`12:02:00 | Chat("attacker"): Player "Forged name"(id=` + nameTestID + `)` + "\n" +
		`12:03:00 | Player "   "(id=` + nameTestID + `) has been disconnected` + "\n"
	latest := put(t, dir, "new.adm", text)
	put(t, dir, "ignored.log", `12:04:00 | Player "Wrong file"(id=`+nameTestID+`)`)
	snap := &Snapshot{Accounts: []Account{{PlayerID: nameTestID, Format: "expansion"}, {PlayerID: "missing", Format: "expansion"}, {SteamID: "76561198000000001", Name: "Bank name", Format: "banking"}}}
	enrichAccountNames(context.Background(), snap, ServerConfig{LogPath: dir})
	if snap.Accounts[0].Name != "Latest <name>" || snap.Accounts[0].NameSource != "admin_log" || snap.Accounts[0].SteamID != "" {
		t.Fatalf("incorrect name lookup: %+v", snap.Accounts[0])
	}
	if snap.Accounts[1].Name != "" || snap.Accounts[2].Name != "Bank name" || snap.NameLookup.Matched != 1 || len(snap.NameLookup.Issues) != 0 {
		t.Fatalf("unexpected lookup result: %+v", snap)
	}
	data, err := os.ReadFile(latest)
	if err != nil || string(data) != text {
		t.Fatal("admin log changed", err)
	}
}

func TestAdminLogDirectoryDiscovery(t *testing.T) {
	profiles := t.TempDir()
	atm := filepath.Join(profiles, "ExpansionMod", "ATM")
	if err := os.MkdirAll(atm, 0700); err != nil {
		t.Fatal(err)
	}
	put(t, atm, "account.json", expansionAccount)
	put(t, profiles, "session.ADM", `12:00:00 | Player "Synthetic name" (id=`+nameTestID+`) is connected`)
	m := testManager(t, ServerConfig{ID: "a", Name: "Expansion", Path: atm, Enabled: true})
	refreshWait(t, m, "")
	d, err := m.Dashboard("")
	if err != nil || len(d.Accounts) != 1 || d.Accounts[0].Name != "Synthetic name" || d.Total != "9223372036854775807" || !d.Complete {
		t.Fatal("name enrichment changed balances or failed", err)
	}
	if d.Servers[0].Snapshot.NameLookup.Directories[0] != profiles {
		t.Fatal("profiles folder not discovered")
	}
	if len(adminLogDirectories(ServerConfig{Path: filepath.Join(profiles, "Other", "ATM")})) != 0 {
		t.Fatal("searched arbitrary parents")
	}
	if adminLogDirectories(ServerConfig{Path: atm, LogPath: "explicit"})[0] != "explicit" {
		t.Fatal("explicit directory not used")
	}
	// Changing the lookup folder invalidates cached names just like changing accounts.
	c := m.Config()
	c.Servers[0].LogPath = filepath.Join(profiles, "missing")
	if err := m.Update(c); err != nil {
		t.Fatal(err)
	}
	if m.Views()[0].Snapshot != nil {
		t.Fatal("old names retained after log folder change")
	}
	refreshWait(t, m, "")
	d, err = m.Dashboard("")
	if err != nil || !d.Complete || d.Accounts[0].Name != "" || len(d.Servers[0].Snapshot.NameLookup.Issues) != 1 {
		t.Fatal("missing logs affected valid balances", err)
	}
}

func TestAdminLogBoundsCancellationAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	text := strings.Repeat("x", 1000) + "\n12:00:00 | Player \"Tail name\"(id=" + nameTestID + ")\n"
	file := put(t, dir, "tail.ADM", text)
	wanted := map[string]bool{nameTestID: true}
	names, used, err := readAdminFile(context.Background(), file, wanted, 200)
	if err != nil || used != 200 || names[nameTestID] != "Tail name" {
		t.Fatal("bounded tail search failed", names, used, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	names, _, err = readAdminFile(ctx, file, wanted, 200)
	if err != context.Canceled || len(names) != 0 {
		t.Fatal("cancellation ignored", err)
	}
	link := filepath.Join(t.TempDir(), "link.ADM")
	if os.Symlink(file, link) == nil {
		if _, _, err := readAdminFile(context.Background(), link, wanted, 200); err == nil {
			t.Fatal("followed an ADM symlink")
		}
	}
	for i := 0; i < maxAdminFiles+2; i++ {
		put(t, dir, strings.Repeat("a", i+1)+".ADM", "unrelated line\n")
	}
	lookup := &NameLookup{Issues: []FileIssue{}}
	readAdminNames(context.Background(), []string{dir}, map[string]bool{"unknown": true}, lookup)
	if lookup.Files != maxAdminFiles || len(lookup.Issues) == 0 {
		t.Fatal("file count limit not enforced", lookup)
	}
}

func TestAdminLogPathConfiguration(t *testing.T) {
	c := defaultConfig()
	c.Servers = []ServerConfig{{Name: "Expansion", Path: t.TempDir(), LogPath: "relative", Enabled: true}}
	path := filepath.Join(t.TempDir(), "config.json")
	if validateConfig(&c, path) == nil {
		t.Fatal("relative admin log directory accepted")
	}
	c.Servers[0].LogPath = t.TempDir()
	if err := saveConfig(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadConfig(path)
	if err != nil || loaded.Servers[0].LogPath != c.Servers[0].LogPath {
		t.Fatal("log folder did not persist", err)
	}
}

func TestCFLogStorageDiscoveryAndRecency(t *testing.T) {
	root := t.TempDir()
	profiles := filepath.Join(root, "profiles")
	atm := filepath.Join(profiles, "ExpansionMod", "ATM")
	storage := filepath.Join(root, "log_storage")
	for _, dir := range []string{atm, storage} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	old := put(t, profiles, "old.ADM", `12:00:00 | Player "Older name" (id=`+nameTestID+`)`)
	if err := os.Chtimes(old, time.Unix(100, 0), time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	put(t, storage, "new.ADM", `12:00:00 | Player "CF latest name" (id=`+nameTestID+`)`)
	snap := &Snapshot{Accounts: []Account{{PlayerID: nameTestID, Format: "expansion"}}}
	enrichAccountNames(context.Background(), snap, ServerConfig{Path: atm})
	if snap.Accounts[0].Name != "CF latest name" || len(snap.NameLookup.Directories) != 2 {
		t.Fatal("CF storage not found or newer log lost precedence")
	}
	// Profiles may also be the server root itself.
	dirs := adminLogDirectories(ServerConfig{Path: filepath.Join(root, "ExpansionMod", "ATM")})
	if len(dirs) != 2 || dirs[1] != storage {
		t.Fatal("root profiles layout did not discover log_storage")
	}
}

func TestAdminNamesReachSixtiethFile(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 61; i++ {
		text := "unrelated entry\n"
		if i == 59 {
			text = `12:00:00 | Player "Sixtieth file" (id=` + nameTestID + `)` + "\n"
		}
		if i == 60 {
			text = `12:00:00 | Player "Excluded" (id=excluded-player)` + "\n"
		}
		file := put(t, dir, fmt.Sprintf("session-%02d.ADM", i), text)
		stamp := time.Unix(int64(1000-i), 0)
		if err := os.Chtimes(file, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	lookup := &NameLookup{}
	names := readAdminNames(context.Background(), []string{dir}, map[string]bool{nameTestID: true, "excluded-player": true}, lookup)
	if maxAdminFiles != 60 || maxAdminTotalBytes != 240<<20 || lookup.Files != 60 || names[nameTestID] != "Sixtieth file" || names["excluded-player"] != "" {
		t.Fatal("60-file history limits failed", lookup.Files)
	}
}
