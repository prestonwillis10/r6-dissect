package wire_test

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
	wire "github.com/redraskal/r6-dissect/dissect/wire"
	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}

const validDir = "../test/data/replays/valid"

// fixtureRecs returns the absolute, sorted paths of every *.rec under the given seasons.
func fixtureRecs(t *testing.T, seasons ...string) []string {
	t.Helper()
	var out []string
	for _, s := range seasons {
		m, err := filepath.Glob(filepath.Join(validDir, s, "*.rec"))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range m {
			abs, err := filepath.Abs(p)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, abs)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("no .rec fixtures for seasons %v", seasons)
	}
	return out
}

// fixture returns the absolute path of one fixture.
func fixture(t *testing.T, season, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(validDir, season, name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("missing fixture: %v", err)
	}
	return p
}

// firstRec returns the basename of the first .rec of a season.
func firstRec(t *testing.T, season string) string {
	t.Helper()
	return filepath.Base(fixtureRecs(t, season)[0])
}

// decompress opens path with dissect.NewReader and returns the decompressed buffer.
func decompress(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(%s): %v", path, err)
	}
	return r.Data()
}

func le16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }
func le32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func mustHex(s string, max int) []byte {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) < 1 || len(b) > max {
		panic("bad hex " + s)
	}
	return b
}

// h builds a Handle from 1..8 hex bytes copied into the front of a zero Handle.
func h(hexs string) wire.Handle {
	var out wire.Handle
	copy(out[:], mustHex(hexs, 8))
	return out
}

// fid builds a FieldID from 8 hex chars in file byte order.
func fid(hex8 string) wire.FieldID {
	var out wire.FieldID
	if len(hex8) != 8 {
		panic("bad field id " + hex8)
	}
	copy(out[:], mustHex(hex8, 4))
	return out
}

// cid builds a ClassID from 8 hex chars in file byte order.
func cid(hex8 string) wire.ClassID {
	var out wire.ClassID
	if len(hex8) != 8 {
		panic("bad class id " + hex8)
	}
	copy(out[:], mustHex(hex8, 4))
	return out
}

// testLead is the lead u32 used by channelHeader: 03 b9 00 00.
const testLead = 0xb903

// channelHeader builds the 45-byte header of an empty channel in the observed
// grammar: lead 03 b9 00 00, id8 at 12, count 2 at 25 (no envelopes follow).
func channelHeader(id [8]byte) []byte { return channelHeaderN(id, testLead, 0, 0) }

// channelHeaderN builds the 45-byte header of a channel that carries the given
// number of envelopes: u32 lead at 0, a free byte 4 and zeros to 11, id8 at 12,
// a free u32 at 20, zero at 24, u32 count (envelopes+2) at 25, zeros to 32,
// u32 u33 at 33 and zeros to 44.
func channelHeaderN(id [8]byte, lead uint32, envelopes int, u33 uint32) []byte {
	return concat(le32(lead), make([]byte, 8), id[:], le32(1), []byte{0}, le32(uint32(envelopes+2)), make([]byte, 4), le32(u33), make([]byte, 8))
}

// terminator is the 12-byte buffer end: the lead plus 8 zero bytes.
func terminator(lead uint32) []byte { return concat(le32(lead), make([]byte, 8)) }

// envelope parses le32(tick)+le32(len)+le32(0)+payload at offset 0; it panics if that does not parse.
func envelope(tick uint32, payload []byte) wire.Envelope {
	e, ok := wire.ParseEnvelopeAt(envBytes(tick, payload), 0)
	if !ok {
		panic("envelope does not parse")
	}
	return e
}

func envBytes(tick uint32, payload []byte) []byte {
	return concat(le32(tick), le32(uint32(len(payload))), le32(0), payload)
}

// assertTicksMonotonic reports an error when an envelope's tick is lower than its predecessor's.
func assertTicksMonotonic(t *testing.T, c wire.Channel) {
	t.Helper()
	for i := 1; i < len(c.Envelopes); i++ {
		if c.Envelopes[i].Tick < c.Envelopes[i-1].Tick {
			t.Errorf("channel at %d: envelope %d tick %d < previous %d", c.Start, i, c.Envelopes[i].Tick, c.Envelopes[i-1].Tick)
			return
		}
	}
}
