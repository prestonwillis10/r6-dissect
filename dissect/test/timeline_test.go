package test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/redraskal/r6-dissect/dissect"
)

func timelineFixtures(t *testing.T, seasons ...string) []string {
	t.Helper()
	var out []string
	for _, s := range seasons {
		m, err := filepath.Glob(filepath.Join("data/replays/valid", s, "*.rec"))
		if err != nil || len(m) == 0 {
			t.Fatalf("no fixtures for %s: %v", s, err)
		}
		out = append(out, m...)
	}
	return out
}

func readWithTimeline(t *testing.T, f *os.File) (*dissect.Reader, *dissect.Timeline, error) {
	t.Helper()
	r, err := dissect.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	r.EnableTimeline()
	if err := r.Read(); !dissect.Ok(err) {
		t.Fatal(err)
	}
	tl, err := r.Timeline()
	return r, tl, err
}

func TestTimeline_Parity(t *testing.T) {
	for _, path := range timelineFixtures(t, "Y11S2", "Y11S3") {
		t.Run(path, withFile(path, func(f *os.File, t *testing.T) {
			r, tl, err := readWithTimeline(t, f)
			if err != nil {
				t.Fatal(err)
			}
			if tl.Version != 1 {
				t.Fatal("version", tl.Version)
			}
			if len(tl.Events) != len(r.MatchFeedback) {
				t.Fatalf("events %d != feedback %d", len(tl.Events), len(r.MatchFeedback))
			}
			var lastTick uint32
			for i, e := range tl.Events {
				u := r.MatchFeedback[i]
				if e.Seq != i || e.Type != u.Type || e.Actor != u.Username || e.Target != u.Target || e.Phase != u.Phase {
					t.Errorf("event %d: %+v vs %+v", i, e, u)
				}
				// post-plant: the legacy clock rounds the plant time and the post-plant timer separately, so its error can approach 2 s (1.421 s measured)
				tol := 1.5
				if e.Phase == dissect.PostPlantPhase {
					tol = 2.0
				}
				if d := float64(e.ElapsedMs)/1000 - u.ElapsedSeconds; d < -tol || d > tol {
					t.Errorf("event %d elapsed %d ms vs %.0f s", i, e.ElapsedMs, u.ElapsedSeconds)
				}
				if e.Tick < lastTick {
					t.Errorf("event %d tick %d < %d", i, e.Tick, lastTick)
				}
				lastTick = e.Tick
				if e.Tick < tl.Clock.TickFirst || e.Tick > tl.Clock.TickLast {
					t.Errorf("event %d tick %d outside [%d,%d]", i, e.Tick, tl.Clock.TickFirst, tl.Clock.TickLast)
				}
				if u.Type == dissect.Kill && e.Status != "observed" {
					t.Errorf("kill %d status %s", i, e.Status)
				}
				if u.Headshot != nil {
					if hs, ok := e.Data["headshot"]; !ok || hs != *u.Headshot {
						t.Errorf("event %d headshot", i)
					}
				}
			}
			// a kill happens after the first tick of the round: a tick of 0 means the offset mapping failed
			for _, e := range tl.Events {
				if e.Type == dissect.Kill && e.Tick <= tl.Clock.TickFirst {
					t.Errorf("kill at tick %d, first tick %d", e.Tick, tl.Clock.TickFirst)
				}
			}
			players := 0
			for _, en := range tl.Entities {
				if en.Kind == "player" && en.Owner != "" {
					players++
				}
			}
			if players != 10 {
				t.Errorf("player entities = %d", players)
			}
			ph := tl.Clock.Phases
			if len(ph) < 2 || ph[0].Phase != dissect.PrepPhase || ph[1].Phase != dissect.ActionPhase {
				t.Errorf("phases %+v", ph)
			}
			if tl.Coverage.TLVErrors != 0 || tl.Coverage.UnplacedFraction > 0.005 {
				t.Errorf("coverage %+v", tl.Coverage)
			}
		}))
	}
}

func TestTimeline_Disabled(t *testing.T) {
	path := timelineFixtures(t, "Y11S3")[0]
	withFile(path, func(f *os.File, t *testing.T) {
		r, err := dissect.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Read(); !dissect.Ok(err) {
			t.Fatal(err)
		}
		if tl, err := r.Timeline(); tl != nil || !errors.Is(err, dissect.ErrTimelineDisabled) {
			t.Fatal(tl, err)
		}
	})(t)
}

func TestTimeline_LegacyLayout(t *testing.T) {
	path := timelineFixtures(t, "Y8S1")[0]
	withFile(path, func(f *os.File, t *testing.T) {
		_, tl, err := readWithTimeline(t, f)
		var te *dissect.TimelineError
		if tl != nil || !errors.As(err, &te) || te.Reason == "" {
			t.Fatal(tl, err)
		}
	})(t)
}

func TestTimeline_PlantPhase(t *testing.T) {
	path := "data/replays/valid/Y11S3/custom_fortress_plant.rec"
	withFile(path, func(f *os.File, t *testing.T) {
		_, tl, err := readWithTimeline(t, f)
		if err != nil {
			t.Fatal(err)
		}
		plant := -1
		for i, e := range tl.Events {
			if e.Type == dissect.DefuserPlantComplete {
				plant = i
				break
			}
		}
		if plant < 0 {
			t.Fatal("no plant event")
		}
		if tl.Events[plant].Phase != dissect.ActionPhase {
			t.Errorf("plant phase %s", tl.Events[plant].Phase)
		}
		kill := -1
		for i := plant + 1; i < len(tl.Events); i++ {
			if tl.Events[i].Type == dissect.Kill {
				kill = i
				break
			}
		}
		if len(tl.Clock.Phases) != 3 {
			t.Fatalf("phases %+v", tl.Clock.Phases)
		}
		if kill < 0 || tl.Events[kill].Phase != dissect.PostPlantPhase {
			t.Fatalf("first kill after plant: %d", kill)
		}
		if tl.Events[plant].ElapsedMs > tl.Events[kill].ElapsedMs {
			t.Errorf("plant %d ms after kill %d ms", tl.Events[plant].ElapsedMs, tl.Events[kill].ElapsedMs)
		}
		if tl.Events[plant].Tick > tl.Clock.Phases[2].Tick {
			t.Errorf("plant tick %d after the post-plant phase tick %d", tl.Events[plant].Tick, tl.Clock.Phases[2].Tick)
		}
	})(t)
}
