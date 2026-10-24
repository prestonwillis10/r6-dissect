package wire

import (
	"encoding/binary"
	"errors"
)

// PayloadKind is the sniffed format of a channel's envelope payloads.
type PayloadKind int

const (
	PayloadUnknown  PayloadKind = iota
	PayloadTLV                  // payload[0] == 0x04 and payload ends with 12 zero bytes
	PayloadMovement             // payload = u16 count + count records whose tag ends 73 85 fe
)

func (k PayloadKind) String() string {
	switch k {
	case PayloadTLV:
		return "tlv"
	case PayloadMovement:
		return "movement"
	}
	return "unknown"
}

// Envelope is one tick-stamped payload: u32 tick, u32 len, u32 0, payload[len].
type Envelope struct {
	Start, End int // [Start, End) of the whole envelope in the buffer
	Tick       uint32
	Payload    []byte // slice of the buffer, len == the envelope's len field
}

// Channel is a run of envelopes introduced by a 45-byte header (or, for the
// leading channel, by nothing).
type Channel struct {
	Start, End int     // header start to end of the last envelope
	HasHeader  bool    // false for the leading channel that starts without a header
	ID         [8]byte // the header's id8 (zero when HasHeader is false)
	Kind       PayloadKind
	Envelopes  []Envelope
}

// Gap is a run of bytes not covered by a channel header or envelope.
type Gap struct{ Start, End int }

// Layout is the result of Walk.
type Layout struct {
	Size     int
	Prelude  int // bytes before the first channel or envelope
	Channels []Channel
	Gaps     []Gap // excludes the prelude
}

// UnplacedBytes is Prelude plus the size of every gap.
func (l Layout) UnplacedBytes() int {
	n := l.Prelude
	for _, g := range l.Gaps {
		n += g.End - g.Start
	}
	return n
}

// ChannelsOf returns the channels whose Kind == kind, in buffer order.
func (l Layout) ChannelsOf(kind PayloadKind) []Channel {
	var out []Channel
	for _, c := range l.Channels {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}

// ErrNoEnvelopes is returned when the buffer holds no recognizable channel or envelope.
var ErrNoEnvelopes = errors.New("wire: no channels or envelopes found")

const channelHeaderLen = 45

// header is a parsed channel header.
type header struct {
	lead  uint32
	count uint32 // the channel carries count-2 envelopes
	id    [8]byte
}

// parseHeader checks the observed 45-byte header grammar at p:
//
//	off 0..3    u32 lead: the same for every header of a file, >= every tick
//	off 4       free byte
//	off 5..11   zero
//	off 12..19  id8, not all zero
//	off 20..23  free u32
//	off 24      zero
//	off 25..28  u32 count >= 2; count-2 envelopes follow
//	off 29..32  zero
//	off 33..36  free u32 (non-zero for some channels)
//	off 37..44  zero
//
// The lead is a small number: a non-zero low half and a zero high half.
func parseHeader(b []byte, p int) (h header, ok bool) {
	if p < 0 || p > len(b)-channelHeaderLen {
		return h, false
	}
	x := b[p : p+channelHeaderLen]
	if (x[0] == 0 && x[1] == 0) || x[2] != 0 || x[3] != 0 || !allZero(x[5:12]) ||
		x[24] != 0 || !allZero(x[29:33]) || !allZero(x[37:45]) {
		return h, false
	}
	copy(h.id[:], x[12:20])
	if h.id == ([8]byte{}) {
		return h, false
	}
	h.lead = binary.LittleEndian.Uint32(x)
	h.count = binary.LittleEndian.Uint32(x[25:])
	return h, h.count >= 2
}

// ParseChannelHeaderAt reports whether a 45-byte channel header starts at p and
// returns its id8. It checks the shape only; Walk also checks the chain of
// envelopes behind the header, which is what tells a real header from a
// header-shaped run of data.
func ParseChannelHeaderAt(b []byte, p int) (id [8]byte, ok bool) {
	h, ok := parseHeader(b, p)
	if !ok {
		return id, false
	}
	return h.id, true
}

// ParseEnvelopeAt parses u32 tick, u32 len, u32 zero, payload[len] at p.
// ok is false when p+12 > len(b), the zero word is not zero, len == 0, or p+12+len > len(b).
func ParseEnvelopeAt(b []byte, p int) (Envelope, bool) {
	if p < 0 || p > len(b)-12 {
		return Envelope{}, false
	}
	if binary.LittleEndian.Uint32(b[p+8:]) != 0 {
		return Envelope{}, false
	}
	n := int(binary.LittleEndian.Uint32(b[p+4:]))
	if n == 0 || n > len(b)-p-12 {
		return Envelope{}, false
	}
	return Envelope{
		Start:   p,
		End:     p + 12 + n,
		Tick:    binary.LittleEndian.Uint32(b[p:]),
		Payload: b[p+12 : p+12+n : p+12+n],
	}, true
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// sniffPayload classifies one envelope payload.
func sniffPayload(p []byte) PayloadKind {
	if len(p) >= 13 && p[0] == 0x04 && allZero(p[len(p)-12:]) {
		return PayloadTLV
	}
	// u16 count, then record 0: handle8 u32 L tag4; tag = 6x 73 85 fe.
	if len(p) >= 2+12+4 && binary.LittleEndian.Uint16(p) > 0 &&
		p[2+8+4+1] == 0x73 && p[2+8+4+2] == 0x85 && p[2+8+4+3] == 0xfe {
		return PayloadMovement
	}
	return PayloadUnknown
}

// tickOK reports whether an envelope tick fits a channel: not after the lead
// and not before the previous envelope.
func tickOK(tick, last, lead uint32) bool { return tick <= lead && tick >= last }

type envStatus int

const (
	envOK      envStatus = iota
	envBad               // does not fit the channel: a gap starts here
	envOverrun           // runs past the end of the buffer: the buffer was cut here
)

// nextEnvelope parses the envelope at q for a channel with the given lead whose
// previous tick is last.
func nextEnvelope(b []byte, q int, lead, last uint32) (Envelope, envStatus) {
	if q > len(b)-12 {
		return Envelope{}, envOverrun
	}
	tick := binary.LittleEndian.Uint32(b[q:])
	n := int(binary.LittleEndian.Uint32(b[q+4:]))
	if binary.LittleEndian.Uint32(b[q+8:]) != 0 || n == 0 || !tickOK(tick, last, lead) {
		return Envelope{}, envBad
	}
	if n > len(b)-q-12 {
		return Envelope{}, envOverrun
	}
	e, _ := ParseEnvelopeAt(b, q)
	return e, envOK
}

// isTerminator reports whether the 12 bytes at q are the buffer end: the lead
// plus 8 zero bytes.
func isTerminator(b []byte, q int, lead uint32) bool {
	return q == len(b)-12 && binary.LittleEndian.Uint32(b[q:]) == lead && allZero(b[q+4:])
}

// sameLeadHeaderAt reports whether a header with the given lead starts at q.
func sameLeadHeaderAt(b []byte, q int, lead uint32) bool {
	h, ok := parseHeader(b, q)
	return ok && h.lead == lead
}

// firstHeaderProbe bounds the work spent on one first-header candidate: at
// most this many envelopes are chained before the candidate is accepted. A
// channel with fewer envelopes is checked to its end (the next header or the
// terminator). The bound keeps the scan linear in the buffer whatever the
// input: without it, crafted inputs whose header-shaped runs share long
// envelope runs cost a chain walk per candidate (quadratic time), and the
// memo that avoids re-walking them grows with the number of distinct leads.
// A candidate with firstHeaderProbe chained envelopes behind a strict header
// is a channel by the grammar; if its chain breaks later the main loop
// records the break as a gap.
const firstHeaderProbe = 64

// validFirstHeader accepts the header at p as the opening one of the buffer
// when its envelopes parse with non-decreasing ticks no later than the lead:
// all count-2 of them, followed by a same-lead header or the terminator, when
// the channel has at most firstHeaderProbe envelopes; the first
// firstHeaderProbe otherwise. A buffer cut inside the envelopes is accepted
// when at least two parsed, and one cut inside the next header when the
// channel is complete and that header is a stub.
func validFirstHeader(b []byte, p int) bool {
	h, ok := parseHeader(b, p)
	if !ok {
		return false
	}
	m := int(h.count) - 2
	q, last := p+channelHeaderLen, uint32(0)
	for i := 0; i < m && i < firstHeaderProbe; i++ {
		e, st := nextEnvelope(b, q, h.lead, last)
		switch st {
		case envBad:
			return false
		case envOverrun:
			return i >= 2
		}
		q, last = e.End, e.Tick
	}
	if m > firstHeaderProbe {
		return true
	}
	return isTerminator(b, q, h.lead) || sameLeadHeaderAt(b, q, h.lead) ||
		(len(b)-q < channelHeaderLen && m >= 2)
}

// leadingRun finds the headerless channel directly in front of the first
// header at first: the earliest start from which envelopes with non-decreasing
// ticks no later than lead run exactly up to first. It returns -1 when none
// does. One pass from the right: a start is good when its envelope ends at
// first, or at a good start whose tick is not below its own.
func leadingRun(b []byte, first int, lead uint32) int {
	good := make([]bool, first+1)
	good[first] = true
	best := -1
	for s := first - 1; s >= 0; s-- {
		e, st := nextEnvelope(b[:first], s, lead, 0)
		if st != envOK || !good[e.End] {
			continue
		}
		if e.End != first && e.Tick > binary.LittleEndian.Uint32(b[e.End:]) {
			continue
		}
		good[s], best = true, s
	}
	return best
}

// Walk splits the decompressed buffer (dissect.Reader.Data()) into channels and
// envelopes. Every header of a file shares one lead; a header says how many
// envelopes follow it, and after them comes the next header or the terminator.
// Where that does not hold the bytes become a gap, and the walk resumes at the
// next same-lead header; envelopes are never picked out of a gap. A buffer with
// no valid first header (see validFirstHeader) yields ErrNoEnvelopes.
func Walk(b []byte) (Layout, error) {
	first := -1
	for p := 0; p <= len(b)-channelHeaderLen; p++ {
		if validFirstHeader(b, p) {
			first = p
			break
		}
	}
	if first < 0 {
		return Layout{Size: len(b), Prelude: len(b)}, ErrNoEnvelopes
	}
	h0, _ := parseHeader(b, first)
	lead := h0.lead
	l := Layout{Size: len(b), Prelude: first}

	if s := leadingRun(b, first, lead); s >= 0 {
		c := Channel{Start: s, End: first}
		for q := s; q < first; {
			e, _ := nextEnvelope(b, q, lead, 0)
			c.Envelopes = append(c.Envelopes, e)
			q = e.End
		}
		c.Kind = sniffPayload(c.Envelopes[0].Payload)
		l.Prelude = s
		l.Channels = append(l.Channels, c)
	}

	p := first
	for {
		h, _ := parseHeader(b, p)
		c := Channel{Start: p, End: p + channelHeaderLen, HasHeader: true, ID: h.id}
		q, last := c.End, uint32(0)
		gapAt := -1 // where a mismatch starts a gap, or -1
		for i := 0; i < int(h.count)-2 && gapAt < 0; i++ {
			e, st := nextEnvelope(b, q, h.lead, last)
			switch st {
			case envOK:
				if len(c.Envelopes) == 0 {
					c.Kind = sniffPayload(e.Payload)
				}
				c.Envelopes = append(c.Envelopes, e)
				c.End, last, q = e.End, e.Tick, e.End
			case envBad:
				gapAt = q
			case envOverrun:
				l.Channels = append(l.Channels, c)
				if q < len(b) {
					l.Gaps = append(l.Gaps, Gap{Start: q, End: len(b)})
				}
				return l, nil
			}
		}
		l.Channels = append(l.Channels, c)
		if gapAt < 0 {
			switch {
			case q == len(b):
				return l, nil
			case isTerminator(b, q, lead):
				l.Gaps = append(l.Gaps, Gap{Start: q, End: len(b)})
				return l, nil
			case sameLeadHeaderAt(b, q, lead):
				p = q
				continue
			}
			gapAt = q
		}
		r := gapAt
		for r <= len(b)-channelHeaderLen && !sameLeadHeaderAt(b, r, lead) {
			r++
		}
		if r > len(b)-channelHeaderLen {
			r = len(b)
		}
		if r > gapAt {
			l.Gaps = append(l.Gaps, Gap{Start: gapAt, End: r})
		}
		if r == len(b) {
			return l, nil
		}
		p = r
	}
}
