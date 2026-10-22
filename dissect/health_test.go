package dissect

import "testing"

// healthReplay builds the bytes of a health link record and a health state record for one player.
func healthReplay(slot, health []byte, state byte) (b []byte, linkOffset, stateOffset int) {
	// link: 1B <slot> 00000000 41 54 DC C4 <health> 00000000
	b = append(b, 0x1B)
	b = append(b, slot...)
	b = append(b, 0, 0, 0, 0)
	b = append(b, healthLinkIndicator...)
	linkOffset = len(b)
	b = append(b, health...)
	b = append(b, 0, 0, 0, 0)
	// 23 <health> 00000000 25 26 76 C9 04 <0> 22 E7 88 F6 A5 04 <state>
	b = append(b, 0x23)
	b = append(b, health...)
	b = append(b, 0, 0, 0, 0, 0x25, 0x26, 0x76, 0xC9, 0x04, 0, 0, 0, 0, 0x22)
	b = append(b, healthStateIndicator...)
	stateOffset = len(b)
	b = append(b, state, 0, 0, 0)
	b = append(b, make([]byte, 16)...)
	return b, linkOffset, stateOffset
}

func newHealthReader(b []byte, slot []byte) *Reader {
	return &Reader{
		b:                      b,
		pendingDefuserPlantIdx: -1,
		healthToSlot:           make(map[[4]byte][4]byte),
		healthState:            make(map[int]uint32),
		Header:                 Header{Players: []Player{{Username: "downed", slotID: slot}}},
	}
}

func TestHealthStateDown(t *testing.T) {
	slot := []byte{0x30, 0xbb, 0x2b, 0xf0}
	b, linkOffset, stateOffset := healthReplay(slot, []byte{0x38, 0xbb, 0x2b, 0xf0}, healthStateDowned)
	r := newHealthReader(b, slot)
	r.clock.started = true
	r.clock.phase = ActionPhase
	r.offset = linkOffset
	if err := readHealthLink(r); err != nil {
		t.Fatal(err)
	}
	r.offset = stateOffset
	if err := readHealthState(r); err != nil {
		t.Fatal(err)
	}
	if len(r.MatchFeedback) != 1 || r.MatchFeedback[0].Type != DBNO || r.MatchFeedback[0].Target != "downed" ||
		r.MatchFeedback[0].healthTransition == 0 || r.MatchFeedback[0].Phase != ActionPhase {
		t.Fatalf("want one stamped DBNO of 'downed', got %+v", r.MatchFeedback)
	}
	// the same state again is not a new down
	if err := readHealthState(r); err != nil {
		t.Fatal(err)
	}
	if len(r.MatchFeedback) != 1 {
		t.Fatalf("repeated state must be ignored, got %d events", len(r.MatchFeedback))
	}
}

func TestHealthStateIgnoredBeforeClock(t *testing.T) {
	slot := []byte{0x30, 0xbb, 0x2b, 0xf0}
	b, linkOffset, stateOffset := healthReplay(slot, []byte{0x38, 0xbb, 0x2b, 0xf0}, healthStateDowned)
	r := newHealthReader(b, slot)
	r.offset = linkOffset
	if err := readHealthLink(r); err != nil {
		t.Fatal(err)
	}
	r.offset = stateOffset
	if err := readHealthState(r); err != nil {
		t.Fatal(err)
	}
	if len(r.MatchFeedback) != 0 {
		t.Fatalf("states before the clock started must be ignored, got %+v", r.MatchFeedback)
	}
}

// resolveReader has a1, a2 on team 0 and d1, d2 on team 1.
func resolveReader(transitions []healthTransition, feed ...MatchUpdate) *Reader {
	return &Reader{
		pendingDefuserPlantIdx: -1,
		healthTransitions:      transitions,
		MatchFeedback:          feed,
		Header: Header{Players: []Player{
			{Username: "a1", TeamIndex: 0}, {Username: "a2", TeamIndex: 0},
			{Username: "d1", TeamIndex: 1}, {Username: "d2", TeamIndex: 1},
		}},
	}
}

func tr(player int, state uint32, elapsed float64) healthTransition {
	return healthTransition{player: player, state: state, elapsed: elapsed, phase: ActionPhase}
}

// down is a DBNO read from healthTransitions[transition].
func down(target string, elapsed float64, transition int) MatchUpdate {
	return MatchUpdate{Type: DBNO, Target: target, Phase: ActionPhase, ElapsedSeconds: elapsed, healthTransition: transition + 1}
}

func findType(r *Reader, typ MatchUpdateType) []MatchUpdate {
	var found []MatchUpdate
	for _, u := range r.MatchFeedback {
		if u.Type == typ {
			found = append(found, u)
		}
	}
	return found
}

func TestResolveHealth_Finished(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateDead, 15)},
		down("a1", 10, 0), kill("d1", "a1", 15))
	r.resolveHealthEvents()
	if d := findType(r, DBNO); len(d) != 1 || d[0].Username != "d1" || d[0].TeamKill {
		t.Fatalf("want the down credited to d1, got %+v", d)
	}
	if rv := findType(r, Revive); len(rv) != 0 {
		t.Fatalf("no revive expected, got %+v", rv)
	}
}

func TestResolveHealth_FinishedByTeamkill(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateDead, 15)},
		down("a1", 10, 0), teamKill("a2", "a1", 15))
	r.resolveHealthEvents()
	if d := findType(r, DBNO); len(d) != 1 || d[0].Username != "a2" || !d[0].TeamKill {
		t.Fatalf("want the down credited to a2 as a teamkill, got %+v", d)
	}
}

func TestResolveHealth_Revived(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20)},
		down("a1", 10, 0), kill("a2", "d1", 25))
	r.scoreRevives = []scoreRevive{{player: 1, elapsed: 21}}
	r.resolveHealthEvents()
	if d := findType(r, DBNO); len(d) != 1 || d[0].Username != "" {
		t.Fatalf("a revived down has no known downer, got %+v", d)
	}
	want := []MatchUpdateType{DBNO, Revive, Kill}
	got := feedTypes(r)
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("want %v, got %v", want, got)
	}
	rv := r.MatchFeedback[1]
	if rv.Username != "a2" || rv.Target != "a1" || rv.ElapsedSeconds != 20 || rv.Phase != ActionPhase {
		t.Fatalf("want a2 reviving a1 at 20, got %+v", rv)
	}
}

func TestResolveHealth_RevivedNoScore(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20)},
		down("a1", 10, 0))
	r.resolveHealthEvents()
	if rv := findType(r, Revive); len(rv) != 1 || rv[0].Username != "" {
		t.Fatalf("want one revive without a reviver, got %+v", rv)
	}
}

func TestResolveHealth_RevivedScoreFromOpponent(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20)},
		down("a1", 10, 0))
	r.scoreRevives = []scoreRevive{{player: 2, elapsed: 20}}
	r.resolveHealthEvents()
	if rv := findType(r, Revive); len(rv) != 1 || rv[0].Username != "" {
		t.Fatalf("an opponent's +50 is not a revive, got %+v", rv)
	}
}

// the state can pass through 2 right before the player dies.
func TestResolveHealth_TransientTwoThenDeath(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 30), tr(0, healthStateDead, 30.5)},
		down("a1", 10, 0), kill("d1", "a1", 30.5))
	r.resolveHealthEvents()
	if rv := findType(r, Revive); len(rv) != 0 {
		t.Fatalf("a death right after leaving the downed state is not a revive, got %+v", rv)
	}
	if d := findType(r, DBNO); d[0].Username != "d1" {
		t.Fatalf("want the down credited to d1, got %+v", d)
	}
}

func TestResolveHealth_RoundEnd(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10)}, down("a1", 10, 0))
	r.resolveHealthEvents()
	if d := findType(r, DBNO); len(d) != 1 || d[0].Username != "" || len(findType(r, Revive)) != 0 {
		t.Fatalf("still down at round end: want no downer and no revive, got %+v", r.MatchFeedback)
	}
}

func TestResolveHealth_DeathNoKiller(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateDead, 40)},
		down("a1", 10, 0), MatchUpdate{Type: Death, Username: "a1", Phase: ActionPhase, ElapsedSeconds: 40})
	r.resolveHealthEvents()
	if d := findType(r, DBNO); d[0].Username != "" {
		t.Fatalf("a bleed out has no downer, got %+v", d)
	}
}

func TestResolveHealth_DownedTwice(t *testing.T) {
	r := resolveReader([]healthTransition{
		tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20),
		tr(0, healthStateDowned, 40), tr(0, healthStateDead, 45),
	}, down("a1", 10, 0), down("a1", 40, 2), kill("d2", "a1", 45))
	r.resolveHealthEvents()
	want := []MatchUpdateType{DBNO, Revive, DBNO, Kill}
	got := feedTypes(r)
	if len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("want %v, got %v", want, got)
	}
	if r.MatchFeedback[0].Username != "" || r.MatchFeedback[2].Username != "d2" {
		t.Fatalf("want the first down uncredited and the second credited to d2, got %+v", r.MatchFeedback)
	}
}

// a revived player who is downed again and finished right after the revive keeps the revive.
func TestResolveHealth_RevivedThenQuickDownAndKill(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20), tr(0, healthStateDowned, 20), tr(0, healthStateDead, 21)},
		down("a1", 10, 0), down("a1", 20, 2), kill("d2", "a1", 21))
	r.resolveHealthEvents()
	if got := feedTypes(r); len(got) != 4 || got[1] != Revive || r.MatchFeedback[2].Username != "d2" {
		t.Fatalf("want [DBNO Revive DBNO Kill] with the second down credited to d2, got %v", r.MatchFeedback)
	}
}

// a Revive lands before its target's next DBNO at the same second even when other events of
// that second sit between the DBNO and the time-ordered insert point.
func TestResolveHealth_RevivedThenQuickDownSameSecondEvent(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20), tr(0, healthStateDowned, 20), tr(0, healthStateDead, 21)},
		down("a1", 10, 0), down("a1", 20, 2), kill("a2", "d1", 20), kill("d2", "a1", 21))
	r.resolveHealthEvents()
	got := feedTypes(r)
	if len(got) != 5 {
		t.Fatalf("want 5 events, got %v", got)
	}
	var downs []int
	revive := -1
	for i, u := range r.MatchFeedback {
		if u.Type == DBNO && u.Target == "a1" {
			downs = append(downs, i)
		}
		if u.Type == Revive && u.Target == "a1" {
			revive = i
		}
	}
	if len(downs) != 2 || revive < downs[0] || revive > downs[1] {
		t.Fatalf("want a Revive of a1 between its two downs, got %v", got)
	}
	for i := 1; i < len(r.MatchFeedback); i++ {
		if r.MatchFeedback[i].ElapsedSeconds < r.MatchFeedback[i-1].ElapsedSeconds {
			t.Fatalf("elapsedSeconds decreases at %d: %+v", i, r.MatchFeedback)
		}
	}
}

// a revive through a healthy state before the second down: the down is not a bleed-out.
func TestResolveHealth_RevivedThenReDownViaHealthy(t *testing.T) {
	r := resolveReader([]healthTransition{
		tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20), tr(0, 0, 20),
		tr(0, healthStateDowned, 21), tr(0, healthStateDead, 21),
	}, down("a1", 10, 0), down("a1", 21, 3), kill("d2", "a1", 21))
	r.resolveHealthEvents()
	want := []MatchUpdateType{DBNO, Revive, DBNO, Kill}
	got := feedTypes(r)
	if len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// the next transition after the exit from the downed state is another down: the first down was
// revived even when a Kill of the target sits inside the death window before the second DBNO in
// the feed (that Kill belongs to the second down).
func TestResolveHealth_RevivedKillBeforeSecondDownInFeed(t *testing.T) {
	r := resolveReader([]healthTransition{
		tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20), tr(0, healthStateDowned, 21), tr(0, healthStateDead, 25),
	}, down("a1", 10, 0), kill("d2", "a1", 21), down("a1", 21, 2))
	r.resolveHealthEvents()
	if got := feedTypes(r); len(got) != 4 || got[1] != Revive {
		t.Fatalf("want the first down revived, got %v", got)
	}
}

// a revive whose state change lands in the same second as the down it ends goes after that
// down, even when another event of that second sits between them (the floor in insertByElapsed).
func TestResolveHealth_RevivedSameSecondAsItsDown(t *testing.T) {
	r := resolveReader([]healthTransition{tr(0, healthStateDowned, 20), tr(0, healthStateWounded, 20)},
		down("a1", 20, 0), kill("a2", "d1", 20))
	r.resolveHealthEvents()
	want := []MatchUpdateType{DBNO, Kill, Revive}
	got := feedTypes(r)
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// two revives of one player in one second: each lands after the down it ends.
func TestResolveHealth_TwoRevivesSameSecond(t *testing.T) {
	r := resolveReader([]healthTransition{
		tr(0, healthStateDowned, 10), tr(0, healthStateWounded, 20), tr(0, healthStateDowned, 20), tr(0, healthStateWounded, 20),
	}, down("a1", 10, 0), down("a1", 20, 2), kill("a2", "d1", 20))
	r.resolveHealthEvents()
	want := []MatchUpdateType{DBNO, Revive, DBNO, Kill, Revive}
	got := feedTypes(r)
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("want %v, got %v", want, got)
		}
	}
}
