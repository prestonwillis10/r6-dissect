# W1 slot log (2026-10-20)

Branch feat/rt01-fixtures-walker, base track/rec-timeline.

## Root cause of the fixture mis-parse
Walk took header-shaped zero runs (float 1.0 = 00 00 80 3f plus zeros) in the prelude for channel headers, and let envelope-shaped garbage after a gap join the open channel regardless of tick order. A third cause: trailer channels after the last game tick use a 29-byte header, and an ordinary envelope can look like that short header.

## Task 1 sanity (plan Step 4)
All three goldens: header.gameVersion Y11S3_Alpha04, 10 players. fortress: one DefuserPlantComplete by Yoejinzera.NKG; lair: one DefuserPlantStop. fortress has 8 Kill events, the plan says 7. Legacy output is the oracle (spec 6), nothing changed.

## Mutation proof (raw)
=== MUTANT a-zero-len: if n == 0 || n > len(b)-p-12 {  ->  if n > len(b)-p-12 {
--- red run: go test ./dissect/wire/ -run 'TestParseEnvelopeAt' -count=1
--- FAIL: TestParseEnvelopeAt (0.00s)
    container_test.go:43: zero len must not parse
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.004s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestParseEnvelopeAt' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT b-trailing-zeros: p[0] == 0x04 && allZero(p[len(p)-12:])  ->  p[0] == 0x04
--- red run: go test ./dissect/wire/ -run 'TestWalk_Synthetic' -count=1
--- FAIL: TestWalk_Synthetic (0.00s)
    container_test.go:103: c2 {Start:149 End:220 HasHeader:true ID:[2 0 0 0 0 0 0 0] Kind:tlv Envelopes:[{Start:194 End:220 Tick:4 Payload:[4 1 2 3 4 5 6 7 8 9 10 11 12 13]}]}
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.004s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_Synthetic' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT c-one-envelope: 	f, ok := ParseEnvelopeAt(b, e.End)
	return ok && f.Tick >= e.Tick
}
// Walk  ->  	return true
}
// Walk
--- red run: go test ./dissect/wire/ -run 'TestWalk_GapLoneEnvelopeIgnored' -count=1
--- FAIL: TestWalk_GapLoneEnvelopeIgnored (0.00s)
    container_test.go:133: channels [{Start:0 End:93 HasHeader:true ID:[1 0 0 0 0 0 0 0] Kind:movement Envelopes:[{Start:45 End:77 Tick:2 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]} {Start:79 End:93 Tick:99 Payload:[9 9]}]}]
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.004s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_GapLoneEnvelopeIgnored' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT d-header-follower: 		q := p + channelHeaderLen
		if q == len(b) {  ->  		return id, channelHeaderLen, true
		q := p + channelHeaderLen
		if q == len(b) {
--- red run: go test ./dissect/wire/ -run 'TestWalk_HeaderShapedRunInPreludeIsNotAHeader' -count=1
--- FAIL: TestWalk_HeaderShapedRunInPreludeIsNotAHeader (0.00s)
    container_test.go:176: prelude = 0, want 57
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.004s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_HeaderShapedRunInPreludeIsNotAHeader' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.004s
=== MUTANT e-envelope-wins: isEnv {  ->  isEnv && false {
--- red run: go test ./dissect/wire/ -run 'TestWalk_EnvelopeShapedLikeShortHeaderStaysAnEnvelope' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.004s
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_EnvelopeShapedLikeShortHeaderStaysAnEnvelope' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT f-tick-order: !ok || e.Tick < minTick {  ->  !ok {
--- red run: go test ./dissect/wire/ -run 'TestWalk_GapPairOlderThanChannelIsIgnored' -count=1
--- FAIL: TestWalk_GapPairOlderThanChannelIsIgnored (0.00s)
    container_test.go:244: channels [{Start:0 End:169 HasHeader:true ID:[1 0 0 0 0 0 0 0] Kind:movement Envelopes:[{Start:45 End:77 Tick:50 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]} {Start:79 End:92 Tick:5 Payload:[9]} {Start:92 End:105 Tick:6 Payload:[9]} {Start:105 End:137 Tick:60 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]} {Start:137 End:169 Tick:61 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]}]}]
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.003s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_GapPairOlderThanChannelIsIgnored' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT g-short-form: headerAt(b, p, true); ok {
			l.Channels  ->  headerAt(b, p, false); ok {
			l.Channels
--- red run: go test ./dissect/wire/ -run 'TestWalk_TrailerShortHeader' -count=1
--- FAIL: TestWalk_TrailerShortHeader (0.00s)
    container_test.go:204: channels [{Start:0 End:206 HasHeader:true ID:[1 0 0 0 0 0 0 0] Kind:movement Envelopes:[{Start:45 End:77 Tick:2 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]} {Start:77 End:109 Tick:3 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]} {Start:154 End:180 Tick:6 Payload:[4 1 2 3 4 5 6 7 8 9 10 11 12 13]} {Start:180 End:206 Tick:7 Payload:[4 1 2 3 4 5 6 7 8 9 10 11 12 13]}]}] gaps [{109 154}]
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.004s
FAIL
--- reverted, green run: go test ./dissect/wire/ -run 'TestWalk_TrailerShortHeader' -count=1
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s
=== MUTANT e-envelope-wins: isEnv {  ->  isEnv && false {
--- red run: go test ./dissect/wire/ -run TestWalk_EnvelopeShapedLikeShortHeaderStaysAnEnvelope -count=1
--- FAIL: TestWalk_EnvelopeShapedLikeShortHeaderStaysAnEnvelope (0.00s)
    container_test.go:232: channels [{Start:0 End:77 HasHeader:true ID:[1 0 0 0 0 0 0 0] Kind:movement Envelopes:[{Start:45 End:77 Tick:2 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]}]} {Start:77 End:343 HasHeader:true ID:[1 2 3 4 5 6 7 8] Kind:unknown Envelopes:[{Start:106 End:122 Tick:0 Payload:[1 2 3 4]} {Start:122 End:138 Tick:1 Payload:[1 2 3 4]} {Start:311 End:343 Tick:18294 Payload:[1 0 0 0 0 0 0 0 0 0 6 0 0 0 96 115 133 254 0 0]}]}] gaps [{138 311}]
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.003s
FAIL
--- reverted, green run:
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.003s

## Implementation reverted, tests kept
$ go test ./dissect/wire/ (container.go reverted to 69152d2, tests kept)
--- FAIL: TestWalk_Synthetic (0.00s)
--- FAIL: TestWalk_HeaderShapedRunInPreludeIsNotAHeader (0.00s)
--- FAIL: TestWalk_TrailerShortHeader (0.00s)
--- FAIL: TestWalk_GapPairOlderThanChannelIsIgnored (0.00s)
--- FAIL: TestWalk_Empty (0.00s)
--- FAIL: TestWalk_Fixtures (0.63s)
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect/wire	0.883s
FAIL

## Suite
$ go vet ./... && go build ./... && go test ./... -count=1
?   	github.com/redraskal/r6-dissect	[no test files]
ok  	github.com/redraskal/r6-dissect/dissect	0.004s
ok  	github.com/redraskal/r6-dissect/dissect/test	3.671s
?   	github.com/redraskal/r6-dissect/dissect/ubi	[no test files]
ok  	github.com/redraskal/r6-dissect/dissect/wire	3.027s
?   	github.com/redraskal/r6-dissect/exports	[no test files]

## Coverage (TestWalk_Fixtures -v)
    container_test.go:311: size=27619511 unplaced=82428 (0.2984%) channels=8 tlv=1 movement=1 movement_envelopes=2707 gaps=2 prelude=81322
    container_test.go:311: size=49710628 unplaced=103463 (0.2081%) channels=8 tlv=1 movement=1 movement_envelopes=4545 gaps=3 prelude=84768
    container_test.go:311: size=53549064 unplaced=85339 (0.1594%) channels=8 tlv=1 movement=1 movement_envelopes=5419 gaps=2 prelude=83746
    container_test.go:311: size=76179853 unplaced=86407 (0.1134%) channels=8 tlv=1 movement=1 movement_envelopes=6320 gaps=3 prelude=83126
    container_test.go:311: size=66236205 unplaced=86895 (0.1312%) channels=8 tlv=1 movement=1 movement_envelopes=5632 gaps=2 prelude=84450
--- PASS: TestWalk_Fixtures (0.62s)
    --- PASS: TestWalk_Fixtures/custom_early_plant.rec (0.08s)
    --- PASS: TestWalk_Fixtures/custom_solid_snake.rec (0.12s)
    --- PASS: TestWalk_Fixtures/custom_clubhouse_short.rec (0.13s)
    --- PASS: TestWalk_Fixtures/custom_fortress_plant.rec (0.16s)
    --- PASS: TestWalk_Fixtures/custom_lair_plant_stop.rec (0.13s)
    container_test.go:326: err=<nil> channels=1 unplaced=47666002 of 125007852
--- PASS: TestWalk_LegacyLayoutDoesNotPanic (0.49s)

## Premise: Data() vs --dump (clubhouse)
Data() len=53549064 sha256=7501958f...b961; r6-dissect --dump output 53549064 bytes sha256 7501958f...b961 (identical)

## Fix round 1
Reviewer verdict NEEDS-CHANGES on 4d91da9. The walker was rewritten to the observed header grammar (45 bytes, u32 lead at 0, id8 at 12, u32 count at 25 with count-2 envelopes following, free u32 at 33, a 12-byte terminator). The short header form, the gap envelope resync and the legacy no-panic test were removed. Code head: a547139. Evidence and red/green runs are in the PR body.

## Observations
- Y9S1 is not a Y11 layout but the walker finds headers in it. Command and output (a throwaway test calling Walk on the fixture, removed afterwards):
```
Y9S1 err=<nil> channels=7 envelopes=16134 gaps=2 unplaced=259330 prelude=259269 size=129201607 lastgap=[{129140909 129140958} {129201595 129201607}]
```
  Walk is documented in `dissect/wire/container.go` as `// Walk splits the decompressed buffer`. Acceptance: decide whether Y9S1 is a supported layout and, if so, add it to the fixture count table in `TestWalk_Fixtures`. Verified-against: a547139

## Fix round 2
Reviewer found two regressions in round 1. The buffer cut inside the next header dropped the whole layout, and the leading-run and first-header scans were quadratic. Code head: cf03131. The leading-run search is now a single right-to-left pass, the first-header scan memoizes envelope chains per lead, and a stub header at the end of a cut buffer is accepted after a complete channel. Red and green runs are in the PR body.
