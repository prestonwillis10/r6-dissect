# Replay Timeline 01: Wire Decoders and dissect-lab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a structural decoder for `.rec` replays to the r6-dissect fork (`dissect/wire`) and a research CLI (`cmd/dissect-lab`) that can split any Y11S2+ round into channels, envelopes, TLV tokens and movement records with measured coverage, without changing the existing JSON output.

**Architecture:** A new leaf package `dissect/wire` holds pure byte-level decoders with no game semantics: `Walk` splits the decompressed buffer into channels and tick envelopes and sniffs each channel's payload kind; `DecodeBlock`/`DecodeTLV` turn TLV payloads into tokens with object attribution; `DecodeMovement` turns movement payloads into entity records. `cmd/dissect-lab` is a thin CLI over these that prints JSONL. The only change to the existing `dissect` package is an accessor for the decompressed buffer.

**Tech Stack:** Go 1.26 (module `github.com/redraskal/r6-dissect`), standard library only for the new code (`encoding/binary`, `encoding/json`, `flag`), existing test deps (`go-test/deep`). Tests run with `go test ./...`.

**Spec:** `docs/superpowers/specs/2026-10-19-rec-timeline-framework-design.md` (§5.1, §5.2, §7 M1) with the byte grammar in `docs/research/2026-10-19-rec-format-findings.md` (§2.1, §3, §4). Both live in the R6Stats repo; the code lives in the fork clone at `/home/prest/projects/R6Stats/r6-dissect-fix1` (branch `fork/integration`, push to `origin`, never upstream).

## Global Constraints

- Go `1.26.0` floor (`go.mod`); CI builds on Go 1.26 and 1.27 across Linux, macOS, Windows and also builds `exports/exports.go` with `-buildmode=c-shared`. New code must compile on all three OSes (no `syscall`, no platform paths).
- The existing per-round JSON (`Header`, `matchFeedback`, `stats`) and the golden files under `dissect/test/data/replays/valid/**/*.rec.json` must not change. `go test ./...` is green at the start (verified 2026-10-19 on `c521f1a`) and must stay green after every task.
- `dissect/wire` must not import `dissect` (the `dissect` package will import `wire` in plan 02).
- Hex spellings follow the findings doc: handles, field ids and class ids print as lowercase hex **in file byte order** (e.g. object `a07305f000000000`, field `1f07efc9`).
- Decoders never panic on malformed input: every bounds check returns `ok=false` or a typed error; coverage is reported, never assumed.
- Acceptance on every valid fixture (two Y11S2 already in the repo plus the three Y11S3 added in Task 1): bytes outside any channel header or envelope ≤ 0.5 %; TLV unknown opcodes = 0; movement bytes skipped ≤ 0.3 % of the movement channels.
- Commit messages use the fork's conventional style (`feat:`, `fix:`, `test:`, `docs:`). Trailer rule (lane convention, amended 2026-10-20): commits made by dispatched workers through `/ship` carry no `Co-Authored-By` trailer; commits made by the orchestrator end with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.

## Review Focus

1. A non-chunked (Y7S1–Y8S3) replay has a different buffer layout: `Walk` must return a result with everything marked `Unknown` and no error-free false positives, never a panic (pinned in Task 2 with the Y8S1 fixture).
2. A buffer truncated mid-envelope or mid-record (a partial upload): every decoder must stop at the truncation, report it in stats, and never loop or index past the end (pinned in Tasks 2–4 with sliced fixtures).
3. An envelope whose `len` exceeds the remaining buffer, or a movement record whose `L < 4` (no room for a tag): the parse-at functions must return `ok=false` (pinned in Tasks 2 and 4 with synthetic bytes).
4. A zero-length TLV value (`22 field 00`, observed for the defuser timer) and a null reference target (16 zero bytes): tokens must carry an empty value / null target without changing the current object (pinned in Task 3).
5. The CLI given a missing path, a directory, or an unknown subcommand must exit non-zero with a one-line message on stderr and nothing on stdout (pinned in Task 5).

---

### Task 1: Y11S3 fixtures and golden files

**Files:**
- Create: `dissect/test/data/replays/valid/Y11S3/custom_fortress_plant.rec` (copy of `/mnt/c/Users/prest/Downloads/TEST REPLAYS/Match-2026-10-18_18-19-54-14012/Match-2026-10-18_18-19-54-14012-R01.rec`, 10.1 MB, defuser plant, DefusedBomb)
- Create: `dissect/test/data/replays/valid/Y11S3/custom_clubhouse_short.rec` (copy of `.../Match-2026-10-18_19-08-03-14012/Match-2026-10-18_19-08-03-14012-R12.rec`, 6.9 MB, smallest round)
- Create: `dissect/test/data/replays/valid/Y11S3/custom_lair_plant_stop.rec` (copy of `.../Match-2026-10-18_21-07-35-14012/Match-2026-10-18_21-07-35-14012-R04.rec`, 8.7 MB, one DefuserPlantStop and three operator swaps)
- Create: the three matching `*.rec.json` golden files (generated)

**Interfaces:**
- Produces: five Y11S2+ fixtures under `dissect/test/data/replays/valid/{Y11S2,Y11S3}` that every later task's tests walk.

- [ ] **Step 1: Copy the three rounds** into `dissect/test/data/replays/valid/Y11S3/` with the names above (`cp`; keep the `.rec` extension so `TestReader_Read` picks them up).

- [ ] **Step 2: Run the existing golden tests to confirm they fail for lack of golden files**

Run: `go test ./dissect/test -run 'TestNewReader|TestReader_Read' 2>&1 | tail -5`
Expected: FAIL with `could not read ".../custom_fortress_plant.rec.json"` (and the other two).

- [ ] **Step 3: Generate the golden files**

Run: `go test ./dissect/test -run TestReader_Read -update`
Expected: PASS; three new `*.rec.json` files exist.

- [ ] **Step 4: Sanity-check the goldens** by reading each JSON: `custom_fortress_plant` has a `DefuserPlantComplete` by `Yoejinzera.NKG` and 7 `Kill` events; `custom_lair_plant_stop` has one `DefuserPlantStop`; all three have 10 players and `gameVersion` `Y11S3_Alpha04`. Fix nothing here: if a value looks wrong, note it in the commit message, the legacy output is the oracle (spec §6).

- [ ] **Step 5: Run the whole suite**

Run: `go test ./... 2>&1 | tail -5`
Expected: all `ok`.

- [ ] **Step 6: Commit**

```bash
git add dissect/test/data/replays/valid/Y11S3
git commit -m "test: add Y11S3 custom game fixtures (Fortress plant, Club House, Lair plant stop)"
```

---

### Task 2: Container walker (`wire.Walk`)

**Files:**
- Create: `dissect/wire/ids.go`
- Create: `dissect/wire/container.go`
- Create: `dissect/wire/container_test.go`
- Modify: `dissect/reader.go:410` (next to `Write`)

**Interfaces:**
- Consumes: nothing from earlier tasks (fixtures only).
- Produces (package `wire`):

```go
// ids.go
type Handle [8]byte            // object/entity handle in file byte order
func (h Handle) String() string // hex.EncodeToString(h[:])
func (h Handle) IsNull() bool
type FieldID [4]byte           // String(): 8 lowercase hex chars
type ClassID [4]byte           // String(): 8 lowercase hex chars
func handleAt(b []byte, p int) Handle   // copies b[p:p+8]; caller guarantees bounds

// container.go
type PayloadKind int
const (
    PayloadUnknown PayloadKind = iota
    PayloadTLV      // payload[0] == 0x04 and payload ends with 12 zero bytes
    PayloadMovement // payload = u16 count + count records whose tag ends 73 85 fe
)
func (k PayloadKind) String() string // "unknown", "tlv", "movement"

type Envelope struct {
    Start, End int   // [Start, End) of the whole envelope in the buffer
    Tick uint32
    Payload []byte   // slice of the buffer, len == the envelope's len field
}
type Channel struct {
    Start, End int        // header start to end of the last envelope
    HasHeader bool        // false for the leading channel that starts without a 45-byte header
    ID [8]byte            // the header's id8 (zero when HasHeader is false)
    Kind PayloadKind      // sniffed from the first envelope with a non-empty payload
    Envelopes []Envelope
}
type Gap struct{ Start, End int } // bytes not covered by a channel header or envelope
type Layout struct {
    Size int
    Prelude int           // bytes before the first channel or envelope
    Channels []Channel
    Gaps []Gap            // excludes the prelude
}
func (l Layout) UnplacedBytes() int   // Prelude + sum of Gaps
func (l Layout) ChannelsOf(kind PayloadKind) []Channel // channels whose Kind == kind, in buffer order

var ErrNoEnvelopes = errors.New("wire: no channels or envelopes found")

// ParseChannelHeaderAt reports whether a 45-byte channel header starts at p (grammar amended 2026-10-20, findings §2.1):
// u32 lead, one free byte, 7 zero bytes, id8 (non-zero), u32, 00, u32 count (>= 2; count-2 envelopes follow), 4 zero bytes, u32, 8 zero bytes.
func ParseChannelHeaderAt(b []byte, p int) (id [8]byte, ok bool)
// ParseEnvelopeAt parses u32 tick, u32 len, u32 zero, payload[len] at p.
// ok is false when p+12 > len(b), the zero word is not zero, len == 0, or p+12+len > len(b).
func ParseEnvelopeAt(b []byte, p int) (Envelope, bool)
// Walk splits the decompressed buffer (dissect.Reader.Data()) into channels and envelopes.
func Walk(b []byte) (Layout, error)
```

Also in `dissect/reader.go`:

```go
// Data returns the decompressed replay. It is only available between NewReader and Read,
// which releases the buffer.
func (r *Reader) Data() []byte { return r.b }
```

- [ ] **Step 1: Write the synthetic tests in `container_test.go`**

```go
func TestParseEnvelopeAt(t *testing.T) {
    b := append(le32(7), le32(3)...); b = append(b, le32(0)...); b = append(b, 1, 2, 3)
    e, ok := ParseEnvelopeAt(b, 0)
    if !ok || e.Tick != 7 || e.Start != 0 || e.End != 15 || !bytes.Equal(e.Payload, []byte{1, 2, 3}) { t.Fatalf("%+v %v", e, ok) }
    if _, ok := ParseEnvelopeAt(b[:14], 0); ok { t.Fatal("len past buffer must not parse") }
    bad := append(le32(7), le32(3)...); bad = append(bad, le32(9)...); bad = append(bad, 1, 2, 3)
    if _, ok := ParseEnvelopeAt(bad, 0); ok { t.Fatal("non-zero third word must not parse") }
    zero := append(le32(7), le32(0)...); zero = append(zero, le32(0)...)
    if _, ok := ParseEnvelopeAt(zero, 0); ok { t.Fatal("zero len must not parse") }
}
func TestParseChannelHeaderAt(t *testing.T) {
    h := channelHeader([8]byte{0xaa, 1, 2, 3, 4, 5, 6, 7})   // test helper building the 45 bytes
    id, ok := ParseChannelHeaderAt(h, 0)
    if !ok || id[0] != 0xaa { t.Fatal() }
    if _, ok := ParseChannelHeaderAt(h[:44], 0); ok { t.Fatal("short header") }
}
func TestWalk_Synthetic(t *testing.T) {
    // prelude(5 bytes) + headerless TLV envelope + channel(movement-looking payload, 2 envelopes) + 3 junk bytes + channel(unknown payload)
    ... assert len(l.Channels) == 3, l.Channels[0].HasHeader == false, l.Channels[0].Kind == PayloadTLV,
        l.Channels[1].Kind == PayloadMovement, len(l.Channels[1].Envelopes) == 2, l.Channels[2].Kind == PayloadUnknown,
        l.Prelude == 5, l.Gaps == []Gap{{Start: x, End: x+3}}, l.UnplacedBytes() == 8
}
func TestWalk_Empty(t *testing.T) { _, err := Walk([]byte{1,2,3}); if !errors.Is(err, ErrNoEnvelopes) { t.Fatal(err) } }
```

The TLV-looking payload for the synthetic test is `04` + `le32(1)` + `0x18`-free filler + 12 zero bytes; the movement-looking payload is `le16(1)` + one record `handle8 le32(6) 60 73 85 fe 00 00`.

- [ ] **Step 2: Write the fixture tests in `container_test.go`**

```go
func TestWalk_Fixtures(t *testing.T) {
    for _, path := range fixtureRecs(t, "Y11S2", "Y11S3") {     // helper: globs ../test/data/replays/valid/<season>/*.rec
        t.Run(filepath.Base(path), func(t *testing.T) {
            b := decompress(t, path)                            // helper: dissect.NewReader(f).Data() -- see note below
            l, err := Walk(b)
            if err != nil { t.Fatal(err) }
            if frac := float64(l.UnplacedBytes()) / float64(l.Size); frac > 0.005 { t.Errorf("unplaced %.3f%%", frac*100) }
            if n := len(l.ChannelsOf(PayloadTLV)); n < 1 { t.Errorf("tlv channels = %d", n) }
            mv := l.ChannelsOf(PayloadMovement)
            if len(mv) < 1 { t.Fatal("no movement channel") }
            var recs int; for _, c := range mv { recs += len(c.Envelopes) }
            if recs < 1000 { t.Errorf("movement envelopes = %d", recs) }
            for _, c := range l.Channels { assertTicksMonotonic(t, c) }
        })
    }
}
func TestWalk_LegacyLayoutDoesNotPanic(t *testing.T) {    // Review Focus 1
    b := decompress(t, "../test/data/replays/valid/Y8S1/" + firstRec(t, "Y8S1"))
    l, err := Walk(b)
    if err != nil && !errors.Is(err, ErrNoEnvelopes) { t.Fatal(err) }
    _ = l   // any Layout is acceptable; the test only pins "no panic"
}
func TestWalk_Truncated(t *testing.T) {                    // Review Focus 2
    b := decompress(t, fixtureRecs(t, "Y11S3")[0]); b = b[:len(b)/2]
    l, err := Walk(b); if err != nil { t.Fatal(err) }
    if l.Size != len(b) { t.Fatal() }
}
```

Note on the test helper: `wire_test` cannot import `dissect` if `dissect` will later import `wire` (import cycle in tests is allowed only for external test packages). Put the helpers in `package wire_test` (external test package) and import `github.com/redraskal/r6-dissect/dissect` there.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./dissect/wire/ 2>&1 | head`
Expected: build failure, `undefined: Walk` (package does not exist yet).

- [ ] **Step 4: Implement `ids.go`, `Reader.Data()`, and `container.go`**

`Walk` algorithm (the part the tests do not determine):
1. `p = 0`; find the first position where `ParseChannelHeaderAt` or `ParseEnvelopeAt`-with-sniffable-payload succeeds; everything before it is `Prelude`. "Sniffable" for the leading headerless channel means `sniffPayload(payload) != PayloadUnknown`, so random data in the prelude cannot start a channel.
2. Loop while `p < len(b)`: if a channel header starts at `p`, open a new channel (`HasHeader = true`, `ID`), `p += 45`; else if an envelope parses at `p`, append it to the current channel (opening a headerless one if none is open) and `p = e.End`; else record a gap: advance `p` byte by byte until a channel header or an envelope parses **and** the next envelope after it also parses (two in a row, so a random 12-byte coincidence does not end a gap), then append `Gap{start, p}`.
3. A channel's `Kind` is `sniffPayload` of its first envelope with `len(Payload) > 0`.
4. `sniffPayload`: `PayloadTLV` when `payload[0] == 0x04 && len >= 13 && last 12 bytes are zero`; `PayloadMovement` when `len >= 2+12+4`, `count = le16(payload[0:2])`, `count > 0`, and the first record's bytes `payload[2+8+4+1 : 2+8+4+4]` equal `73 85 fe`; else `PayloadUnknown`.

- [ ] **Step 5: Run the tests**

Run: `go test ./dissect/wire/ -run 'TestParse|TestWalk' -v 2>&1 | tail -15`
Expected: PASS for all (fixture tests take a few seconds each; decompression dominates).

- [ ] **Step 6: Run the full suite and vet**

Run: `go vet ./... && go test ./... 2>&1 | tail -6`
Expected: no vet output; all `ok`.

- [ ] **Step 7: Commit**

```bash
git add dissect/wire dissect/reader.go
git commit -m "feat(wire): walk replay channels and tick envelopes"
```

---

### Task 3: TLV token decoder

**Files:**
- Create: `dissect/wire/tlv.go`
- Create: `dissect/wire/tlv_test.go`

**Interfaces:**
- Consumes: `Handle`, `FieldID`, `ClassID`, `Envelope`, `Channel`, `Layout.ChannelsOf` from Task 2.
- Produces:

```go
type Op byte
const (
    OpBatch           Op = 0x18 // handle8 field4 u32 count
    OpRef             Op = 0x1A // field4 handle8 class4
    OpRefNamed        Op = 0x1B // handle8 field4 handle8 class4
    OpRefIndexed      Op = 0x1E // field4 index4 handle8 class4
    OpRefNamedIndexed Op = 0x1F // handle8 field4 index4 handle8 class4
    OpValue           Op = 0x22 // field4 len1 payload
    OpValueNamed      Op = 0x23 // handle8 field4 len1 payload
    OpValueIndexed    Op = 0x26 // field4 index4 len1 payload
    OpValueNamedIndexed Op = 0x27 // handle8 field4 index4 len1 payload
)
func (o Op) String() string      // "batch", "ref", "refNamed", ... ; unknown ops print "op0x%02x"
func (o Op) IsRef() bool; func (o Op) IsValue() bool

type Token struct {
    Offset  int      // offset of the opcode byte in the buffer
    Op      Op
    Object  Handle   // the object this token applies to (named handle, or the current object)
    Field   FieldID
    Index   uint32   // for indexed ops
    HasIndex bool
    Value   []byte   // value ops: slice of the buffer (may be empty)
    Target  Handle   // ref ops: referenced object (may be null)
    Class   ClassID  // ref ops: class of Target
    Count   uint32   // OpBatch
}
func (t Token) Uint32() (uint32, bool)   // ok when len(Value) == 4
func (t Token) Float32() (float32, bool) // ok when len(Value) == 4
func (t Token) Uint64() (uint64, bool)   // ok when len(Value) == 8
func (t Token) ValueHandle() (Handle, bool) // ok when len(Value) == 8 (value holds an object handle)

type DecodeError struct { Offset int; Op byte; Reason string }  // implements error: "wire: <Reason> at offset <Offset> (op 0x%02x)"

type Block struct { Start, End int } // absolute buffer bounds of one `u32 C` block payload inside a TLV envelope
// Blocks splits a TLV envelope payload: skip the leading 0x04, then repeat {u32 C, payload C} until the trailing 12 zero bytes.
// Offsets are absolute (e.Start + 12 is the payload start).
func Blocks(e Envelope) ([]Block, error)
// DecodeBlock walks the tokens of b[blk.Start:blk.End], tracking the current object:
// OpBatch, OpValueNamed and OpValueNamedIndexed set it to their handle; the ref ops set it to Target when
// Target is non-null, otherwise (named refs) to their handle. emit returning a non-nil error stops the walk.
// An unknown opcode or a token overrunning blk.End returns *DecodeError.
func DecodeBlock(b []byte, blk Block, emit func(Token) error) error

type TLVStats struct { Envelopes, Blocks, Tokens int; Errors []DecodeError }
// DecodeTLV decodes every envelope of every PayloadTLV channel in l, in buffer order; emit receives the envelope
// tick with each token. Decode errors are collected per block (the block is abandoned, the next one continues).
func DecodeTLV(b []byte, l Layout, emit func(tick uint32, t Token) error) (TLVStats, error)
```

- [ ] **Step 1: Write synthetic token tests in `tlv_test.go`** (helpers `tok(...)` build byte slices):

```go
func TestDecodeBlock_EveryOpcode(t *testing.T) {
    h1 := Handle{0xa0,0x73,0x05,0xf0}; h2 := Handle{0x30,0x3b,0x04,0xf0}
    blk := concat(
        batch(h1, fid("1784e409"), 5),                 // 18
        valueNamed(h2, fid("e58c06e9"), le32(2)),      // 23 -> current = h2
        value(fid("a9c858d9"), nil),                   // 22, zero-length value (Review Focus 4)
        refNamed(h1, fid("27c08dca"), h2, cid("b2216bf3")), // 1b -> current = h2
        ref(fid("1ed17fd6"), Handle{}, ClassID{}),     // 1a with null target: current stays h2
        refIndexed(fid("9a984fa2"), 3, h1, cid("b430ebc4")), // 1e -> current = h1
        valueIndexed(fid("51b44c48"), 1, []byte{0}),   // 26 on h1
    )
    var got []Token
    if err := DecodeBlock(blk, Block{0, len(blk)}, func(t Token) error { got = append(got, t); return nil }); err != nil { t.Fatal(err) }
    want := []struct{ op Op; obj Handle }{{OpBatch, h1}, {OpValueNamed, h2}, {OpValue, h2}, {OpRefNamed, h1}, {OpRef, h2}, {OpRefIndexed, h2}, {OpValueIndexed, h1}}
    ... compare len, each Op and Object; assert got[2].Value is empty, got[4].Target.IsNull(), got[5].Index == 3 && got[5].HasIndex, got[0].Count == 5
}
func TestDecodeBlock_UnknownOpcode(t *testing.T) {
    blk := concat(value(fid("1f07efc9"), le32(44)), []byte{0x99, 1, 2})
    err := DecodeBlock(blk, Block{0, len(blk)}, func(Token) error { return nil })
    var de *DecodeError
    if !errors.As(err, &de) || de.Offset != 10 || de.Op != 0x99 { t.Fatalf("%v", err) }
}
func TestDecodeBlock_Overrun(t *testing.T) {     // Review Focus 2/3: a value whose len runs past the block
    f := fid("1f07efc9"); blk := concat([]byte{0x22}, f[:], []byte{9, 1, 2})   // arrays returned by a call are unaddressable: assign before slicing
    err := DecodeBlock(blk, Block{0, len(blk)}, func(Token) error { return nil })
    var de *DecodeError; if !errors.As(err, &de) || de.Reason != "token overruns block" { t.Fatal(err) }
}
func TestBlocks(t *testing.T) {   // envelope at offset 0 (tick, len, zero = 12 bytes), payload: 04 | C=3 | a b c | C=1 | d | 00*12
    e, _ := ParseEnvelopeAt(buf, 0)
    blocks, err := Blocks(e)
    assert err == nil && blocks == []Block{{17, 20}, {24, 25}} (absolute offsets); a payload not ending in 12 zero bytes returns an error
}
func TestToken_Accessors(t *testing.T) { ... Uint32 ok for 4 bytes, !ok for 5; Uint64/ValueHandle ok for 8 }
```

- [ ] **Step 2: Write the fixture tests in `tlv_test.go`**

```go
func TestDecodeTLV_Fixtures(t *testing.T) {
    for _, path := range fixtureRecs(t, "Y11S2", "Y11S3") {
        t.Run(filepath.Base(path), func(t *testing.T) {
            b := decompress(t, path); l, _ := Walk(b)
            var tokens, frames, msFrames int
            var lastTick uint32; var lastTickSet bool
            msField := fid("1837466c"); timerField := fid("1f07efc9")
            var timerObj Handle
            st, err := DecodeTLV(b, l, func(tick uint32, tk Token) error {
                tokens++
                if tk.Op == OpBatch { frames++; if lastTickSet && tick < lastTick { t.Errorf("tick went backwards %d -> %d", lastTick, tick) }; lastTick, lastTickSet = tick, true }
                if tk.Field == timerField && tk.Op.IsValue() { timerObj = tk.Object }
                if tk.Field == msField && tk.Op.IsValue() { msFrames++ }
                return nil
            })
            if err != nil { t.Fatal(err) }
            if len(st.Errors) != 0 { t.Errorf("decode errors: %d, first %v", len(st.Errors), st.Errors[0]) }
            if st.Envelopes < 1000 || tokens < 5000 { t.Errorf("envelopes=%d tokens=%d", st.Envelopes, tokens) }
            if float64(msFrames) < 0.95*float64(frames) { t.Errorf("ms clock present in %d of %d batches", msFrames, frames) }
            if timerObj.IsNull() { t.Error("round timer object not found") }
        })
    }
}
```

The ms-clock expectation (≥ 95 % of batches carry field `1837466c`) is the measured fact from the findings doc for Y11S3; if a Y11S2 fixture fails it, lower only that fixture's threshold in the test with a comment stating the measured value (do not relax the Y11S3 ones).

- [ ] **Step 3: Run to verify failure**

Run: `go test ./dissect/wire/ -run 'TestDecodeBlock|TestBlocks|TestToken|TestDecodeTLV' 2>&1 | head -5`
Expected: build failure, `undefined: DecodeBlock`.

- [ ] **Step 4: Implement `tlv.go`** per the Interfaces block. Token layouts are fixed by the opcode table; the only judgement call is the current-object rule, which is stated in the `DecodeBlock` comment above.

- [ ] **Step 5: Run the tests**

Run: `go test ./dissect/wire/ -v -run 'TestDecodeBlock|TestBlocks|TestToken|TestDecodeTLV' 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 6: Full suite, then commit**

Run: `go vet ./... && go test ./... 2>&1 | tail -6` → all `ok`.

```bash
git add dissect/wire/tlv.go dissect/wire/tlv_test.go
git commit -m "feat(wire): decode TLV property tokens with object attribution"
```

---

### Task 4: Movement record decoder

**Files:**
- Create: `dissect/wire/movement.go`
- Create: `dissect/wire/movement_test.go`

**Interfaces:**
- Consumes: `Handle`, `Envelope`, `Channel`, `Layout` from Task 2.
- Produces:

```go
type Record struct {
    Offset  int       // offset of the handle in the buffer
    Entity  Handle
    Tag     [4]byte   // observed: 60|61|62|63 73 85 fe
    Payload []byte    // bytes after the tag (len L-4)
}
type Group struct { Envelope Envelope; Records []Record }
// ParseGroup parses a movement envelope payload: u16 count, then count x (handle8, u32 L, L bytes with L >= 4).
// ok is false when a record does not fit, when the records do not end exactly at the payload end,
// or when any tag does not end in 73 85 fe.
func ParseGroup(e Envelope) (Group, bool)

type MovementStats struct {
    Envelopes, Groups, Records int
    BadEnvelopes int      // envelopes in movement channels that ParseGroup rejected
    BadBytes int          // their payload bytes
    FirstTick, LastTick uint32
    TicksMonotonic bool
}
// DecodeMovement parses every envelope of every PayloadMovement channel in l, in buffer order.
func DecodeMovement(b []byte, l Layout, emit func(Group) error) (MovementStats, error)
```

- [ ] **Step 1: Write synthetic tests**

```go
func TestParseGroup(t *testing.T) {
    e := envelope(1, concat(le16(2), rec(h("5cc804f0"), tag60, []byte{1,2,3}), rec(h("3dca04f0"), tag61, nil)))
    g, ok := ParseGroup(e)
    if !ok || len(g.Records) != 2 || g.Records[0].Entity != h("5cc804f0") || !bytes.Equal(g.Records[0].Payload, []byte{1,2,3}) || len(g.Records[1].Payload) != 0 { t.Fatal() }
    e5 := h("5cc804f0"); if _, ok := ParseGroup(envelope(1, concat(le16(1), e5[:], le32(3), []byte{0x60,0x73,0x85}))); ok { t.Fatal("L<4 must fail") }   // assign before slicing   // Review Focus 3
    if _, ok := ParseGroup(envelope(1, concat(le16(1), rec(h("5cc804f0"), [4]byte{1,2,3,4}, nil)))); ok { t.Fatal("bad tag must fail") }
    if _, ok := ParseGroup(envelope(1, concat(le16(2), rec(h("5cc804f0"), tag60, nil)))); ok { t.Fatal("count > records must fail") }
    if _, ok := ParseGroup(envelope(1, concat(le16(1), rec(h("5cc804f0"), tag60, nil), []byte{0}))); ok { t.Fatal("trailing bytes must fail") }
}
```

- [ ] **Step 2: Write the fixture test**

```go
func TestDecodeMovement_Fixtures(t *testing.T) {
    for _, path := range fixtureRecs(t, "Y11S2", "Y11S3") {
        t.Run(filepath.Base(path), func(t *testing.T) {
            b := decompress(t, path); l, _ := Walk(b)
            entities := map[Handle]int{}
            st, err := DecodeMovement(b, l, func(g Group) error { for _, r := range g.Records { entities[r.Entity]++ }; return nil })
            if err != nil { t.Fatal(err) }
            if st.Records < 100000 { t.Errorf("records = %d", st.Records) }
            if !st.TicksMonotonic { t.Error("ticks not monotonic") }
            var total int; for _, c := range l.ChannelsOf(PayloadMovement) { total += c.End - c.Start }
            if frac := float64(st.BadBytes) / float64(total); frac > 0.003 { t.Errorf("bad bytes %.3f%%", frac*100) }
            // ten always-present movers (player pawns): at least 10 entities with >= 3000 records
            var busy int; for _, n := range entities { if n >= 3000 { busy++ } }
            if busy < 10 { t.Errorf("busy entities = %d, want >= 10", busy) }
        })
    }
}
func TestDecodeMovement_Truncated(t *testing.T) {   // Review Focus 2
    b := decompress(t, fixtureRecs(t, "Y11S3")[0]); l, _ := Walk(b[:30_000_000])
    if _, err := DecodeMovement(b[:30_000_000], l, func(Group) error { return nil }); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 3: Run to verify failure** — `go test ./dissect/wire/ -run 'TestParseGroup|TestDecodeMovement'` → `undefined: ParseGroup`.

- [ ] **Step 4: Implement `movement.go`** per the Interfaces block.

- [ ] **Step 5: Run the tests** — `go test ./dissect/wire/ -v -run 'TestParseGroup|TestDecodeMovement' 2>&1 | tail -12` → PASS.

- [ ] **Step 6: Full suite, commit**

```bash
go vet ./... && go test ./... 2>&1 | tail -6
git add dissect/wire/movement.go dissect/wire/movement_test.go
git commit -m "feat(wire): decode movement groups and entity records"
```

---

### Task 5: `dissect-lab` CLI

**Files:**
- Create: `cmd/dissect-lab/main.go` (dispatch, flags, exit codes)
- Create: `cmd/dissect-lab/commands.go` (`segments`, `tlv`, `movement`, `coverage`)
- Create: `cmd/dissect-lab/main_test.go`
- Modify: `README.md` (new "Research CLI" section after "Shared Library (FFI)")
- Modify: `.gitignore` (add `/dissect-lab` binary)

**Interfaces:**
- Consumes: `wire.Walk`, `wire.DecodeTLV`, `wire.DecodeMovement`, `wire.Blocks`, `dissect.NewReader(...).Data()`.
- Produces: the executable `dissect-lab` with the usage below, and an internal `run(args []string, stdout, stderr io.Writer) int` used by the test.

```
dissect-lab segments <round.rec>              one JSON object per channel: {"channel":0,"hasHeader":false,"id":"…","kind":"tlv","start":2626,"end":49670,"envelopes":12,"tickFirst":0,"tickLast":0}
                                              then one per gap: {"gap":true,"start":…,"end":…}
                                              then {"summary":true,"size":N,"prelude":N,"unplacedBytes":N,"unplacedFraction":0.0012}
dissect-lab tlv <round.rec> [--limit N]       one JSON object per token: {"tick":44,"offset":…,"op":"valueNamed","object":"a51505f000000000","field":"1f07efc9","len":4,"value":"2c000000"}
                                              ref ops: {"tick":…,"offset":…,"op":"refNamed","object":"…","field":"27c08dca","target":"303b04f000000000","class":"b2216bf3"}
                                              indexed ops add "index":N; batch adds "count":N
dissect-lab movement <round.rec> [--limit N] [--no-payload]
                                              one JSON object per record: {"tick":1,"offset":…,"entity":"5cc804f000000000","tag":"607385fe","len":1090,"payload":"<hex>"}
dissect-lab coverage <round.rec>              one JSON object: {"size":N,"unplacedFraction":…,"channels":[…as segments…],"tlv":{"envelopes":…,"blocks":…,"tokens":…,"errors":N},"movement":{"envelopes":…,"groups":…,"records":…,"badBytes":…,"ticksMonotonic":true}}
```

Exit codes: 0 success; 2 usage error (unknown command, missing path, flag error) with usage on stderr; 1 runtime error (open/decode failure) with `dissect-lab: <error>` on stderr. `--limit 0` (default) means unlimited. Input must be a single `.rec` file (a directory is a usage error: "dissect-lab works on one round file").

- [ ] **Step 1: Write `main_test.go`**

```go
func TestRun_Segments(t *testing.T) {
    var out, errb bytes.Buffer
    if code := run([]string{"segments", fixture(t, "Y11S3", "custom_clubhouse_short.rec")}, &out, &errb); code != 0 { t.Fatal(code, errb.String()) }
    lines := nonEmptyLines(out.String())
    var last map[string]any; json.Unmarshal([]byte(lines[len(lines)-1]), &last)
    if last["summary"] != true || last["unplacedFraction"].(float64) > 0.005 { t.Fatal(last) }
}
func TestRun_TLVLimit(t *testing.T) {
    var out, errb bytes.Buffer
    if code := run([]string{"tlv", "--limit", "3", fixture(...)}, &out, &errb); code != 0 { t.Fatal() }
    if n := len(nonEmptyLines(out.String())); n != 3 { t.Fatalf("lines = %d", n) }
}
func TestRun_Movement(t *testing.T) { ... "movement", "--limit", "2", "--no-payload" → 2 lines, each has "entity" and no "payload" key }
func TestRun_Coverage(t *testing.T) { ... one line; obj["tlv"].(map)["errors"] == 0.0 }
func TestRun_Errors(t *testing.T) {     // Review Focus 5
    for _, args := range [][]string{{}, {"bogus", "x.rec"}, {"segments"}, {"segments", "/nonexistent.rec"}, {"segments", t.TempDir()}} {
        var out, errb bytes.Buffer
        if code := run(args, &out, &errb); code == 0 || out.Len() != 0 || errb.Len() == 0 { t.Errorf("%v: code=%d out=%q err=%q", args, code, out.String(), errb.String()) }
    }
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./cmd/dissect-lab/` → `undefined: run`.

- [ ] **Step 3: Implement `main.go` and `commands.go`.** `main()` calls `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))`. Use `flag.NewFlagSet` per subcommand with `ContinueOnError` and output redirected to `stderr`. Encode each line with `json.NewEncoder(stdout)` (no indent). Set `zerolog.SetGlobalLevel(zerolog.Disabled)` before `dissect.NewReader` so the reader's logs do not pollute stderr.

- [ ] **Step 4: Run the tests** — `go test ./cmd/dissect-lab/ -v 2>&1 | tail -12` → PASS.

- [ ] **Step 5: Try it on a real round and eyeball the output**

Run: `go run ./cmd/dissect-lab coverage "dissect/test/data/replays/valid/Y11S3/custom_fortress_plant.rec" | python3 -m json.tool | head -40`
Expected: `unplacedFraction` ≤ 0.005, `tlv.errors` 0, `movement.records` > 100000.

- [ ] **Step 6: Document** — add to `README.md` a "Research CLI (`dissect-lab`)" section: one paragraph (what it is, not part of releases), the four commands with their one-line outputs, and a pointer to the R6Stats findings doc path. Add `/dissect-lab` to `.gitignore`.

- [ ] **Step 7: Full suite, build everything, commit**

```bash
go vet ./... && go build ./... && go test ./... 2>&1 | tail -6
git add cmd/dissect-lab README.md .gitignore
git commit -m "feat: add dissect-lab research CLI (segments, tlv, movement, coverage)"
```

---

### Task 6: Coverage report in the design's acceptance terms

**Files:**
- Create: `docs/wire-coverage.md` (in the fork)

**Interfaces:**
- Consumes: `dissect-lab coverage` from Task 5.

- [ ] **Step 1: Generate** `dissect-lab coverage` for all five Y11S2/Y11S3 fixtures and tabulate: file, size, unplaced %, TLV envelopes/tokens/errors, movement records/bad %, busy entities. Put the table in `docs/wire-coverage.md` with the command used and the commit hash.

- [ ] **Step 2: Check each row against the Global Constraints** (unplaced ≤ 0.5 %, TLV errors 0, movement bad ≤ 0.3 %). If a row fails, that is a bug in Tasks 2–4: fix it there (with a test) before continuing; do not edit the thresholds.

- [ ] **Step 3: Commit and push**

```bash
git add docs/wire-coverage.md
git commit -m "docs: wire decoder coverage on Y11S2/Y11S3 fixtures"
git push origin fork/integration
```
