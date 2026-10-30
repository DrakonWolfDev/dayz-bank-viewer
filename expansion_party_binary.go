package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"unicode/utf8"
)

// Expansion's FileSerializer writes little-endian int32 values (including
// booleans), length-prefixed UTF-8 strings and a length-prefixed 3-float vector.
// Follow OnStoreLoad in ExpansionPartyData, ExpansionPartyPlayerData,
// ExpansionPartyInviteData and ExpansionMarkerData. Never infer money from a
// tail offset: every preceding field must parse, and the file must end exactly.
// Versions before 39 depend on whether Market was enabled when saved.
func readExpansionPartyBinary(data []byte) (Account, error) {
	r := &partyBinaryReader{data: data}
	version := r.integer()
	if version < 39 || version > 42 {
		return Account{}, fmt.Errorf("unsupported Expansion party binary version %d; supported versions are 39–42", version)
	}
	id := r.integer()
	if id < 0 {
		return Account{}, fmt.Errorf("invalid Expansion party ID")
	}
	name, owner, ownerName := r.text(512), r.text(128), r.text(512)
	for count := r.count(); count > 0 && r.err == nil; count-- {
		r.text(128) // Player UID
		r.text(512) // Player name
		r.integer() // Permissions
		r.marker()
	}
	for count := r.count(); count > 0 && r.err == nil; count-- {
		r.integer() // Invited party ID
		r.text(512) // Party name
		r.text(128) // Invited player UID
	}
	for count := r.count(); count > 0 && r.err == nil; count-- {
		r.text(512) // Marker UID
		r.marker()
	}
	money := r.integer()
	tag := ""
	if version >= 40 {
		tag = r.text(512)
	}
	if version >= 42 {
		r.integer() // Last member leave timestamp
	}
	if r.err != nil {
		return Account{}, fmt.Errorf("invalid or truncated Expansion party binary: %w", r.err)
	}
	if len(r.data) != 0 {
		return Account{}, fmt.Errorf("unexpected trailing data in Expansion party binary")
	}
	return Account{GroupID: strconv.FormatInt(int64(id), 10), Name: name, OwnerID: owner, OwnerName: ownerName, Tag: tag, Balance: Amount(money), Format: "expansion_party"}, nil
}

type partyBinaryReader struct {
	data []byte
	err  error
}

func (r *partyBinaryReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	b := r.data[:n]
	r.data = r.data[n:]
	return b
}

func (r *partyBinaryReader) integer() int32 {
	b := r.take(4)
	if b == nil {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(b))
}

func (r *partyBinaryReader) text(limit int32) string {
	n := r.integer()
	if n < 0 || n > limit {
		r.err = fmt.Errorf("string length outside 0–%d bytes", limit)
		return ""
	}
	b := r.take(int(n))
	if !utf8.Valid(b) || !validGroupText(string(b), int(limit), true) {
		r.err = fmt.Errorf("invalid text")
		return ""
	}
	return string(b)
}

func (r *partyBinaryReader) count() int32 {
	n := r.integer()
	if n < 0 || n > 10000 {
		r.err = fmt.Errorf("entry count outside 0–10000")
		return 0
	}
	return n
}

func (r *partyBinaryReader) boolean() {
	if n := r.integer(); n != 0 && n != 1 {
		r.err = fmt.Errorf("invalid boolean")
	}
}

func (r *partyBinaryReader) marker() {
	r.boolean()
	r.text(4096) // Marker text
	r.text(4096) // Icon path
	r.integer()  // Colour
	if r.integer() != 3 {
		r.err = fmt.Errorf("marker position must contain three coordinates")
		return
	}
	for i := 0; i < 3; i++ {
		f := math.Float32frombits(uint32(r.integer()))
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			r.err = fmt.Errorf("invalid marker coordinate")
		}
	}
	r.boolean() // Locked, present since version 38.
}
