package wire

import "encoding/binary"

// Record is one entity record of a movement group.
type Record struct {
	Offset  int     // offset of the handle in the buffer
	Entity  Handle  //
	Tag     [4]byte // observed: 60|61|62|63 73 85 fe
	Payload []byte  // bytes after the tag (len L-4), a slice of the buffer
}

// Group is the parsed payload of one movement envelope.
type Group struct {
	Envelope Envelope
	Records  []Record
}

// recordHeaderLen is handle8 + u32 L.
const recordHeaderLen = 12

// ParseGroup parses a movement envelope payload: u16 count, then count x
// (handle8, u32 L, L bytes with L >= 4). ok is false when a record does not
// fit, when the records do not end exactly at the payload end, or when any tag
// does not end in 73 85 fe. Payloads are not interpreted.
func ParseGroup(e Envelope) (Group, bool) {
	p := e.Payload
	if len(p) < 2 {
		return Group{}, false
	}
	count := int(binary.LittleEndian.Uint16(p))
	pos := 2
	// Every record takes at least recordHeaderLen+4 bytes, which bounds the allocation.
	recs := make([]Record, 0, min(count, (len(p)-2)/(recordHeaderLen+4)))
	for i := 0; i < count; i++ {
		if len(p)-pos < recordHeaderLen {
			return Group{}, false
		}
		l := int(binary.LittleEndian.Uint32(p[pos+8:]))
		if l < 4 || l > len(p)-pos-recordHeaderLen {
			return Group{}, false
		}
		t := p[pos+recordHeaderLen : pos+recordHeaderLen+4]
		if t[1] != 0x73 || t[2] != 0x85 || t[3] != 0xfe {
			return Group{}, false
		}
		r := Record{
			Offset:  e.Start + 12 + pos,
			Entity:  handleAt(p, pos),
			Payload: p[pos+recordHeaderLen+4 : pos+recordHeaderLen+l : pos+recordHeaderLen+l],
		}
		copy(r.Tag[:], t)
		recs = append(recs, r)
		pos += recordHeaderLen + l
	}
	if pos != len(p) {
		return Group{}, false
	}
	return Group{Envelope: e, Records: recs}, true
}

// MovementStats is the coverage report of DecodeMovement.
type MovementStats struct {
	Envelopes, Groups, Records int
	BadEnvelopes               int // envelopes in movement channels that ParseGroup rejected
	BadBytes                   int // their payload bytes
	FirstTick, LastTick        uint32
	TicksMonotonic             bool // envelope ticks never decrease within a channel
}

// DecodeMovement parses every envelope of every PayloadMovement channel in l, in
// buffer order, and calls emit for each group that parses. A rejected envelope
// is counted, not fatal. The first error from emit stops the decode and is
// returned with the stats so far. A buffer cut short shows as fewer envelopes:
// Walk leaves a partial envelope out of its channel.
func DecodeMovement(b []byte, l Layout, emit func(Group) error) (MovementStats, error) {
	st := MovementStats{TicksMonotonic: true}
	seen := false
	for _, c := range l.ChannelsOf(PayloadMovement) {
		var prev uint32
		for i, e := range c.Envelopes {
			st.Envelopes++
			if !seen {
				st.FirstTick, seen = e.Tick, true
			}
			if i > 0 && e.Tick < prev {
				st.TicksMonotonic = false
			}
			prev, st.LastTick = e.Tick, e.Tick
			g, ok := ParseGroup(e)
			if !ok {
				st.BadEnvelopes++
				st.BadBytes += len(e.Payload)
				continue
			}
			st.Groups++
			st.Records += len(g.Records)
			if emit != nil {
				if err := emit(g); err != nil {
					return st, err
				}
			}
		}
	}
	return st, nil
}
