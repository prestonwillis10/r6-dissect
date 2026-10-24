# W5 slot log: timeline assembly and --timeline sidecar

Branch feat/rt02-timeline-sidecar, cut from track/rec-timeline. Plan 02 Tasks 3 and 4.
Simplify and verify passes were done inline over the diff (no nested agents).

## Gate evidence

Parity per Y11S2 and Y11S3 fixture (timeline events vs matchFeedback, max |elapsedMs/1000 - elapsedSeconds|):

```
custom_clubhouse_short events 9 feedback 9 maxdiff_s 0.77 players 10 pawns 20
   phases [('Prep', 14, 44947), ('Action', 8648, 179856)]
custom_early_plant events 13 feedback 13 maxdiff_s 0.74 players 10 pawns 20
   phases [('Prep', 14, 44914), ('Action', 5436, 179825), ('PostPlant', 12147, 44977)]
custom_fortress_plant events 11 feedback 11 maxdiff_s 0.86 players 10 pawns 20
   phases [('Prep', 44, 44832), ('Action', 10910, 179917), ('PostPlant', 45872, 44938)]
custom_lair_plant_stop events 13 feedback 13 maxdiff_s 0.77 players 10 pawns 20
   phases [('Prep', 12, 44952), ('Action', 10321, 179854)]
custom_solid_snake events 15 feedback 15 maxdiff_s 0.86 players 10 pawns 20
   phases [('Prep', 6, 44929), ('Action', 5414, 179875), ('PostPlant', 17517, 44946)]
```

Mutation runs (raw):

```
### MUTANT a-tickAt-zero (red expected)
--- FAIL: TestTimeline_Parity (1.36s)
        timeline_test.go:66: event 0 tick 0 outside [4,12560]
        timeline_test.go:66: event 1 tick 0 outside [4,12560]
        timeline_test.go:66: event 2 tick 0 outside [4,12560]
        timeline_test.go:66: event 3 tick 0 outside [4,12560]
        timeline_test.go:66: event 4 tick 0 outside [4,12560]
        timeline_test.go:66: event 5 tick 0 outside [4,12560]
        timeline_test.go:66: event 6 tick 0 outside [4,12560]
### REVERT a-tickAt-zero (green)
ok  	github.com/redraskal/r6-dissect/dissect/test	1.611s
### MUTANT b-minus1-tick0 (red expected)
--- FAIL: TestTickIndex_TickAt (0.00s)
    timeline_internal_test.go:98: tickAt(-1) = 0, want 30
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.006s
FAIL
### REVERT b-minus1-tick0 (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.005s
### MUTANT c-no-recompute (red expected)
FAIL	github.com/redraskal/r6-dissect/dissect [build failed]
FAIL
### REVERT c-no-recompute (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.007s
### MUTANT d-derived-ignored (red expected)
--- FAIL: TestBuildEvent_StatusAndData (0.00s)
    timeline_internal_test.go:115: {Seq:3 Tick:2 ElapsedMs:0 RoundClockMs:179917 Phase:Action Type:Kill Actor: Target: Data:map[headshot:true message:m operator:Ash teamKill:true] Status:observed}
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.006s
FAIL
### REVERT d-derived-ignored (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.007s
### MUTANT e-sidecar-on-error (red expected)
--- FAIL: TestWriteRound_LegacyGetsTimelineError (1.07s)
    main_test.go:91: no sidecar on error, found 1 entries
FAIL
FAIL	github.com/redraskal/r6-dissect	1.109s
FAIL
### REVERT e-sidecar-on-error (green)
ok  	github.com/redraskal/r6-dissect	1.052s
### MUTANT e2-match-sidecar-on-error (red expected)
--- FAIL: TestWriteMatch_LegacyRoundGetsTimelineError (1.56s)
    main_test.go:196: [- a_legacy.timeline.json - b_modern.timeline.json]
FAIL
FAIL	github.com/redraskal/r6-dissect	1.621s
FAIL
### REVERT e2-match-sidecar-on-error (green)
ok  	github.com/redraskal/r6-dissect	1.550s
### MUTANT f-field-without-flag (red expected)
ok  	github.com/redraskal/r6-dissect	1.124s
### REVERT f-field-without-flag (green)
ok  	github.com/redraskal/r6-dissect	0.579s
### MUTANT c-no-recompute (red expected)
--- FAIL: TestBuildEvent_PlantInJumpFrame (0.00s)
    timeline_internal_test.go:19: plant in the jump frame: phase Action elapsed 67
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.004s
FAIL
### REVERT c-no-recompute (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.004s
### MUTANT f-field-without-flag (red expected)
ok  	github.com/redraskal/r6-dissect	1.287s
### REVERT f-field-without-flag (green)
ok  	github.com/redraskal/r6-dissect	0.596s
### MUTANT f-field-without-flag (timelineError emitted unconditionally; red expected)
--- FAIL: TestWriteRound_NoFlagNoField (0.95s)
    main_test.go:101: timelineError without --timeline
FAIL
FAIL	github.com/redraskal/r6-dissect	0.997s
FAIL
### REVERT f (green)
ok  	github.com/redraskal/r6-dissect	0.463s
### MUTANT g-wire-nil-guard (red expected)
--- FAIL: TestDecodeTLV_NilEmit (0.00s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
panic({0x93fa40?, 0x9b24c0?})
	/home/prest/r6-dissect-worktrees/w5-timeline/dissect/wire/tlv_test.go:398 +0x10d9
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.009s
FAIL
### REVERT g-wire-nil-guard (green)
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.005s
### MUTANT h-crt13-no-synthetic-phase (red expected)
--- FAIL: TestTimelinePhases_PlantWithoutJump (0.00s)
    timeline_internal_test.go:55: [{Phase:PostPlant Tick:1000 RoundClockMs:45500}]
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.005s
FAIL
### REVERT h-crt13-no-synthetic-phase (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.005s
### MUTANT i-crt12-not-derived (red expected)
--- FAIL: TestStopDefuserRunAtRoundEnd_IsDerived (0.00s)
    timeline_internal_test.go:77: {Type:DefuserDisableStop Username:defender Target: Headshot:<nil> Time: TimeInSeconds:0 Phase: ElapsedSeconds:0 Message: Operator:Operator(0) TeamKill:false scoreboardKiller: offset:-1 derived:false}
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.005s
FAIL
### REVERT i-crt12-not-derived (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.005s
### MUTANT j-crt12-listener-offset (red expected)
ok  	github.com/redraskal/r6-dissect/dissect/test	3.288s
### REVERT j-crt12-listener-offset (green)
ok  	github.com/redraskal/r6-dissect/dissect/test	3.433s
### MUTANT j-crt12-listener-offset (Read calls the plain stop; red expected)
--- FAIL: TestRead_RoundEndDefuserStopIsDerived (0.00s)
    timeline_internal_test.go:142: &{Type:DefuserDisableStop Username:defender Target: Headshot:<nil> Time: TimeInSeconds:0 Phase: ElapsedSeconds:0 Message: Operator:Operator(0) TeamKill:false scoreboardKiller: offset:0 derived:false}
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.011s
### REVERT j (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.009s
```

## Premises probed

Real match folder, seven rounds, first round:

```
-rwxr-xr-x 1 prest prest 53904 Oct 20 13:33 match.json

tl:
total 64
drwxr-xr-x 2 prest prest 4096 Oct 20 13:33 .
drwxr-xr-x 3 prest prest 4096 Oct 20 13:33 ..
-rw-r--r-- 1 prest prest 5473 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R01.timeline.json
-rw-r--r-- 1 prest prest 5486 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R02.timeline.json
-rw-r--r-- 1 prest prest 6028 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R03.timeline.json
-rw-r--r-- 1 prest prest 5294 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R04.timeline.json
-rw-r--r-- 1 prest prest 5056 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R05.timeline.json
-rw-r--r-- 1 prest prest 5071 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R06.timeline.json
-rw-r--r-- 1 prest prest 4659 Oct 20 13:33 Match-2026-10-18_18-19-54-14012-R07.timeline.json
Match-2026-10-18_18-19-54-14012-R01.timeline.json
{'tickFirst': 7, 'tickLast': 47357, 'phases': [{'phase': 'Prep', 'tick': 44, 'roundClockMs': 44832}, {'phase': 'Action', 'tick': 10910, 'roundClockMs': 179917}, {'phase': 'PostPlant', 'tick': 45872, 'roundClockMs': 44938}]}
[('OperatorSwap', 'NickB2A', 3385, -30792, 30792), ('Kill', 'FallacyAsTR.NKG', 25743, 73379, 106538), ('Kill', 'Breck.HRC', 27099, 79683, 100234), ('Kill', 'Yoejinzera.NKG', 29228, 90861, 89056)]
30 {'unplacedFraction': 0.0010913384146330658, 'tlvErrors': 0, 'tlvTokens': 42007, 'movementRecords': 213613, 'movementBadFraction': 0}
[False, False, False, False, False, False, False]
```

## Observations

- The first attempt at the no-flag mutant only removed EnableTimeline and survived; it is equivalent because the error field is also gated on the flag. The mutant that drops both gates is red.
- Review Focus 1 recompute changes elapsedMs only when a pre-jump clock sample shares the jump tick, because the post-plant base already equals the elapsed time of the last Action sample. No fixture exercises it; the pin is a synthetic clock series.
- C-RT-12: the DefuserPlantStop in custom_lair_plant_stop comes from the listener (observed, tick 36539, not the last tick). No fixture produces a round-end stop, so the pin is synthetic, both at the helper and through Read.
- C-RT-13: no fixture has a plant with 44 to 46 s left; the pin is synthetic.
- The ten player entities have an empty class in every fixture; the twenty pawns carry the graph class.
- Reading from stdin names the sidecar after the file name os.Stdin reports, so it is stdin.timeline.json.

## Fix round 1

Red run before the fix:

```
--- FAIL: TestRead_ListenerEOFStillSettlesTimeline (0.53s)
    timeline_internal_test.go:180: dissect: timeline not built; call Read first
--- FAIL: TestTimeline_CorruptTLVIsTimelineError (0.34s)
    timeline_internal_test.go:206: <nil> <nil>
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.910s
FAIL
```

Mutants:

```
--- MUTANT gate-threshold-1.0 (red expected)
--- FAIL: TestTimeline_CorruptTLVIsTimelineError (0.41s)
    timeline_internal_test.go:206: <nil> <nil>
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.443s
FAIL
--- REVERT gate-threshold-1.0 (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.508s
--- MUTANT settle-removed (red expected)
--- FAIL: TestRead_ListenerEOFStillSettlesTimeline (0.38s)
    timeline_internal_test.go:180: dissect: timeline not built; call Read first
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.402s
FAIL
--- REVERT settle-removed (green)
ok  	github.com/redraskal/r6-dissect/dissect	0.431s
```
