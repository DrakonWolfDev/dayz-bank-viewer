package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Amount is encoded as a decimal string so JavaScript never rounds an integer.
type Amount int64

func (a Amount) MarshalJSON() ([]byte, error) { return json.Marshal(strconv.FormatInt(int64(a), 10)) }

type Account struct {
	SteamID       string  `json:"steam_id,omitempty"`
	PlayerID      string  `json:"player_id,omitempty"`
	Format        string  `json:"format"`
	Name          string  `json:"name"`
	NameSource    string  `json:"name_source,omitempty"`
	Balance       Amount  `json:"balance"`
	Bonus         *Amount `json:"bonus,omitempty"`
	Paycheck      *Amount `json:"paycheck,omitempty"`
	ClanID        string  `json:"clan_id,omitempty"`
	CapacityBonus *Amount `json:"capacity_bonus,omitempty"`
}

// AccountReader is the boundary between a mod's storage and the dashboard.
type AccountReader interface {
	Read(io.Reader) (Account, error)
}
type BankingReader struct{}

func (BankingReader) Read(r io.Reader) (Account, error) {
	data, err := readAccountJSON(r)
	if err != nil {
		return Account{}, err
	}
	return readBankingJSON(data)
}

func readAccountJSON(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("account exceeds the 1 MiB limit")
	}
	return bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), nil
}

func readBankingJSON(data []byte) (Account, error) {
	var raw struct {
		ID       string  `json:"m_Steam64ID"`
		Name     *string `json:"m_PlayerName"`
		Balance  *int64  `json:"m_OwnedCurrency"`
		Bonus    *int64  `json:"m_BonusCurrency"`
		Paycheck *int64  `json:"m_PayCheckBonus"`
		ClanID   string  `json:"m_ClanID"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Account{}, errors.New("invalid banking JSON or field types")
	}
	if !validSteamID(raw.ID) {
		return Account{}, errors.New("m_Steam64ID must be a 17-digit string")
	}
	if raw.Name == nil || raw.Balance == nil {
		return Account{}, errors.New("missing player name or m_OwnedCurrency")
	}
	a := Account{SteamID: raw.ID, Format: "banking", Name: *raw.Name, Balance: Amount(*raw.Balance), ClanID: raw.ClanID}
	if raw.Bonus != nil {
		v := Amount(*raw.Bonus)
		a.Bonus = &v
	}
	if raw.Paycheck != nil {
		v := Amount(*raw.Paycheck)
		a.Paycheck = &v
	}
	return a, nil
}

type FileIssue struct {
	File    string `json:"file"`
	Message string `json:"message"`
}
type Snapshot struct {
	Accounts   []Account   `json:"accounts"`
	ScannedAt  time.Time   `json:"scanned_at"`
	Files      int         `json:"files"`
	Issues     []FileIssue `json:"issues"`
	Formats    []string    `json:"formats"`
	NameLookup *NameLookup `json:"name_lookup,omitempty"`
}

func readWithRetry(ctx context.Context, path string, reader AccountReader) (Account, error) {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return Account{}, err
		}
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Account{}, ctx.Err()
			case <-time.After(time.Duration(attempt) * 40 * time.Millisecond):
			}
		}
		info, err := os.Lstat(path)
		if err == nil && !info.Mode().IsRegular() {
			return Account{}, errors.New("not a regular file (links are not followed)")
		}
		if err == nil {
			var f *os.File
			f, err = os.Open(path) // Read-only: source files are never opened for writing.
			if err == nil {
				a, parseErr := reader.Read(f)
				closeErr := f.Close()
				if parseErr == nil && closeErr == nil {
					return a, nil
				}
				err = errors.Join(parseErr, closeErr)
			}
		}
		last = err
	}
	return Account{}, last
}

func scanDirectory(ctx context.Context, path string, reader AccountReader) (*Snapshot, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read banking directory: %w", err)
	}
	snap := &Snapshot{Accounts: []Account{}, Issues: []FileIssue{}, Formats: []string{}}
	seen := map[string]bool{}
	formats := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		snap.Files++
		a, err := readWithRetry(ctx, filepath.Join(path, e.Name()), reader)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil && seen[a.identityKey()] {
			err = errors.New("duplicate player ID; first valid file retained")
		}
		if err != nil {
			snap.Issues = append(snap.Issues, FileIssue{e.Name(), err.Error()})
			continue
		}
		seen[a.identityKey()] = true
		formats[a.Format] = true
		snap.Accounts = append(snap.Accounts, a)
	}
	for format := range formats {
		snap.Formats = append(snap.Formats, format)
	}
	sort.Strings(snap.Formats)
	if snap.Files > 0 && len(snap.Accounts) == 0 {
		return snap, fmt.Errorf("none of %d JSON files could be read as accounts; previous results retained", snap.Files)
	}
	sort.Slice(snap.Accounts, func(i, j int) bool {
		a, b := snap.Accounts[i], snap.Accounts[j]
		if a.Balance != b.Balance {
			return a.Balance > b.Balance
		}
		return a.identityKey() < b.identityKey()
	})
	snap.ScannedAt = time.Now()
	return snap, nil
}
