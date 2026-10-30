package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
)

// These adapters describe storage schemas, not proof of an installed mod.
// In particular, Banking and KR Banking use the same player-account fields.
type accountFormat struct {
	idField string
	parse   func([]byte) (Account, error)
}

var accountFormats = []accountFormat{
	{"m_Steam64ID", readBankingJSON},
	{"m_PlainID", readDCBankingJSON},
	{"PlayerID", readExpansionJSON},
}

type AutoBankingReader struct{}

func (AutoBankingReader) Read(r io.Reader) (Account, error) {
	data, err := readAccountJSON(r)
	if err != nil {
		return Account{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return Account{}, errors.New("invalid banking JSON; expected an account object")
	}
	var selected *accountFormat
	for i := range accountFormats {
		format := &accountFormats[i]
		if _, ok := fields[format.idField]; !ok {
			continue
		}
		if selected != nil {
			return Account{}, errors.New("ambiguous banking format: multiple account ID fields")
		}
		selected = format
	}
	if selected == nil {
		return Account{}, errors.New("unrecognised banking format; select a Banking/KR player database, DC_Banking/PlayerDatabase or ExpansionMod/ATM folder")
	}
	return selected.parse(data)
}

func readDCBankingJSON(data []byte) (Account, error) {
	var raw struct {
		ID            string  `json:"m_PlainID"`
		Name          *string `json:"m_Username"`
		Balance       *int64  `json:"m_OwnedCurrency"`
		CapacityBonus *int64  `json:"m_MaxOwnedCurrencyBonus"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Account{}, errors.New("invalid DayZ-Banking JSON or field types")
	}
	if !validSteamID(raw.ID) {
		return Account{}, errors.New("m_PlainID must be a 17-digit string")
	}
	if raw.Name == nil || raw.Balance == nil {
		return Account{}, errors.New("missing m_Username or m_OwnedCurrency")
	}
	a := Account{SteamID: raw.ID, Name: *raw.Name, Balance: Amount(*raw.Balance), Format: "dc_banking"}
	if raw.CapacityBonus != nil {
		v := Amount(*raw.CapacityBonus)
		a.CapacityBonus = &v
	}
	return a, nil
}

func readExpansionJSON(data []byte) (Account, error) {
	var raw struct {
		ID      string `json:"PlayerID"`
		Balance *int64 `json:"MoneyDeposited"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Account{}, errors.New("invalid Expansion ATM JSON or field types")
	}
	if raw.ID == "" || len(raw.ID) > 128 || strings.TrimSpace(raw.ID) != raw.ID || strings.IndexFunc(raw.ID, unicode.IsControl) >= 0 {
		return Account{}, errors.New("PlayerID must be a non-empty player ID string of at most 128 bytes without surrounding whitespace or control characters")
	}
	if raw.Balance == nil {
		return Account{}, errors.New("missing MoneyDeposited")
	}
	// Expansion stores PlayerIdentity.GetId(), not GetPlainId() (Steam64).
	// This file supplies neither a Steam64 ID nor a display name.
	return Account{PlayerID: raw.ID, Balance: Amount(*raw.Balance), Format: "expansion"}, nil
}

func validSteamID(id string) bool {
	return len(id) == 17 && strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// Identity namespaces prevent a DayZ ID from being merged with a Steam64 ID.
func (a Account) identityKey() string {
	if a.GroupID != "" {
		return a.Format + ":" + a.GroupID
	}
	if a.PlayerID != "" {
		return "dayz:" + a.PlayerID
	}
	return "steam:" + a.SteamID
}
