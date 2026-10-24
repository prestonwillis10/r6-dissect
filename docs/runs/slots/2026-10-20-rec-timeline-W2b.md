# W2b slot log: movement record decoder

Branch feat/rt01-movement, base track/rec-timeline. Task 4 of plan 01.

## Deviations from the plan

- Fixture test floors lowered: the shortest fixture has 60796 records and 2707 groups, so the plan's 100000 records and 3000 records per busy entity cannot hold there. Floors are now 50000 records and 1500 records per entity, still at least 10 entities.
- Added tests: synthetic DecodeMovement stats, emit error, hand-built non-monotonic layout, and a truncation test that compares against the full buffer.

## Fixture stats

```
records=60796 groups=2707 bad=0 badBytes=0 (0.0000%) busy=17 ticks=1..12561
records=109106 groups=4545 bad=0 badBytes=0 (0.0000%) busy=31 ticks=1..19552
records=126380 groups=5419 bad=0 badBytes=0 (0.0000%) busy=36 ticks=1..39013
records=213613 groups=6320 bad=0 badBytes=0 (0.0000%) busy=51 ticks=1..47358
records=164853 groups=5632 bad=0 badBytes=0 (0.0000%) busy=43 ticks=1..38032
```

## Premises

```
custom_early_plant.rec     movement_channels=1 envelopes=2707 groups=2707 records=60796 records_per_group=1..189
   tags: 607385fe=60530 617385fe=201 627385fe=51 637385fe=14
custom_solid_snake.rec     movement_channels=1 envelopes=4545 groups=4545 records=109106 records_per_group=1..203
   tags: 607385fe=108811 617385fe=220 627385fe=51 637385fe=24
custom_clubhouse_short.rec movement_channels=1 envelopes=5419 groups=5419 records=126380 records_per_group=1..213
   tags: 607385fe=125964 617385fe=281 627385fe=89 637385fe=46
custom_fortress_plant.rec  movement_channels=1 envelopes=6320 groups=6320 records=213613 records_per_group=1..209
   tags: 607385fe=213256 617385fe=211 627385fe=89 637385fe=57
custom_lair_plant_stop.rec movement_channels=1 envelopes=5632 groups=5632 records=164853 records_per_group=1..206
   tags: 607385fe=164506 617385fe=246 627385fe=47 637385fe=54
```

## Suite

```
ok  	github.com/redraskal/r6-dissect/dissect	0.009s
ok  	github.com/redraskal/r6-dissect/dissect/test	4.489s
ok  	github.com/redraskal/r6-dissect/dissect/wire	13.141s
```

## Mutation proof (raw output)

```
=== mutant a_tag_check_dropped: RED run (TestParseGroup)
--- FAIL: TestParseGroup (0.00s)
    movement_test.go:37: bad tag must fail
=== mutant a_tag_check_dropped reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.011s
=== mutant b_L_lt4_accepted: RED run (TestParseGroup)
--- FAIL: TestParseGroup (0.00s)
panic: runtime error: slice bounds out of range [:18] with capacity 17 [recovered, repanicked]
panic({0x940ef0?, 0x29b705276138?})
=== mutant b_L_lt4_accepted reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.017s
=== mutant c_trailing_accepted: RED run (TestParseGroup)
--- FAIL: TestParseGroup (0.00s)
    movement_test.go:43: trailing bytes must fail
=== mutant c_trailing_accepted reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.007s
=== mutant d_overrun_accepted: RED run (TestParseGroup)
--- FAIL: TestParseGroup (0.00s)
panic: runtime error: slice bounds out of range [::114] with capacity 20 [recovered, repanicked]
panic({0x940ef0?, 0xf6671388078?})
=== mutant d_overrun_accepted reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.012s
=== mutant e_badbytes_dropped: RED run (TestDecodeMovement_Synthetic)
--- FAIL: TestDecodeMovement_Synthetic (0.00s)
    movement_test.go:84: stats = {Envelopes:3 Groups:2 Records:2 BadEnvelopes:1 BadBytes:0 FirstTick:1 LastTick:3 TicksMonotonic:true} groups=2
=== mutant e_badbytes_dropped reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.005s
=== mutant f_monotonic_dropped: RED run (TestDecodeMovement_TicksNotMonotonic)
--- FAIL: TestDecodeMovement_TicksNotMonotonic (0.00s)
    movement_test.go:186: stats = {Envelopes:2 Groups:2 Records:2 BadEnvelopes:0 BadBytes:0 FirstTick:5 LastTick:4 TicksMonotonic:true} err=<nil>
=== mutant f_monotonic_dropped reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.005s
=== mutant g_emit_err_swallowed: RED run (TestDecodeMovement_EmitError)
--- FAIL: TestDecodeMovement_EmitError (0.00s)
    movement_test.go:96: err = <nil>, want emit error
=== mutant g_emit_err_swallowed reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.004s
=== mutant h_offset_wrong: RED run (TestParseGroup)
--- FAIL: TestParseGroup (0.00s)
    movement_test.go:30: offsets = 2, 21
=== mutant h_offset_wrong reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	0.005s
=== mutant [s/st.Records += len(g.Records)/st.Records += 0/]: RED run (Fixtures|Truncated)
--- FAIL: TestDecodeMovement_Fixtures (1.21s)
    --- FAIL: TestDecodeMovement_Fixtures/custom_early_plant.rec (0.15s)
    --- FAIL: TestDecodeMovement_Fixtures/custom_solid_snake.rec (0.22s)
    --- FAIL: TestDecodeMovement_Fixtures/custom_clubhouse_short.rec (0.25s)
    --- FAIL: TestDecodeMovement_Fixtures/custom_fortress_plant.rec (0.30s)
    --- FAIL: TestDecodeMovement_Fixtures/custom_lair_plant_stop.rec (0.29s)
--- FAIL: TestDecodeMovement_Truncated (0.22s)
=== reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	1.520s
=== mutant [s/prev, st.LastTick = e.Tick, e.Tick/prev = e.Tick/]: RED run (Fixtures|Truncated)
--- FAIL: TestDecodeMovement_Truncated (0.26s)
=== reverted: GREEN run
ok  	github.com/redraskal/r6-dissect/dissect/wire	1.381s
```
