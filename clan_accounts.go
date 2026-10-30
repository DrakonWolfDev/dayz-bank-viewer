package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
)

func (s ServerConfig) accountType() string {
	if s.AccountType == "clan" {
		return "clan"
	}
	return "player"
}

// Clan accounts have their own reader: a player's clan membership is not a
// shared balance, even when that player record contains m_ClanID.
type AutoClanReader struct{}

func (AutoClanReader) Read(r io.Reader) (Account, error) {
	data, err := readAccountJSON(r)
	if err != nil {
		return Account{}, err
	}
	if len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] != '{' {
		return readExpansionPartyBinary(data)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return Account{}, errors.New("invalid clan JSON; expected a shared account object")
	}
	for _, key := range []string{"m_Steam64ID", "m_PlainID", "PlayerID"} {
		if _, ok := fields[key]; ok {
			return Account{}, errors.New("player account found in a clan source; choose the shared account folder")
		}
	}
	_, kr := fields["m_ClanID"]
	_, expansion := fields["PartyID"]
	if kr && expansion {
		return Account{}, errors.New("ambiguous clan format: multiple group ID fields")
	}
	if kr {
		var raw struct {
			ID      string  `json:"m_ClanID"`
			Name    *string `json:"m_ClanName"`
			Balance *int64  `json:"m_BankAmount"`
			Tag     string  `json:"m_Prefix"`
			Owner   string  `json:"m_OwnersPlainID"`
		}
		if err := json.Unmarshal(data, &raw); err != nil || !validGroupText(raw.ID, 128, false) || raw.Name == nil || !validGroupText(*raw.Name, 512, true) || raw.Balance == nil {
			return Account{}, errors.New("invalid KR clan account; expected m_ClanID, m_ClanName and integer m_BankAmount")
		}
		return Account{GroupID: raw.ID, Name: *raw.Name, Balance: Amount(*raw.Balance), Format: "kr_clan", Tag: raw.Tag, OwnerID: raw.Owner}, nil
	}
	if expansion {
		var raw struct {
			ID        *int64  `json:"PartyID"`
			Name      *string `json:"PartyName"`
			Balance   *int64  `json:"MoneyDeposited"`
			Tag       string  `json:"PartyTag"`
			Owner     string  `json:"OwnerUID"`
			OwnerName string  `json:"OwnerName"`
		}
		if err := json.Unmarshal(data, &raw); err != nil || raw.ID == nil || *raw.ID < 0 || *raw.ID > 2147483647 || raw.Name == nil || !validGroupText(*raw.Name, 512, true) || raw.Balance == nil {
			return Account{}, errors.New("invalid Expansion party JSON; expected integer PartyID, PartyName and integer MoneyDeposited")
		}
		return Account{GroupID: strconv.FormatInt(*raw.ID, 10), Name: *raw.Name, Balance: Amount(*raw.Balance), Format: "expansion_party", Tag: raw.Tag, OwnerID: raw.Owner, OwnerName: raw.OwnerName}, nil
	}
	return Account{}, errors.New("unrecognised clan format; select KR_BANKING/ClanDataBase or ExpansionMod/Groups")
}

func validGroupText(value string, limit int, blank bool) bool {
	return len(value) <= limit && (blank || strings.TrimSpace(value) != "") && strings.IndexFunc(value, unicode.IsControl) < 0
}
