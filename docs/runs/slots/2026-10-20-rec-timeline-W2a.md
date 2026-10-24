# W2a slot log: TLV token decoder

Branch `feat/rt01-tlv`, base `track/rec-timeline`. Code at `98985b4`.

## Gates

Suite at the code head:

```
ok  	github.com/redraskal/r6-dissect/dissect	0.012s
ok  	github.com/redraskal/r6-dissect/dissect/test	6.543s
ok  	github.com/redraskal/r6-dissect/dissect/wire	12.209s
```

Fixture decode (`TestDecodeTLV_Fixtures`):

```
    tlv_test.go:291: envelopes=2619 blocks=2661 tokens=21498 errors=0 batches=2661 msClock=2536
    tlv_test.go:291: envelopes=4398 blocks=4448 tokens=31469 errors=0 batches=4448 msClock=4342
    tlv_test.go:291: envelopes=5155 blocks=5200 tokens=32996 errors=0 batches=5200 msClock=5125
    tlv_test.go:291: envelopes=6123 blocks=6169 tokens=42007 errors=0 batches=6169 msClock=6037
    tlv_test.go:291: envelopes=5458 blocks=5502 tokens=39243 errors=0 batches=5502 msClock=5377
```

## Deviation from the plan: Blocks grammar

The plan and findings say one or two blocks followed by 12 zero bytes. The first test run on the five fixtures showed otherwise; envelopes whose payload does not end in 12 zero bytes:

```
custom_early_plant.rec       tlvChannels=1 envelopes=2619 not04=0 no12zeroTail=33
custom_solid_snake.rec       tlvChannels=1 envelopes=4398 not04=0 no12zeroTail=39
custom_clubhouse_short.rec   tlvChannels=1 envelopes=5155 not04=0 no12zeroTail=35
custom_fortress_plant.rec    tlvChannels=1 envelopes=6123 not04=0 no12zeroTail=36
custom_lair_plant_stop.rec   tlvChannels=1 envelopes=5458 not04=0 no12zeroTail=35
```

The observed shapes (B = block, Z = zero u32 word) over all five fixtures: 23575 BZZZ, 124 BBZZ, 50 BBZB, 3 BZZB, 1 ZBZZ. `Blocks` therefore skips zero words after the leading 0x04 and requires the payload to be consumed exactly. Overruns, a missing 0x04 and dangling bytes stay errors.

## Mutation proof

Raw output: `/tmp/claude-1001/w2a/mutation-final.txt`. Twelve mutants, each red with the named test and green after revert.

## Candidates

- Findings doc section 3 and the walker's TLV sniff assume a 12-zero-byte trailer.
  Observed: the premises table above, 33 to 39 envelopes per fixture end in fewer than 12 zero bytes.
  Acceptance: findings doc describes the zero-word grammar; sniff does not rely on the trailer for envelopes after the first.
  Verified-against: 98985b4
