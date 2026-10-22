# Replay timeline framework: design (draft 2026-10-19)

Status: **approved in substance on 2026-10-19** (Preston answered the §9 questions; answers are recorded there). Items marked *(my call)* are decisions I made that can still be overridden.

## 1. Intent

Turn Rainbow Six Siege match replays (`.rec`) into a complete, time-stamped record of what every player did in a round, so R6Stats can quantify operator performance beyond the kill/death/plant scoreboard that siege.gg and r6replaystats show. Today the fork of r6-dissect recovers a fixed list of 13 event types by scanning the whole decompressed replay for known byte signatures. The research in `docs/research/2026-10-19-rec-format-findings.md` shows the replay is actually three structured streams with a shared tick counter and a millisecond round clock, which makes a full "database timeline" of state changes achievable: every property update, every entity transform, with the object it belongs to and the player who owns it.

**Why now.** R6Stats phase 1 (`docs/superpowers/specs/2026-10-18-r6stats-design.md`) is being built on the fork's current JSON. Its stats floor is "everything r6replaystats shows + everything r6-dissect emits". The ceiling of the product is the parser. Structural decoding is the only route to more data, and it also fixes the fragility (hard-coded skips, duplicate kill packets, inferred defuser attribution) that the fork has accumulated.

## 2. Goals

1. **Structural decoding** of Y11S2+ replays: split the decompressed stream into its regions, parse the TLV property stream into tokens and the movement stream into entity records, and attribute every token to an object, class, tick and millisecond clock. Coverage is measured, not assumed.
2. **A research workflow that turns unknown fields into named semantics** with recorded evidence, so that adding "health" or "ammo" to the output is a repeatable, verifiable process rather than a one-off reverse-engineering session.
3. **A timeline output contract** consumed by R6Stats: ordered events with `tick`, `elapsedMs`, `phase`, actor and target attribution, plus entity positions when available. The existing r6-dissect JSON stays byte-compatible (the timeline is additive).
4. **New per-round facts** that siege.gg does not have, delivered in this order of confidence: exact event timing (ms) → player positions and derived spatial facts (site presence, entry, rotation, trade distance) → damage/health, shots/ammo, gadget deploy/destroy once their fields are verified.
5. **Regression safety**: golden fixtures per season; the structural path must reproduce the current `matchFeedback` and `stats` exactly on the fixtures before it replaces any listener.

## 3. Non-goals and anti-goals

- **Not** a replay viewer or 3D renderer. Positions are data, not pictures.
- **Not** anti-cheat analytics (the PR #126 `anticheat.go` stays removed). SiegeWatch exists for that.
- **Not** support for Y7–Y10 replays in the structural decoder *(my call)*: the legacy signature path keeps serving them; CollegiateR6 only produces current-season replays.
- **Anti-goal:** emitting an inferred meaning as if it were a fact. Every field name in the output carries a status (`observed`, `derived`, `inferred`) and R6Stats treats anything below `derived` as experimental.
- **Anti-goal:** breaking R6Stats' ingestion. Field additions only; removals or renames need a `timelineVersion` bump.
- **Anti-goal (decided by Preston, 2026-10-19):** analysing the game executable. All research uses the replay bytes we have and in-game ground truth only. Field names are recovered by correlation, never from the binary.

## 4. Constraints

- Parser code is Go, in the fork (`prestonwillis10/r6-dissect`, `fork/integration`), because the R6Stats worker shells out to the `r6-dissect` binary and the fork is canonical. No PRs upstream.
- A round is 75–95 MB decompressed; the full structural decode must stay under a few seconds per round so a BO3 upload (≤ 36 rounds) parses in well under a minute on the worker.
- Fixtures: whole rounds are 7–12 MB each; keep ≤ 3 per season in `dissect/test/data/replays`, and record expected outputs as golden JSON (existing convention). CollegiateR6 usernames in fixtures are acceptable (decided 2026-10-18).
- Available data: Y11S2 (two custom games, spectator-recorded) and Y11S3 (three custom games, player-recorded, 23 rounds) in `~/Downloads/TEST REPLAYS`. Ranked and other modes are not available yet.
- Python is allowed for exploratory analysis but must not be on the production path.

## 5. Architecture

Five layers, all in the fork. Names are mine *(my call)*.

```
.rec ──► 0 container (existing zstd section reader)
      ──► 1 segmenter        dissect/wire: Region{kind,start,end}; kinds: header, initTable, tlvInitial, blockA, movement, blockB, tlvTicks, trailer
      ──► 2 decoders         wire.TLV   : Frame → Block → Token{tick, op, object, field, index, len, value, target, class}
                              wire.Move  : Group → Record{tick, entity, tag, mask, payload} → Transform{pos, rot} (mask schema, learned)
                              wire.Graph : ownership edges → class per object, owner chain → player
      ──► 3 lab CLI          cmd/dissect-lab: segments | tlv | movement | graph | catalog | join | verify | diff | coverage  (JSONL out)
      ──► 4 knowledge base   schema/fields.yaml, schema/classes.yaml, schema/movement-masks.yaml  (hash → name, type, status, evidence)
      ──► 5 timeline         dissect.Timeline built from 2 + 4; emitted by `r6-dissect --timeline`; consumed by R6Stats
```

### 5.1 Segmenter and decoders (layer 1–2)

- Grammar as documented in the findings doc §2.1 and §3–4: the file is a sequence of channels (45-byte header) carrying tick-stamped envelopes (`u32 tick, u32 len, u32 0, payload`); payload formats are TLV, movement, or not yet decoded. The segmenter walks channels and envelopes, sniffs each channel's payload format, and reports every byte range it could not place. Acceptance: ≤ 0.5 % of bytes outside any channel header or envelope on every fixture, and the TLV and movement channels identified.
- Walker semantics (settled in PR #2's review, 2026-10-20): a channel header carries the file's `lead` word and the envelope count (findings §2.1, amended); the walk is count-driven and ends at the 12-byte terminator. The opening header is accepted when its envelopes chain with non-decreasing ticks not above the lead: the whole channel followed by a same-lead header or the terminator when it holds at most `firstHeaderProbe` (64) envelopes, otherwise the first 64 of them (bounded work per candidate keeps the walk linear in time and memory on any input; measured false chains behind header-shaped runs are at most 2 envelopes on legacy buffers). After a break inside a channel the walk resumes only at headers carrying the opening lead, and the skipped bytes are a gap; envelopes are never taken out of a gap. A buffer with no acceptable opening header yields `ErrNoEnvelopes` (legacy Y7–Y8 layouts). Known limit, left to a later plan: a crafted or corrupted prefix that passes the probe fixes a wrong lead and hides the real channels behind one gap (run-log candidate C-RT-5).
- The TLV decoder is strict: an unknown opcode or a token overrunning its block is an error that is counted and reported, never silently skipped. Acceptance: zero unknown opcodes on fixtures.
- The movement decoder splits groups and records without interpreting payloads; `Transform` decoding is gated on `movement-masks.yaml` entries with status ≥ `derived`.
- The graph decoder builds `owner → (field) → child:class` from `1A/1B/1E/1F` tokens over the initial state and the tick stream, and resolves any object to a player through its parent chain.
- The tick clock: each frame carries `tick`; the game-state object's `1837466C` value gives `roundClockMs`; the phase machine from `time.go` is re-expressed on top of these two.

### 5.2 Lab CLI (layer 3)

`dissect-lab` is a research tool, not shipped in releases. Commands emit JSONL so results can be queried with duckdb or pandas:

- `segments <rec>` — region table and coverage.
- `tlv <rec> [--region initial|ticks]` — every token with tick/ms/object/class/owner.
- `movement <rec>` — every record with tick/entity/mask/len; `--decode` adds transforms for known masks.
- `graph <rec>` — object tree rooted at the world object and at each player.
- `catalog <dir>` — across a corpus: per `(field, class)` counts, value types, distinct objects, sample values, first/last tick; per class: object counts and field sets. This is the discovery surface.
- `join <rec>` — known events (kills, plants, scoreboard deltas from the legacy path) aligned to ticks and ms; the ground truth used by `verify`.
- `verify <hypothesis.yaml> <dir>` — runs a hypothesis's checks over a corpus and prints pass rates (see §6).
- `diff <rec-a> <rec-b>` — field/class sets present in one round and not the other (season changes, map-specific objects).
- `coverage <rec>` — which bytes the production parser used and which it did not.

### 5.3 Knowledge base (layer 4)

Three YAML files are the single source of truth for semantics. Example entry:

```yaml
- field: 1837466C
  name: roundClockMs
  type: u32
  class: a51505f0-gamestate      # object class or well-known handle it appears on
  status: observed               # observed | derived | inferred | unknown
  since: Y11S2
  evidence:
    - docs/research/2026-10-19-rec-format-findings.md#round-clock-in-milliseconds
    - lab/verify/roundClockMs-2026-10-19.jsonl   # pass rate 100 % on 23 rounds
  checks:                        # what `verify` runs
    - monotone_decreasing_within_phase
    - matches_timer_field_floor_seconds
```

Every `evidence` entry points at a JSONL file whose records carry the replay's sha256 and the byte offsets the conclusion was read from, the raw bytes next to the normalized value, a `coverage` of `complete | partial | unknown` (how much of the corpus the check actually examined), and a `limitations` list. Partial coverage is reported as partial, never as success.

**Unknowns registry.** `schema/unknowns.yaml` holds the open questions as first-class records, each with an id, the question, `status` (`open | investigating | blocked | contradicted | resolved`), severity, links to supporting and contradicting evidence, the confidence required to close it, and recommended probes. The two first entries are the pawn → player attribution gap and the movement mask schema (findings doc §4). Resolving an unknown requires an evidence file that meets its required confidence; the `verify` command checks that bar.

Statuses, adopted from rea's observation/derivation/inference/unknown vocabulary: `observed` = read directly and reproduced on every fixture; `derived` = computed from observed facts by a stated rule; `inferred` = consistent with data but not verified against ground truth; `unknown` = catalogued only. Promotion requires a `verify` run recorded under `evidence`.

### 5.4 Timeline output (layer 5)

Added to the per-round JSON under `timeline` when `--timeline` is passed *(my call: opt-in until M2 lands in R6Stats, then default)*:

```json
"timeline": {
  "version": 1,
  "clock": { "tickFirst": 44, "tickLast": 47357, "phases": [
      { "phase": "Prep", "tick": 44, "roundClockMs": 44832 },
      { "phase": "Action", "tick": 10910, "roundClockMs": 179917 },
      { "phase": "PostPlant", "tick": 46200, "roundClockMs": 45000 } ] },
  "entities": [ { "id": "5cc804f0", "kind": "pawn", "owner": "NickB2A", "class": "b2216bf3", "status": "inferred" } ],
  "events": [
    { "seq": 12, "tick": 25743, "elapsedMs": 73462, "phase": "Action", "type": "Kill",
      "actor": "FallacyAsTR.NKG", "target": "drew.DD", "data": { "headshot": true }, "status": "observed" },
    { "seq": 13, "tick": 25743, "elapsedMs": 73462, "phase": "Action", "type": "HealthChange",
      "actor": "drew.DD", "data": { "from": 0.31, "to": 0.0 }, "status": "inferred" } ],
  "positions": { "sampleHz": 2, "samples": [ { "entity": "5cc804f0", "tick": 394, "x": -67.2, "y": 85.76, "z": 27.0 } ] },
  "coverage": { "bytesAttributed": 0.997, "unknownOpcodes": 0, "movementRecordsDecoded": 0.0 }
}
```

- `elapsedMs` is derived from `roundClockMs` and the phase table (same semantics as today's `elapsedSeconds`, exact).
- Event types v1: the existing 13. Phase changes are delivered as `clock.phases` (tick and `roundClockMs` per phase), not as `events` entries, because `events` maps one-to-one onto `matchFeedback` (amended 2026-10-20, plan 02 parity rule). Candidates gated on verification: `HealthChange`, `AmmoChange`/`ShotFired`, `GadgetDeploy`, `GadgetDestroy`, `Reinforce`, `Barricade`, `DroneDeploy`, `DroneDestroy`, `SpectateTarget`.
- `positions` are downsampled to 2 Hz in the JSON *(my call)*; full-rate transforms are available through `dissect-lab movement --decode` and are not stored by R6Stats in v1.
- R6Stats stores `events` in a new `round_timeline_event` table (keeps `round_event` for the 13 legacy types), `positions` in `round_position_sample`, and reads `status` to decide what to show.

**Failure mode *(my call)*.** The timeline never fails an upload. If the segmenter attributes less than 99.5 % of bytes, the round read stops before the timeline can be built, or the version is outside the supported range, the round JSON is emitted exactly as today with `timeline` omitted and a `timelineError: { reason, coverage }` field added. Amended 2026-10-20 (PR #8 review): a per-block TLV decode error (an unknown opcode or an overrunning token) does NOT void the timeline on its own — the block is abandoned, the error is counted in `coverage.tlvErrors`, and the attribution gate catches gross corruption; plan 02 rule 2 is the rule. The legacy signature scan keeps running for every version in v1; the structural decoder is a second pass over the identified regions (cheap: ~1 % of the buffer for game state). Replacing a legacy listener with its field lookup happens per listener, behind a flag that flips only after M2 parity holds on every fixture.

**Delivery shape *(my call; Preston expressed no preference)*.** The timeline is a sidecar file per round (`<round>.timeline.json`, written next to the JSON output or into the `-o` directory) rather than embedded in the round object: with 2 Hz positions and verified state events a round grows from tens of KB to 1–2 MB, and R6Stats plan 02 stores the round object verbatim in JSONB. With a sidecar, `rounds.raw` is unchanged, R6Stats loads events and samples into their tables, and keeps the sidecar in object storage.

## 6. Research workflow (how a field becomes a fact)

1. **Catalog**: `dissect-lab catalog` over all available rounds produces the field/class table; new or changed fields show up in `diff` after a season patch.
2. **Hypothesise**: write `lab/hypotheses/<name>.yaml` with the field, the claimed meaning, and checks expressed against ground truth that the legacy path already provides (kills, plants, scoreboard deltas, usernames, operator ids, the recorder's own actions) or against invariants (monotone within phase, resets on respawn, hits 0 at death).
3. **Verify**: `dissect-lab verify` runs the checks over the corpus and writes a JSONL evidence file with per-round pass/fail. Thresholds are part of the hypothesis (e.g. health reaches 0 at the kill tick in ≥ 95 % of deaths over ≥ 50 rounds).
4. **Promote**: on pass, the field moves to `fields.yaml` with `status: derived` (or `observed` when it is read directly) and the evidence path. On fail, the hypothesis file records what was wrong so the next attempt starts from there.
5. **Emit**: the timeline builder reads `fields.yaml`; a field with status `inferred` may be emitted only with `status: inferred` on each event, and R6Stats hides it by default.

**Oracle *(my call)*.** The golden `.rec.json` fixtures are the oracle. When the structural path disagrees with a golden value, it is a bug on whichever side until an evidence file shows which is right; a golden changes only together with that evidence file. `elapsedSeconds` in the legacy JSON keeps today's semantics (R6Stats phase 1 already stores it); exact timing is `elapsedMs` under `timeline` only.

**Ground truth for positions.** Nothing in the fork knows world coordinates, so position hypotheses are verified against facts the replay itself carries: the attacker spawn name and defender site per player at round start (pawns of one team must cluster at tick 1, attackers in up to three clusters), each defender's `startingRoom` and the later room-name updates of the same room object (`site.go`), which must coincide with position jumps between rooms, and the kill tick, at which the victim's pawn stops moving. These checks can take positions to `derived`. Pinning coordinates to a map image (for site-presence heatmaps) needs one calibration round per map where a player walks a known route; that is an optional accelerator, not a gate.

Worked example (first targets):

| Hypothesis | Ground truth to test against | Unlocks |
|------------|------------------------------|---------|
| `1837466C` is the round clock in ms | floor(ms/1000) equals `1F07EFC9` in the same frame; monotone within phase | exact timing of every event |
| the ten always-present movement handles are the player pawns | one stops updating at each kill's tick; tick-1 positions match attacker spawns / defender site | positions, site presence, entry/rotation |
| `5C8373D3` is health | reaches 0 at the owner's death tick; never increases except on revive | damage events, trades by damage |
| `B6F474A3` is magazine ammo | decreases by 1 per shot; jumps up after a reload progress field completes | shots fired, accuracy-adjacent stats |
| Block B envelopes bridge movement handles to TLV objects | a decoded link agrees with the death-tick attribution on every round | robust pawn → player attribution |

## 7. Milestones

| M | Deliverable | Acceptance |
|---|-------------|------------|
| M1 | `dissect/wire` container walker + TLV decoder + movement group decoder; `dissect-lab segments/tlv/movement/coverage`; 3 new Y11S3 fixtures | ≤ 0.5 % of bytes outside channel headers and envelopes, 0 unknown TLV opcodes, ≤ 0.3 % movement bytes skipped on the Y11S3 fixtures **and** the two Y11S2 fixtures already in the repo (the TLV token layout is documented in `loadout.go` as verified back to Y8S1, so older versions are best-effort, not gated); existing tests green |
| M2 | graph + clock + `join`; `--timeline` v1 (13 types with tick/ms; phase changes in `clock.phases`); legacy listeners re-implemented as field lookups behind a flag; R6Stats ingests `elapsedMs` | timeline reproduces `matchFeedback` exactly on all fixtures; ms clock verified |
| M3 | `catalog`, `verify`, `fields.yaml`, `unknowns.yaml`; first promoted semantic fields (health, ammo, gadget state) and their events in the sidecar; R6Stats shows verified events in the round feed and adds the derived per-round facts (damage dealt/taken, shots fired, utility deployed/destroyed) to `round_player` and the stats tables | each promoted field has a recorded verify run meeting its threshold; stats recomputable from stored timeline rows |
| M4 | movement mask schema for position-bearing masks; pawn → player attribution (death-tick + spawn proximity + room-name transitions, Block B if found); `positions` at 2 Hz in the sidecar; R6Stats stores `round_position_sample` | ≥ 95 % of pawn records decoded to positions; attribution agrees across methods on every fixture |
| M5 | spatial facts in the R6Stats stats engine (time to first contact, site presence %, entry attempts, trade distance and windows in ms, drone time) | design §8 of the R6Stats spec updated; stats recomputable from stored rows |

M1 and M2 are independent of any still-unknown semantics and can start immediately. M3 before M4 is Preston's priority (2026-10-19): raw stat numbers before positions. R6Stats work in M2–M5 is appended to the phase-1 wave plan as follow-on waves; it does not modify the waves already in flight on `track/r6stats-phase1`.

## 8. What is left to the builder

- Exact Go package layout inside `dissect/wire`, error types, and how `Reader` adopts the decoders (likely `Reader` keeps its public API and gains `Timeline()`).
- JSONL schemas for lab output; duckdb/pandas helpers under `lab/` are optional.
- Fixture selection (shortest rounds that still contain a plant and a DBNO).
- How `verify` expresses checks (Go functions registered by name is fine; a DSL is not required).
- Downsampling method for positions (nearest sample at 500 ms boundaries is fine).

## 9. Questions for Preston (resolved 2026-10-19)

1. **Game-binary analysis:** no. Only the `.rec` files we have are analysed.
2. **Priority:** both tracks, but raw stat numbers (health, ammo, gadgets) before positions. Milestones reordered accordingly (§7).
3. **Positions in R6Stats:** 2 Hz sampling accepted (Preston had no preference; my call stands).
4. **R6Stats scope:** in scope. The parser changes and the knowledge gained feed directly into R6Stats changes (ingestion, tables, stats engine, UI). Sidecar delivery is my call.

## 10. Decisions already made (my calls, override freely)

- Framework lives in the fork as `dissect/wire` + `cmd/dissect-lab` + `schema/*.yaml`; research notes stay in R6Stats `docs/research/`.
- Structural decoder targets Y11S2+ only; legacy path untouched.
- Statuses `observed / derived / inferred / unknown` and the hypothesis → verify → promote loop are the evidence model (borrowed from morluto/rea).

## 11. What was taken from rea, and what was not

rea (`morluto/rea`) is a general reverse-engineering MCP for executables, apps and captures. Surveyed on 2026-10-19 (its `src/domain/evidence.ts`, `residualUnknown.ts`, `evidenceBundle.ts`, `docs/tool-design.md`, `docs/testing.md`):

- **Taken:** the evidence record shape (content-hash id, `confidence` observed/derived/inferred, raw result kept next to the normalized one, typed locations, limitations, three-state coverage); unknowns as first-class records with a status, severity, required confidence and recommended probes; fixtures built with recorded oracle values and known-answer grading that checks a claim's confidence, not just its value; "unknown coverage is not absence"; never a silent truncated success; one owner per interpretation, consumers reuse parsed facts.
- **Not applicable:** rea has no tooling for opaque data files (its "repeated values" tool is a TShark wrapper for pcap only; `inspect_binary_layout` is ELF-only). The decoders in §5.1 have to be ours.
- **Not used (decided):** rea's Ghidra bridge would recover property names from the executable; binary analysis is out of scope, so field names come from correlation against in-game ground truth only.
- `--timeline` is opt-in until R6Stats consumes it; the existing JSON never changes shape.
- Positions downsampled to 2 Hz in the JSON; no anti-cheat metrics.
- Name `dissect-lab` for the CLI.
- Timeline failures never fail an upload: legacy JSON unchanged, `timeline` omitted, `timelineError` reported; the legacy scan keeps running in v1.
- Golden fixtures are the oracle; `elapsedSeconds` keeps its current semantics; ms timing lives only in `timeline`.
- Positions are verified against spawn/site/room-name/death facts from the replay itself; map calibration is optional.
- The verify corpus is every round R6Stats has ingested (uploads are already retained as originals); only rounds Preston approves are committed as fixtures. Until uploads exist, the 23 Y11S3 + 2 Y11S2 rounds are the corpus and thresholds scale down accordingly.
