# W3 slot log: dissect-lab CLI and coverage report

Branch feat/rt01-dissect-lab, base track/rec-timeline. Tasks 5 and 6 of plan 01.

## Deviations from the plan

- DecodeTLV calls its emit callback unconditionally, so coverage passes a no-op instead of nil. The wire files are frozen for this slot.
- Output is buffered and written only on success, so a failure after the first line still leaves stdout empty.
- TestRun_Errors also pins exit codes (2 for usage, 1 for a missing file) and message text. With only a non-zero check, a directory mutant is equivalent, because reading a directory fails with exit 1 anyway.
- Busy entities are not in the coverage JSON, which keeps the plan's shape. They were computed from the movement command output.

## Mutation proof (raw: /tmp/claude-1001/w3-mutants.txt)

```
=== a-limit-ignored
--- FAIL: TestRun_TLVLimit (0.23s)
=== b-unknown-exit0
--- FAIL: TestRun_Errors (0.00s)
=== c-dir-accepted
=== d-nopayload-ignored
--- FAIL: TestRun_Movement (0.35s)
=== c-dir-accepted (retry)
--- FAIL: TestRun_Errors (0.00s)
```

## Coverage capture (raw: /tmp/claude-1001/w3-coverage.txt)

```
## Y11S2/custom_early_plant
{"channels": 11, "movement": {"badBytes": 0, "envelopes": 2707, "groups": 2707, "records": 60796, "ticksMonotonic": true}, "size": 27619511, "tlv": {"blocks": 2661, "envelopes": 2619, "errors": 0, "tokens": 21498}, "unplacedFraction": 0.0029448023174631875}
entities 441 busy3000 0 busy1500 17
size_bytes 3499541
## Y11S2/custom_solid_snake
{"channels": 11, "movement": {"badBytes": 0, "envelopes": 4545, "groups": 4545, "records": 109106, "ticksMonotonic": true}, "size": 49710628, "tlv": {"blocks": 4448, "envelopes": 4398, "errors": 0, "tokens": 31469}, "unplacedFraction": 0.001705470307073972}
entities 474 busy3000 16 busy1500 31
size_bytes 5836660
## Y11S3/custom_clubhouse_short
{"channels": 10, "movement": {"badBytes": 0, "envelopes": 5419, "groups": 5419, "records": 126380, "ticksMonotonic": true}, "size": 53549064, "tlv": {"blocks": 5200, "envelopes": 5155, "errors": 0, "tokens": 32996}, "unplacedFraction": 0.0015641356495045367}
entities 583 busy3000 14 busy1500 36
size_bytes 6928486
## Y11S3/custom_fortress_plant
{"channels": 10, "movement": {"badBytes": 0, "envelopes": 6320, "groups": 6320, "records": 213613, "ticksMonotonic": true}, "size": 76179853, "tlv": {"blocks": 6169, "envelopes": 6123, "errors": 0, "tokens": 42007}, "unplacedFraction": 0.0010913384146330658}
entities 507 busy3000 40 busy1500 51
size_bytes 10059296
## Y11S3/custom_lair_plant_stop
{"channels": 10, "movement": {"badBytes": 0, "envelopes": 5632, "groups": 5632, "records": 164853, "ticksMonotonic": true}, "size": 66236205, "tlv": {"blocks": 5502, "envelopes": 5458, "errors": 0, "tokens": 39243}, "unplacedFraction": 0.0012751636359601218}
entities 499 busy3000 25 busy1500 43
size_bytes 8710584
```

## Inline passes

Simplify and verify were done inline over the diff, with no nested agents.
