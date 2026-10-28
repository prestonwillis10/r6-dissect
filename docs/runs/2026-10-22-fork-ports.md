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
| W5 | S35 | #35 | 11 (Part B, 11c) | the reconcile pass order in `reconcileKillsFromScoreboard`; credits closed PR #12's sort and its `break` note | S26 |
| W4 | S28 | #28 | 11 (Part A) | `MatchUpdate` weapon fields (`feedback.go`), `classifyKillWeapons` at the end of `Read` | S31 |

## Worktree ledger

worktree: /home/prest/r6-dissect-worktrees/fork-ports branch=track/fork-ports for=orchestrator
worktree: /home/prest/r6-dissect-worktrees/fp-s23 branch=fp/s23-incomplete-match for=S23
worktree: /home/prest/r6-dissect-worktrees/fp-s17 branch=fp/s17-map-ids for=S17
worktree: /home/prest/r6-dissect-worktrees/fp-s32 branch=fp/s32-decode-memory for=S32
worktree: /home/prest/r6-dissect-worktrees/fp-s29 branch=fp/s29-match-corpus for=S29
worktree: /home/prest/r6-dissect-worktrees/rev-37 branch=(detached 75cce63) for=rev-37
worktree: /home/prest/r6-dissect-worktrees/rev-38 branch=(detached 1a15d3d) for=rev-38
worktree: /home/prest/r6-dissect-worktrees/rev-3-b branch=(detached ce05e79) for=rev-3-b (PR #3)
worktree: /home/prest/projects/R6Stats/r6-dissect-dbno branch=fork/health-dbno for=PR #3 (pre-existing; used for the main merge and the fix worker)
worktree: /home/prest/r6-dissect-worktrees/val branch=(detached) for=orchestrator validation merges
worktree: /home/prest/r6-dissect-worktrees/rev-39 branch=(detached 7bc5821) for=rev-39
worktree: /home/prest/r6-dissect-worktrees/rev-39-r2 branch=(detached 1fea036) for=rev-39 round 2
worktree: /home/prest/r6-dissect-worktrees/rev-40 branch=(detached d21b0c4) for=rev-40
worktree: /home/prest/r6-dissect-worktrees/ci-main-only branch=ci/pr-into-main-only for=PR #41
worktree: /home/prest/r6-dissect-worktrees/rev-40-r2 branch=(detached 3d4ae8e) for=rev-40 round 2
worktree: /home/prest/r6-dissect-worktrees/rev-3-b-r4 branch=(detached 2fbc240) for=rev-3-b round 4
worktree: /home/prest/r6-dissect-worktrees/fp-s33 branch=fp/s33-pattern-scan for=S33
worktree: /home/prest/r6-dissect-worktrees/fp-s34 branch=fp/s34-alpha03-markers for=S34
worktree: /home/prest/r6-dissect-worktrees/fp-s19 branch=fp/s19-undecided-round for=S19
worktree: /home/prest/r6-dissect-worktrees/fp-s30 branch=fp/s30-feedback-array for=S30
worktree: /home/prest/r6-dissect-worktrees/rev-42 branch=(detached 6be291d) for=rev-42
worktree: /home/prest/r6-dissect-worktrees/rev-43 branch=(detached 6b3617d) for=rev-43
worktree: /home/prest/r6-dissect-worktrees/rev-44 branch=(detached 138274f) for=rev-44
worktree: /home/prest/r6-dissect-worktrees/rev-45 branch=(detached a2246f8) for=rev-45

## Event log

- 2026-10-22 gate zero written; track cut (`1352567`); run log `5fdfc3d`.
- 2026-10-22 W1 dispatched: S23, S17, S32, S29 (Sonnet).
- 2026-10-22 operator directives: close external PR #12 (done, comment points at CONTRIBUTING.md and at #35/#26); merge docs PRs (r6-dissect #36 → `389ef14`, R6Stats #19 → `0ea6b0d`); bring PR #3 up to date. #35 is unblocked by #12's closure (credit #12's sort and its `break` note when S35 lands).
- 2026-10-22 PR #3: rev-3 round-2 verdict harvested from the previous session's transcript (`agent-aa162b2d7b15ed0e1.jsonl`): NEEDS-CHANGES at `c608cf8`, F4 BLOCKING FIX-INTRODUCED (`insertByElapsed` tie rule only looks at `at-1`), F5 ADVISORY. `origin/main` merged into `fork/health-dbno` cleanly; build, c-shared, gofmt, vet, tidy and staticcheck PASS.
- 2026-10-22 env: local govulncheck exits 3 on GO-2026-6809 (crypto/x509, go1.27.2; fixed in 1.27.3). CI's `go-version: "1.27"` resolves the patched release, so CI vulncheck is authoritative; the local toolchain is not swapped mid-lane.
- 2026-10-22 S17 → PR #37 (head `75cce63`); base track/fork-ports, worktree clean; ci-settle armed.
- 2026-10-22 PR #37 CI SETTLED PASS at `75cce63`; preflight 14 pass / 3 warn / 0 fail; rev-37 dispatched (Opus).
- 2026-10-22 PR #3: merge of origin/main pushed (`6d8ce77`); -race tests and goldens PASS locally; Sonnet fix worker dispatched for F4/F5 in `/home/prest/projects/R6Stats/r6-dissect-dbno`.
- 2026-10-22 rev-37 round 1 MERGE-AFTER-MECHANICAL (F1 EVIDENCE: fork provenance missing, Deviations said None); body patched (byte-equal read-back), checks `4`/`0`, strict preflight 0 FAIL; round 2 MERGE at `75cce63`; PR #37 merged into the track.
- 2026-10-22 S23 → PR #38 (head `1a15d3d`); CI SETTLED PASS; preflight 16 pass / 1 warn / 0 fail; rev-38 dispatched (Opus).
- 2026-10-22 watchdog fix: transcripts are symlinks, so silence is measured with `stat -L`.
- 2026-10-22 operator: "you have the helm" — landings go through `/deduce-pr` to a MERGE verdict and then merge to main; re-sync with main before each landing (other lanes merge too). Coordination message sent to peer session r6stats-3b.
- 2026-10-22 operator: CI is local — slot pushes carry `[skip ci]` on the head commit; local gates are the evidence; the orchestrator re-runs the full suite at merge; full GitHub CI only on landing PRs and PR #3's fix push. Briefs updated; running implementers told.
- 2026-10-22 host memory incident: `go test -race ./dissect/test` peaked at 16 GB (S29) and 15 GB (S32), swap 27/31 GB. Both killed by pid (S29's by hand, S32's by `lane/memguard.sh`, a lane-scoped 12 GB guard). Rule: implementers run `-race` only on `./dissect ./exports .` with `GOMAXPROCS=2 -p=1`; the full race suite is an orchestrator gate, one at a time.
- 2026-10-22 PR #3: fix worker pushed `a923a40` + `ce05e79` (F4 in `insertByElapsed`, F5 tests + a third pinning test); CI SETTLED PASS at `ce05e79`; body patched (round 2 section, main sync, pseudonyms; byte-equal read-back); strict preflight 0 FAIL; rev-3-b (fresh Opus, delta + final) dispatched in `/home/prest/r6-dissect-worktrees/rev-3-b`.
- 2026-10-22 rev-38 round 1 MERGE at `1a15d3d` (4 ADVISORY: F1 red intermediate commits `33ed51d`/`dc7589d` → slot is squash-merged into the track; F2 `RoundAt` out-of-range panic pre-existing; F3 R6Stats `skippedRounds` consumer unfiled; F4 absolute host path in `skippedRounds[].error`). Local re-validation of #38 merged onto the track (`60f6616`) running in `/home/prest/r6-dissect-worktrees/val` via `lane/gates.sh` (race suite at `-parallel=1`).
- 2026-10-22 S29 → PR #39 (head `7bc5821`, `[skip ci]`); offline preflight 11 pass / 5 warn / 0 fail; rev-39 dispatched (Opus).
- 2026-10-22 operator check-in: slot PRs use merge commits (the attempted squash of #38 was rejected); `[skip ci]` approved; CI to run only on PRs into main → PR #41 (r6-dissect) and R6Stats #23 opened (`ci: run CI only on pull requests into main`). PR #38 merged into the track with `--merge` (`05cf447`).
- 2026-10-22 rev-3-b round 3 MERGE-AFTER-MECHANICAL at `ce05e79` (N1 floor rule unpinned, N2 body names `1437695` not `389ef14`, N3 long comment). Applied by the orchestrator: `f69f093` (two tests; floor-deleted mutant fails both) and `2fbc240` (rewrap); body draft ready; local CI running before push.
- 2026-10-22 rev-39 round 1 MERGE-AFTER-MECHANICAL at `7bc5821` (F1: `matchJSON` collides with #38's type; F2–F8 ADVISORY). Rename applied by the orchestrator (`1fea036`, `[skip ci]`); local CI on track(05cf447)+#39 green (race suite 181 s); rev-39 round 2 charged.
- 2026-10-22 S32 → PR #40 (`d21b0c4`); preflight evidence FAILs fixed in the body (fenced mutation runs, three citations); offline preflight 0 FAIL; rev-40 dispatched.
- 2026-10-22 wave 2 held until PR #3 lands on main (it reshapes `reader.go`/`stats.go`/`header.go`); then main is merged into the track and S33/S19/S30 (+S34 after #40, S25/S27 in W3, S35 in W5) are cut.
- 2026-10-22 PR #39: rev-39 round 2 MERGE-WITH-NITS (F6 evidence nit applied in the body; strict preflight 0 FAIL) → merged into the track (`b81ade4`). Candidates F2+F3 (expected-failure matching) and F4+F5 (fetch.sh symlink/shallow fetch) recorded below.
- 2026-10-22 PR #41 (CI trigger) and R6Stats #23: preflight premises fixed; rev-41 MERGE, rev-r6s-23 MERGE-WITH-NITS (body paren fixed).
- 2026-10-22→27 session paused ~5 days; on resume both `main`s and tracks were unchanged. rev-r6s-21/22 had stalled on background monitors; resumed with foreground-only charges.
- 2026-10-27 PR #41 merged into main (`b9edf5a`); main merged into the track (`22822c7`) so slot PRs no longer start CI (rev-41 F1).
- 2026-10-27 PR #40: rev-40 round 1 MERGE-AFTER-MECHANICAL (F1 pin guard Y8S2-only) → patch applied (`3d4ae8e`, slotID mutant fails the Y11S2 subtest); local CI on `b81ade4`+`3d4ae8e` green (race `dissect/test 73.713s`); round 2 MERGE-WITH-NITS (stale evidence prose fixed, `1d310a8`) → merged into the track (`74de332`). **Wave 1 complete: #37, #38, #39, #40.**
- 2026-10-27 PR #3: round-3 fixes pushed (`f69f093`, `2fbc240`); local CI + GitHub CI green; strict preflight 0 FAIL; rev-3-b round 4 charged.
- 2026-10-27 rev-3-b round 4 MERGE at `2fbc240` → **PR #3 merged into main** (`7d37745`; operator approval for MERGE verdicts; main had not moved past `b9edf5a`). Main merged into the track (`24024f6`): no conflicts; local CI green (race `dissect/test 70.915s`); PR #3's intended golden diffs; acceptance on the user's 5 matches `rounds 42, DBNO 73 (69 credited), Revive 3 (2 credited)`; corpus: the same 5 expected failures, 0 unexpected. #25 and #27 unblocked.
- 2026-10-27 W2 dispatched from `24024f6`: S33 (#33, Opus), S34 (#34), S19 (#19; third-party fixture replaced by a synthetic test; must not edit `knownMatchFailures`), S30 (#30; owns the `knownMatchFailures` edits this wave). Slots run `[skip ci]` pushes and the memory rule.
- 2026-10-27 S30 → PR #42 (`6be291d`; preflight mutation-proof FAIL fixed by pasting the slot log's fenced mutant block); S19 → PR #43 (`6b3617d`; the 21916 score-drop entry is NOT #19's: round 5 is decided 2→3 with a missing scoreboard — relabel at reconciliation); S34 → PR #44 (`138274f`; citation `player.go:10` → `:11` fixed). Offline preflights 0 FAIL. rev-42/43/44 dispatched (Opus). S33 still implementing.
- 2026-10-27 rev-42 MERGE → local CI on #42's head green (race `dissect/test 70.769s`) → #42 merged (`da2264b`). rev-44 MERGE-WITH-NITS (N2 test comment, N4 stale `:63` cite → `9bfa55a`, `b0bd37b`, body patched). rev-43 MERGE-WITH-NITS (F2 README scope → `e307b07`; F3 → candidate 6). S33 → PR #45 (`a2246f8`; `Seek` also rewritten on `bytes.Index`; benchmark scan 80→17 ms solid_snake); preflight 0 FAIL; rev-45 dispatched.
- 2026-10-28 validation-tree incident: the `val` worktree still held #40's uncommitted validation merge, so the first #44 run executed on a stale tree (detected from the `VAL` header, discarded). `lane/val.sh` now refuses a dirty or mid-merge tree and always aborts its merge.
- 2026-10-28 W2 merges, each after `lane/val.sh` on the current track: #44 (`43a747a`, race `dissect/test 65.064s`), #43 (`9af84a4`, `61.850s`), #45 (`999e2af`, `44.347s` — the new scanner). Corpus expected failures unchanged throughout. **Wave 2 complete: #42, #43, #44, #45.**
- 2026-10-28 reconciliation (`ef23785`): the 21916 `score-drop` entry relabelled from #19 to candidate 6 (snapshot rows dropped), with a comment.

## Findings

finding: PR=42 round=1 tier=ADVISORY origin=EVIDENCE kind=other author-catchable=no
finding: PR=43 round=1 tier=ADVISORY origin=IN-DIFF kind=prose author-catchable=yes (README scope)
finding: PR=44 round=1 tier=ADVISORY origin=EVIDENCE kind=prose author-catchable=yes (stale cite)
finding: PR=45 round=1 tier=ADVISORY origin=IN-DIFF kind=other author-catchable=no (observations only)
finding: PR=39 round=2 tier=ADVISORY origin=EVIDENCE kind=prose author-catchable=yes
finding: PR=40 round=1 tier=MECHANICAL origin=IN-DIFF kind=vacuous-test author-catchable=yes (pin guard never ran Y10S4+ listeners)
finding: PR=40 round=2 tier=ADVISORY origin=EVIDENCE kind=prose author-catchable=yes
finding: PR=41 round=1 tier=ADVISORY origin=EVIDENCE kind=other author-catchable=no (track keeps the old trigger until main is merged in)
finding: PR=3 round=3 tier=MECHANICAL origin=FIX-INTRODUCED kind=vacuous-test author-catchable=yes
finding: PR=3 round=3 tier=MECHANICAL origin=FIX-INTRODUCED kind=false-claim author-catchable=yes
finding: PR=39 round=1 tier=MECHANICAL origin=IN-DIFF kind=other author-catchable=no (cross-PR identifier collision)
finding: PR=37 round=1 tier=MECHANICAL origin=EVIDENCE kind=false-claim author-catchable=yes
finding: PR=38 round=1 tier=ADVISORY origin=IN-DIFF kind=other author-catchable=yes (red intermediate commits)
finding: PR=38 round=1 tier=ADVISORY origin=PRE-EXISTING kind=other author-catchable=no
finding: PR=38 round=1 tier=ADVISORY origin=EVIDENCE kind=other author-catchable=no
finding: PR=38 round=1 tier=ADVISORY origin=IN-DIFF kind=other author-catchable=yes

## Candidates

1. **match: `MatchReader.RoundAt` panics on an out-of-range index instead of returning `ErrInvalidFile`** — from PR #38 rev-38 F2. Observed in a scratch copy of `1a15d3d`: `m.RoundAt(2)` on a 2-file folder → `panic: runtime error: index out of range [2] with length 2`. `dissect/match.go` `RoundAt` indexes `if m.rounds[i] == nil {` before `read()` checks `if i < 0 || i >= len(m.paths) {`. Not reachable from the CLI or exports (they call `FirstRound` only). Acceptance: a unit test calling `RoundAt(len(paths))` gets `ErrInvalidFile`, red today. Verified-against: 1a15d3d.
2. **R6Stats ingest: rounds the parser skipped (`skippedRounds`, r6-dissect #23) are dropped without an upload warning** — from PR #38 rev-38 F3; called for by issue #23 Scope and plan 01 Risks. After #23 lands the CLI exits 0 with `skippedRounds` in the match JSON; R6Stats `api/r6stats/worker/jobs.py` discards stderr and nothing reads `skippedRounds`. Acceptance: an ingest test with a match JSON carrying `skippedRounds` produces an upload warning naming the file. Verified-against: R6Stats 826dc36 / r6-dissect 1a15d3d.
3. **match: `skippedRounds[].error` carries the absolute host path of an unreadable file** — from PR #38 rev-38 F4. Observed: `"error":"open /tmp/…/R02.rec: permission denied"` in stdout JSON (exit 0). R6Stats stores the raw match JSON. Acceptance: with an unreadable R02, the JSON error string contains no `/` path prefix (file name only). Verified-against: 1a15d3d.

4. **test: `TestAcceptance_Matches` matches expected failures by kind only, and the flaky entry can never turn red** — from PR #39 rev-39 F2+F3. A second finding of the same kind in a known folder stays green (`compareToKnown`); the 47572 `flaky: true` nondeterminism entry stays green after #26 lands, contrary to `scripts/corpus/README.md` ("When a fix lands, the test goes red for that entry"). Acceptance: adding a second null round to a known folder's expected output, or fixing the 47572 nondeterminism, turns the test red. Verified-against: 1fea036.
5. **scripts: `fetch.sh` follows symlinks already in the output dir and cannot recover blobs that exist only in a fork's history** — from PR #39 rev-39 F4+F5. Probe: a symlink `out/<folder>` → `victim/` received the file; `--depth=1` reported a history-only blob `missing`. `scripts/corpus/README.md` overstates force-push safety. Acceptance: fetch.sh refuses a symlinked target path and fetches full history for the pinned refs (or the README says tips only). Verified-against: 1fea036.

6. **scoreboard: the Y10S4+ score reader drops round-start snapshot rows (marker byte `0xa3`)** — from PR #43 rev-43 F3. In nicktstewart match3 R05 (21916), Mori.CU's 930 appears once, in the round-start snapshot at offset 20630 with byte −18 = `0xa3`; `scoreboardEntryHasMarker` requires `0x23`, so it is dropped and the player's cumulative score reads 930 → 0 in a round with no score change. Not owned by #17–#35 (#26 scopes snapshot detection out). The corpus entry `knownMatchFailures["nick-Match-2026-03-08_19-55-32-21916"]` `score-drop` is mislabelled `#19`; relabel to this candidate at reconciliation. Acceptance: the corpus test's 21916 folder passes the score-drop invariant. Verified-against: 6b3617d.

## Observations
