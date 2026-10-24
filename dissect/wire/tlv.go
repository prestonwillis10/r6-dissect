package wire

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Op is a TLV token opcode.
type Op byte

const (
	OpBatch             Op = 0x18 // handle8 field4 u32 count
	OpRef               Op = 0x1A // field4 handle8 class4
	OpRefNamed          Op = 0x1B // handle8 field4 handle8 class4
	OpRefIndexed        Op = 0x1E // field4 index4 handle8 class4
	OpRefNamedIndexed   Op = 0x1F // handle8 field4 index4 handle8 class4
	OpValue             Op = 0x22 // field4 len1 payload
	OpValueNamed        Op = 0x23 // handle8 field4 len1 payload
	OpValueIndexed      Op = 0x26 // field4 index4 len1 payload
	OpValueNamedIndexed Op = 0x27 // handle8 field4 index4 len1 payload
)

func (o Op) String() string {
	switch o {
	case OpBatch:
		return "batch"
	case OpRef:
		return "ref"
	case OpRefNamed:
		return "refNamed"
	case OpRefIndexed:
		return "refIndexed"
	case OpRefNamedIndexed:
		return "refNamedIndexed"
	case OpValue:
		return "value"
	case OpValueNamed:
		return "valueNamed"
	case OpValueIndexed:
		return "valueIndexed"
	case OpValueNamedIndexed:
		return "valueNamedIndexed"
	}
	return fmt.Sprintf("op0x%02x", byte(o))
}

// IsRef reports whether o is one of the four reference ops.
func (o Op) IsRef() bool {
	return o == OpRef || o == OpRefNamed || o == OpRefIndexed || o == OpRefNamedIndexed
}

// IsValue reports whether o is one of the four value ops.
func (o Op) IsValue() bool {
	return o == OpValue || o == OpValueNamed || o == OpValueIndexed || o == OpValueNamedIndexed
}

// Token is one decoded TLV token.
type Token struct {
	Offset   int     // offset of the opcode byte in the buffer
	Op       Op      //
	Object   Handle  // the object this token applies to (named handle, or the current object)
	Field    FieldID //
	Index    uint32  // for indexed ops
	HasIndex bool    //
	Value    []byte  // value ops: slice of the buffer (may be empty)
	Target   Handle  // ref ops: referenced object (may be null)
	Class    ClassID // ref ops: class of Target
	Count    uint32  // OpBatch
}

// Uint32 returns the value as a little-endian u32; ok when len(Value) == 4.
func (t Token) Uint32() (uint32, bool) {
	if len(t.Value) != 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(t.Value), true
}

// Float32 returns the value as a little-endian f32; ok when len(Value) == 4.
func (t Token) Float32() (float32, bool) {
	u, ok := t.Uint32()
	return math.Float32frombits(u), ok
}

// Uint64 returns the value as a little-endian u64; ok when len(Value) == 8.
func (t Token) Uint64() (uint64, bool) {
	if len(t.Value) != 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint64(t.Value), true
}

// ValueHandle returns the value as an object handle; ok when len(Value) == 8.
func (t Token) ValueHandle() (Handle, bool) {
	if len(t.Value) != 8 {
		return Handle{}, false
	}
	return handleAt(t.Value, 0), true
}

// DecodeError is a structural TLV error.
type DecodeError struct {
	Offset int
	Op     byte
	Reason string
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("wire: %s at offset %d (op 0x%02x)", e.Reason, e.Offset, e.Op)
}

// Block is the absolute buffer bounds of one `u32 C` block payload inside a TLV envelope.
type Block struct{ Start, End int }

// Blocks splits a TLV envelope payload. The grammar observed on every TLV
// envelope of the Y11 fixtures (PR #5 review) is
//
//	payload := 0x04 slot slot slot slot      exactly four slots
//	slot    := u32 C  bytes(C)               C may be 0 (an empty slot)
//
// so the "12 zero bytes" the findings doc describes are three empty slots,
// "8 zero bytes" are two, and a zero word between blocks is one. Only the
// non-empty slots are returned, with absolute offsets (e.Start + 12 is the
// payload start). A payload with fewer or more than four slots, a slot that
// overruns the payload, or a missing leading 0x04 is a *DecodeError.
func Blocks(e Envelope) ([]Block, error) {
	p := e.Payload
	base := e.Start + 12
	if len(p) < 1 {
		return nil, &DecodeError{Offset: base, Reason: "payload too short"}
	}
	if p[0] != 0x04 {
		return nil, &DecodeError{Offset: base, Op: p[0], Reason: "payload does not start with 0x04"}
	}
	var out []Block
	pos := 1
	for slot := 0; slot < 4; slot++ {
		if len(p)-pos < 4 {
			return out, &DecodeError{Offset: base + pos, Reason: "fewer than four length slots"}
		}
		c := int(binary.LittleEndian.Uint32(p[pos:]))
		pos += 4
		if c == 0 {
			continue
		}
		if c > len(p)-pos {
			return out, &DecodeError{Offset: base + pos - 4, Reason: "block overruns payload"}
		}
		out = append(out, Block{Start: base + pos, End: base + pos + c})
		pos += c
	}
	if pos != len(p) {
		return out, &DecodeError{Offset: base + pos, Reason: "bytes after the fourth slot"}
	}
	return out, nil
}

// DecodeBlock walks the tokens of b[blk.Start:blk.End], tracking the current object:
// OpBatch, OpValueNamed and OpValueNamedIndexed set it to their handle; the ref ops set it to Target when
// Target is non-null, otherwise (named refs) to their handle. A zero-length value and a null target of an
// unnamed ref leave the current object unchanged. emit returning a non-nil error stops the walk.
// An unknown opcode or a token overrunning blk.End returns *DecodeError.
func DecodeBlock(b []byte, blk Block, emit func(Token) error) error {
	if blk.Start < 0 || blk.End > len(b) || blk.Start > blk.End {
		return &DecodeError{Offset: blk.Start, Reason: "block outside buffer"}
	}
	var cur Handle
	p := blk.Start
	for p < blk.End {
		t := Token{Offset: p, Op: Op(b[p])}
		overrun := func() error {
			return &DecodeError{Offset: p, Op: b[p], Reason: "token overruns block"}
		}
		q := p + 1
		named, indexed := false, false
		switch t.Op {
		case OpBatch:
			named = true
		case OpRef, OpValue:
		case OpRefNamed, OpValueNamed:
			named = true
		case OpRefIndexed, OpValueIndexed:
			indexed = true
		case OpRefNamedIndexed, OpValueNamedIndexed:
			named, indexed = true, true
		default:
			return &DecodeError{Offset: p, Op: b[p], Reason: "unknown opcode"}
		}
		if named {
			if blk.End-q < 8 {
				return overrun()
			}
			t.Object = handleAt(b, q)
			q += 8
		} else {
			t.Object = cur
		}
		if blk.End-q < 4 {
			return overrun()
		}
		copy(t.Field[:], b[q:q+4])
		q += 4
		if indexed {
			if blk.End-q < 4 {
				return overrun()
			}
			t.Index, t.HasIndex = binary.LittleEndian.Uint32(b[q:]), true
			q += 4
		}
		switch {
		case t.Op == OpBatch:
			if blk.End-q < 4 {
				return overrun()
			}
			t.Count = binary.LittleEndian.Uint32(b[q:])
			q += 4
		case t.Op.IsRef():
			if blk.End-q < 12 {
				return overrun()
			}
			t.Target = handleAt(b, q)
			copy(t.Class[:], b[q+8:q+12])
			q += 12
		default:
			if blk.End-q < 1 {
				return overrun()
			}
			n := int(b[q])
			q++
			if blk.End-q < n {
				return overrun()
			}
			t.Value = b[q : q+n : q+n]
			q += n
		}
		switch {
		case t.Op == OpBatch, t.Op == OpValueNamed, t.Op == OpValueNamedIndexed:
			cur = t.Object
		case t.Op.IsRef():
			if !t.Target.IsNull() {
				cur = t.Target
			} else if named {
				cur = t.Object
			}
		}
		p = q
		if err := emit(t); err != nil {
			return err
		}
	}
	return nil
}

// TLVStats counts what DecodeTLV walked.
type TLVStats struct {
	Envelopes, Blocks, Tokens int
	Errors                    []DecodeError
}

// DecodeTLV decodes every envelope of every PayloadTLV channel in l, in buffer order; emit receives the envelope
// tick with each token. Decode errors are collected per block (the block is abandoned, the next one continues).
// A nil emit only counts. A non-nil error from emit stops the walk and is returned.
func DecodeTLV(b []byte, l Layout, emit func(tick uint32, t Token) error) (TLVStats, error) {
	var st TLVStats
	for _, c := range l.ChannelsOf(PayloadTLV) {
		for _, e := range c.Envelopes {
			st.Envelopes++
			blocks, err := Blocks(e)
			if err != nil {
				if de, ok := err.(*DecodeError); ok {
					st.Errors = append(st.Errors, *de)
				}
			}
			for _, blk := range blocks {
				st.Blocks++
				var emitErr error
				err := DecodeBlock(b, blk, func(t Token) error {
					st.Tokens++
					if emit != nil {
						emitErr = emit(e.Tick, t)
					}
					return emitErr
				})
				if emitErr != nil {
					return st, emitErr
				}
				if de, ok := err.(*DecodeError); ok {
					st.Errors = append(st.Errors, *de)
				} else if err != nil {
					return st, err
				}
			}
		}
	}
	return st, nil
}
