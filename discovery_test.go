package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func discoveryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Match discovery's canonical root. Windows runner temp paths can contain
	// short (8.3) names that EvalSymlinks expands to their long spelling.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"SC/eXpAnSiOnMoD/AtM", "SC/eXpAnSiOnMoD/Groups", "SC/KR_BANKING/PlayerDataBase", "SC/KR_BANKING/ClanDataBase", "SC/DC_Banking/PlayerDatabase", "Log_Storage", "config"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	put(t, root, "serverDZ.cfg", `passwordAdmin="synthetic-secret"; template="ignored.enoch"; // template="ignored.sakhal";
 /* class Missions { class Wrong { template="wrong.chernarusplus"; }; }; */
 class Missions { class DayZ { template="dayzOffline.namalsk"; }; };`)
	put(t, filepath.Join(root, "SC/eXpAnSiOnMoD/AtM"), "player.json", expansionAccount)
	os.WriteFile(filepath.Join(root, "SC/eXpAnSiOnMoD/Groups/7.bin"), binaryPartyFixture(39), 0600)
	put(t, filepath.Join(root, "SC/KR_BANKING/PlayerDataBase"), "player.json", fixture("76561198000000001", "Private synthetic player", 42))
	put(t, filepath.Join(root, "SC/KR_BANKING/ClanDataBase"), "clan.json", krClanAccount)
	put(t, filepath.Join(root, "SC"), "session.ADM", "synthetic log text, not served")
	return root
}
func TestDiscoverServerMultiModAndLogs(t *testing.T) {
	root := discoveryFixture(t)
	d, err := discoverServer(context.Background(), root, "")
	if err != nil || len(d.Sources) != 5 || len(d.Missions) != 1 || d.Missions[0].Map != "Namalsk" {
		t.Fatal("discovery missing layouts or mission", err)
	}
	verified := 0
	for _, s := range d.Sources {
		if s.Verified {
			verified++
		}
		if s.Format == "expansion" && !reflect.DeepEqual(s.LogDirectories, []string{filepath.Join(root, "SC"), filepath.Join(root, "Log_Storage")}) {
			t.Fatal("SC/log storage association failed", s.LogDirectories)
		}
		if s.Format != "expansion" && len(s.LogDirectories) != 0 {
			t.Fatal("attached ADM logs to another mod")
		}
	}
	if verified != 4 {
		t.Fatal("empty source was verified", verified)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"synthetic-secret", "Private synthetic player", "9223372036854775807", "synthetic log text", "synthetic-owner"} {
		if strings.Contains(string(b), private) {
			t.Fatal("discovery exposed file contents")
		}
	}
	if len(d.Warnings) != 0 {
		t.Fatal("unexpected discovery warnings", d.Warnings)
	}
}
func TestMissionConfigParser(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{`class Missions { class DayZ { template = "dayzOffline.chernarusplus"; }; };`, []string{"dayzOffline.chernarusplus"}},
		{`passwordAdmin="class Missions { template=\"fake.enoch\"; }"; class Missions { class DayZ { /* comment */ template="custom.deerisle"; }; };`, []string{"custom.deerisle"}},
		{"\ufeffclass Missions { class First {template=\"dayzOffline.namalsk\";}; class Second: First {template=\"dayzOffline.sakhal\";};};", []string{"dayzOffline.namalsk", "dayzOffline.sakhal"}},
		{`// class Missions { template="bad.enoch"; }`, []string{}},
		{`template="outside.enoch"; class Other { template="outside.sakhal"; };`, []string{}},
	} {
		got, err := parseMissionTemplates(tc.input)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatal("wrong mission parse", got, err)
		}
	}
	for _, text := range []string{`class Missions {`, `}`, `class Missions { template="../other"; };`, `class Missions {template="oops;};`, `/* unfinished`, "#include \"outside.cfg\"", string([]byte{0xff})} {
		if _, err := parseMissionTemplates(text); err == nil {
			t.Fatal("accepted malformed or unsupported config")
		}
	}
	if missionMap("custom.unknownworld") != "unknownworld" {
		t.Fatal("invented unknown map name")
	}
}
func TestDiscoveryBoundariesAndCustomConfig(t *testing.T) {
	root := discoveryFixture(t)
	put(t, filepath.Join(root, "config"), "server.cfg", `class Missions { class DayZ { template="custom.enoch"; }; };`)
	d, err := discoverServer(context.Background(), root, "")
	if err != nil || len(d.Missions) != 2 || len(d.Warnings) == 0 {
		t.Fatal("ambiguous missions not surfaced")
	}
	chosen := filepath.Join(root, "custom.cfg")
	put(t, root, "custom.cfg", `class Missions { class DayZ { template="custom.sakhal"; }; };`)
	d, err = discoverServer(context.Background(), root, chosen)
	if err != nil || len(d.Missions) != 1 || d.Missions[0].Map != "Sakhal" {
		t.Fatal("custom configuration not respected")
	}
	outside := t.TempDir()
	os.MkdirAll(filepath.Join(outside, "PlayerDataBase"), 0700)
	os.Symlink(outside, filepath.Join(root, "Banking"))
	os.MkdirAll(filepath.Join(root, "backups/another-server/Banking/PlayerDataBase"), 0700)
	d, err = discoverServer(context.Background(), root, "")
	if err != nil || len(d.Sources) != 5 {
		t.Fatal("followed unrelated or symlinked folder")
	}
	if _, err := discoverServer(context.Background(), "relative", ""); err == nil {
		t.Fatal("accepted relative root")
	}
	if _, err := discoverServer(context.Background(), root, filepath.Join(outside, "server.cfg")); err == nil {
		t.Fatal("accepted config outside root")
	}
	link := filepath.Join(root, "linked.cfg")
	os.Symlink(chosen, link)
	if _, err := discoverServer(context.Background(), root, link); err == nil {
		t.Fatal("followed config symlink")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discoverServer(ctx, root, ""); err != context.Canceled && (err == nil || !strings.Contains(err.Error(), "canceled")) {
		t.Fatal("ignored cancellation", err)
	}
}
func TestDiscoveryEndpointIsPreviewOnly(t *testing.T) {
	root := discoveryFixture(t)
	atm := filepath.Join(root, "SC/eXpAnSiOnMoD/AtM")
	m := testManager(t, ServerConfig{ID: "existing", Name: "Existing", Path: atm, Enabled: true})
	h := handler(m, true)
	body, _ := json.Marshal(map[string]string{"root": root})
	r := request(h, http.MethodPost, "/api/discover", string(body), "", "localhost")
	if r.Code != 200 {
		t.Fatal("discovery failed", r.Body.String())
	}
	var d Discovery
	if err := json.Unmarshal(r.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range d.Sources {
		if s.Path == atm {
			found = s.AlreadyConfigured
		}
	}
	if !found || len(m.Config().Servers) != 1 || m.Views()[0].Snapshot != nil || m.Views()[0].Status != "not_scanned" {
		t.Fatal("discovery changed settings or scanned accounts")
	}
	if _, err := os.Stat(m.configPath); !os.IsNotExist(err) {
		t.Fatal("discovery wrote settings")
	}
	for _, body := range []string{`{"root":"relative"}`, `{"root":"/missing-discovery-folder"}`, `{"root":"/tmp","unexpected":true}`} {
		if r := request(h, http.MethodPost, "/api/discover", body, "", "localhost"); r.Code != 400 {
			t.Fatal("invalid request accepted")
		}
	}
	if r := request(h, http.MethodPost, "/api/discover", string(body), "https://external.invalid", "localhost"); r.Code != 403 {
		t.Fatal("cross-origin discovery accepted")
	}
}
func TestPrivateServerDiscoveryReadOnly(t *testing.T) {
	root := os.Getenv("DAYZ_BANK_TEST_SERVER_ROOT")
	if root == "" {
		t.Skip("set DAYZ_BANK_TEST_SERVER_ROOT to a local server backup")
	}
	paths := []string{filepath.Join(root, "serverDZ.cfg")}
	if _, err := os.Stat(filepath.Join(root, "Config.cfg")); err == nil {
		paths = append(paths, filepath.Join(root, "Config.cfg"))
	}
	hashes := map[string][32]byte{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal("private configuration unreadable")
		}
		hashes[path] = sha256.Sum256(b)
	}
	d, err := discoverServer(context.Background(), root, "")
	if err != nil || len(d.Missions) == 0 || len(d.Sources) == 0 {
		t.Fatal("private server discovery failed")
	}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil || sha256.Sum256(b) != hashes[path] {
			t.Fatal("server configuration changed")
		}
	}
	player, clan := 0, 0
	for _, s := range d.Sources {
		if s.Verified {
			if s.AccountType == "clan" {
				clan++
			} else {
				player++
			}
		}
	}
	if player == 0 || clan == 0 {
		t.Fatal("private backup did not expose both account views")
	}
	for _, mission := range d.Missions {
		t.Logf("Map inferred: %s (%s)", mission.Map, filepath.Base(mission.ConfigPath))
	}
	t.Logf("Detected %d mission settings, %d verified player folders, %d verified clan folders and %d log locations; configuration hash unchanged", len(d.Missions), player, clan, len(d.LogDirectories))
}

func TestDiscoveryFindsCustomConfigurationNames(t *testing.T) {
	root := discoveryFixture(t)
	for path, template := range map[string]string{"Config.cfg": "custom.chernarusplus", "SC/alternate.CONF": "custom.enoch", "second.config": "custom.sakhal"} {
		put(t, filepath.Dir(filepath.Join(root, path)), filepath.Base(path), `class Missions { class DayZ { template="`+template+`"; }; };`)
	}
	put(t, root, "gamelabs.cfg", `password="not-a-server-mission";`)
	d, err := discoverServer(context.Background(), root, "")
	if err != nil || len(d.Missions) != 4 {
		t.Fatal("custom configurations not discovered", len(d.Missions), err)
	}
	maps := map[string]bool{}
	for _, mission := range d.Missions {
		maps[mission.Map] = true
		if filepath.Base(mission.ConfigPath) == "gamelabs.cfg" {
			t.Fatal("non-mission configuration offered as a mission")
		}
	}
	for _, name := range []string{"Namalsk", "Chernarus", "Livonia", "Sakhal"} {
		if !maps[name] {
			t.Fatal("missing mission choice", name)
		}
	}
}

func TestDiscoveryConfigurationReadLimit(t *testing.T) {
	root := discoveryFixture(t)
	for i := 0; i < maxDiscoveryConfigFiles; i++ {
		put(t, root, fmt.Sprintf("alternate-%03d.cfg", i), `class Missions { class DayZ { template="custom.enoch"; }; };`)
	}
	d, err := discoverServer(context.Background(), root, "")
	if err != nil || len(d.Missions) != maxDiscoveryConfigFiles {
		t.Fatal("configuration limit not applied", err)
	}
	foundDefault, limited := false, false
	for _, m := range d.Missions {
		if filepath.Base(m.ConfigPath) == "serverDZ.cfg" {
			foundDefault = true
		}
	}
	for _, warning := range d.Warnings {
		if strings.Contains(warning, "limited to 64 files") {
			limited = true
		}
	}
	if !foundDefault || !limited {
		t.Fatal("default configuration not prioritised or limit not disclosed")
	}
	custom := filepath.Join(root, fmt.Sprintf("alternate-%03d.cfg", maxDiscoveryConfigFiles-1))
	d, err = discoverServer(context.Background(), root, custom)
	if err != nil || len(d.Missions) != 1 || d.Missions[0].ConfigPath != custom {
		t.Fatal("override could not select a file beyond search limit")
	}
}
