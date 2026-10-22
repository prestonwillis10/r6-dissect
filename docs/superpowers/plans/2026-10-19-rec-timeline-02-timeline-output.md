# Replay Timeline 02: Timeline Sidecar with Tick and Millisecond Timing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Emit a per-round `timeline` sidecar (`r6-dissect --timeline <dir>`) that reproduces every existing `matchFeedback` event exactly and adds a tick, a millisecond elapsed time, phase changes, the player/pawn entity table and decoder coverage, built on the `dissect/wire` decoders from plan 01.

**Architecture:** The legacy listeners stay the source of events (parity by construction): each `MatchUpdate` records the buffer offset where it was read, and the timeline maps that offset to its TLV envelope (tick) and to the millisecond round clock carried by the game-state object. A new `wire.Graph` indexes ownership edges so pawns resolve to players. The timeline is assembled inside `Reader.Read()` while the buffer is still available and written as a sidecar by the CLI; the round JSON is untouched.

**Tech Stack:** Go 1.26, `dissect/wire` (plan 01), `encoding/json`, `pflag`/`viper` already used by `main.go`.

**Spec:** `docs/superpowers/specs/2026-10-19-rec-timeline-framework-design.md` §5.1 (graph, clock), §5.4 (output contract, failure mode, sidecar), §6 (oracle rule), §7 M2. Findings: `docs/research/2026-10-19-rec-format-findings.md` §3 (ms clock, object graph). Code lives in `/home/prest/projects/R6Stats/r6-dissect-fix1` on `fork/integration`; plan 01 must be merged first.

## Global Constraints

- Everything in plan 01's Global Constraints (Go 1.26, three OSes, c-shared build, goldens frozen, suite green after every task, `wire` never imports `dissect`).
- Parity: for every fixture, `timeline.events` has exactly one entry per `matchFeedback` entry, in the same order, with identical `type`, `username`/`actor`, `target`, `headshot`, `teamKill`, `operator`, `phase`; `elapsedSeconds` in the round JSON keeps its current value (spec §6 oracle rule).
- The timeline never fails a parse: on any timeline error the round JSON is written unchanged, the sidecar is **not** written, and the round JSON gains `"timelineError": {"reason": "<text>", "coverage": {...}}` (spec §5.4). `timelineError` is only present when `--timeline` was requested.
- Sidecar path: `<dir>/<basename of the .rec without extension>.timeline.json`, `dir` created if missing (spec §5.4, my call recorded there).
- `timeline.version` is `1`. Event `status` is `"observed"` for events read from a packet and `"derived"` for events the legacy code infers (round-end `DefuserDisableComplete`, a defuser actor resolved from the +100 score bonus or the last-alive fallback).
- Commit messages as in plan 01 (trailer rule: none on worker commits, the orchestrator's carry `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`).

## Review Focus

1. A round where the plant happens in the same tick frame as the ms-clock jump: the event keeps the legacy phase (`Action`) while the clock already shows post-plant; `elapsedMs` must still be computed from the action-phase base (pinned in Task 2's clock test and Task 3's parity tolerance).
2. A round with no action phase (everyone leaves in prep, or a one-sided custom game): the clock has only a `Prep` phase; `elapsedMs` stays negative and `clock.phases` has one entry (pinned in Task 2 with a synthetic series).
3. Legacy replays (Y7–Y10) with `--timeline`: `Walk` finds no channels; the round JSON must carry `timelineError` and the CLI must exit 0 (pinned in Task 4 with the Y8S1 fixture).
4. Events created at round end with no buffer offset (`roundEnd` inferences): they must take the last tick, not tick 0, so ordering by `(tick, seq)` matches `seq` order (pinned in Task 3 by asserting ticks are non-decreasing in `seq` order on every fixture).
5. A match folder with `--timeline`: one sidecar per round file, names derived from each round's own file name, and the match JSON unchanged (pinned in Task 4).

---

### Task 1: Object graph (`wire.Graph`) and `HandleOf`

**Files:**
- Create: `dissect/wire/graph.go`
- Create: `dissect/wire/graph_test.go`
- Modify: `dissect/wire/ids.go` (add `HandleOf`)

**Interfaces:**
- Consumes: `Token`, `Op`, `Handle`, `FieldID`, `ClassID` (plan 01 Task 3).
- Produces:

```go
// ids.go
// HandleOf converts a little-endian uint64 handle (as dissect.readHandle returns) into file byte order.
func HandleOf(v uint64) Handle

// graph.go
type Edge struct { Owner Handle; Field FieldID; Target Handle; Class ClassID; Index uint32; HasIndex bool; Tick uint32 }
type Graph struct { /* parents map[Handle]Edge (last edge that targeted the handle), classes map[Handle]ClassID, children map[Handle][]Edge */ }
func NewGraph() *Graph
// Add records t when t.Op.IsRef() and !t.Target.IsNull(); other tokens are ignored.
func (g *Graph) Add(tick uint32, t Token)
func (g *Graph) Class(h Handle) (ClassID, bool)
func (g *Graph) Parent(h Handle) (Edge, bool)
func (g *Graph) Children(h Handle) []Edge
// Ancestor walks Parent links from h (at most 16 steps) and returns the first handle for which stop returns true.
func (g *Graph) Ancestor(h Handle, stop func(Handle) bool) (Handle, bool)
```

- [ ] **Step 1: Write `graph_test.go`**

```go
func TestGraph(t *testing.T) {
    g := NewGraph()
    player, pawn, inv := h("a07305f0"), h("303b04f0"), h("c4d404f0")
    g.Add(1, Token{Op: OpRefNamed, Object: player, Field: fid("27c08dca"), Target: pawn, Class: cid("b2216bf3")})
    g.Add(2, Token{Op: OpRef, Object: pawn, Field: fid("e8d1e539"), Target: inv, Class: cid("579fb57b")})
    g.Add(3, Token{Op: OpRef, Object: pawn, Field: fid("1ed17fd6"), Target: Handle{}, Class: ClassID{}}) // null: ignored
    g.Add(4, Token{Op: OpValue, Object: pawn, Field: fid("e58c06e9"), Value: []byte{2,0,0,0}})     // value: ignored
    if c, ok := g.Class(pawn); !ok || c != cid("b2216bf3") { t.Fatal() }
    if e, ok := g.Parent(inv); !ok || e.Owner != pawn || e.Field != fid("e8d1e539") || e.Tick != 2 { t.Fatal() }
    if len(g.Children(pawn)) != 1 { t.Fatal() }
    isPlayer := func(x Handle) bool { return x == player }
    if a, ok := g.Ancestor(inv, isPlayer); !ok || a != player { t.Fatal() }
    if _, ok := g.Ancestor(player, func(Handle) bool { return false }); ok { t.Fatal("no ancestor must be reported as !ok") }
}
func TestHandleOf(t *testing.T) {
    if HandleOf(0x00000000f00573a0) != h("a07305f0") { t.Fatal() }  // LE bytes a0 73 05 f0 00 00 00 00
}
func TestGraph_Fixture(t *testing.T) {
    b := decompress(t, fixture(t, "Y11S3", "custom_fortress_plant.rec")); l, _ := Walk(b)
    g := NewGraph()
    DecodeTLV(b, l, func(tick uint32, tk Token) error { g.Add(tick, tk); return nil })
    // the ten player objects each own a pawn through field 27c08dca (findings §3)
    var pawnEdges int
    for ... each child edge with Field == fid("27c08dca") across all owners { pawnEdges++ }
    if pawnEdges < 10 { t.Errorf("pawn edges = %d", pawnEdges) }
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./dissect/wire/ -run 'TestGraph|TestHandleOf'` → `undefined: NewGraph`.

- [ ] **Step 3: Implement `graph.go` and `HandleOf`.** `Ancestor` must guard against cycles with the 16-step cap.

- [ ] **Step 4: Run the tests** — `go test ./dissect/wire/ -run 'TestGraph|TestHandleOf' -v 2>&1 | tail -8` → PASS.

- [ ] **Step 5: Commit**

```bash
git add dissect/wire/graph.go dissect/wire/graph_test.go dissect/wire/ids.go
git commit -m "feat(wire): object ownership graph"
```

---

### Task 2: Millisecond clock and event offsets

**Files:**
- Create: `dissect/msclock.go`
- Create: `dissect/msclock_test.go` (package `dissect`, like `defuse_test.go`)
- Modify: `dissect/feedback.go:30-47` (MatchUpdate fields), `dissect/feedback.go:139,200,236`, `dissect/defuse.go:228,270,363,412,447`, `dissect/player.go:225,263`, `dissect/time.go:295` (append sites), `dissect/scoreboard.go` (`readScoreboardScoreY10S4`: set `derived` when it fills a pending defuser actor)

**Interfaces:**
- Consumes: `wire.Token`, `wire.Op`, `wire.FieldID`, `wire.Handle` (plan 01).
- Produces (package `dissect`):

```go
// feedback.go: two unexported fields on MatchUpdate (deep.Equal ignores unexported fields, so goldens are unaffected)
type MatchUpdate struct {
    ... existing fields ...
    offset  int  // buffer offset of the listener that produced the event; -1 when inferred at round end
    derived bool // the event or its actor was inferred, not read from a packet
}
// addUpdate stamps u.offset = r.offset and appends; every current `r.MatchFeedback = append(r.MatchFeedback, u)` becomes r.addUpdate(u),
// except time.go:295 which sets offset -1 and derived true first.
func (r *Reader) addUpdate(u MatchUpdate)
// derived is also set true in defuse.go: startDefuserRun when playerIndex came from lastAliveDefuserPlayer, and
// in readScoreboardScoreY10S4 when it fills r.MatchFeedback[r.pendingDefuserPlantIdx].Username.

// msclock.go
var (
    fieldRoundTimer = wire.FieldID{0x1f, 0x07, 0xef, 0xc9} // seconds, read by readTime
    fieldRoundClockMs = wire.FieldID{0x18, 0x37, 0x46, 0x6c} // milliseconds remaining in the current phase
)
type msPhase struct { Phase RoundPhase; Tick uint32; StartMs uint32; BaseMs int64 } // BaseMs: elapsedMs at the phase start (Prep: 0 with negative elapsed; Action: 0; PostPlant: elapsed at the jump)
type msClock struct { /* samples []msSample{Tick, Ms}; phases []msPhase; gameState wire.Handle */ }
func newMsClock() *msClock
// observe feeds one TLV token: the first value token carrying fieldRoundTimer fixes the game-state object; value tokens of
// fieldRoundClockMs on that object are samples. A new phase starts on the first sample and whenever the clock jumps by more
// than 1000 ms in EITHER direction (amended 2026-10-20, PR #7 review: a plant with more than 45 s of action time left makes the
// clock DROP to 45000; the original rise-only rule missed two of the five fixtures); a discontinuous drop to exactly 0 is the
// round-end reset — not recorded as a sample, starts no phase (the legacy clock treats 0 the same way). Known limit: a plant
// with 44–46 s left makes no jump either way (run-log candidate C-RT-13).
// Phase labels follow time.go: the first phase is Prep when its first sample is <= 60000 ms, else Action; the phase after
// Action is PostPlant; a phase after PostPlant is also PostPlant.
func (c *msClock) observe(tick uint32, t wire.Token)
func (c *msClock) phases() []msPhase
// at returns the phase in force and the elapsed milliseconds at tick (the last sample at or before tick; ok=false before the first sample).
// elapsed = phase.BaseMs + int64(phase.StartMs) - int64(ms) ; during Prep elapsed = -int64(ms).
func (c *msClock) at(tick uint32) (phase RoundPhase, roundClockMs uint32, elapsedMs int64, ok bool)
// sampleBefore returns the last sample at or before tick (used by Task 3 for Review Focus 1).
func (c *msClock) sampleBefore(tick uint32) (ms uint32, ok bool)
```

- [ ] **Step 1: Write `msclock_test.go`** with synthetic token series (helper `clockTok(field, u32)` builds a `wire.Token{Op: wire.OpValueNamed, Object: gs, Field: field, Value: le32}`):

```go
func TestMsClock_Phases(t *testing.T) {
    c := newMsClock()
    c.observe(10, clockTok(fieldRoundTimer, 44))
    for i, ms := range []uint32{44832, 44757, 44720, 44683} { c.observe(uint32(20+i*9), clockTok(fieldRoundClockMs, ms)) }   // amended: a continuous Prep series
    c.observe(100, clockTok(fieldRoundClockMs, 179917))   // jump up: Action
    c.observe(109, clockTok(fieldRoundClockMs, 179880))
    c.observe(500, clockTok(fieldRoundClockMs, 45000))    // jump DOWN (179880 -> 45000): PostPlant, a drop — the amended rule
    c.observe(509, clockTok(fieldRoundClockMs, 44960))
    ph := c.phases()
    want := []msPhase{{PrepPhase, 20, 44832, 0}, {ActionPhase, 100, 179917, 0}, {PostPlantPhase, 500, 45000, 179917 - 179880}}
    ... compare Phase, Tick, StartMs, BaseMs (BaseMs of PostPlant = elapsed at the last Action sample = 37)
    p, ms, el, ok := c.at(25); if !ok || p != PrepPhase || ms != 44757 || el != -44757 { t.Fatal(p, ms, el) }
    p, ms, el, ok = c.at(109); if p != ActionPhase || ms != 179880 || el != 37 { t.Fatal() }
    p, ms, el, ok = c.at(600); if p != PostPlantPhase || ms != 44960 || el != 37+40 { t.Fatal() }
    if _, _, _, ok := c.at(5); ok { t.Fatal("before first sample") }
}
func TestMsClock_PrepOnly(t *testing.T) {   // Review Focus 2
    c := newMsClock(); c.observe(1, clockTok(fieldRoundTimer, 45))
    c.observe(2, clockTok(fieldRoundClockMs, 45000)); c.observe(9, clockTok(fieldRoundClockMs, 44000))
    if len(c.phases()) != 1 { t.Fatal() }
    if _, _, el, _ := c.at(9); el != -44000 { t.Fatal(el) }
}
func TestMsClock_IgnoresOtherObjects(t *testing.T) { ... a fieldRoundClockMs token on a different object before the timer is seen is not a sample }
func TestMsClock_RoundEndResetIsNotASample(t *testing.T) { ... 179917 @2, 179880 @9, 0 @16 → one phase; at(16) = Action, 179880, elapsed 37 }   // amended 2026-10-20
func TestMsClock_PlantAboveCountdown(t *testing.T) { ... 100035 @2, 100000 @10, 45000 @20 → two phases, phases()[1] == {PostPlant, 20, 45000, 35} }
func TestMsClock_Fixtures(t *testing.T) { ... early_plant PostPlant {12147, 44977, 55881}; solid_snake {17517, 44946, 100895}; fortress {45872, 44938, 167292}; clubhouse/lair exactly two phases }
```

- [ ] **Step 2: Run to verify failure** — `go test ./dissect/ -run TestMsClock` → `undefined: newMsClock`.

- [ ] **Step 3: Implement `msclock.go`**, add the two fields and `addUpdate`, and replace the eleven append sites (`defuse.go` 228, 270, 363, 412, 447; `feedback.go` 139, 200, 236; `player.go` 225, 263; `time.go` 295).

- [ ] **Step 4: Run the clock tests and the goldens**

Run: `go test ./dissect/ -run TestMsClock -v 2>&1 | tail -6 && go test ./dissect/test/ 2>&1 | tail -3`
Expected: PASS; goldens unchanged (`ok`).

- [ ] **Step 5: Commit**

```bash
git add dissect/msclock.go dissect/msclock_test.go dissect/feedback.go dissect/defuse.go dissect/player.go dissect/time.go dissect/scoreboard.go
git commit -m "feat: millisecond round clock and event buffer offsets"
```

---

### Task 3: Timeline assembly in `Reader`

**Files:**
- Create: `dissect/timeline.go`
- Create: `dissect/test/timeline_test.go` (package `test`)
- Modify: `dissect/reader.go:239-290` (`Read`: build the timeline before `r.b = nil`), `dissect/reader.go:20-45` (Reader fields)
- Modify: `dissect/match.go:53-76` (`read`: enable per round), add `Rounds()`/`Paths()`

**Interfaces:**
- Consumes: `wire.Walk`, `wire.DecodeTLV`, `wire.DecodeMovement`, `wire.NewGraph`, `wire.HandleOf`, `msClock`, `MatchUpdate.offset/derived`, `r.inventory.players` (player object handle → username, loadout.go), `r.defuserPawnToSlot` + `Player.slotID` (pawn → player, defuse.go).
- Produces:

```go
type Timeline struct {
    Version  int                `json:"version"`            // 1
    Clock    TimelineClock      `json:"clock"`
    Entities []TimelineEntity   `json:"entities"`
    Events   []TimelineEvent    `json:"events"`
    Positions *TimelinePositions `json:"positions,omitempty"` // nil in this plan
    Coverage TimelineCoverage   `json:"coverage"`
}
type TimelineClock struct {
    TickFirst uint32 `json:"tickFirst"`
    TickLast  uint32 `json:"tickLast"`
    Phases    []TimelinePhase `json:"phases"`
}
type TimelinePhase struct { Phase RoundPhase `json:"phase"`; Tick uint32 `json:"tick"`; RoundClockMs uint32 `json:"roundClockMs"` }
type TimelineEntity struct { ID string `json:"id"`; Kind string `json:"kind"`; Owner string `json:"owner,omitempty"`; Class string `json:"class"`; Status string `json:"status"` } // Kind: "player" | "pawn"
type TimelineEvent struct {
    Seq int `json:"seq"`; Tick uint32 `json:"tick"`; ElapsedMs int64 `json:"elapsedMs"`; RoundClockMs uint32 `json:"roundClockMs"`
    Phase RoundPhase `json:"phase"`; Type MatchUpdateType `json:"type"`
    Actor string `json:"actor,omitempty"`; Target string `json:"target,omitempty"`
    Data map[string]any `json:"data,omitempty"`   // headshot, teamKill, operator (as in MatchUpdate), message
    Status string `json:"status"`                 // "observed" | "derived"
}
type TimelinePositions struct { SampleHz int `json:"sampleHz"`; Samples []any `json:"samples"` }
type TimelineCoverage struct {
    UnplacedFraction float64 `json:"unplacedFraction"`; TLVErrors int `json:"tlvErrors"`; TLVTokens int `json:"tlvTokens"`
    MovementRecords int `json:"movementRecords"`; MovementBadFraction float64 `json:"movementBadFraction"`
}
type TimelineError struct { Reason string `json:"reason"`; Coverage *TimelineCoverage `json:"coverage,omitempty"` }
var ErrTimelineDisabled = errors.New("dissect: timeline not enabled; call EnableTimeline before Read")

// EnableTimeline makes Read build the timeline before releasing the buffer. Must be called before Read.
func (r *Reader) EnableTimeline()
// Timeline returns the timeline built by Read, or (nil, *TimelineError) when building failed, or ErrTimelineDisabled.
func (r *Reader) Timeline() (*Timeline, error)
func (m *MatchReader) EnableTimeline()          // applies to every round reader created by read(i)
func (m *MatchReader) Rounds() []*Reader         // after Read
func (m *MatchReader) Paths() []string
```

Assembly rules (`buildTimeline(r *Reader) (*Timeline, *TimelineError)`, called from `Read` after `applyLoadouts` and before `r.b = nil`):
1. `l, err := wire.Walk(r.b)`; if `err != nil` or `len(l.ChannelsOf(wire.PayloadTLV)) == 0` → `TimelineError{Reason: "no TLV channels (legacy or unsupported layout)"}`; then, if `r.Header.CodeVersion < Y11S2`, `TimelineError{Reason: "unsupported version …"}` (design §5.4, amended by the integration review).
2. One pass of `wire.DecodeTLV` feeding `msClock.observe` and `wire.Graph.Add`; one `wire.DecodeMovement` pass for coverage only. Any `DecodeTLV` error (not per-block `Errors`) → `TimelineError`.
3. Envelope index: a sorted slice of `(Start, End, Tick)` over all TLV envelopes; `tickAt(offset)` = binary search; an offset inside no envelope takes the tick of the nearest earlier envelope; `offset == -1` takes `TickLast`.
4. Events: for `i, u := range r.MatchFeedback`: `tick = tickAt(u.offset)`; `phase, ms, elapsed, ok = clock.at(tick)`; if `!ok` (event before the first sample) use `ms = 0`, `elapsed = int64(u.ElapsedSeconds) * 1000`. **Phase is `u.Phase` (legacy), never the clock's.** When the clock's phase at `tick` differs from `u.Phase` (Review Focus 1, plant in the jump frame), recompute `elapsed` with the previous phase's base: `elapsed = prevPhase.BaseMs + int64(prevPhase.StartMs) - int64(clock.sampleBefore(tick-1))`. `Data` carries `headshot`, `teamKill`, `operator`, `message`, `dbnoBy`/`finishedBy`/`bleedOut` only when set. `Status = "derived"` if `u.derived`, else `"observed"`.
5. Entities: for each `(handle, p)` in `r.inventory.players`: `{ID: wire.HandleOf(handle).String(), Kind: "player", Owner: p.username, Class: graph class or "", Status: "observed"}`; for each `(pawn, slot)` in `r.defuserPawnToSlot` with a matching `Player.slotID`: `{Kind: "pawn", Owner: username}` where the pawn's `wire.Handle` is its 4-byte id followed by four zero bytes (`copy(h[:4], pawn[:])`). Sort by ID.
6. Clock: `TickFirst`/`TickLast` from the TLV envelopes; `Phases` from `clock.phases()`.
7. Coverage from the `Walk`/`DecodeTLV`/`DecodeMovement` stats.

- [ ] **Step 1: Write `dissect/test/timeline_test.go`**

```go
func TestTimeline_Parity(t *testing.T) {
    for _, path := range fixtures(t, "Y11S2", "Y11S3") {   // same walker style as reader_test.go
        t.Run(path, withFile(path, func(f *os.File, t *testing.T) {
            r, _ := dissect.NewReader(f); r.EnableTimeline()
            if err := r.Read(); !dissect.Ok(err) { t.Fatal(err) }
            tl, err := r.Timeline(); if err != nil { t.Fatal(err) }
            if tl.Version != 1 { t.Fatal() }
            if len(tl.Events) != len(r.MatchFeedback) { t.Fatalf("events %d != feedback %d", len(tl.Events), len(r.MatchFeedback)) }
            var lastTick uint32
            for i, e := range tl.Events {
                u := r.MatchFeedback[i]
                if e.Seq != i || e.Type != u.Type || e.Actor != u.Username || e.Target != u.Target || e.Phase != u.Phase { t.Errorf("event %d: %+v vs %+v", i, e, u) }
                if d := float64(e.ElapsedMs)/1000 - u.ElapsedSeconds; d < -1.5 || d > 1.5 { t.Errorf("event %d elapsed %d ms vs %.0f s", i, e.ElapsedMs, u.ElapsedSeconds) }
                if e.Tick < lastTick { t.Errorf("event %d tick %d < %d", i, e.Tick, lastTick) }   // Review Focus 4
                lastTick = e.Tick
                if u.Type == dissect.Kill && e.Status != "observed" { t.Errorf("kill %d status %s", i, e.Status) }
                if hs, ok := e.Data["headshot"]; u.Headshot != nil && (!ok || hs != *u.Headshot) { t.Errorf("event %d headshot", i) }
            }
            players := 0; for _, en := range tl.Entities { if en.Kind == "player" && en.Owner != "" { players++ } }
            if players != 10 { t.Errorf("player entities = %d", players) }
            if len(tl.Clock.Phases) < 2 || tl.Clock.Phases[0].Phase != dissect.PrepPhase || tl.Clock.Phases[1].Phase != dissect.ActionPhase { t.Errorf("phases %+v", tl.Clock.Phases) }
            if tl.Coverage.TLVErrors != 0 || tl.Coverage.UnplacedFraction > 0.005 { t.Errorf("coverage %+v", tl.Coverage) }
        }))
    }
}
func TestTimeline_Disabled(t *testing.T) { ... Read without EnableTimeline → Timeline() returns ErrTimelineDisabled }
func TestTimeline_LegacyLayout(t *testing.T) {   // Review Focus 3
    r := open Y8S1 fixture; r.EnableTimeline(); Read; tl, err := r.Timeline()
    var te *dissect.TimelineError; if tl != nil || !errors.As(err, &te) || te.Reason == "" { t.Fatal(err) }
}
func TestTimeline_PlantPhase(t *testing.T) {    // Review Focus 1 on the real plant fixture
    custom_fortress_plant.rec: the DefuserPlantComplete event has Phase == ActionPhase and the first Kill after it has Phase == PostPlantPhase;
    ElapsedMs of the plant <= ElapsedMs of that kill.
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./dissect/test/ -run TestTimeline` → `r.EnableTimeline undefined`.

- [ ] **Step 3: Implement `timeline.go` and the `Reader`/`MatchReader` changes** per the assembly rules. `Read()` change: after `r.applyLoadouts()` add `if r.timelineEnabled { r.timeline, r.timelineErr = buildTimeline(r) }` before `r.b = nil`.

- [ ] **Step 4: Run the timeline tests and the goldens**

Run: `go test ./dissect/test/ -run 'TestTimeline|TestReader_Read' -v 2>&1 | tail -20`
Expected: PASS; goldens unchanged.

- [ ] **Step 5: Full suite, commit**

```bash
go vet ./... && go test ./... 2>&1 | tail -6
git add dissect/timeline.go dissect/reader.go dissect/match.go dissect/test/timeline_test.go
git commit -m "feat: build round timeline with ticks, ms clock, entities and coverage"
```

---

### Task 4: `--timeline <dir>` sidecar in the CLI

**Files:**
- Modify: `main.go:55-70` (flag), `main.go:135-175` (`writeMatch`, `writeRound`)
- Create: `main_test.go` (package `main`)
- Modify: `README.md` (Timeline section with the JSON shape from spec §5.4 and the failure mode)

**Interfaces:**
- Consumes: `Reader.EnableTimeline/Timeline`, `MatchReader.EnableTimeline/Rounds/Paths`, `TimelineError`.
- Produces:

```go
// main.go
// pflag.String("timeline", "", "write a <round>.timeline.json sidecar per round into this directory")
func writeRound(in *os.File, out io.Writer, timelineDir string) error
func writeMatch(in *os.File, format OutputFormat, out io.Writer, timelineDir string) error
// sidecarPath returns filepath.Join(dir, strings.TrimSuffix(filepath.Base(recPath), filepath.Ext(recPath)) + ".timeline.json")
func sidecarPath(dir, recPath string) string
// writeSidecar marshals tl with json.NewEncoder (no indent) to sidecarPath(dir, recPath), creating dir with 0o755.
func writeSidecar(dir, recPath string, tl *dissect.Timeline) error
```

Round JSON: the anonymous `output` struct in `writeRound` (and the `round` struct in `MatchReader.Data()`) gains `TimelineError *dissect.TimelineError \`json:"timelineError,omitempty"\``, set only when `--timeline` was given and `Timeline()` returned a `*TimelineError`. Excel output ignores the flag. `in` for `writeRound` needs the file name: it is an `*os.File` already (`in.Name()`).

- [ ] **Step 1: Write `main_test.go`**

```go
func TestWriteRound_Sidecar(t *testing.T) {
    dir := t.TempDir(); f := openFixture(t, "dissect/test/data/replays/valid/Y11S3/custom_clubhouse_short.rec")
    var out bytes.Buffer
    if err := writeRound(f, &out, dir); err != nil { t.Fatal(err) }
    var round map[string]any; json.Unmarshal(out.Bytes(), &round)
    if _, has := round["timelineError"]; has { t.Fatal(round["timelineError"]) }
    if _, has := round["timeline"]; has { t.Fatal("timeline must not be embedded") }
    b, err := os.ReadFile(filepath.Join(dir, "custom_clubhouse_short.timeline.json")); if err != nil { t.Fatal(err) }
    var tl dissect.Timeline; if json.Unmarshal(b, &tl) != nil || tl.Version != 1 || len(tl.Events) == 0 { t.Fatal() }
}
func TestWriteRound_LegacyGetsTimelineError(t *testing.T) {   // Review Focus 3
    dir := t.TempDir(); f := openFixture(t, "dissect/test/data/replays/valid/Y8S1/<first .rec>")
    var out bytes.Buffer
    if err := writeRound(f, &out, dir); err != nil { t.Fatal(err) }   // exit 0 path
    var round map[string]any; json.Unmarshal(out.Bytes(), &round)
    if te, ok := round["timelineError"].(map[string]any); !ok || te["reason"] == "" { t.Fatal(round["timelineError"]) }
    if entries, _ := os.ReadDir(dir); len(entries) != 0 { t.Fatal("no sidecar on error") }
}
func TestWriteRound_NoFlagNoField(t *testing.T) { ... writeRound(f, &out, "") → no "timelineError" key even for the Y8S1 fixture }
func TestWriteMatch_Sidecars(t *testing.T) {     // Review Focus 5
    dir := t.TempDir(); folder := copy the three Y11S3 fixtures into t.TempDir() as a match folder
    writeMatch(openDir(folder), JSON, &out, dir)
    entries := os.ReadDir(dir) → exactly 3 files named <fixture base>.timeline.json; match JSON has "rounds" with 3 entries and no "timeline"
}
```

- [ ] **Step 2: Run to verify failure** — `go test . -run TestWrite` → `too many arguments in call to writeRound`.

- [ ] **Step 3: Implement** the flag, the signatures, `sidecarPath`, `writeSidecar`, and the `timelineError` fields.

- [ ] **Step 4: Run the tests** — `go test . -v -run TestWrite 2>&1 | tail -10` → PASS.

- [ ] **Step 5: End-to-end check on a real folder**

Run: `go run . --timeline /tmp/tl "/mnt/c/Users/prest/Downloads/TEST REPLAYS/Match-2026-10-18_18-19-54-14012" -o /tmp/tl/match.json && ls /tmp/tl && python3 -c "import json;d=json.load(open('/tmp/tl/Match-2026-10-18_18-19-54-14012-R01.timeline.json'));print(d['clock']);print([ (e['type']['name'],e['actor'],e['tick'],e['elapsedMs']) for e in d['events']][:6])"`
Expected: seven `*.timeline.json` files plus `match.json`; R01 shows phases Prep/Action/PostPlant and the first kill (`FallacyAsTR.NKG`, tick 25743) with `elapsedMs` ≈ 73 000.

- [ ] **Step 6: Document** `--timeline` in README (usage, sidecar name, the JSON shape from spec §5.4 reduced to what this plan emits, `timelineError` semantics, statuses).

- [ ] **Step 7: Full suite, commit**

```bash
go vet ./... && go build ./... && go test ./... 2>&1 | tail -6
git add main.go main_test.go README.md dissect/match.go
git commit -m "feat: --timeline writes a per-round timeline sidecar"
```

---

### Task 5: `dissect-lab join` and `graph`

**Files:**
- Modify: `cmd/dissect-lab/commands.go`, `cmd/dissect-lab/main.go` (dispatch, usage)
- Modify: `cmd/dissect-lab/main_test.go`

**Interfaces:**
- Consumes: `Reader.EnableTimeline/Timeline`, `wire.Graph`.
- Produces:

```
dissect-lab join <round.rec>     one JSON object per timeline event: {"seq":0,"type":"Kill","actor":"…","target":"…","tick":25743,"elapsedMs":73462,"roundClockMs":106538,"phase":"Action","status":"observed"}
dissect-lab graph <round.rec>    one JSON object per edge, in tick order: {"tick":1,"owner":"a07305f0…","field":"27c08dca","target":"303b04f0…","class":"b2216bf3","index":0,"hasIndex":false,"ownerPlayer":"NickB2A"}
                                 ownerPlayer is the username when Graph.Ancestor reaches a player object (from the timeline's entity table), else omitted
```

- [ ] **Step 1: Add tests** `TestRun_Join` (number of lines equals `len(matchFeedback)` of the same fixture parsed with `dissect`, every line has `tick` and `elapsedMs`) and `TestRun_Graph` (≥ 10 lines with `"field":"27c08dca"` and an `ownerPlayer`).

- [ ] **Step 2: Run to verify failure** — `go test ./cmd/dissect-lab/ -run 'TestRun_Join|TestRun_Graph'` → unknown command exit code 2.

- [ ] **Step 3: Implement** both commands (`join` enables the timeline on a `dissect.Reader`; `graph` re-runs `wire.DecodeTLV` into a `wire.Graph` and labels owners through the timeline entity table).

- [ ] **Step 4: Run the tests** — `go test ./cmd/dissect-lab/ -v 2>&1 | tail -12` → PASS.

- [ ] **Step 5: README** — add the two commands to the Research CLI section.

- [ ] **Step 6: Full suite, commit, push**

```bash
go vet ./... && go test ./... 2>&1 | tail -6
git add cmd/dissect-lab README.md
git commit -m "feat(dissect-lab): join and graph commands"
git push origin fork/integration
```

---

### Follow-ons (not in this plan)

- R6Stats ingestion of the sidecar (`round_timeline_event`, `elapsedMs`) is a follow-on wave on the R6Stats repo once the phase-1 waves land (spec §7).
- Replacing legacy listeners with field lookups, the catalog/verify tooling (M3) and movement decoding (M4) are separate plans; their interfaces start from `wire.Token`, `wire.Graph` and `Timeline` as defined here.
