package dissect

import (
	"errors"
	"fmt"
	"sort"

	wire "github.com/redraskal/r6-dissect/dissect/wire"
)

// Timeline is the per-round sidecar: every legacy match feedback event with a tick and a
// millisecond elapsed time, the phase changes of the round clock, the player/pawn entities
// and the decoder coverage.
type Timeline struct {
	Version   int                `json:"version"`
	Clock     TimelineClock      `json:"clock"`
	Entities  []TimelineEntity   `json:"entities"`
	Events    []TimelineEvent    `json:"events"`
	Positions *TimelinePositions `json:"positions,omitempty"`
	Coverage  TimelineCoverage   `json:"coverage"`
}

type TimelineClock struct {
	TickFirst uint32          `json:"tickFirst"`
	TickLast  uint32          `json:"tickLast"`
	Phases    []TimelinePhase `json:"phases"`
}

type TimelinePhase struct {
	Phase        RoundPhase `json:"phase"`
	Tick         uint32     `json:"tick"`
	RoundClockMs uint32     `json:"roundClockMs"`
}

// TimelineEntity is a player object or a defuser pawn. Kind is "player" or "pawn".
type TimelineEntity struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Owner  string `json:"owner,omitempty"`
	Class  string `json:"class"`
	Status string `json:"status"`
}

type TimelineEvent struct {
	Seq          int             `json:"seq"`
	Tick         uint32          `json:"tick"`
	ElapsedMs    int64           `json:"elapsedMs"`
	RoundClockMs uint32          `json:"roundClockMs"`
	Phase        RoundPhase      `json:"phase"`
	Type         MatchUpdateType `json:"type"`
	Actor        string          `json:"actor,omitempty"`
	Target       string          `json:"target,omitempty"`
	Data         map[string]any  `json:"data,omitempty"`
	Status       string          `json:"status"` // "observed" | "derived"
}

// TimelinePositions is reserved for a later plan and always nil today.
type TimelinePositions struct {
	SampleHz int   `json:"sampleHz"`
	Samples  []any `json:"samples"`
}

type TimelineCoverage struct {
	UnplacedFraction    float64 `json:"unplacedFraction"`
	TLVErrors           int     `json:"tlvErrors"`
	TLVTokens           int     `json:"tlvTokens"`
	MovementRecords     int     `json:"movementRecords"`
	MovementBadFraction float64 `json:"movementBadFraction"`
}

// TimelineError explains why no timeline was built. The round itself is still read.
type TimelineError struct {
	Reason   string            `json:"reason"`
	Coverage *TimelineCoverage `json:"coverage,omitempty"`
}

func (e *TimelineError) Error() string { return "dissect: timeline: " + e.Reason }

var ErrTimelineDisabled = errors.New("dissect: timeline not enabled; call EnableTimeline before Read")

var errTimelineNotRead = errors.New("dissect: timeline not built; call Read first")

// EnableTimeline makes Read build the timeline before releasing the buffer. Call it before Read.
func (r *Reader) EnableTimeline() { r.timelineEnabled = true }

// Timeline returns the timeline built by Read, a *TimelineError when building failed,
// or ErrTimelineDisabled when EnableTimeline was not called.
func (r *Reader) Timeline() (*Timeline, error) {
	if !r.timelineEnabled {
		return nil, ErrTimelineDisabled
	}
	if r.timelineErr != nil {
		return nil, r.timelineErr
	}
	if r.timeline == nil {
		return nil, errTimelineNotRead
	}
	return r.timeline, nil
}

// EnableTimeline applies to every round reader created afterwards.
func (m *MatchReader) EnableTimeline() { m.timeline = true }

// Rounds returns the round readers; entries are nil until their round is read.
func (m *MatchReader) Rounds() []*Reader { return m.rounds }

// Paths returns the round file paths in round order.
func (m *MatchReader) Paths() []string { return m.paths }

// maxUnplacedFraction is the share of bytes the segmenter may leave unplaced (design 5.4:
// less than 99.5% of bytes attributed voids the timeline). Per-block TLV errors do not gate.
const maxUnplacedFraction = 0.005

type tickSpan struct {
	start, end int
	tick       uint32
}

// tickIndex maps a buffer offset to the tick of the TLV envelope holding it.
type tickIndex struct {
	spans       []tickSpan
	first, last uint32
}

func newTickIndex(l wire.Layout) *tickIndex {
	x := &tickIndex{}
	for _, c := range l.ChannelsOf(wire.PayloadTLV) {
		for _, e := range c.Envelopes {
			x.spans = append(x.spans, tickSpan{e.Start, e.End, e.Tick})
		}
	}
	sort.Slice(x.spans, func(i, j int) bool { return x.spans[i].start < x.spans[j].start })
	for i, s := range x.spans {
		if i == 0 || s.tick < x.first {
			x.first = s.tick
		}
		if i == 0 || s.tick > x.last {
			x.last = s.tick
		}
	}
	return x
}

// tickAt returns the tick of the envelope holding offset, the nearest earlier envelope when
// offset lies in no envelope, the first tick before all of them and the last tick for -1.
func (x *tickIndex) tickAt(offset int) uint32 {
	if offset < 0 {
		return x.last
	}
	i := sort.Search(len(x.spans), func(i int) bool { return x.spans[i].start > offset })
	if i == 0 {
		return x.first
	}
	return x.spans[i-1].tick
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func buildTimeline(r *Reader) (*Timeline, *TimelineError) {
	l, err := wire.Walk(r.b)
	if err != nil || len(l.ChannelsOf(wire.PayloadTLV)) == 0 {
		return nil, &TimelineError{Reason: "no TLV channels (legacy or unsupported layout)"}
	}
	if r.Header.CodeVersion < Y11S2 {
		return nil, &TimelineError{Reason: fmt.Sprintf("unsupported version %s (code %d): the timeline needs Y11S2 or later", r.Header.GameVersion, r.Header.CodeVersion)}
	}
	idx := newTickIndex(l)
	if len(idx.spans) == 0 {
		return nil, &TimelineError{Reason: "no TLV envelopes"}
	}
	clock := newMsClock()
	graph := wire.NewGraph()
	st, err := wire.DecodeTLV(r.b, l, func(tick uint32, t wire.Token) error {
		clock.observe(tick, t)
		graph.Add(tick, t)
		return nil
	})
	cov := TimelineCoverage{
		UnplacedFraction: ratio(l.UnplacedBytes(), l.Size),
		TLVErrors:        len(st.Errors),
		TLVTokens:        st.Tokens,
	}
	if err != nil {
		return nil, &TimelineError{Reason: "TLV decode: " + err.Error(), Coverage: &cov}
	}
	ms, err := wire.DecodeMovement(r.b, l, nil)
	if err != nil {
		return nil, &TimelineError{Reason: "movement decode: " + err.Error(), Coverage: &cov}
	}
	cov.MovementRecords = ms.Records
	cov.MovementBadFraction = ratio(ms.BadEnvelopes, ms.Envelopes)
	if cov.UnplacedFraction > maxUnplacedFraction {
		return nil, &TimelineError{
			Reason:   fmt.Sprintf("segmenter left %.2f%% of bytes unplaced (limit 0.5%%)", 100*cov.UnplacedFraction),
			Coverage: &cov,
		}
	}

	tl := &Timeline{
		Version:  1,
		Clock:    TimelineClock{TickFirst: idx.first, TickLast: idx.last},
		Entities: buildEntities(r, graph),
		Events:   make([]TimelineEvent, 0, len(r.MatchFeedback)),
		Coverage: cov,
	}
	for i, u := range r.MatchFeedback {
		tl.Events = append(tl.Events, buildEvent(i, u, idx.tickAt(u.offset), clock))
	}
	tl.Clock.Phases = timelinePhases(clock, tl.Events)
	return tl, nil
}

func buildEvent(seq int, u MatchUpdate, tick uint32, clock *msClock) TimelineEvent {
	e := TimelineEvent{
		Seq:    seq,
		Tick:   tick,
		Phase:  u.Phase,
		Type:   u.Type,
		Actor:  u.Username,
		Target: u.Target,
		Status: "observed",
	}
	if u.derived {
		e.Status = "derived"
	}
	cp, ms, elapsed, ok := clock.at(tick)
	if !ok {
		ms, elapsed = 0, int64(u.ElapsedSeconds)*1000
	} else if cp != u.Phase && tick > 0 {
		// The clock already shows the next phase in this tick frame (a plant in the frame of
		// the clock jump) while the legacy event keeps the earlier phase: use the earlier base.
		if pp, _, pel, pok := clock.at(tick - 1); pok && pp == u.Phase {
			elapsed = pel
		}
	}
	e.RoundClockMs, e.ElapsedMs = ms, elapsed
	data := map[string]any{}
	if u.Headshot != nil {
		data["headshot"] = *u.Headshot
	}
	if u.TeamKill {
		data["teamKill"] = true
	}
	if u.Operator != 0 {
		data["operator"] = u.Operator
	}
	if u.Message != "" {
		data["message"] = u.Message
	}
	if len(data) > 0 {
		e.Data = data
	}
	return e
}

// timelinePhases lists the clock phases. A plant with 44 to 46 s of action time left makes no
// clock jump, so the clock has no PostPlant phase although the round has a plant: the phase
// table then gets a PostPlant entry at the tick of the plant event.
func timelinePhases(clock *msClock, events []TimelineEvent) []TimelinePhase {
	out := []TimelinePhase{}
	hasPost := false
	for _, p := range clock.phases() {
		out = append(out, TimelinePhase{Phase: p.Phase, Tick: p.Tick, RoundClockMs: p.StartMs})
		hasPost = hasPost || p.Phase == PostPlantPhase
	}
	if hasPost {
		return out
	}
	for _, e := range events {
		if e.Type == DefuserPlantComplete {
			return append(out, TimelinePhase{Phase: PostPlantPhase, Tick: e.Tick, RoundClockMs: e.RoundClockMs})
		}
	}
	return out
}

func buildEntities(r *Reader, g *wire.Graph) []TimelineEntity {
	class := func(h wire.Handle) string {
		if c, ok := g.Class(h); ok {
			return c.String()
		}
		return ""
	}
	out := []TimelineEntity{}
	if r.inventory != nil {
		for handle, p := range r.inventory.players {
			h := wire.HandleOf(handle)
			out = append(out, TimelineEntity{ID: h.String(), Kind: "player", Owner: p.username, Class: class(h), Status: "observed"})
		}
	}
	for pawn, slot := range r.defuserPawnToSlot {
		owner := ""
		for _, p := range r.Header.Players {
			if string(p.slotID) == string(slot[:]) {
				owner = p.Username
				break
			}
		}
		if owner == "" {
			continue
		}
		var h wire.Handle
		copy(h[:4], pawn[:])
		out = append(out, TimelineEntity{ID: h.String(), Kind: "pawn", Owner: owner, Class: class(h), Status: "observed"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
