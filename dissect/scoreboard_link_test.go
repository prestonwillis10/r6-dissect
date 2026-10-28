package dissect

import (
	"encoding/binary"
	"os"
	"testing"
)

// scoreboardLink writes op <player obj> EB219B38 <entity> <class>, the reference from a player
// object to its scoreboard entity. A 0x1A op writes the reference form, which has no owner handle.
func (b *inventoryBuilder) scoreboardLink(op byte, playerObj, entity uint64, class []byte) {
	b.WriteByte(op)
	if op != 0x1A {
		b.handle(playerObj)
	}
	b.Write(scoreboardEntityField)
	b.handle(entity)
	b.Write(class)
}

// sbID returns the scoreboard entry ID of an entity handle: its low 4 bytes, little-endian.
func sbID(entity uint64) [4]byte {
	var id [4]byte
	binary.LittleEndian.PutUint32(id[:], uint32(entity))
	return id
}

// readLinks runs readScoreboardEntity at every scoreboardEntityField in r.b.
func readLinks(r *Reader) {
	for i := 0; i+len(scoreboardEntityField) <= len(r.b); i++ {
		if string(r.b[i:i+len(scoreboardEntityField)]) == string(scoreboardEntityField) {
			r.offset = i + len(scoreboardEntityField)
			if err := readScoreboardEntity(r); err != nil {
				panic(err)
			}
		}
	}
}

func TestReadScoreboardEntity(t *testing.T) {
	const obj, ent = 0xF02BBB30, 0xF02BBB39
	for _, tc := range []struct {
		name  string
		build func(b *inventoryBuilder)
		want  bool
	}{
		{"named ref", func(b *inventoryBuilder) { b.scoreboardLink(0x1B, obj, ent, scoreboardEntityClass) }, true},
		{"named ref after other data", func(b *inventoryBuilder) {
			b.Write([]byte{0x22, 0x01, 0x02, 0x03, 0x04, 0x01, 0x00})
			b.scoreboardLink(0x1B, obj, ent, scoreboardEntityClass)
			b.Write([]byte{0x23})
		}, true},
		// the departed player's entity in a Y11S1 round: the reference form, after an empty inventory
		{"unnamed ref after an empty inventory", func(b *inventoryBuilder) {
			b.WriteByte(0x1B)
			b.handle(obj)
			b.Write(inventoryOwnerField)
			b.Write(make([]byte, 12))
			b.scoreboardLink(0x1A, 0, ent, scoreboardEntityClass)
		}, false},
		{"unnamed ref with an owner-sized prefix", func(b *inventoryBuilder) { b.scoreboardLink(0x1A, obj, ent, scoreboardEntityClass) }, false},
		{"other op", func(b *inventoryBuilder) {
			b.WriteByte(0x23)
			b.handle(obj)
			b.Write(scoreboardEntityField)
			b.handle(ent)
			b.Write(scoreboardEntityClass)
		}, false},
		{"other class", func(b *inventoryBuilder) { b.scoreboardLink(0x1B, obj, ent, []byte{1, 2, 3, 4}) }, false},
		{"non-zero high half", func(b *inventoryBuilder) { b.scoreboardLink(0x1B, obj, 0x00000001F02BBB39, scoreboardEntityClass) }, false},
		{"zero owner", func(b *inventoryBuilder) { b.scoreboardLink(0x1B, 0, ent, scoreboardEntityClass) }, false},
		{"zero entity", func(b *inventoryBuilder) { b.scoreboardLink(0x1B, obj, 0, scoreboardEntityClass) }, false},
		{"truncated class", func(b *inventoryBuilder) {
			b.scoreboardLink(0x1B, obj, ent, scoreboardEntityClass)
			b.Truncate(b.Len() - 1)
		}, false},
		{"truncated entity", func(b *inventoryBuilder) {
			b.scoreboardLink(0x1B, obj, ent, scoreboardEntityClass)
			b.Truncate(b.Len() - 9)
		}, false},
		{"field at the start", func(b *inventoryBuilder) {
			b.Write(scoreboardEntityField)
			b.handle(ent)
			b.Write(scoreboardEntityClass)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b inventoryBuilder
			tc.build(&b)
			r := &Reader{b: b.Bytes(), scoreboardEntityOwner: map[[4]byte]uint64{}}
			readLinks(r)
			owner, ok := r.scoreboardEntityOwner[sbID(ent)]
			if tc.want && (!ok || owner != obj) {
				t.Fatalf("got %v (owner %#x), want a link to %#x", r.scoreboardEntityOwner, owner, uint64(obj))
			}
			if !tc.want && len(r.scoreboardEntityOwner) != 0 {
				t.Fatalf("got %v, want no link", r.scoreboardEntityOwner)
			}
		})
	}
}

func readFixture(t *testing.T, path string) *Reader {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Read(); !Ok(err) {
		t.Fatal(err)
	}
	return r
}

// On Y10S4+ every player's scoreboard entity is linked to their player object; before Y10S4 the
// listener is not registered (the field is used tens of thousands of times per round there).
func TestScoreboardEntityLink_Fixtures(t *testing.T) {
	for _, path := range []string{
		"test/data/replays/valid/Y11S2/custom_solid_snake.rec",
		"test/data/replays/valid/Y11S2/custom_early_plant.rec",
	} {
		t.Run(path, func(t *testing.T) {
			r := readFixture(t, path)
			linked := map[int]int{}
			for _, id := range r.pendingSBIDs {
				owner, ok := r.scoreboardEntityOwner[id]
				if !ok {
					t.Errorf("sbID %x has no link", id)
					continue
				}
				i := r.playerIndexByObject(owner)
				if i < 0 {
					t.Errorf("sbID %x: owner %#x is not a player object", id, owner)
					continue
				}
				linked[i]++
				if got := r.scoreboardIDToPlayer[id]; got != i {
					t.Errorf("sbID %x mapped to %d, linked to %d", id, got, i)
				}
			}
			if len(linked) != len(r.Header.Players) || len(r.pendingSBIDs) != len(r.Header.Players) {
				t.Errorf("%d players linked, %d sbIDs, %d players", len(linked), len(r.pendingSBIDs), len(r.Header.Players))
			}
		})
	}
	r := readFixture(t, "test/data/replays/valid/Y8S1/quick_1.rec")
	if len(r.scoreboardEntityOwner) != 0 {
		t.Errorf("Y8S1: %d links read, want none", len(r.scoreboardEntityOwner))
	}
}
