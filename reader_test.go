package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(id, name string, balance int64) string {
	b, _ := json.Marshal(map[string]any{"m_Steam64ID": id, "m_PlayerName": name, "m_OwnedCurrency": balance, "m_BonusCurrency": int64(900), "m_PayCheckBonus": int64(600), "m_ClanID": "test-clan"})
	return string(b)
}
func put(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestBankingReaderPrecisionAndValidation(t *testing.T) {
	reader := BankingReader{}
	a, err := reader.Read(strings.NewReader("\ufeff" + fixture("76561198000000001", "Synthetic <Player>", 9223372036854775807)))
	if err != nil {
		t.Fatal(err)
	}
	if a.Balance != 9223372036854775807 || a.SteamID != "76561198000000001" || *a.Bonus != 900 || *a.Paycheck != 600 {
		t.Fatalf("unexpected account: %+v", a)
	}
	data, _ := json.Marshal(a)
	if !strings.Contains(string(data), `"balance":"9223372036854775807"`) {
		t.Fatal("balance lost precision", string(data))
	}
	for _, s := range []string{`null`, `[]`, `{}`, `{"m_Steam64ID":76561198000000001,"m_PlayerName":"A","m_OwnedCurrency":1}`, `{"m_Steam64ID":"invalid","m_PlayerName":"A","m_OwnedCurrency":1}`, `{"m_Steam64ID":"76561198000000001","m_PlayerName":"A","m_OwnedCurrency":1.5}`, `{"m_Steam64ID":"76561198000000001","m_PlayerName":"A","m_OwnedCurrency":9223372036854775808}`, fixture("76561198000000001", "A", 1) + `{}`, strings.Repeat(" ", 1<<20) + `{}`} {
		if _, err := reader.Read(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted invalid input %.100s", s)
		}
	}
	a, err = reader.Read(strings.NewReader(`{"m_Steam64ID":"76561198000000001","m_PlayerName":"","m_OwnedCurrency":-10}`))
	if err != nil || a.Bonus != nil || a.Balance != -10 {
		t.Fatal("optional fields or signed integers mishandled", a, err)
	}
}
func TestDirectoryScanReportsSkippedFiles(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "a.json", fixture("76561198000000001", "One", 500))
	put(t, dir, "b.JSON", fixture("76561198000000002", "Two", 1000))
	put(t, dir, "c.json", fixture("76561198000000001", "Duplicate", 999999))
	put(t, dir, "broken.json", "{broken")
	put(t, dir, "notes.txt", "ignored")
	snap, err := scanDirectory(context.Background(), dir, BankingReader{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Files != 4 || len(snap.Accounts) != 2 || len(snap.Issues) != 2 {
		t.Fatalf("scan counts: %+v", snap)
	}
	if snap.Accounts[0].Name != "Two" || snap.Accounts[1].Balance != 500 {
		t.Fatal("incorrect ranking or duplicate handling")
	}
	if _, err := scanDirectory(context.Background(), filepath.Join(dir, "missing"), BankingReader{}); err == nil {
		t.Fatal("missing directory accepted")
	}
	empty, err := scanDirectory(context.Background(), t.TempDir(), BankingReader{})
	if err != nil || len(empty.Accounts) != 0 {
		t.Fatal("empty directory must be a valid empty snapshot")
	}
	broken := t.TempDir()
	put(t, broken, "bad.json", "{}")
	failed, err := scanDirectory(context.Background(), broken, BankingReader{})
	if err == nil || failed == nil || len(failed.Issues) != 1 {
		t.Fatal("total failure must retain skipped-file diagnostics")
	}
}

type transientReader struct{ attempts int }

func (r *transientReader) Read(input io.Reader) (Account, error) {
	r.attempts++
	if r.attempts < 3 {
		return Account{}, errors.New("temporary partial JSON")
	}
	return (BankingReader{}).Read(input)
}
func TestReadRetriesAndCancellation(t *testing.T) {
	path := put(t, t.TempDir(), "account.json", fixture("76561198000000001", "One", 123))
	r := &transientReader{}
	a, err := readWithRetry(context.Background(), path, r)
	if err != nil || a.Balance != 123 || r.attempts != 3 {
		t.Fatal("transient read did not recover", a, err, r.attempts)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readWithRetry(ctx, path, r); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored", err)
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(path, link); err == nil {
		if _, err := readWithRetry(context.Background(), link, BankingReader{}); err == nil {
			t.Fatal("followed an account symlink")
		}
	}
}
func ExampleAmount() {
	data, _ := json.Marshal(Amount(9007199254740993))
	fmt.Println(string(data)) // Output: "9007199254740993"
}
