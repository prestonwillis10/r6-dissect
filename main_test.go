package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	os.Exit(m.Run())
}

const (
	validDir   = "dissect/test/data/replays/valid"
	clubhouse  = validDir + "/Y11S3/custom_clubhouse_short.rec"
	legacyRec  = validDir + "/Y8S1/quick_1.rec"
	y11s3Rec   = validDir + "/Y11S3"
	sidecarExt = ".timeline.json"
)

func openFixture(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func decodeMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestWriteRound_Sidecar(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "made", "on", "demand")
	var out bytes.Buffer
	if err := writeRound(openFixture(t, clubhouse), &out, dir); err != nil {
		t.Fatal(err)
	}
	round := decodeMap(t, out.Bytes())
	if _, has := round["timelineError"]; has {
		t.Fatal(round["timelineError"])
	}
	if _, has := round["timeline"]; has {
		t.Fatal("timeline must not be embedded")
	}
	b, err := os.ReadFile(filepath.Join(dir, "custom_clubhouse_short"+sidecarExt))
	if err != nil {
		t.Fatal(err)
	}
	var tl dissect.Timeline
	if json.Unmarshal(b, &tl) != nil || tl.Version != 1 || len(tl.Events) == 0 {
		t.Fatal("bad sidecar")
	}
	if bytes.Contains(bytes.TrimRight(b, "\n"), []byte("\n")) {
		t.Fatal("sidecar must not be indented")
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o755) {
		t.Fatal(fi, err) // Windows reports 0o777 for every directory
	}
	if len(round["matchFeedback"].([]any)) != len(tl.Events) {
		t.Fatal("round and sidecar disagree on the event count")
	}
}

func TestWriteRound_LegacyGetsTimelineError(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := writeRound(openFixture(t, legacyRec), &out, dir); err != nil {
		t.Fatal(err)
	}
	round := decodeMap(t, out.Bytes())
	te, ok := round["timelineError"].(map[string]any)
	if !ok || te["reason"] == "" {
		t.Fatal(round["timelineError"])
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("no sidecar on error, found %d entries", len(entries))
	}
}

func TestWriteRound_NoFlagNoField(t *testing.T) {
	var out bytes.Buffer
	if err := writeRound(openFixture(t, legacyRec), &out, ""); err != nil {
		t.Fatal(err)
	}
	if _, has := decodeMap(t, out.Bytes())["timelineError"]; has {
		t.Fatal("timelineError without --timeline")
	}
}

func TestWriteMatch_Sidecars(t *testing.T) {
	folder := t.TempDir()
	recs, err := filepath.Glob(filepath.Join(y11s3Rec, "*.rec"))
	if err != nil || len(recs) != 3 {
		t.Fatal(recs, err)
	}
	for _, p := range recs {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	var out, plain bytes.Buffer
	if err := writeMatch(openFixture(t, folder), JSON, &out, dir); err != nil {
		t.Fatal(err)
	}
	if err := writeMatch(openFixture(t, folder), JSON, &plain, ""); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 3 {
		t.Fatalf("%d sidecars", len(entries))
	}
	for _, p := range recs {
		base := filepath.Base(p)
		if _, err := os.Stat(filepath.Join(dir, base[:len(base)-len(".rec")]+sidecarExt)); err != nil {
			t.Error(err)
		}
	}
	match := decodeMap(t, out.Bytes())
	if _, has := match["timeline"]; has || len(match["rounds"].([]any)) != 3 {
		t.Fatal("match JSON shape")
	}
	if !bytes.Equal(out.Bytes(), plain.Bytes()) {
		t.Fatal("match JSON changed by --timeline")
	}
}

func TestWriteMatch_ExcelIgnoresTimeline(t *testing.T) {
	folder := t.TempDir()
	b, err := os.ReadFile(clubhouse)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "r1.rec"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "tl")
	if err := writeMatch(openFixture(t, folder), Excel, io.Discard, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("excel run wrote a timeline", err)
	}
}

func TestSidecarPath(t *testing.T) {
	got := sidecarPath("out", filepath.Join("a b", "Match-R01.rec"))
	if want := filepath.Join("out", "Match-R01.timeline.json"); got != want {
		t.Fatal(got)
	}
}

func TestWriteMatch_LegacyRoundGetsTimelineError(t *testing.T) {
	folder := t.TempDir()
	for name, src := range map[string]string{"a_legacy.rec": legacyRec, "b_modern.rec": clubhouse} {
		b, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	var out bytes.Buffer
	if err := writeMatch(openFixture(t, folder), JSON, &out, dir); err != nil {
		t.Fatal(err)
	}
	rounds := decodeMap(t, out.Bytes())["rounds"].([]any)
	if _, has := rounds[0].(map[string]any)["timelineError"]; !has {
		t.Error("legacy round has no timelineError")
	}
	if _, has := rounds[1].(map[string]any)["timelineError"]; has {
		t.Error("modern round has a timelineError")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || entries[0].Name() != "b_modern.timeline.json" {
		t.Fatal(entries)
	}
}
