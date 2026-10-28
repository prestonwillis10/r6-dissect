package dissect

import (
	"encoding/binary"
	"slices"
	"testing"
)

// killHeaders are the bytes readMatchFeedback reads between the match feedback listener pattern
// (59 34 E5 8B 04) and the size byte, for each of its three code version ranges.
var killHeaders = []struct {
	name        string
	codeVersion int
	header      []byte
}{
	{"Y8S2", Y8S2, slices.Concat([]byte{0x00}, activity2)},                      // Skip(1), Seek(activity2)
	{"Y9S1", Y9S1, slices.Concat(make([]byte, 9), []byte{4}, make([]byte, 24))}, // Skip(9), valid 4, Skip(24)
	{"Y11S2", Y11S2, make([]byte, 38)},                                          // Skip(38)
}

// killPacket builds a kill packet as readMatchFeedback reads it: header, size 0 (kill), the kill
// indicator, killer, 15 bytes, target, 56 bytes, the headshot byte, then tail. The result has no
// spare capacity, so reading past its end panics instead of reading zeros.
func killPacket(header []byte, killer, target string, headshot byte, tail []byte) []byte {
	p := slices.Concat(header, []byte{0x00}, killIndicator)
	p = append(p, byte(len(killer)))
	p = append(p, killer...)
	p = append(p, make([]byte, 15)...)
	p = append(p, byte(len(target)))
	p = append(p, target...)
	p = append(p, make([]byte, 56)...)
	p = append(p, headshot)
	return slices.Clip(append(p, tail...))
}

func weaponTail(id uint64) []byte {
	return binary.LittleEndian.AppendUint64(slices.Clone(killWeaponField), id)
}

func killReader(codeVersion int, b []byte) *Reader {
	return &Reader{b: b, Header: Header{CodeVersion: codeVersion, Players: []Player{
		{Username: "atk", TeamIndex: 0}, {Username: "def", TeamIndex: 1},
	}}}
}

// readOneKill runs readMatchFeedback on b and returns the one Kill it adds and the offset it
// leaves, which must be just past the headshot byte (the weapon read does not move it).
func readOneKill(t *testing.T, codeVersion int, b []byte, tailLen int) MatchUpdate {
	t.Helper()
	r := killReader(codeVersion, b)
	if err := readMatchFeedback(r); err != nil {
		t.Fatalf("readMatchFeedback: %v", err)
	}
	if len(r.MatchFeedback) != 1 || r.MatchFeedback[0].Type != Kill {
		t.Fatalf("want one Kill, got %+v", r.MatchFeedback)
	}
	if want := len(b) - tailLen; r.offset != want {
		t.Errorf("offset %d, want %d (just past the headshot byte)", r.offset, want)
	}
	return r.MatchFeedback[0]
}

// The weapon field directly follows the headshot byte on every season, and the packet may end
// with it at the very end of the buffer.
func TestReadMatchFeedback_KillWeaponID(t *testing.T) {
	for _, h := range killHeaders {
		t.Run(h.name, func(t *testing.T) {
			tail := weaponTail(1366019496)
			u := readOneKill(t, h.codeVersion, killPacket(h.header, "atk", "def", 1, tail), len(tail))
			if u.WeaponID != 1366019496 {
				t.Errorf("WeaponID %d, want 1366019496", u.WeaponID)
			}
			if u.feedKiller != "atk" {
				t.Errorf("feedKiller %q, want atk", u.feedKiller)
			}
		})
	}
}

// A tail that does not hold the weapon field, or holds only part of it, gives weaponID 0 and the
// kill is still read.
func TestReadMatchFeedback_KillWithoutWeaponField(t *testing.T) {
	full := weaponTail(1366019496)
	for _, tc := range []struct {
		name string
		tail []byte
	}{
		{"other_field", []byte{0x22, 0x41, 0x9D, 0x9D, 0xD7, 0x08, 1, 2, 3, 4, 5, 6, 7, 8, 0}},
		// the weapon tag with another size byte: a resized field is not the weapon field
		{"resized_field", []byte{0x22, 0xF8, 0x6C, 0xDD, 0x65, 0x04, 1, 2, 3, 4, 0x22, 0x41, 0x9D, 0x9D, 0xD7}},
		{"truncated", full[:len(full)-1]},
		{"tag_cut", full[:3]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := readOneKill(t, Y11S2, killPacket(make([]byte, 38), "atk", "def", 0, tc.tail), len(tc.tail))
			if u.WeaponID != 0 {
				t.Errorf("WeaponID %d, want 0", u.WeaponID)
			}
			if u.feedKiller != "atk" {
				t.Errorf("feedKiller %q, want atk", u.feedKiller)
			}
		})
	}
}
