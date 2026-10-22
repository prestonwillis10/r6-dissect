package wire_test

import (
	"errors"
	"path/filepath"
	"testing"

	wire "github.com/redraskal/r6-dissect/dissect/wire"
)

func tlvBatch(o wire.Handle, f wire.FieldID, n uint32) []byte {
	return concat([]byte{0x18}, o[:], f[:], le32(n))
}
func tlvValue(f wire.FieldID, v []byte) []byte {
	return concat([]byte{0x22}, f[:], []byte{byte(len(v))}, v)
}
func tlvValueNamed(o wire.Handle, f wire.FieldID, v []byte) []byte {
	return concat([]byte{0x23}, o[:], f[:], []byte{byte(len(v))}, v)
}
func tlvValueIndexed(f wire.FieldID, i uint32, v []byte) []byte {
	return concat([]byte{0x26}, f[:], le32(i), []byte{byte(len(v))}, v)
}
func tlvValueNamedIndexed(o wire.Handle, f wire.FieldID, i uint32, v []byte) []byte {
	return concat([]byte{0x27}, o[:], f[:], le32(i), []byte{byte(len(v))}, v)
}
func tlvRef(f wire.FieldID, tg wire.Handle, c wire.ClassID) []byte {
	return concat([]byte{0x1a}, f[:], tg[:], c[:])
}
func tlvRefNamed(o wire.Handle, f wire.FieldID, tg wire.Handle, c wire.ClassID) []byte {
	return concat([]byte{0x1b}, o[:], f[:], tg[:], c[:])
}
func tlvRefIndexed(f wire.FieldID, i uint32, tg wire.Handle, c wire.ClassID) []byte {
	return concat([]byte{0x1e}, f[:], le32(i), tg[:], c[:])
}
func tlvRefNamedIndexed(o wire.Handle, f wire.FieldID, i uint32, tg wire.Handle, c wire.ClassID) []byte {
	return concat([]byte{0x1f}, o[:], f[:], le32(i), tg[:], c[:])
}

func tlvDecode(t *testing.T, blk []byte) []wire.Token {
	t.Helper()
	var got []wire.Token
	if err := wire.DecodeBlock(blk, wire.Block{Start: 0, End: len(blk)}, func(tk wire.Token) error { got = append(got, tk); return nil }); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDecodeBlock_EveryOpcode(t *testing.T) {
	h1, h2, h3 := h("a07305f0"), h("303b04f0"), h("111111f0")
	blk := concat(
		tlvBatch(h1, fid("1784e409"), 5),                                          // current = h1
		tlvValue(fid("00000005"), []byte{1}),                                      // on h1 (batch set it)
		tlvValueNamed(h2, fid("e58c06e9"), le32(2)),                               // current = h2
		tlvValue(fid("a9c858d9"), nil),                                            // zero-length value on h2
		tlvRefNamed(h1, fid("27c08dca"), h2, cid("b2216bf3")),                     // current = h2 (target)
		tlvRef(fid("1ed17fd6"), wire.Handle{}, wire.ClassID{}),                    // null target: current stays h2
		tlvRefIndexed(fid("9a984fa2"), 3, h1, cid("b430ebc4")),                    // current = h1
		tlvValueIndexed(fid("51b44c48"), 1, []byte{0}),                            // on h1
		tlvValueNamedIndexed(h3, fid("0badf00d"), 7, []byte{1, 2}),                // current = h3
		tlvValue(fid("00000004"), []byte{9}),                                      // on h3
		tlvRefNamedIndexed(h1, fid("0defaced"), 9, wire.Handle{}, wire.ClassID{}), // null target on named: current = h1
		tlvValue(fid("00000001"), []byte{9}),                                      // on h1
		tlvRefNamed(h2, fid("00000002"), wire.Handle{}, wire.ClassID{}),           // null target on named: current = h2
		tlvValue(fid("00000003"), []byte{9}),                                      // on h2
	)
	got := tlvDecode(t, blk)
	want := []struct {
		op  wire.Op
		obj wire.Handle
	}{{wire.OpBatch, h1}, {wire.OpValue, h1}, {wire.OpValueNamed, h2}, {wire.OpValue, h2}, {wire.OpRefNamed, h1}, {wire.OpRef, h2},
		{wire.OpRefIndexed, h2}, {wire.OpValueIndexed, h1}, {wire.OpValueNamedIndexed, h3}, {wire.OpValue, h3}, {wire.OpRefNamedIndexed, h1},
		{wire.OpValue, h1}, {wire.OpRefNamed, h2}, {wire.OpValue, h2}}
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Op != w.op || got[i].Object != w.obj {
			t.Errorf("token %d: got %v on %v, want %v on %v", i, got[i].Op, got[i].Object, w.op, w.obj)
		}
	}
	if len(got[3].Value) != 0 {
		t.Error("zero-length value not empty")
	}
	if !got[5].Target.IsNull() {
		t.Error("ref 4 target not null")
	}
	if got[6].Index != 3 || !got[6].HasIndex || got[0].HasIndex {
		t.Error("index fields wrong")
	}
	if got[0].Count != 5 {
		t.Errorf("count %d", got[0].Count)
	}
	if got[4].Target != h2 || got[4].Class != cid("b2216bf3") || got[4].Field != fid("27c08dca") {
		t.Errorf("refNamed fields: %+v", got[4])
	}
	if got[8].Index != 7 || len(got[8].Value) != 2 {
		t.Errorf("valueNamedIndexed fields: %+v", got[8])
	}
	if got[0].Offset != 0 || got[1].Offset != 17 || got[2].Offset != 24 {
		t.Errorf("offsets %d %d %d", got[0].Offset, got[1].Offset, got[2].Offset)
	}
}

func TestDecodeBlock_UnknownOpcode(t *testing.T) {
	blk := concat(tlvValue(fid("1f07efc9"), le32(44)), []byte{0x99, 1, 2})
	err := wire.DecodeBlock(blk, wire.Block{Start: 0, End: len(blk)}, func(wire.Token) error { return nil })
	var de *wire.DecodeError
	if !errors.As(err, &de) || de.Offset != 10 || de.Op != 0x99 {
		t.Fatalf("%v", err)
	}
}

func TestDecodeBlock_Overrun(t *testing.T) {
	f := fid("1f07efc9")
	blk := concat([]byte{0x22}, f[:], []byte{9, 1, 2})
	err := wire.DecodeBlock(blk, wire.Block{Start: 0, End: len(blk)}, func(wire.Token) error { return nil })
	var de *wire.DecodeError
	if !errors.As(err, &de) || de.Reason != "token overruns block" {
		t.Fatal(err)
	}
	// The block end, not the buffer end, bounds the token: a valid value that would fit the buffer but not the block.
	full := concat(tlvValue(f, []byte{1, 2, 3, 4}), []byte{0xAA})
	err = wire.DecodeBlock(full, wire.Block{Start: 0, End: 8}, func(wire.Token) error { return nil })
	if !errors.As(err, &de) || de.Reason != "token overruns block" {
		t.Fatalf("block-bounded: %v", err)
	}
	// Every truncation of a token of each opcode is an error, never a panic.
	h1 := h("a07305f0")
	for _, tok := range [][]byte{
		tlvBatch(h1, f, 1), tlvRef(f, h1, cid("b2216bf3")), tlvRefNamed(h1, f, h1, cid("b2216bf3")),
		tlvRefIndexed(f, 1, h1, cid("b2216bf3")), tlvRefNamedIndexed(h1, f, 1, h1, cid("b2216bf3")),
		tlvValue(f, []byte{1}), tlvValueNamed(h1, f, []byte{1}), tlvValueIndexed(f, 1, []byte{1}), tlvValueNamedIndexed(h1, f, 1, []byte{1}),
	} {
		for n := 1; n < len(tok); n++ {
			err := wire.DecodeBlock(tok[:n], wire.Block{Start: 0, End: n}, func(wire.Token) error { return nil })
			if !errors.As(err, &de) || de.Reason != "token overruns block" {
				t.Errorf("op %#x truncated to %d: %v", tok[0], n, err)
			}
		}
	}
	// A block outside the buffer is an error.
	if err := wire.DecodeBlock(blk, wire.Block{Start: 0, End: len(blk) + 1}, func(wire.Token) error { return nil }); err == nil {
		t.Error("block past buffer accepted")
	}
}

func TestDecodeBlock_EmitError(t *testing.T) {
	blk := concat(tlvValue(fid("1f07efc9"), le32(1)), tlvValue(fid("1f07efc9"), le32(2)))
	stop := errors.New("stop")
	n := 0
	err := wire.DecodeBlock(blk, wire.Block{Start: 0, End: len(blk)}, func(wire.Token) error { n++; return stop })
	if err != stop || n != 1 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestBlocks(t *testing.T) {
	buf := envBytes(7, concat([]byte{4}, le32(3), []byte("abc"), le32(1), []byte("d"), make([]byte, 8))) // B B Z Z: four slots
	e, ok := wire.ParseEnvelopeAt(buf, 0)
	if !ok {
		t.Fatal("envelope")
	}
	blocks, err := wire.Blocks(e)
	want := []wire.Block{{Start: 17, End: 20}, {Start: 24, End: 25}}
	if err != nil || len(blocks) != 2 || blocks[0] != want[0] || blocks[1] != want[1] {
		t.Fatalf("%v %v", blocks, err)
	}
	// An envelope at a non-zero offset gets absolute bounds.
	shifted, _ := wire.ParseEnvelopeAt(concat(make([]byte, 100), buf), 100)
	if bs, _ := wire.Blocks(shifted); len(bs) != 2 || bs[0].Start != 117 {
		t.Errorf("absolute offsets: %v", bs)
	}
	bad := map[string][]byte{
		"dangling after zeros": concat([]byte{4}, le32(1), []byte("d"), make([]byte, 11), []byte{1}),
		"no leading 04":        concat([]byte{5}, le32(1), []byte("d"), make([]byte, 12)),
		"block overruns":       concat([]byte{4}, le32(99), []byte("d"), make([]byte, 12)),
		"short header":         concat([]byte{4}, []byte{1, 0}, make([]byte, 12)),
		"dangling 3 bytes":     concat([]byte{4}, le32(1), []byte("d"), []byte{0, 0, 0}),
		"three slots":          concat([]byte{4}, le32(1), []byte("d"), make([]byte, 8)),
		"five slots":           concat([]byte{4}, le32(1), []byte("d"), make([]byte, 16)),
	}
	for name, p := range bad {
		if _, err := wire.Blocks(envelope(1, p)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if bs, err := wire.Blocks(envelope(1, concat([]byte{4}, make([]byte, 16)))); err != nil || len(bs) != 0 { // four empty slots
		t.Errorf("empty payload: %v %v", bs, err)
	}
}

func TestToken_Accessors(t *testing.T) {
	tk := wire.Token{Value: le32(0x3f800000)}
	if v, ok := tk.Uint32(); !ok || v != 0x3f800000 {
		t.Error("Uint32")
	}
	if v, ok := tk.Float32(); !ok || v != 1.0 {
		t.Error("Float32")
	}
	if _, ok := (wire.Token{Value: make([]byte, 5)}).Uint32(); ok {
		t.Error("Uint32 accepted 5 bytes")
	}
	if _, ok := (wire.Token{Value: make([]byte, 5)}).Float32(); ok {
		t.Error("Float32 accepted 5 bytes")
	}
	h8 := h("a07305f0")
	v8 := concat(h8[:], nil)
	tk = wire.Token{Value: v8}
	if v, ok := tk.Uint64(); !ok || v != 0xf00573a0 {
		t.Errorf("Uint64 %x", v)
	}
	if hh, ok := tk.ValueHandle(); !ok || hh != h("a07305f0") {
		t.Error("ValueHandle")
	}
	if _, ok := (wire.Token{Value: le32(1)}).Uint64(); ok {
		t.Error("Uint64 accepted 4 bytes")
	}
	if _, ok := (wire.Token{Value: le32(1)}).ValueHandle(); ok {
		t.Error("ValueHandle accepted 4 bytes")
	}
}

func TestOp_Strings(t *testing.T) {
	if wire.OpRefNamed.String() != "refNamed" || wire.Op(0x99).String() != "op0x99" {
		t.Error("String")
	}
	if !wire.OpRef.IsRef() || wire.OpRef.IsValue() || !wire.OpValue.IsValue() || wire.OpBatch.IsRef() || wire.OpBatch.IsValue() {
		t.Error("IsRef/IsValue")
	}
	e := &wire.DecodeError{Offset: 10, Op: 0x99, Reason: "x"}
	if e.Error() != "wire: x at offset 10 (op 0x99)" {
		t.Error(e.Error())
	}
}

func TestDecodeTLV_Synthetic(t *testing.T) {
	good := concat([]byte{4}, le32(uint32(len(tlvValue(fid("1f07efc9"), le32(1))))), tlvValue(fid("1f07efc9"), le32(1)), make([]byte, 12))
	badBlk := concat([]byte{4}, le32(3), []byte{0x99, 1, 2}, make([]byte, 12))
	badEnv := concat([]byte{4}, make([]byte, 12), []byte{1}) // does not end in zeros
	hdr := channelHeaderN([8]byte{1}, testLead, 3, 0)
	buf := concat(hdr, envBytes(1, good), envBytes(2, badBlk), envBytes(3, concat(badEnv)), terminator(testLead))
	l, err := wire.Walk(buf)
	if err != nil {
		t.Fatal(err)
	}
	var ticks []uint32
	st, err := wire.DecodeTLV(buf, l, func(tick uint32, tk wire.Token) error { ticks = append(ticks, tick); return nil })
	if err != nil {
		t.Fatal(err)
	}
	// The bad block is abandoned and the walk continues with later envelopes.
	if st.Envelopes != 3 || st.Tokens != 1 || len(st.Errors) != 2 || len(ticks) != 1 || ticks[0] != 1 {
		t.Fatalf("%+v ticks=%v", st, ticks)
	}
	if st.Errors[0].Reason != "unknown opcode" {
		t.Errorf("first error %v", st.Errors[0])
	}
}

func TestDecodeTLV_Fixtures(t *testing.T) {
	for _, path := range fixtureRecs(t, "Y11S2", "Y11S3") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			b := decompress(t, path)
			l, err := wire.Walk(b)
			if err != nil {
				t.Fatal(err)
			}
			var tokens, frames, msFrames int
			var lastTick uint32
			var lastTickSet bool
			msField, timerField := fid("1837466c"), fid("1f07efc9")
			var timerObj wire.Handle
			st, err := wire.DecodeTLV(b, l, func(tick uint32, tk wire.Token) error {
				tokens++
				if tk.Op == wire.OpBatch {
					frames++
					if lastTickSet && tick < lastTick {
						t.Errorf("tick went backwards %d -> %d", lastTick, tick)
					}
					lastTick, lastTickSet = tick, true
				}
				if tk.Field == timerField && tk.Op.IsValue() {
					timerObj = tk.Object
				}
				if tk.Field == msField && tk.Op.IsValue() {
					msFrames++
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("envelopes=%d blocks=%d tokens=%d errors=%d batches=%d msClock=%d", st.Envelopes, st.Blocks, tokens, len(st.Errors), frames, msFrames)
			if st.Tokens != tokens {
				t.Errorf("stats tokens %d != emitted %d", st.Tokens, tokens)
			}
			if len(st.Errors) != 0 {
				t.Errorf("decode errors: %d, first %v", len(st.Errors), st.Errors[0])
			}
			if st.Envelopes < 1000 || tokens < 5000 {
				t.Errorf("envelopes=%d tokens=%d", st.Envelopes, tokens)
			}
			if float64(msFrames) < 0.95*float64(frames) {
				t.Errorf("ms clock present in %d of %d batches", msFrames, frames)
			}
			if timerObj.IsNull() {
				t.Error("round timer object not found")
			}
		})
	}
}

// The shapes seen on the Y11 fixtures besides one block plus 12 zero bytes.
func TestBlocks_ObservedVariants(t *testing.T) {
	z := le32(0)
	cases := []struct {
		name    string
		payload []byte
		want    []wire.Block
	}{
		{"BBZZ", concat([]byte{4}, le32(2), []byte("ab"), le32(1), []byte("c"), z, z), []wire.Block{{Start: 17, End: 19}, {Start: 23, End: 24}}},
		{"BBZB", concat([]byte{4}, le32(2), []byte("ab"), le32(1), []byte("c"), z, le32(1), []byte("d")), []wire.Block{{Start: 17, End: 19}, {Start: 23, End: 24}, {Start: 32, End: 33}}},
		{"BZZB", concat([]byte{4}, le32(1), []byte("a"), z, z, le32(1), []byte("b")), []wire.Block{{Start: 17, End: 18}, {Start: 30, End: 31}}},
		{"ZBZZ", concat([]byte{4}, z, le32(1), []byte("a"), z, z), []wire.Block{{Start: 21, End: 22}}},
	}
	for _, c := range cases {
		bs, err := wire.Blocks(envelope(1, c.payload))
		if err != nil || len(bs) != len(c.want) {
			t.Errorf("%s: %v %v", c.name, bs, err)
			continue
		}
		for i := range bs {
			if bs[i] != c.want[i] {
				t.Errorf("%s: block %d = %v, want %v", c.name, i, bs[i], c.want[i])
			}
		}
	}
}

// An emit error that happens to be a *DecodeError is still the consumer's
// error: it stops the walk and is returned, not recorded as a block error.
func TestDecodeTLV_EmitDecodeErrorStops(t *testing.T) {
	good := concat([]byte{4}, le32(10), tlvValue(fid("1f07efc9"), le32(1)), make([]byte, 12))
	buf := concat(channelHeaderN([8]byte{1}, testLead, 2, 0), envBytes(1, good), envBytes(2, good), terminator(testLead))
	l, err := wire.Walk(buf)
	if err != nil {
		t.Fatal(err)
	}
	stop := &wire.DecodeError{Reason: "consumer says stop"}
	calls := 0
	st, err := wire.DecodeTLV(buf, l, func(uint32, wire.Token) error { calls++; return stop })
	if err != stop || calls != 1 || len(st.Errors) != 0 {
		t.Fatalf("err=%v calls=%d stats=%+v", err, calls, st)
	}
}

// A buffer cut inside the last TLV envelope: Walk leaves the partial envelope
// out of the channel (a trailing gap), so DecodeTLV sees one envelope fewer
// and no error; the cut shows in Layout.Gaps, not in TLVStats.
func TestDecodeTLV_Truncated(t *testing.T) {
	b := decompress(t, fixture(t, "Y11S3", "custom_clubhouse_short.rec"))
	lf, err := wire.Walk(b)
	if err != nil {
		t.Fatal(err)
	}
	full, err := wire.DecodeTLV(b, lf, func(uint32, wire.Token) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	tlv := lf.ChannelsOf(wire.PayloadTLV)
	last := tlv[len(tlv)-1].Envelopes
	le := last[len(last)-1]
	cut := b[:(le.Start+le.End)/2]
	l, err := wire.Walk(cut)
	if err != nil {
		t.Fatal(err)
	}
	st, err := wire.DecodeTLV(cut, l, func(uint32, wire.Token) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if st.Envelopes != full.Envelopes-1 || len(st.Errors) != 0 {
		t.Errorf("truncated envelopes=%d errors=%d, full envelopes=%d", st.Envelopes, len(st.Errors), full.Envelopes)
	}
	if g := l.Gaps[len(l.Gaps)-1]; g.End != len(cut) || g.Start != le.Start {
		t.Errorf("last gap %v, want {%d %d}", g, le.Start, len(cut))
	}
}

// A nil emit counts without panicking, as DecodeMovement does.
func TestDecodeTLV_NilEmit(t *testing.T) {
	good := concat([]byte{4}, le32(10), tlvValue(fid("1f07efc9"), le32(1)), make([]byte, 12))
	buf := concat(channelHeaderN([8]byte{1}, testLead, 2, 0), envBytes(1, good), envBytes(2, good), terminator(testLead))
	l, err := wire.Walk(buf)
	if err != nil {
		t.Fatal(err)
	}
	st, err := wire.DecodeTLV(buf, l, nil)
	if err != nil || st.Envelopes != 2 || st.Tokens != 2 {
		t.Fatalf("err=%v stats=%+v", err, st)
	}
}
