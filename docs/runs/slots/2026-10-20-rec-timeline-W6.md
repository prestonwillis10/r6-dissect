# W6 slot log: dissect-lab join and graph

Branch `feat/rt02-lab-join-graph`, base `track/rec-timeline`. Gate evidence below.

## Gate
Suite green: vet, build, test, and GOOS=windows / GOOS=darwin vet. Simplify and verify passes done inline.

## Mutation proof (raw)
```
--- baseline green
ok  	github.com/redraskal/r6-dissect/cmd/dissect-lab	4.093s
=== (a) join drops last event ===
--- FAIL: TestRun_Join (1.09s)
    main_test.go:188: lines = 10, matchFeedback = 11
FAIL
FAIL	github.com/redraskal/r6-dissect/cmd/dissect-lab	4.051s
FAIL
=== (b) ownerPlayer never set ===
--- FAIL: TestRun_Graph (0.53s)
    main_test.go:248: 27c08dca edges with ownerPlayer = 0
FAIL
FAIL	github.com/redraskal/r6-dissect/cmd/dissect-lab	4.081s
FAIL
=== (c) TimelineError swallowed ===
--- FAIL: TestRun_JoinLegacy (1.08s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
[signal SIGSEGV: segmentation violation code=0x1 addr=0x40 pc=0x682099]

goroutine 2779 [running]:
testing.tRunner.func1.2({0x90de00, 0x981480})
	/home/prest/.local/go/src/testing/testing.go:2125 +0x232
testing.tRunner.func1()
=== (d) edges reversed ===
--- FAIL: TestRun_Graph (0.49s)
    main_test.go:239: line 1 tick 45257 before 46668
FAIL
FAIL	github.com/redraskal/r6-dissect/cmd/dissect-lab	3.842s
FAIL
--- reverted
ok  	github.com/redraskal/r6-dissect/cmd/dissect-lab	3.481s
=== (c') join swallows TimelineError: exit 0, empty stdout ===
--- FAIL: TestRun_Errors (0.00s)
    main_test.go:163: [join /nonexistent.rec]: code=0 out="" err=""
--- FAIL: TestRun_JoinLegacy (1.14s)
    main_test.go:217: code=0 out="" err=""
FAIL
FAIL	github.com/redraskal/r6-dissect/cmd/dissect-lab	4.180s
```

## Premises probed
```
--- join head custom_fortress_plant
{"seq":0,"type":"OperatorSwap","actor":"NickB2A","tick":3385,"elapsedMs":-30792,"roundClockMs":30792,"phase":"Prep","status":"observed"}
{"seq":1,"type":"Kill","actor":"FallacyAsTR.NKG","target":"drew.DD","tick":25743,"elapsedMs":73379,"roundClockMs":106538,"phase":"Action","status":"observed"}
Y11S2/custom_early_plant graph=730 27c08dca+ownerPlayer=20 join=13
Y11S2/custom_solid_snake graph=868 27c08dca+ownerPlayer=30 join=15
Y11S3/custom_clubhouse_short graph=673 27c08dca+ownerPlayer=20 join=9
Y11S3/custom_fortress_plant graph=598 27c08dca+ownerPlayer=20 join=11
Y11S3/custom_lair_plant_stop graph=698 27c08dca+ownerPlayer=20 join=13
```
