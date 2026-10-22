package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
	"github.com/redraskal/r6-dissect/dissect/wire"
)

func fixture(t *testing.T, season, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "dissect", "test", "data", "replays", "valid", season, name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("missing fixture: %v", err)
	}
	return p
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

func TestRun_Segments(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"segments", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	lines := nonEmptyLines(out.String())
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	if last["summary"] != true || last["unplacedFraction"].(float64) > 0.005 {
		t.Fatal(last)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"channel", "hasHeader", "id", "kind", "start", "end", "envelopes", "tickFirst", "tickLast"} {
		if _, ok := first[k]; !ok {
			t.Errorf("channel line lacks %q: %v", k, first)
		}
	}
}

func TestRun_TLVLimit(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"tlv", "--limit", "3", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	lines := nonEmptyLines(out.String())
	if len(lines) != 3 {
		t.Fatalf("lines = %d", len(lines))
	}
	var tok map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &tok); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"tick", "offset", "op", "object", "field"} {
		if _, ok := tok[k]; !ok {
			t.Errorf("token lacks %q: %v", k, tok)
		}
	}
}

func TestRun_Movement(t *testing.T) {
	path := fixture(t, "Y11S3", "custom_clubhouse_short.rec")
	var out, errb bytes.Buffer
	if code := run([]string{"movement", "--limit", "2", "--no-payload", path}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	lines := nonEmptyLines(out.String())
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	for _, ln := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Fatal(err)
		}
		if _, ok := rec["entity"]; !ok {
			t.Errorf("record lacks entity: %v", rec)
		}
		if _, ok := rec["payload"]; ok {
			t.Errorf("--no-payload kept payload: %v", rec)
		}
	}
	// Without the flag the payload is present.
	out.Reset()
	errb.Reset()
	if code := run([]string{"movement", "--limit", "1", path}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(nonEmptyLines(out.String())[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec["payload"]; !ok {
		t.Errorf("payload missing by default: %v", rec)
	}
}

func TestRun_Coverage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"coverage", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	lines := nonEmptyLines(out.String())
	if len(lines) != 1 {
		t.Fatalf("lines = %d", len(lines))
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["tlv"].(map[string]any)["errors"] != 0.0 {
		t.Fatal(obj["tlv"])
	}
	if obj["movement"].(map[string]any)["records"].(float64) <= 0 {
		t.Fatal(obj["movement"])
	}
}

func TestRun_Errors(t *testing.T) { // Review Focus 5
	for _, tc := range []struct {
		args []string
		code int
		msg  string
	}{
		{[]string{}, 2, "usage"},
		{[]string{"bogus", "x.rec"}, 2, "unknown command"},
		{[]string{"segments"}, 2, "missing"},
		{[]string{"segments", "/nonexistent.rec"}, 1, "nonexistent"},
		{[]string{"segments", t.TempDir()}, 2, "one round file"},
		{[]string{"tlv", "--limit", "x", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, 2, "-limit"},
		{[]string{"movement", "--bogus", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, 2, "bogus"},
		{[]string{"tlv", "--limit", "-1", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, 2, "limit"},
		{[]string{"movement", "--limit", "-1", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, 2, "limit"},
		{[]string{"join", "/nonexistent.rec"}, 1, "nonexistent"},
		{[]string{"graph", "/nonexistent.rec"}, 1, "nonexistent"},
		{[]string{"join"}, 2, "missing"},
		{[]string{"graph", t.TempDir()}, 2, "one round file"},
		{[]string{"tlv", "-h"}, 2, "[--limit N]"},
		{[]string{"movement", "--help", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, 2, "[--no-payload]"},
	} {
		var out, errb bytes.Buffer
		code := run(tc.args, &out, &errb)
		if code != tc.code || out.Len() != 0 || len(nonEmptyLines(errb.String())) != 1 || !strings.Contains(errb.String(), tc.msg) {
			t.Errorf("%v: code=%d out=%q err=%q", tc.args, code, clip(out.String()), errb.String())
		}
	}
}

func TestRun_Join(t *testing.T) {
	path := fixture(t, "Y11S3", "custom_fortress_plant.rec")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Read(); !dissect.Ok(err) {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"join", path}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	lines := nonEmptyLines(out.String())
	if len(lines) != len(r.MatchFeedback) {
		t.Fatalf("lines = %d, matchFeedback = %d", len(lines), len(r.MatchFeedback))
	}
	found := false
	for i, ln := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(ln), &ev); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"seq", "type", "tick", "elapsedMs", "roundClockMs", "phase", "status"} {
			if _, ok := ev[k]; !ok {
				t.Errorf("line %d lacks %q: %s", i, k, ln)
			}
		}
		if ev["type"] == "Kill" && ev["actor"] == "FallacyAsTR.NKG" && !found {
			found = true
			if ev["tick"] != 25743.0 || ev["roundClockMs"] != 106538.0 {
				t.Errorf("first fortress kill: %s", ln)
			}
		}
	}
	if !found {
		t.Error("no Kill by FallacyAsTR.NKG")
	}
}

func TestRun_JoinLegacy(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"join", fixture(t, "Y8S1", "quick_1.rec")}, &out, &errb)
	if code != 1 || out.Len() != 0 || len(nonEmptyLines(errb.String())) != 1 || !strings.Contains(errb.String(), "no TLV channels") {
		t.Errorf("code=%d out=%q err=%q", code, clip(out.String()), errb.String())
	}
}

func TestRun_Graph(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"graph", fixture(t, "Y11S3", "custom_fortress_plant.rec")}, &out, &errb); code != 0 {
		t.Fatal(code, errb.String())
	}
	n := 0
	var last float64
	for i, ln := range nonEmptyLines(out.String()) {
		var e map[string]any
		if err := json.Unmarshal([]byte(ln), &e); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"tick", "owner", "field", "target", "class", "index", "hasIndex"} {
			if _, ok := e[k]; !ok {
				t.Fatalf("line %d lacks %q: %s", i, k, ln)
			}
		}
		if tick := e["tick"].(float64); tick < last {
			t.Fatalf("line %d tick %v before %v", i, tick, last)
		} else {
			last = tick
		}
		if e["field"] == "27c08dca" && e["ownerPlayer"] != nil && e["ownerPlayer"] != "" {
			n++
		}
	}
	if n < 10 {
		t.Errorf("27c08dca edges with ownerPlayer = %d", n)
	}
	// The line count equals an independent decode's edge count (ref tokens with a target).
	if got, want := len(nonEmptyLines(out.String())), refEdges(t, fixture(t, "Y11S3", "custom_fortress_plant.rec")); got != want {
		t.Errorf("graph lines = %d, independent edge count = %d", got, want)
	}
}

// refEdges counts the tokens wire.Graph.Add records: ref ops with a non-null target.
func refEdges(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b := r.Data()
	l, err := wire.Walk(b)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if _, err := wire.DecodeTLV(b, l, func(_ uint32, tk wire.Token) error {
		if tk.Op.IsRef() && !tk.Target.IsNull() {
			n++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRun_GraphLegacy(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"graph", fixture(t, "Y8S1", "quick_1.rec")}, &out, &errb)
	if code != 1 || out.Len() != 0 || len(nonEmptyLines(errb.String())) != 1 || !strings.Contains(errb.String(), "no TLV channels") {
		t.Errorf("code=%d out=%q err=%q", code, clip(out.String()), errb.String())
	}
}

// clip keeps a failure message readable when a command wrongly writes a whole stream to stdout.
func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
