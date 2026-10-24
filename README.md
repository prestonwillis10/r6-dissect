# r6-dissect
[![](https://discordapp.com/api/guilds/936737628756271114/widget.png?style=shield)](https://discord.gg/XdEXWQZZAa)
[![Go Reference](https://pkg.go.dev/badge/github.com/redraskal/r6-dissect.svg)](https://pkg.go.dev/github.com/redraskal/r6-dissect)

Match Replay API/CLI for Rainbow Six: Siege's Dissect (.rec) format.

Supports replays from **Y7S1 through Y11S2** (and counting).

Download the latest version here: https://github.com/redraskal/r6-dissect/releases

## Features
- **Match Info** — Game version, map, gamemode, match type, teams, players, operators
- **Match Feedback** — Kills, teamkills, headshots, objective locates, defuser plants/disables, BattlEye bans, DCs
- **Round Timeline** — Every event has a round phase and a monotonic time since the action phase started
- **Scoreboard** — Per-player kills, assists, deaths, and scores with kill/death reconciliation
- **Output Formats** — JSON or Excel (.xlsx)

### See roadmap at https://github.com/users/redraskal/projects/1.

## CLI Usage
Print a match overview by specifying a match folder or .rec file:
```bash
r6-dissect --info Match-2026-03-26_21-19-17-21932
# or
r6-dissect --info Match-2026-03-26_21-19-17-21932-R01.rec
```
Export round stats to a JSON file:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932-R01.rec -o round.json
```
Example output (abbreviated):
```json
{
  "gameVersion": "Y11S1_Alpha03",
  "codeVersion": 9578674,
  "timestamp": "2026-03-25T23:16:52Z",
  "matchType": { "name": "Ranked", "id": 2 },
  "map": { "name": "KafeDostoyevskyY10", "id": 413845419788 },
  "gamemode": { "name": "Bomb", "id": 327933806 },
  "teams": [
    { "name": "YOUR TEAM", "score": 1, "won": true, "winCondition": "KilledOpponents", "role": "Attack" },
    { "name": "OPPONENTS", "score": 0, "won": false, "role": "Defense" }
  ],
  "players": [
    {
      "id": 1830934665040226621,
      "username": "IanFiftyForty",
      "teamIndex": 0,
      "operator": { "name": "Oryx", "id": 104189664155 },
      "loadout": { "primary": 53995319912, "secondary": 127174503972, "gadget": 127363389074, "ability": 201523838476 }
    }
  ],
  "matchFeedback": [
    {
      "type": { "name": "Kill", "id": 0 },
      "username": "ReithYT",
      "target": "Ambatakum.",
      "headshot": false,
      "time": "1:51",
      "timeInSeconds": 111,
      "phase": "Action",
      "elapsedSeconds": 68
    }
  ]
}
```
Loadouts (Y8S1+) are the game's asset IDs for the primary/secondary weapon, secondary gadget and operator ability at the end of the preparation phase. IDs are stable across seasons and shared by every operator who can equip the item; names are not resolved.

Export the entire match:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932 -o match.json
```
Export an Excel spreadsheet:
```bash
r6-dissect Match-2026-03-26_21-19-17-21932 -o match.xlsx
```
Output JSON to stdout:
```bash
# entire match
r6-dissect Match-2026-03-26_21-19-17-21932
# single round
r6-dissect Match-2026-03-26_21-19-17-21932-R01.rec
```

See example outputs in [/examples](https://github.com/redraskal/r6-dissect/tree/main/examples).

### Player fields
| Field | Description |
|---|---|
| `spawn` | The attacker spawn location. For defenders, this is the defended site. |
| `startingRoom` | The room a defender was in when the round started (Y9S1–Y11S2; Y11S3 replays no longer contain per-player room names). |

### Match feedback fields
| Field | Description |
|---|---|
| `time` / `timeInSeconds` | The in-game timer when the event happened. The timer restarts its countdown in every phase, so after a plant it shows the defuser timer, and when a round ends early it reads 0:00 for the events that end the round. |
| `phase` | `Prep`, `Action` or `PostPlant` (after the defuser was planted). |
| `elapsedSeconds` | Seconds since the action phase started. Always increases during a round; negative during the preparation phase. Use this to order or compare events. |
| `teamKill` | The kill was a teamkill. Teamkills count as a death for the victim but not as a kill. |

On Y10S4+ replays, the defuser planter/disabler is read from the defuser packets; if that player cannot be resolved, they are inferred from the +100 score bonus or the last player alive. A `DefuserDisableComplete` is added when defense wins a round after a plant without one being recorded. On Y10S4+ replays, the killer of a kill can be corrected from the scoreboard when the kill feed and the scoreboard disagree.

## Timeline

`--timeline <dir>` writes one sidecar per round next to the normal output. The round or match JSON is unchanged.

```bash
r6-dissect --timeline out/ Match-2026-10-18_18-19-54-14012 -o match.json
# out/Match-2026-10-18_18-19-54-14012-R01.timeline.json, one per round file
```

The sidecar is named after the `.rec` file without its extension plus `.timeline.json`, and `<dir>` is created if missing. It is compact JSON. Excel output ignores the flag.

```jsonc
{
  "version": 1,
  "clock": { "tickFirst": 7, "tickLast": 47357,
             "phases": [{ "phase": "Prep", "tick": 44, "roundClockMs": 44832 }] },
  "entities": [{ "id": "<16 hex>", "kind": "player" | "pawn", "owner": "<username>", "class": "<8 hex or empty>", "status": "observed" }],
  "events": [{ "seq": 0, "tick": 25743, "elapsedMs": 73379, "roundClockMs": 106538, "phase": "Action",
               "type": { "name": "Kill", "id": 0 }, "actor": "...", "target": "...",
               "data": { "headshot": true }, "status": "observed" }],
  "coverage": { "unplacedFraction": 0, "tlvErrors": 0, "tlvTokens": 0, "movementRecords": 0, "movementBadFraction": 0 }
}
```

- `events` has one entry per `matchFeedback` entry, in the same order, with the same `type`, `actor`/`target` and `phase`. `seq` is the index into `matchFeedback`.
- `tick` is the replay tick of the packet the event was read from. `elapsedMs` is the time since the action phase began (negative during prep), from the millisecond round clock; `elapsedSeconds` in the round JSON is unchanged.
- `status` is `observed` for events read from a packet and `derived` for events the reader inferred, such as a defuser disable at round end or a defuser actor resolved from the score bonus. A derived event at round end takes the last tick.
- `phases` lists the clock phases (`Prep`, `Action`, `PostPlant`) with the tick and clock value where each starts.
- `entities` lists the player objects and the defuser pawns that resolve to a player (`owner`).

### timelineError

The timeline never fails a parse. When it cannot be built, for example on replays older than Y11S2, no sidecar is written and the round JSON (and the round entry in the match JSON) gains a `timelineError` with a `reason`. The CLI still exits 0 when the timeline cannot be built; only an output error (an unwritable sidecar directory, a failed write) fails the run. When decoder coverage voids the timeline (more than 0.5 % of the bytes left unplaced by the segmenter, or a TLV or movement decode failure), it also carries `coverage` with the sidecar's coverage fields. A round read stopped by a listener error gives the reason `round read stopped: <error>`; a replay older than Y11S2 whose layout walks gives `unsupported version …`. The sidecar directory must be writable; a failure to create it or to write a sidecar is an output error that fails the run like any other output error (it is not a parse failure). The field is only present when `--timeline` was given.

```json
"timelineError": { "reason": "no TLV channels (legacy or unsupported layout)" }
```

## Importing a .rec file
```go
package main

import (
	"log"
	"os"

	"github.com/redraskal/r6-dissect/dissect"
)

func main() {
	f, err := os.Open("Match-2026-03-26_21-19-17-21932-R01.rec")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r, err := dissect.NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	// Use r.ReadPartial() for faster reads with less data
	// dissect.Ok(err) returns true if the error only pertains to EOF (read was successful)
	if err := r.Read(); !dissect.Ok(err) {
		log.Fatal(err)
	}
	print(r.Header.GameVersion) // Y11S1_Alpha03
}
```

## Shared Library (FFI)
Each release includes `libr6dissect` for Linux and Windows, for use from other languages via FFI.
```c
char* dissect_read(char* path); // returns {"data": ...} or {"error": "..."} as JSON
void dissect_free(char* ptr);   // release the string returned by dissect_read
```
Example with Python's `ctypes`:
```python
import ctypes, json
lib = ctypes.CDLL("./libr6dissect-linux-amd64.so")
lib.dissect_read.restype = ctypes.c_void_p
lib.dissect_read.argtypes = [ctypes.c_char_p]
lib.dissect_free.argtypes = [ctypes.c_void_p]
ptr = lib.dissect_read(b"Match-2026-03-26_21-19-17-21932")
result = json.loads(ctypes.string_at(ptr))
lib.dissect_free(ptr)
```

## Research CLI (`dissect-lab`)
`dissect-lab` is a research tool for the `.rec` wire format. It opens one round file, decompresses it, and prints what the byte-level decoders in `dissect/wire` find, one JSON object per line. It is built from `cmd/dissect-lab` with `go run ./cmd/dissect-lab <command> <round.rec>`, is not part of releases, and does not change the per-round JSON.

| Command | Output |
|---|---|
| `segments <round.rec>` | One line per channel, one per unplaced gap, then a summary with `unplacedFraction`. |
| `tlv <round.rec> [--limit N]` | One line per TLV token with tick, offset, op, object, field and value or target. |
| `movement <round.rec> [--limit N] [--no-payload]` | One line per movement record with tick, entity, tag and payload hex. |
| `coverage <round.rec>` | A single line with layout, TLV and movement statistics. |
| `join <round.rec>` | One line per timeline event with seq, type, actor, target, tick, elapsed and round-clock milliseconds, phase and status. Fails on layouts the timeline cannot read. |
| `graph <round.rec>` | One line per ownership edge in tick order, with `ownerPlayer` set when the owner belongs to a player. Fails on layouts the timeline cannot read. |

Exit codes are 0 on success, 2 on a usage error and 1 on a runtime error. A failed run prints one line to stderr and nothing to stdout.

The byte grammar is documented in `docs/research/2026-10-19-rec-format-findings.md`; measured decoder coverage on the fixtures is in `docs/wire-coverage.md`.

## Updating Operator Data
When new operators are added to the game, regenerate the operator roles mapping:
```bash
go generate ./dissect/...
```
This scrapes the current operator list from Ubisoft's website and updates `dissect/operator_roles.go`.

The tests comparing our operators with Ubisoft's website only run when `R6_DISSECT_NETWORK_TESTS=1` is set, and weekly in the Operators workflow.

## Tests
Each `.rec` file in `dissect/test/data/replays/valid` is parsed and compared with the `.rec.json` file next to it. After an intended change in output, regenerate them with:
```bash
go test ./dissect/test -run TestReader_Read -update
```

## Contributing
Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for how pull requests are reviewed and merged.

#
I would like to thank [stnokott](https://github.com/stnokott) for their work on r6-dissect, along with [draguve](https://github.com/draguve) & other contributors at [draguve/R6-Replays](https://github.com/draguve/R6-Replays) for their additional reverse engineering work.
