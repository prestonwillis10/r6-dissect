package dissect

import (
	"sort"

	wire "github.com/redraskal/r6-dissect/dissect/wire"
)

var (
	fieldRoundTimer   = wire.FieldID{0x1f, 0x07, 0xef, 0xc9} // seconds, read by readTime
	fieldRoundClockMs = wire.FieldID{0x18, 0x37, 0x46, 0x6c} // milliseconds remaining in the current phase
)

// msPhaseMaxPrepMs is the longest first sample that still labels the first phase as Prep.
const msPhaseMaxPrepMs = prepPhaseMax * 1000

// msPhaseJumpMs is how far a sample must differ from its predecessor, in either direction, to start a new phase.
const msPhaseJumpMs = 1000

// msPhase is one countdown of the millisecond round clock. BaseMs is the elapsed
// milliseconds at the phase start: 0 for Prep and Action, the elapsed time at the
// last sample of the previous phase for PostPlant.
type msPhase struct {
	Phase   RoundPhase
	Tick    uint32
	StartMs uint32
	BaseMs  int64
}

type msSample struct {
	Tick  uint32
	Ms    uint32
	phase int // index into msClock.ph
}

// msClock rebuilds the phases and elapsed milliseconds of a round from the
// game-state object's millisecond clock.
type msClock struct {
	samples   []msSample
	ph        []msPhase
	gameState wire.Handle
	haveGS    bool
}

func newMsClock() *msClock { return &msClock{} }

// observe feeds one TLV token. The first value token carrying fieldRoundTimer fixes the
// game-state object; value tokens of fieldRoundClockMs on that object are samples. Samples
// seen before the game-state object is known are not recorded.
func (c *msClock) observe(tick uint32, t wire.Token) {
	if !t.Op.IsValue() {
		return
	}
	if !c.haveGS {
		if t.Field == fieldRoundTimer {
			c.gameState, c.haveGS = t.Object, true
		}
		return
	}
	if t.Field != fieldRoundClockMs || t.Object != c.gameState {
		return
	}
	ms, ok := t.Uint32()
	if !ok {
		return
	}
	if n := len(c.samples); n == 0 {
		c.startPhase(tick, ms)
	} else if prev := int64(c.samples[n-1].Ms); int64(ms) > prev+msPhaseJumpMs {
		c.startPhase(tick, ms)
	} else if prev-int64(ms) > msPhaseJumpMs {
		if ms == 0 {
			// a discontinuous drop to 0 is the round-end reset, as in the legacy clock
			return
		}
		c.startPhase(tick, ms)
	}
	c.samples = append(c.samples, msSample{Tick: tick, Ms: ms, phase: len(c.ph) - 1})
}

func (c *msClock) startPhase(tick, ms uint32) {
	p := msPhase{Tick: tick, StartMs: ms}
	switch {
	case len(c.ph) == 0:
		if ms <= msPhaseMaxPrepMs {
			p.Phase = PrepPhase
		} else {
			p.Phase = ActionPhase
		}
	case c.ph[len(c.ph)-1].Phase == PrepPhase:
		p.Phase = ActionPhase
	default:
		p.Phase = PostPlantPhase
		last := c.samples[len(c.samples)-1]
		p.BaseMs = c.elapsedOf(last)
	}
	c.ph = append(c.ph, p)
}

func (c *msClock) elapsedOf(s msSample) int64 {
	p := c.ph[s.phase]
	if p.Phase == PrepPhase {
		return -int64(s.Ms)
	}
	return p.BaseMs + int64(p.StartMs) - int64(s.Ms)
}

// phases returns the phases in order.
func (c *msClock) phases() []msPhase { return append([]msPhase(nil), c.ph...) }

// sampleBefore returns the last sample at or before tick.
func (c *msClock) sampleBefore(tick uint32) (ms uint32, ok bool) {
	i := sort.Search(len(c.samples), func(i int) bool { return c.samples[i].Tick > tick })
	if i == 0 {
		return 0, false
	}
	return c.samples[i-1].Ms, true
}

// at returns the phase in force and the elapsed milliseconds at tick, from the last
// sample at or before tick; ok is false before the first sample.
func (c *msClock) at(tick uint32) (phase RoundPhase, roundClockMs uint32, elapsedMs int64, ok bool) {
	i := sort.Search(len(c.samples), func(i int) bool { return c.samples[i].Tick > tick })
	if i == 0 {
		return "", 0, 0, false
	}
	s := c.samples[i-1]
	return c.ph[s.phase].Phase, s.Ms, c.elapsedOf(s), true
}
