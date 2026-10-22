# W7 slot log: version gate and doc contract

Branch `feat/rt02-version-gate`, base `track/rec-timeline` at 2a5dffb. Constants used: `Y11S2`, `Y10S1` (both exist in dissect/version.go). Simplify and verify passes done inline.

## Mutation proof (raw)
```
=== green (gate present)
=== RUN   TestTimeline_VersionOutsideRange
=== RUN   TestTimeline_VersionOutsideRange/pre-Y11S2_voids_the_timeline
=== RUN   TestTimeline_VersionOutsideRange/control:_unmodified_fixture_builds
--- PASS: TestTimeline_VersionOutsideRange (0.48s)
    --- PASS: TestTimeline_VersionOutsideRange/pre-Y11S2_voids_the_timeline (0.27s)
    --- PASS: TestTimeline_VersionOutsideRange/control:_unmodified_fixture_builds (0.20s)
PASS
ok  	github.com/redraskal/r6-dissect/dissect	0.501s
=== red (gate removed)
--- FAIL: TestTimeline_VersionOutsideRange (0.52s)
    --- FAIL: TestTimeline_VersionOutsideRange/pre-Y11S2_voids_the_timeline (0.26s)
        timeline_internal_test.go:234: tl=&{1 {7 47357 [{Prep 44 44832} {Action 10910 179917} {PostPlant 45872 44938}]} [{517305f000000000 player Ren.HRC  observed} {5c7305f000000000 player ZagHLB.HRC
FAIL
FAIL	github.com/redraskal/r6-dissect/dissect	0.541s
FAIL
```

## Sweep
```
Y11S2/custom_early_plant.rec base==new sidecars=1
Y11S2/custom_solid_snake.rec base==new sidecars=1
Y11S3/custom_clubhouse_short.rec base==new sidecars=1
Y11S3/custom_fortress_plant.rec base==new sidecars=1
Y11S3/custom_lair_plant_stop.rec base==new sidecars=1
Y8S1/quick_1.rec base==new sidecars=0
Y8S1/ranked_1.rec base==new sidecars=0
Y8S1/ranked_2.rec base==new sidecars=0
Y8S2/attacker_recorder_site_test.rec base==new sidecars=0
Y8S2/defender_recorder_site_test_1.rec base==new sidecars=0
Y8S2/defender_recorder_site_test_2.rec base==new sidecars=0
Y8S2/spectator_recorder_site_test_1.rec base==new sidecars=0
Y8S2/spectator_recorder_site_test_2.rec base==new sidecars=0
Y9S1/custom_1.rec base==new sidecars=0
```
