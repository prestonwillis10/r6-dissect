package dissect

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	wire "github.com/redraskal/r6-dissect/dissect/wire"
)

// Review Focus 1: the plant event shares its tick frame with the clock jump. The legacy
// phase is still Action, so elapsed time must come from the Action samples, not from the
// post-plant base that already shows in this frame. Here the last Action sample shares the
// jump's tick, so the base absorbs 30 ms the event must not take.
func TestBuildEvent_PlantInJumpFrame(t *testing.T) {
	c := newMsClock()
	c.observe(1, clockTok(fieldRoundTimer, 179))
	c.observe(2, clockTok(fieldRoundClockMs, 179917))
	c.observe(100, clockTok(fieldRoundClockMs, 179880)) // 37 ms into Action
	c.observe(110, clockTok(fieldRoundClockMs, 179850)) // 67 ms, same tick as the jump
	c.observe(110, clockTok(fieldRoundClockMs, 45000))  // the jump: PostPlant, base 67
	u := MatchUpdate{Type: DefuserPlantComplete, Phase: ActionPhase, ElapsedSeconds: 0}
	e := buildEvent(0, u, 110, c)
	if e.Phase != ActionPhase || e.ElapsedMs != 37 {
		t.Fatalf("plant in the jump frame: phase %s elapsed %d", e.Phase, e.ElapsedMs)
	}
	// an event of the new phase takes the new base
	k := buildEvent(1, MatchUpdate{Type: Kill, Phase: PostPlantPhase}, 120, c)
	if k.Phase != PostPlantPhase || k.ElapsedMs != 67 {
		t.Fatalf("post-plant event: phase %s elapsed %d", k.Phase, k.ElapsedMs)
	}
}

func TestBuildEvent_BeforeFirstSample(t *testing.T) {
	c := newMsClock()
	c.observe(1, clockTok(fieldRoundTimer, 179))
	c.observe(50, clockTok(fieldRoundClockMs, 44000))
	e := buildEvent(0, MatchUpdate{Type: PlayerLeave, Phase: PrepPhase, ElapsedSeconds: -3}, 10, c)
	if e.RoundClockMs != 0 || e.ElapsedMs != -3000 {
		t.Fatalf("%+v", e)
	}
}

// C-RT-13: a plant with 44 to 46 s left makes no clock jump, so the clock has no PostPlant phase.
func TestTimelinePhases_PlantWithoutJump(t *testing.T) {
	c := newMsClock()
	c.observe(1, clockTok(fieldRoundTimer, 179))
	c.observe(2, clockTok(fieldRoundClockMs, 179917))
	for i, ms := range []uint32{179000, 178100, 177200} { // a continuous countdown, no jump
		c.observe(uint32(10+i), clockTok(fieldRoundClockMs, ms))
	}
	ms := uint32(177200)
	for tick := uint32(20); ms > 45400; tick++ {
		ms -= 900
		c.observe(tick, clockTok(fieldRoundClockMs, ms))
	}
	c.observe(1000, clockTok(fieldRoundClockMs, 45500)) // the plant, 45.5 s left: no jump
	plant := buildEvent(0, MatchUpdate{Type: DefuserPlantComplete, Phase: ActionPhase, ElapsedSeconds: 134}, 1000, c)
	got := timelinePhases(c, []TimelineEvent{plant})
	if len(got) != 2 || got[0].Phase != ActionPhase || got[1] != (TimelinePhase{PostPlantPhase, 1000, 45500}) {
		t.Fatalf("%+v", got)
	}
	if plant.ElapsedMs != 179917-45500 {
		t.Fatalf("elapsed %d", plant.ElapsedMs)
	}
	// no plant, no entry
	if got := timelinePhases(c, nil); len(got) != 1 {
		t.Fatalf("%+v", got)
	}
}

// C-RT-12: the stop of a plant/disable still running at round end is an inference.
func TestStopDefuserRunAtRoundEnd_IsDerived(t *testing.T) {
	d := newDefuserTestReplay()
	defPawn := []byte{0x07, 0xB4, 0x0C, 0xF0}
	d.spawn(t, d.r.Header.Players[1].slotID, defPawn)
	d.timer(t, defPawn, defuserStateDisabling, 1, "6.983")
	d.timer(t, defPawn, -1, 0.9, "6.300")
	d.r.stopDefuserRunAtRoundEnd()
	got := d.r.MatchFeedback
	last := got[len(got)-1]
	if last.Type != DefuserDisableStop || last.offset != -1 || !last.derived {
		t.Fatalf("%+v", last)
	}
	for _, u := range got[:len(got)-1] {
		if u.derived || u.offset == -1 {
			t.Errorf("listener event marked as inferred: %+v", u)
		}
	}
}

// Review Focus 4: an event with no buffer offset takes the last tick; one inside no envelope
// takes the nearest earlier envelope; one before every envelope takes the first tick.
func TestTickIndex_TickAt(t *testing.T) {
	x := &tickIndex{
		spans: []tickSpan{{100, 200, 10}, {200, 300, 20}, {400, 500, 30}},
		first: 10, last: 30,
	}
	for _, tc := range []struct {
		off  int
		want uint32
	}{{-1, 30}, {50, 10}, {100, 10}, {199, 10}, {200, 20}, {350, 20}, {400, 30}, {9999, 30}} {
		if got := x.tickAt(tc.off); got != tc.want {
			t.Errorf("tickAt(%d) = %d, want %d", tc.off, got, tc.want)
		}
	}
}

func TestBuildEvent_StatusAndData(t *testing.T) {
	c := newMsClock()
	c.observe(1, clockTok(fieldRoundTimer, 179))
	c.observe(2, clockTok(fieldRoundClockMs, 179917))
	hs := true
	u := MatchUpdate{Type: Kill, Phase: ActionPhase, Headshot: &hs, TeamKill: true, Message: "m", Operator: Ash, offset: 5}
	e := buildEvent(3, u, 2, c)
	if e.Status != "observed" || e.Data["headshot"] != true || e.Data["teamKill"] != true || e.Data["message"] != "m" || e.Data["operator"] != Ash {
		t.Fatalf("%+v", e)
	}
	u.derived = true
	if e := buildEvent(3, u, 2, c); e.Status != "derived" {
		t.Fatalf("%+v", e)
	}
	if e := buildEvent(0, MatchUpdate{Type: Kill, Phase: ActionPhase}, 2, c); e.Data != nil {
		t.Fatalf("empty data should be omitted: %+v", e.Data)
	}
}

// Read closes a plant/disable still running at round end through the derived path.
func TestRead_RoundEndDefuserStopIsDerived(t *testing.T) {
	d := newDefuserTestReplay()
	defPawn := []byte{0x07, 0xB4, 0x0C, 0xF0}
	d.spawn(t, d.r.Header.Players[1].slotID, defPawn)
	d.timer(t, defPawn, defuserStateDisabling, 1, "6.983")
	d.timer(t, defPawn, -1, 0.9, "6.300")
	d.r.b = make([]byte, 4096)
	d.r.offset = 0
	d.r.clock = roundClock{}
	if err := d.r.Read(); !Ok(err) {
		t.Fatal(err)
	}
	var stop *MatchUpdate
	for i := range d.r.MatchFeedback {
		if d.r.MatchFeedback[i].Type == DefuserDisableStop {
			stop = &d.r.MatchFeedback[i]
		}
	}
	if stop == nil || stop.offset != -1 || !stop.derived {
		t.Fatalf("%+v", stop)
	}
}

const fortressFixture = "test/data/replays/valid/Y11S3/custom_fortress_plant.rec"

func openFortress(t *testing.T) *Reader {
	t.Helper()
	f, err := os.Open(fortressFixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A listener that stops the read with io.EOF is not an error, and the timeline must still settle.
func TestRead_ListenerEOFStillSettlesTimeline(t *testing.T) {
	r := openFortress(t)
	r.EnableTimeline()
	r.Listen([]byte{0x1F, 0x07, 0xEF, 0xC9}, func(*Reader) error { return io.EOF })
	if err := r.Read(); !Ok(err) {
		t.Fatal(err)
	}
	_, err := r.Timeline()
	var te *TimelineError
	if err != nil && !errors.As(err, &te) {
		t.Fatal(err)
	}
	if (r.timeline == nil) == (r.timelineErr == nil) {
		t.Fatalf("exactly one of timeline and timelineErr must be set: %v %v", r.timeline, r.timelineErr)
	}
}

// Design 5.4: a segmenter that leaves more than 0.5% of the bytes unplaced voids the timeline.
func TestTimeline_CorruptTLVIsTimelineError(t *testing.T) {
	r := openFortress(t)
	l, err := wire.Walk(r.b)
	if err != nil {
		t.Fatal(err)
	}
	c := l.ChannelsOf(wire.PayloadTLV)[0]
	mid := (c.Start + c.End) / 2
	for i := mid; i < mid+4000; i++ {
		r.b[i] = 0xff
	}
	r.EnableTimeline()
	if err := r.Read(); !Ok(err) {
		t.Fatal(err)
	}
	tl, err := r.Timeline()
	var te *TimelineError
	if tl != nil || !errors.As(err, &te) || te.Coverage == nil || te.Coverage.UnplacedFraction <= 0.005 {
		t.Fatalf("%v %+v", err, te)
	}
}

func TestTimeline_VersionOutsideRange(t *testing.T) {
	read := func(t *testing.T, mutate func(*Reader)) (*Timeline, error) {
		t.Helper()
		f, err := os.Open(fortressFixture)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r, err := NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		mutate(r)
		r.EnableTimeline()
		if err := r.Read(); !Ok(err) {
			t.Fatal(err)
		}
		return r.Timeline()
	}
	t.Run("pre-Y11S2 voids the timeline", func(t *testing.T) {
		tl, err := read(t, func(r *Reader) { r.Header.CodeVersion = Y10S1 })
		var te *TimelineError
		if tl != nil || !errors.As(err, &te) || !strings.Contains(te.Reason, "unsupported version") {
			t.Fatalf("tl=%v err=%v", tl, err)
		}
	})
	t.Run("control: unmodified fixture builds", func(t *testing.T) {
		tl, err := read(t, func(*Reader) {})
		if err != nil || tl == nil {
			t.Fatalf("tl=%v err=%v", tl, err)
		}
	})
}
