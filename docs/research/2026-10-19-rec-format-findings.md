# .rec replay format: structural findings (2026-10-19)

Research notes from probing three Y11S3 rounds (`Match-2026-10-18_18-19-54-14012` R01 and R05, `Match-2026-10-18_21-07-35-14012` R01, all `Y11S3_Alpha04` / code `9950954`, custom games recorded by a player). Everything below was measured with the throwaway probes in `rec-probes/`; nothing here is from documentation. Status tags follow the convention adopted from morluto/rea: **observed** (read directly from the bytes and reproduced on all three files), **derived** (computed from observed facts), **inferred** (consistent with the bytes but not yet verified against ground truth), **unknown**.

## 1. What r6-dissect does today

The fork (`prestonwillis10/r6-dissect`, branch `fork/integration`, head `c521f1a`) decompresses every zstd section of a `.rec`, concatenates them into one buffer, and runs ~20 byte-signature listeners over the whole buffer (`dissect/reader.go` `Listen(...)`). Each listener skips a hard-coded number of bytes and reads a few values. This finds kills, the round timer, defuser timer, scoreboard counters, player packets, loadouts and sites, but it is blind to the stream structure: it does not know where a packet starts or ends, which object a property belongs to, or when (in ticks) a byte was written. The movement research in `research/pr126-extras` (`readPosition`, pattern `60 73 85 FE`) and upstream `feat/movement` (`C0 3F`) scanned for float triples with no entity or time attribution, which is why they were unreliable.

## 2. File layout (observed)

A `.rec` is a `dissect` header followed by zstd sections (Y8S4+). Sections 0–9 are small; from section 10 on they are ~512 KB each and split the inner stream at arbitrary byte positions, so section boundaries carry no meaning. The decompressed stream of R01 (76,179,853 bytes from a 10,059,296-byte file) has seven regions:

| # | Region | Bytes (R01) | Encoding | Content |
|---|--------|-------------|----------|---------|
| 1 | Header + init table | 0 – ~2.6 K | key/value strings, fixed records | `dissect.hexpat`; the init table lists static entity handles (`…46e05c000000`) |
| 2 | Initial TLV state | 2,626 – 83,114 | TLV batches (§3) | world object, 10 player objects with usernames, loadouts, scoreboard init, one timer packet, plus one snapshot group of 207 entity records (tags `61`/`62 73 85 fe`) |
| 3 | Block A | 83,114 – 206,741 | starts `00000000 03b90000 0a000000…` | **unknown** |
| 4 | Movement stream | 206,741 – 66,807,830 (87 %) | tick groups of entity records (§4) | per-tick transforms of ~500 entities |
| 5 | Block B | 66,807,830 – 75,255,571 (8.4 MB) | `03b90000` header, then `[u32 tick][u32 len][u32 0]` envelopes with raw payloads | **unknown**; payloads reference both static handles (`…46e05c`) and movement handles (`33ce04f0`), so this may be the bridge between the two ID spaces |
| 6 | TLV tick stream | 75,255,571 – 76,104,480 (850 KB) | 6,087 framed ticks (§3) | all game-state changes: timer, kills, defuser, scoreboard, pawn state, HUD text |
| 7 | Trailer | 76,104,480 – end (75 KB) | `00*12 03b90000 …` | **unknown** |

Byte totals: regions 4 and 6 together are 99.4 % of the file. The same layout, with the same proportions, holds for R05 (81,379,836 bytes) and the other match's R01 (91,308,739 bytes).

### 2.1 Uniform container (observed, added after the region table)

The regions above are not seven ad-hoc formats. The decompressed stream is a sequence of **channels**, each introduced by a 45-byte header and followed by tick-stamped **envelopes**:

```
channel header := u32 lead  byte  00*7  id8  u32  00  u32 count  00*4  u32  00*8   (45 bytes; `id8` differs per channel)
envelope       := u32 tick  u32 len  u32 0  payload[len]
terminator     := u32 lead  00*8                                                (12 bytes, after the last channel)
```

Amended 2026-10-20 (PR #2 review of the walker, verified on all headers of the five Y11S2/Y11S3 fixtures): the first word is not the constant `03 b9 00 00` but a per-file `lead` that is identical in every header of a file and ≥ every tick in it (observed `lead`/max tick 12565/12561, 19553/19552, 39019/39013, 47363/47358, 38033/38032); byte 4 is `0a`/`0b` on the first channel and zero elsewhere; the u32 at offset 25 is the channel's envelope count + 2; the u32 at offset 33 is non-zero for some channels (`5f87976f`, `26b9c2c1`, `014e3af4`, and `63fe54d3` in one round), so the tail is not `00*17`. A count-driven walk places every byte except the prelude and the 12-byte terminator. Fixtures hold 11/11/10/10/10 channels (early_plant, solid_snake, clubhouse_short, fortress_plant, lair_plant_stop).

Payload formats differ per channel: TLV batches (payload starts `04`, ends with 12 zero bytes, which is why TLV frames looked like `00*12 tick len 0 04 …`), movement groups (`u16 count` + records), and at least two undecoded formats (Block A: 1,025 envelopes, ticks 7 → 47,357, payloads 100–126 bytes starting `09 f3`/`0a eb`; Block B: several channels, the first with 2,452 envelopes, ticks 7 → 47,333). Envelope ticks are monotonic inside every channel probed, so the whole file can be merged into one tick-ordered stream. The world object handle (`ed1505f0` in match 1, `b8442cf0` in match 3) and the game-state object differ per file and must be discovered, while field ids such as `1784e409` (batch), `1F07EFC9` (timer) and `1837466C` (ms clock) are constant.

Key consequence: **all game-state properties r6-dissect reads live in the last ~1 % of the file**, and the 87 % it currently scans byte-by-byte is the movement stream, which it cannot interpret.

## 3. TLV property stream (observed)

Both the initial state (region 2) and the tick stream (region 6) use one grammar.

**Frame (tick stream):**

```
envelope := u32 tick  u32 len  u32 0  0x04  slot slot slot slot   ; exactly four slots (amended 2026-10-20)
slot     := u32 C  bytes(C)                                        ; C may be 0: an empty slot
bytes(C>0) := 0x18 handle8 field4 u32 n  token{n}                  ; batch header, n = tokens that follow; the first slot's batch is on the world object, field 1784e409
```

**Tokens:**

| Op | Layout | Meaning (inferred names) |
|----|--------|--------------------------|
| `22` | field4 len1 payload | value on the current object |
| `23` | handle8 field4 len1 payload | value on a named object; it becomes current |
| `26` | field4 index4 len1 payload | indexed value (array element) on the current object |
| `27` | handle8 field4 index4 len1 payload | indexed value on a named object (predicted, unseen so far) |
| `1A` | field4 handle8 class4 | reference from the current object; the target becomes current |
| `1B` | handle8 field4 handle8 class4 | reference from a named object (`owner.field = target`) |
| `1E` | field4 index4 handle8 class4 | indexed reference (array of objects) |
| `1F` | handle8 field4 index4 handle8 class4 | indexed reference on a named object (predicted, unseen) |
| `18` | handle8 field4 u32 | batch header |

`len` is a byte count, so values are self-describing: `01` bool/u8, `04` u32 or f32, `08` u64 or object handle, `n` string of n bytes (usernames, room names, HUD markup). Handles are 8 bytes (a 32-bit id followed by four zero bytes); a null target is 16 zero bytes. Every Y8S1+ listener pattern in r6-dissect is a 4-byte `field` id in this grammar, sometimes with the opcode or length byte attached (e.g. the round timer `1F07EFC9`, kills `5934E58B`, scoreboard score `ECDA4F80`, username `07949BDC`).

Parsing R01's tick stream with this grammar: 6,087 frames, 22,048 tokens, 206 distinct `(kind, field, len)` keys, 223 objects with a known class, **zero unknown opcodes**. Frame ticks are monotonic (deltas 6–10), 6,087 frames across the ~225 s round ≈ one frame every 35–37 ms.

### Round clock in milliseconds (observed)

Field `1837466C` (u32) on the game-state object `a51505f0` is present in every frame and counts down in ms: 44,832 → 44,757 → 44,720 … while the known timer field `1F07EFC9` on the same object shows 44 → 43. It resets at phase changes (prep 45 s → action 180 s → post-plant 45 s). This gives every event sub-second timing (upstream issue #69) by joining on the frame.

Kill packets joined to their frame (R01): `FallacyAsTR.NKG → drew.DD` at tick 25,743, 106,538 ms left; `Breck.HRC → NickB2A` at tick 27,099, 100,234 ms; the last kill at 0 ms post-plant. The game re-sends each kill packet once (same tick) and also sends the kill-feed HUD markup (`<m-entity tag='DYNAMIC_ICON' …>`) as a string value, which r6-dissect currently sees as a kill with username `ÿÿÿÿÿÿÿÿ` and filters.

### Object graph (observed)

`1A/1B/1E` tokens form an ownership graph. In R01 the initial state yields the 10 player objects (class `2759e897`, e.g. `a07305f0` = NickB2A) and edges such as `player.27C08DCA → pawn (class b2216bf3)`, the inventory chain documented in `loadout.go`, and pawn components. Walking parents from any object reaches a username, so every TLV property update can be attributed to a player.

### Candidate fields in the tick stream (inferred, unverified)

| Field | Where | Observation | Hypothesis |
|-------|-------|-------------|------------|
| `5C8373D3` f32 | class `9e9c89be`, exactly 10 objects | 389 updates, values 1.0 → 0.9949 → 0.9893 … | per-player health fraction (would give damage events) |
| `B6F474A3` u32 | class `c45881f6`, 19 objects | 6,561 updates, values 26, 54, 36, 70 … | per-weapon ammo in magazine (would give shots fired) |
| `C2A17CE9` u32 | class `c348b48b`, 16 objects | small counts | gadget charges |
| `E58C06E9` u32 | pawn | 0/1/2 | defuser action state (already used by `defuse.go`) |
| `55D0E554`, `C84CF4B5`, `CB4F819B` f32 | various | 0 → 1 progressions | reload / ADS / interaction progress |
| `5BE84728` string | 2 objects | usernames, 11 times each | recorder's spectator-camera target after death |
| `A6B10188` u64 | 5 objects | `2461366787` = Blitz roleImage | operator/role asset ids |
| `3EAAD30A` u64 | class `b430ebc4`, 15 objects | values are movement-stream handles | bridge to movement entities (only 25 of 507 handles so far) |

Amended 2026-10-20 (PR #5 review, verified on every TLV envelope of the five Y11S2/Y11S3 fixtures): the "12 zero bytes" trailer described above is three empty slots, "8 zero bytes" are two, and a zero word between blocks is one — every envelope has exactly four length slots (shape counts BZZZ/BBZZ/BBZB/BZZB/ZBZZ: early_plant 2586/22/9/2/0, solid_snake 4359/28/11/0/0, clubhouse 5120/25/10/0/0, fortress 6087/25/10/1/0, lair 5423/24/10/0/1). Opcodes seen on the fixtures: 18, 1a, 1b, 1e, 22, 23, 26; 1f and 27 are unseen. The initial state (region 2) is NOT envelope-framed: it sits in the prelude before the first channel header, and the username field `07949bdc` occurs only there (ten tokens per round), so a channel-only decoder never sees usernames (run-log candidate C-RT-9).

## 4. Movement stream (observed)

```
group  := u32 tick  u32 len  u32 0  u16 count  record*count     ; len counts from `count` to group end
record := handle8  u32 L  tag4  payload(L-4)
tag    := 60|61|62|63 73 85 fe
```

R01: 6,320 groups, 213,613 records, ticks 1 → 47,358 monotonic (deltas 6–10, the same tick counter as the TLV frames), 507 distinct entity handles, 0.2 % of the region skipped by the probe (two resyncs). R05: 6,048 groups, 223,380 records, 559 handles. Other match R01: 6,813 groups, 273,804 records, 520 handles. Ten handles are present from the initial snapshot and update in most groups (5,000–6,100 records each in R01) — hypothesised to be the ten player pawns (unverified; see the attribution gap); another ten appear at ticks ≈10,460–10,490 (around the action-phase start) — likely drones. Amended 2026-10-20 (PR #4 review): "nearly every group" overstates it — on every fixture only 4 entities appear in ≥ 90 % of groups (fortress top-12 record counts 6127, 6126, 6043, 5780, 5575, 5574, 5574, 5316, 5086, 5084, 4902, 4902 of 6320 groups), consistent with pawns that stop updating at death; an entity appears at most once per group.

The payload after the tag begins with a 2–4 byte presence mask (`40 04`, `30 01`, `fc 1f`, `b0 01` …) and the record length varies with the mask (36-byte records with mask `40 04` are the most common, 44 k of them). For some masks a float triple at payload offset 6 is a world position: pawn `5cc804f0` sits at (−67.20, 85.80, 27.00) through ticks 81–385, then moves to (−67.20, 85.36, 27.00) by tick 430 in sub-metre steps. Decoding the mask → field layout is the main open research item for positions.

**Attribution gap (unknown):** none of the 507 movement handles appears as an object handle or edge target in the TLV graph, and the player-pawn movement handles do not appear as `3EAAD30A` values either. Candidate bridges, in order of expected reliability: (a) Block B envelopes, which carry both handle spaces; (b) death-tick correlation (a dead player's pawn stops updating at the kill's tick); (c) spawn-proximity at tick 1 against the known attacker spawn and defender site; (d) the initial snapshot records (tag `61`) that may embed the owner.

## 5. Implications for the parser

1. Replace whole-buffer signature scanning with a segment-aware decoder: parse region 6 with the TLV grammar (every existing listener becomes a `(field, class)` lookup on a decoded token), and parse region 4 as groups. Scanning only region 6 for game state is ~90× less data (850 KB instead of 76 MB).
2. Every event gets `tick` and `roundClockMs` for free; `elapsedSeconds` becomes exact.
3. Object attribution replaces the fragile skip-N-bytes player lookups (`readScoreboardKills`, `defuserPacketPawn`), and the 206 catalogued fields are the discovery surface for new events.
4. Positions become feasible once the movement mask schema and the pawn→player bridge are solved.

## 6. Reproduction

Probes (Python 3, stdlib only) are in `rec-probes/`; each takes a decompressed dump produced by `r6-dissect --dump <round.rec> -o round.bin`:

- `rec_frames.py dump.bin 75000000 catalog.json` — TLV frame/token catalog of the tick stream
- `rec_streams.py dump.bin` — both streams, tick deltas, kill ↔ ms-clock join
- `rec_movement.py dump.bin [brief]` — movement groups, handles, record shapes, position glimpse
- `rec_join.py` / `rec_join2.py dump.bin` — attribution experiments (object graph, u64 bridge)
- `rec_gaps.py`, `rec_groups.py`, `rec_records.py`, `rec_frames2.py`, `tokenize_spike.py` — earlier iterations kept for the record
- `zsections/` — Go tool that splits a raw `.rec` into its zstd sections

These are throwaway spikes, not product code.
