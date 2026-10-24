# Wire decoder coverage

Measured with `dissect-lab coverage` and `dissect-lab movement --no-payload` on the Y11S2 and Y11S3 fixtures, at commit `ed23219` (the decoders under test are unchanged since the merged `dissect/wire` package on `track/rec-timeline`).

Command per fixture, run from the repository root:

```bash
go run ./cmd/dissect-lab coverage dissect/test/data/replays/valid/<season>/<file>.rec
go run ./cmd/dissect-lab movement --no-payload dissect/test/data/replays/valid/<season>/<file>.rec
```

Busy entities are movement entities with at least the stated number of records over the round. The size column is the decompressed buffer that the walker sees.

| File | Size (bytes) | Unplaced % | TLV envelopes | TLV tokens | TLV errors | Movement records | Movement bad % | Busy entities (>= 3000) | Busy entities (>= 1500) |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Y11S2/custom_early_plant | 27619511 | 0.294 | 2619 | 21498 | 0 | 60796 | 0.000 | 0 | 17 |
| Y11S2/custom_solid_snake | 49710628 | 0.171 | 4398 | 31469 | 0 | 109106 | 0.000 | 16 | 31 |
| Y11S3/custom_clubhouse_short | 53549064 | 0.156 | 5155 | 32996 | 0 | 126380 | 0.000 | 14 | 36 |
| Y11S3/custom_fortress_plant | 76179853 | 0.109 | 6123 | 42007 | 0 | 213613 | 0.000 | 40 | 51 |
| Y11S3/custom_lair_plant_stop | 66236205 | 0.128 | 5458 | 39243 | 0 | 164853 | 0.000 | 25 | 43 |

`custom_early_plant` is a short round: it has few movement groups in total, and each group carries at most one record per entity, so no entity can reach the 3000 mark. Its busy count is therefore stated at 1500, and the 1500 column is given for every fixture for comparison.

## Check against the acceptance limits

| Limit | Result |
|---|---|
| Unplaced bytes at most 0.5 % | Every row passes. |
| TLV decode errors equal to 0 | Every row passes. |
| Movement bytes skipped at most 0.3 % | Every row passes; no movement envelope was rejected. |

Movement ticks are monotonic within each movement channel in every row. The unplaced bytes are the prelude before the first channel and the short tail after the last envelope.
