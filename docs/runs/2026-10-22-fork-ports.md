# Run log: fork-port lane (r6-dissect), 2026-10-22

Wave-ship lane that implements the 2026-10-21 fork-survey issues. Repo `prestonwillis10/r6-dissect`.

- **Track:** `track/fork-ports`, cut from `origin/main` at `1437695`. The plans branch `docs/fork-port-plans` (PR #36) is merged in, so the spec is on the track (`1352567`).
- **Spec:** `docs/research/2026-10-21-fork-survey.md` (the index) and `docs/superpowers/plans/2026-10-21-fork-port-NN-*.md`, with prototypes in `docs/research/fork-port-prototypes/`.
- **Companion lane:** R6Stats `track/fork-ports`, run log `docs/runs/2026-10-22-fork-ports.md` in that repo (R6Stats #15–#18).
- **Mode:** autonomous slot merges into the track. The landing PR (track → `main`) is gated on the operator's go, as with the rec-timeline and ci-linux landings.
- **Models:** Sonnet implementers (Opus for #26 and #33, which need judgment beyond the plan). Opus reviewers via `/deduce-pr`, named `rev-<PR#>`.

## Gate zero (2026-10-22)

`origin/main` moved from `bd6ccbe`, where the plans were verified, to `1437695`. That is PR #16 (`docs/contributing`).

```
$ git diff --stat bd6ccbe origin/main
 .github/pull_request_template.md | 21 +++++++++++++++++++
 CONTRIBUTING.md                  | 44 ++++++++++++++++++++++++++++++++++++++++
 README.md                        |  3 +++
```

No Go file changed, so every code anchor the plans cite at `bd6ccbe` holds at `1437695`. The reproductions recorded in the plans (PR #36) carry over unchanged. Every issue is OPEN, and no open PR other than #36 references #17–#35.

| Issue | Claim | Status at 1437695 |
|---|---|---|
| #23/#24 | nil round panics / empty body panics | CONFIRMED (code identical; reproduced in plan 01) |
| #17 | 4 map IDs missing | CONFIRMED (`dissect/header.go` has no `409325881472`) |
| #29 | no Y11S1 / match-level acceptance | CONFIRMED |
| #32 | round buffers kept alive | CONFIRMED (code identical; measured in plan 06) |
| #33 | scanner drops boundary matches | CONFIRMED (latent; plan 05 tests) |
| #34 | 9734089 markers | CONFIRMED (simulated; no real replay exists — `research`) |
| #19 | undecided round → team 1 | CONFIRMED |
| #30 | zero-event round → `matchFeedback: null` | CONFIRMED (reproduced 2026-10-21 on nicktstewart match2 R05) |
| #31 | listener EOF drops rest of round | CONFIRMED |
| #18 | late joiner has no scoreboard row | CONFIRMED |
| #26 | zip mapping shifts after a leaver | CONFIRMED |
| #28 | weapon per kill | CONFIRMED |
| #21/#22 | reinforcements | DEFERRED: needs PR #10 (`track/rec-timeline`) on main |
| #25 | damage taken | DEFERRED: needs PR #3 (`fork/health-dbno`) on main |
| #27 | roleName fallback | DEFERRED: needs PR #3's 10th-player fix (`a17b9f4`) |
| #35 | reconcile pass order | DEFERRED: same function as external PR #12; operator's call on #12 first |

**Policy constraint from PR #16 (`CONTRIBUTING.md`):** "only commit replays you recorded or have permission to share". Plans 07 and 09 propose committing fixtures cut from nicktstewart's replays, which are third-party. Those slots use synthetic unit tests instead. Real-replay acceptance runs locally through #29's env-gated corpus, and nothing from it is committed.

## Waves

The graph follows the shared files. `dissect/reader.go` is the hotspot, edited by nine items. Hunks in distinct functions may run in parallel, and every wave after the first is re-validated against the current track before merge (Hard rule 13).

| Wave | Slot | Issue(s) | Plan | Owns (others must not touch) | After |
|---|---|---|---|---|---|
| W1 | S23 | #23, #24 | 01 | `dissect/match.go` read loop + `Data`/`WriteExcel` guards, `main.go` exit policy, `exports/exports.go`, `dissect/error.go`, the guard at the top of `Reader.Read` | — |
| W1 | S17 | #17 | 04 | `dissect/header.go` Map block, `dissect/map_string.go` | — |
| W1 | S32 | #32 | 06 | `dissect/decompress.go` (new), `readChunkedData`/`readNonChunkedData`, `readPlayer`'s DissectID line | — |
| W1 | S29 | #29 | 15 | `dissect/test/acceptance_match_test.go`, `scripts/corpus/` | — |
| W2 | S33 | #33 | 05 | `Reader.Read` worker/scan + sort, `reader.go` import block | S23 |
| W2 | S34 | #34 | 02 | `readPlayer` marker seeks, `dissect/player_markers.go`, `version.go` comment | S32 |
| W2 | S19 | #19 | 07 | `roundEnd` winner/condition (`time.go`), 1vX in `stats.go` | S23 |
| W2 | S30 | #30 | — (issue body) | `NewReader`'s `MatchFeedback` init | S23 |
| W3 | S31 | #31 | 03 | `Read`'s listener loop, `Seek` fast path, `Warnings` (`warning.go`), `round.warnings` in `Data` | S33 |
| W3 | S18 | #18 | 08 | `scoreboardRow` + the four score/assist writers, `deriveTeamRoles` row filter | S19, S17, S34 |
| W4 | S26 | #26 | 09 | `ensureScoreboardMapping` / the entity-link listener | S18 |
| W4 | S28 | #28 | 11 (Part A) | `MatchUpdate` weapon fields (`feedback.go`), `classifyKillWeapons` at the end of `Read` | S31 |

## Worktree ledger

worktree: /home/prest/r6-dissect-worktrees/fork-ports branch=track/fork-ports for=orchestrator

## Event log

- 2026-10-22 gate zero written; track cut (`1352567`).

## Findings

## Candidates

## Observations
