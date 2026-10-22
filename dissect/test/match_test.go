package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
)

const (
	earlyPlant = "data/replays/valid/Y11S2/custom_early_plant.rec" // header roundNumber 0
	solidSnake = "data/replays/valid/Y11S2/custom_solid_snake.rec" // header roundNumber 2
)

var zstdMagic = []byte{0x28, 0xB5, 0x2F, 0xFD}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// matchFolder writes files into a temp dir and returns it opened.
func matchFolder(t *testing.T, files map[string][]byte) *os.File {
	t.Helper()
	dir := t.TempDir()
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

type matchJSON struct {
	Rounds []struct {
		RoundNumber int `json:"roundNumber"`
	} `json:"rounds"`
	SkippedRounds []struct {
		File  string `json:"file"`
		Index int    `json:"index"`
		Error string `json:"error"`
	} `json:"skippedRounds"`
}

func decodeMatch(t *testing.T, m *dissect.MatchReader) (matchJSON, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := m.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var out matchJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out, buf.Bytes()
}

func TestMatchReader_Read_SkipsUnreadableRounds(t *testing.T) {
	good := readFixture(t, earlyPlant)
	snake := readFixture(t, solidSnake)
	headerEnd := bytes.Index(snake, zstdMagic)
	if headerEnd <= 0 {
		t.Fatal("no zstd frame in fixture")
	}
	tests := []struct {
		name        string
		files       map[string][]byte
		wantRounds  []int
		wantSkipped []string
		wantIndex   []int
	}{
		{"empty last file", map[string][]byte{"R01.rec": good, "R02.rec": {}}, []int{0}, []string{"R02.rec"}, []int{1}},
		{"empty first file", map[string][]byte{"R01.rec": {}, "R02.rec": good}, []int{0}, []string{"R01.rec"}, []int{0}},
		{"empty middle file", map[string][]byte{"R01.rec": good, "R02.rec": {}, "R03.rec": snake}, []int{0, 2}, []string{"R02.rec"}, []int{1}},
		{"header cut short", map[string][]byte{"R01.rec": good, "R02.rec": snake[:2000]}, []int{0}, []string{"R02.rec"}, []int{1}},
		{"data cut short", map[string][]byte{"R01.rec": good, "R02.rec": snake[:len(snake)/2]}, []int{0}, []string{"R02.rec"}, []int{1}},
		{"no data after the header", map[string][]byte{"R01.rec": good, "R02.rec": snake[:headerEnd]}, []int{0}, []string{"R02.rec"}, []int{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dissect.NewMatchReader(matchFolder(t, tt.files))
			if err != nil {
				t.Fatal(err)
			}
			err = m.Read()
			var incomplete *dissect.IncompleteMatchError
			if !errors.As(err, &incomplete) {
				t.Fatalf("Read() = %v, want *IncompleteMatchError", err)
			}
			if dissect.Ok(err) {
				t.Error("Ok(err) = true, want false")
			}
			if incomplete.Total != len(tt.files) {
				t.Errorf("Total = %d, want %d", incomplete.Total, len(tt.files))
			}
			out, _ := decodeMatch(t, m)
			var gotRounds []int
			for _, r := range out.Rounds {
				gotRounds = append(gotRounds, r.RoundNumber)
			}
			if !reflect.DeepEqual(gotRounds, tt.wantRounds) {
				t.Errorf("rounds = %v, want %v", gotRounds, tt.wantRounds)
			}
			var gotSkipped []string
			var gotIndex []int
			for _, s := range out.SkippedRounds {
				gotSkipped = append(gotSkipped, s.File)
				gotIndex = append(gotIndex, s.Index)
				if s.Error == "" {
					t.Errorf("skipped %s has an empty error", s.File)
				}
			}
			if !reflect.DeepEqual(gotSkipped, tt.wantSkipped) {
				t.Errorf("skippedRounds = %v, want %v", gotSkipped, tt.wantSkipped)
			}
			if !reflect.DeepEqual(gotIndex, tt.wantIndex) {
				t.Errorf("skipped indexes = %v, want %v", gotIndex, tt.wantIndex)
			}
			if err := m.WriteExcel(io.Discard); err != nil {
				t.Errorf("WriteExcel: %v", err)
			}
		})
	}
}

func TestMatchReader_Read_NoReadableRound(t *testing.T) {
	snake := readFixture(t, solidSnake)
	m, err := dissect.NewMatchReader(matchFolder(t, map[string][]byte{"R01.rec": {}, "R02.rec": snake[:2000]}))
	if err != nil {
		t.Fatal(err)
	}
	var incomplete *dissect.IncompleteMatchError
	if err := m.Read(); !errors.As(err, &incomplete) {
		t.Fatalf("Read() = %v, want *IncompleteMatchError", err)
	}
	if len(incomplete.Skipped) != 2 || incomplete.Total != 2 {
		t.Fatalf("Skipped=%d Total=%d, want 2 and 2", len(incomplete.Skipped), incomplete.Total)
	}
	first := incomplete.Skipped[0]
	if filepath.Base(first.Path) != "R01.rec" {
		t.Errorf("Skipped[0].Path = %s, want R01.rec", first.Path)
	}
	if !errors.Is(first, io.EOF) {
		t.Errorf("Skipped[0] = %v, want io.EOF", first)
	}
}

func TestMatchReader_Read_CompleteMatchHasNoSkippedRounds(t *testing.T) {
	m, err := dissect.NewMatchReader(matchFolder(t, map[string][]byte{
		"R01.rec": readFixture(t, earlyPlant),
		"R02.rec": readFixture(t, solidSnake),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Read(); err != nil {
		t.Fatal(err)
	}
	if len(m.SkippedRounds()) != 0 {
		t.Errorf("SkippedRounds() = %v, want none", m.SkippedRounds())
	}
	_, raw := decodeMatch(t, m)
	if bytes.Contains(raw, []byte(`"skippedRounds"`)) {
		t.Error("complete match JSON carries skippedRounds")
	}
}

func TestMatchReader_RoundAt_SkippedRound(t *testing.T) {
	m, err := dissect.NewMatchReader(matchFolder(t, map[string][]byte{
		"R01.rec": readFixture(t, earlyPlant),
		"R02.rec": {},
	}))
	if err != nil {
		t.Fatal(err)
	}
	_ = m.Read()
	var first *dissect.RoundError
	for i := range 2 {
		r, err := m.RoundAt(1)
		var re *dissect.RoundError
		if r != nil || !errors.As(err, &re) || re.Index != 1 {
			t.Fatalf("RoundAt(1) = %v, %v; want nil and *RoundError index 1", r, err)
		}
		if i == 0 {
			first = re
		} else if re != first {
			t.Error("second RoundAt(1) returned a different error value; the failure should be cached")
		}
	}
	if r, err := m.RoundAt(0); r == nil || err != nil {
		t.Errorf("RoundAt(0) = %v, %v", r, err)
	}
}

// A round whose Read fails part-way is already stored half-read in the
// reader; it must be dropped, not emitted without its round end.
func TestMatchReader_Read_DropsRoundWhoseReadFailed(t *testing.T) {
	m, err := dissect.NewMatchReader(matchFolder(t, map[string][]byte{
		"R01.rec": readFixture(t, earlyPlant),
		"R02.rec": readFixture(t, solidSnake),
	}))
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	// the player-list pattern, present in both fixtures
	m.Listen([]byte{0x22, 0x07, 0x94, 0x9B, 0xDC}, func(r *dissect.Reader) error {
		if r.Header.RoundNumber == 2 {
			return boom
		}
		return nil
	})
	var incomplete *dissect.IncompleteMatchError
	if err := m.Read(); !errors.As(err, &incomplete) {
		t.Fatalf("Read() = %v, want *IncompleteMatchError", err)
	}
	if len(incomplete.Skipped) != 1 || !errors.Is(incomplete.Skipped[0], boom) {
		t.Fatalf("Skipped = %v, want one wrapping boom", incomplete.Skipped)
	}
	out, _ := decodeMatch(t, m)
	if len(out.Rounds) != 1 || out.Rounds[0].RoundNumber != 0 {
		t.Errorf("rounds = %+v, want only round 0", out.Rounds)
	}
}
