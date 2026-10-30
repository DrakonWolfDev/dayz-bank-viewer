package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const krClanAccount = `{"m_ClanID":"synthetic-clan","m_ClanName":"Synthetic clan","m_BankAmount":9007199254740993,"m_Prefix":"SYN","m_OwnersPlainID":"76561198000000001","m_Members":[],"m_ClanLogs":["private log ignored"]}`
const partyJSONAccount = `{"PartyID":0,"PartyName":"Synthetic party","MoneyDeposited":123,"OwnerUID":"synthetic-owner","OwnerName":"Synthetic owner","PartyTag":"SYN"}`

// Synthetic FileSerializer stream using Expansion's documented field order.
func binaryPartyFixture(version int32) []byte {
	var b bytes.Buffer
	integer := func(n int32) {
		if err := binary.Write(&b, binary.LittleEndian, n); err != nil {
			panic(err)
		}
	}
	text := func(s string) { integer(int32(len(s))); b.WriteString(s) }
	marker := func() {
		integer(1)
		text("Synthetic marker")
		text("icon")
		integer(-1)
		integer(3)
		integer(0)
		integer(0)
		integer(0)
		integer(0)
	}
	integer(version)
	integer(7)
	text("Synthetic party")
	text("synthetic-owner")
	text("Synthetic owner")
	integer(1)
	text("synthetic-member")
	text("Member")
	integer(31)
	marker()
	integer(1)
	integer(7)
	text("Synthetic party")
	text("synthetic-invite")
	integer(1)
	text("synthetic-marker")
	marker()
	integer(456)
	if version >= 40 {
		text("SYN")
	}
	if version >= 42 {
		integer(123456)
	}
	return b.Bytes()
}

func TestSharedAccountAdapters(t *testing.T) {
	for _, tc := range []struct {
		data                []byte
		format, id, balance string
	}{
		{[]byte(krClanAccount), "kr_clan", "synthetic-clan", "9007199254740993"},
		{[]byte(partyJSONAccount), "expansion_party", "0", "123"},
		{binaryPartyFixture(39), "expansion_party", "7", "456"},
		{binaryPartyFixture(40), "expansion_party", "7", "456"},
		{binaryPartyFixture(41), "expansion_party", "7", "456"},
		{binaryPartyFixture(42), "expansion_party", "7", "456"},
	} {
		a, err := (AutoClanReader{}).Read(bytes.NewReader(tc.data))
		if err != nil || a.Format != tc.format || a.GroupID != tc.id || fmt.Sprint(a.Balance) != tc.balance || a.SteamID != "" || a.PlayerID != "" {
			t.Fatal("incorrect shared account", a, err)
		}
		raw, err := json.Marshal(a)
		if err != nil || bytes.Contains(raw, []byte("private log")) || bytes.Contains(raw, []byte("synthetic-member")) {
			t.Fatal("private member/log data exposed")
		}
	}
	for _, data := range []string{fixture("76561198000000001", "Personal", 1), dcAccount, expansionAccount, `{}`, `null`, `{"m_ClanID":"x","m_ClanName":"X","m_BankAmount":1.5}`, `{"m_ClanID":"x","m_ClanName":"X","m_BankAmount":null}`, `{"PartyID":-1,"PartyName":"X","MoneyDeposited":1}`, `{"PartyID":"1","PartyName":"X","MoneyDeposited":1}`, strings.Replace(partyJSONAccount, `123`, `9223372036854775808`, 1), strings.Replace(krClanAccount, `"m_ClanName"`, `"PartyID":1,"m_ClanName"`, 1)} {
		if _, err := (AutoClanReader{}).Read(strings.NewReader(data)); err == nil {
			t.Fatal("accepted invalid shared account")
		}
	}
	if _, err := (AutoBankingReader{}).Read(strings.NewReader(krClanAccount)); err == nil {
		t.Fatal("clan account accepted as player")
	}
}

func TestPartyBinaryRejectsCorruption(t *testing.T) {
	data := binaryPartyFixture(42)
	for end := 0; end < len(data); end++ {
		if _, err := readExpansionPartyBinary(data[:end]); err == nil {
			t.Fatalf("accepted truncated binary at %d", end)
		}
	}
	for _, version := range []int32{0, 26, 38, 43, 2147483647} {
		b := append([]byte{}, data...)
		binary.LittleEndian.PutUint32(b, uint32(version))
		if _, err := readExpansionPartyBinary(b); err == nil {
			t.Fatal("accepted unsupported version", version)
		}
	}
	for _, offset := range []int{4, 8} {
		b := append([]byte{}, data...)
		binary.LittleEndian.PutUint32(b[offset:], 0xffffffff)
		if _, err := readExpansionPartyBinary(b); err == nil {
			t.Fatal("accepted negative ID or string length")
		}
	}
	if _, err := readExpansionPartyBinary(append(data, 0)); err == nil {
		t.Fatal("accepted trailing data")
	}
	for _, n := range []int32{-1, 10001} {
		r := &partyBinaryReader{data: []byte{0, 0, 0, 0}}
		binary.LittleEndian.PutUint32(r.data, uint32(n))
		r.count()
		if r.err == nil {
			t.Fatal("unbounded array count")
		}
	}
	r := &partyBinaryReader{data: []byte{2, 0, 0, 0}}
	r.boolean()
	if r.err == nil {
		t.Fatal("invalid boolean")
	}
}

func TestClanViewSeparationAndRecovery(t *testing.T) {
	players, clans := t.TempDir(), t.TempDir()
	put(t, players, "player.json", fixture("76561198000000001", "Personal", 10))
	clanFile := put(t, clans, "clan.json", krClanAccount)
	if err := os.WriteFile(filepath.Join(clans, "7.bin"), binaryPartyFixture(39), 0600); err != nil {
		t.Fatal(err)
	}
	m := testManager(t, ServerConfig{ID: "players", Name: "Players", Path: players, Enabled: true}, ServerConfig{ID: "clans", Name: "Clans", Path: clans, Enabled: true, AccountType: "clan"})
	refreshWait(t, m, "")
	p, _ := m.Dashboard("")
	c, err := m.DashboardForType("", "clan")
	if err != nil || p.Total != "10" || p.AccountCount != 1 || c.Total != "9007199254741449" || c.AccountCount != 2 || !c.Complete {
		t.Fatal("mixed player/clan totals", p.Total, c.Total, err)
	}
	if c.Accounts[0].Rank != 1 || c.Accounts[0].GroupID != "synthetic-clan" || c.Accounts[1].Rank != 2 {
		t.Fatal("clan ranks incorrect")
	}
	if m.Views()[1].Snapshot.NameLookup != nil {
		t.Fatal("clan scan read ADM files")
	}
	h := handler(m, true)
	res := request(h, http.MethodGet, "/api/dashboard?account_type=clan", "", "", "localhost")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"group_id"`) || strings.Contains(res.Body.String(), `"steam_id"`) {
		t.Fatal("clan API contains player data")
	}
	if res := request(h, http.MethodGet, "/api/dashboard?account_type=other", "", "", "localhost"); res.Code != 400 {
		t.Fatal("invalid view accepted")
	}
	if _, err := m.DashboardForType("players", "clan"); err == nil {
		t.Fatal("wrong source type accepted")
	}
	put(t, clans, "clan.json", "{broken")
	os.Remove(filepath.Join(clans, "7.bin"))
	refreshWait(t, m, "clans")
	c, _ = m.DashboardForType("", "clan")
	p, _ = m.Dashboard("")
	if c.Complete || c.AccountCount != 2 || !p.Complete || p.Total != "10" {
		t.Fatal("clan failure affected player data or discarded cache")
	}
	if err := os.WriteFile(clanFile, []byte(krClanAccount), 0600); err != nil {
		t.Fatal(err)
	}
	refreshWait(t, m, "clans")
	c, _ = m.DashboardForType("", "clan")
	if !c.Complete || c.AccountCount != 1 {
		t.Fatal("clan recovery failed")
	}
	cfg := m.Config()
	cfg.Servers[1].AccountType = "player"
	if err := m.Update(cfg); err != nil {
		t.Fatal(err)
	}
	if m.Views()[1].Snapshot != nil {
		t.Fatal("changing account type retained cache")
	}
}

func TestBinaryPartyTakesPrecedenceOverLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "7.json", partyJSONAccount)
	os.WriteFile(filepath.Join(dir, "7.bin"), binaryPartyFixture(39), 0600)
	snap, err := scanDirectory(context.Background(), dir, AutoClanReader{})
	if err != nil || len(snap.Accounts) != 1 || snap.Accounts[0].Balance != 456 || len(snap.Issues) != 1 {
		t.Fatal("legacy balance duplicated or stale", err)
	}
}

func TestPrivatePartyCorpusReadOnly(t *testing.T) {
	dir := os.Getenv("DAYZ_BANK_TEST_GROUP_CORPUS")
	if dir == "" {
		t.Skip("set DAYZ_BANK_TEST_GROUP_CORPUS to a version 39 Expansion Groups backup")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal("groups backup unavailable")
	}
	hashes := map[string][32]byte{}
	total := new(big.Int)
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(b) < 8 || binary.LittleEndian.Uint32(b) != 39 {
			t.Fatal("corpus requires version 39 files")
		}
		hashes[e.Name()] = sha256.Sum256(b)
		total.Add(total, big.NewInt(int64(int32(binary.LittleEndian.Uint32(b[len(b)-4:])))))
	}
	if len(hashes) == 0 {
		t.Fatal("empty group corpus")
	}
	snap, err := scanDirectory(context.Background(), dir, AutoClanReader{})
	if err != nil || len(snap.Issues) > 0 || len(snap.Accounts) != len(hashes) {
		t.Fatal("private groups scan failed")
	}
	m := testManager(t, ServerConfig{ID: "groups", Name: "Private groups", Path: dir, Enabled: true, AccountType: "clan"})
	refreshWait(t, m, "")
	d, err := m.DashboardForType("", "clan")
	if err != nil || !d.Complete || d.Total != total.String() {
		t.Fatal("groups total differs from independent version 39 calculation")
	}
	for name, hash := range hashes {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || sha256.Sum256(b) != hash {
			t.Fatal("group source changed")
		}
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(entries) {
		t.Fatal("group file list changed")
	}
	t.Logf("Read %d groups; independent total matched; source hashes unchanged", len(hashes))
}
