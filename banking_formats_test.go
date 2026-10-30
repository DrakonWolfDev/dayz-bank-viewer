package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const dcAccount = `{"m_PlainID":"76561198000000002","m_Username":"DC player","m_OwnedCurrency":9007199254740993,"m_MaxOwnedCurrencyBonus":4000}`
const expansionAccount = `{"PlayerID":"synthetic-DayZ-hashed-identity=","MoneyDeposited":9223372036854775807}`

func TestAutomaticBankingFormats(t *testing.T) {
	for _, tc := range []struct {
		name, data, format, id string
		balance                Amount
	}{
		{"existing and KR", fixture("76561198000000001", "Banking player", 123), "banking", "steam:76561198000000001", 123},
		{"DC Banking", "\ufeff" + dcAccount, "dc_banking", "steam:76561198000000002", 9007199254740993},
		{"Expansion", expansionAccount, "expansion", "dayz:synthetic-DayZ-hashed-identity=", 9223372036854775807},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := (AutoBankingReader{}).Read(strings.NewReader(tc.data))
			if err != nil {
				t.Fatal(err)
			}
			if a.Format != tc.format || a.identityKey() != tc.id || a.Balance != tc.balance {
				t.Fatalf("unexpected account: %+v", a)
			}
			if tc.format == "dc_banking" && (a.CapacityBonus == nil || *a.CapacityBonus != 4000 || a.Bonus != nil) {
				t.Fatal("capacity bonus confused with spendable currency")
			}
			if tc.format == "expansion" && (a.SteamID != "" || a.Name != "") {
				t.Fatal("invented a Steam64 ID or name")
			}
			data, err := json.Marshal(a)
			if err != nil || !strings.Contains(string(data), `"balance":"`) {
				t.Fatal("amount is not a decimal string", err)
			}
		})
	}
}

func TestAutomaticReaderRejectsInvalidOrAmbiguousData(t *testing.T) {
	for _, data := range []string{
		`null`, `[]`, `{}`, `{"ATMSystemEnabled":1}`, dcAccount + `{}`,
		strings.Repeat(" ", 1<<20) + dcAccount,
		strings.Replace(dcAccount, `"76561198000000002"`, `76561198000000002`, 1),
		strings.Replace(dcAccount, `"76561198000000002"`, `"invalid"`, 1),
		strings.Replace(dcAccount, `"DC player"`, `null`, 1),
		strings.Replace(dcAccount, `9007199254740993`, `1.5`, 1),
		strings.Replace(dcAccount, `4000`, `"4000"`, 1),
		`{"PlayerID":"id"}`, `{"PlayerID":null,"MoneyDeposited":0}`,
		`{"PlayerID":123,"MoneyDeposited":0}`, `{"PlayerID":"","MoneyDeposited":0}`,
		`{"PlayerID":" id ","MoneyDeposited":0}`, `{"PlayerID":"id\n","MoneyDeposited":0}`,
		`{"PlayerID":"id","MoneyDeposited":null}`, `{"PlayerID":"id","MoneyDeposited":"100"}`,
		`{"PlayerID":"id","MoneyDeposited":1.5}`, `{"PlayerID":"id","MoneyDeposited":9223372036854775808}`,
		`{"PlayerID":"id","MoneyDeposited":0,"m_PlainID":"76561198000000001"}`,
		`{"m_Steam64ID":"76561198000000001","m_PlayerName":"A","m_OwnedCurrency":1,"m_PlainID":null}`,
	} {
		if _, err := (AutoBankingReader{}).Read(strings.NewReader(data)); err == nil {
			t.Fatalf("accepted invalid input %.150s", data)
		}
	}
	for _, data := range []string{`{"PlayerID":"id","MoneyDeposited":-10}`, `{"m_PlainID":"76561198000000001","m_Username":"","m_OwnedCurrency":-10}`} {
		a, err := (AutoBankingReader{}).Read(strings.NewReader(data))
		if err != nil || a.Balance != -10 || a.CapacityBonus != nil {
			t.Fatal("signed balance or optional field mishandled", a, err)
		}
	}
}

func TestMixedFormatsAndIdentityNamespaces(t *testing.T) {
	dir := t.TempDir()
	inputs := map[string]string{
		"a.json":        fixture("76561198000000001", "Existing", 10),
		"b.json":        dcAccount,
		"c.json":        expansionAccount,
		"d.json":        strings.Replace(dcAccount, "76561198000000002", "76561198000000001", 1),
		"e.json":        strings.Replace(expansionAccount, "synthetic-DayZ-hashed-identity=", "76561198000000001", 1),
		"settings.json": `{"ATMSystemEnabled":1}`,
	}
	paths := map[string]string{}
	for name, data := range inputs {
		paths[name] = put(t, dir, name, data)
	}
	snap, err := scanDirectory(context.Background(), dir, AutoBankingReader{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Accounts) != 4 || len(snap.Issues) != 2 || strings.Join(snap.Formats, ",") != "banking,dc_banking,expansion" {
		t.Fatalf("unexpected scan: %+v", snap)
	}
	m := testManager(t, ServerConfig{ID: "a", Name: "Mixed", Path: dir, Enabled: true})
	refreshWait(t, m, "")
	d, err := m.Dashboard("")
	if err != nil || d.AccountCount != 4 || d.UniquePlayers != 4 || d.Total != "18455751272964292617" {
		t.Fatalf("incorrect exact summary: %+v, %v", d, err)
	}
	for name, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != inputs[name] {
			t.Fatal("source changed", name, err)
		}
	}
}
