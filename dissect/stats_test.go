package dissect

import "testing"

func kill(killer, target string, elapsed float64) MatchUpdate {
	return MatchUpdate{Type: Kill, Username: killer, Target: target, Phase: ActionPhase, ElapsedSeconds: elapsed}
}

func teamKill(killer, target string, elapsed float64) MatchUpdate {
	u := kill(killer, target, elapsed)
	u.TeamKill = true
	return u
}

func TestOpeningKill_SkipsTeamKill(t *testing.T) {
	r := &Reader{MatchFeedback: []MatchUpdate{teamKill("a1", "a2", 5), kill("a1", "d1", 8)}}
	if got := r.OpeningKill().Target; got != "d1" {
		t.Fatalf("want the first non-teamkill (target d1), got target %q", got)
	}
}

// other events between two kills do not break or create a trade.
func TestTrades_KillsOnly(t *testing.T) {
	stop := MatchUpdate{Type: DefuserPlantStop, Username: "a2", Phase: ActionPhase, ElapsedSeconds: 11}
	r := &Reader{MatchFeedback: []MatchUpdate{kill("d1", "a1", 10), stop, kill("a2", "d1", 12)}}
	trades := r.Trades()
	if len(trades) != 1 || trades[0][0].Target != "a1" || trades[0][1].Target != "d1" {
		t.Fatalf("want one trade [d1→a1, a2→d1], got %+v", trades)
	}
}

func TestTrades_SkipsTeamKill(t *testing.T) {
	r := &Reader{MatchFeedback: []MatchUpdate{kill("d1", "a1", 10), teamKill("a2", "a3", 11), kill("a2", "d1", 12)}}
	trades := r.Trades()
	if len(trades) != 1 || trades[0][0].Target != "a1" || trades[0][1].Target != "d1" {
		t.Fatalf("want one trade [d1→a1, a2→d1] without the teamkill, got %+v", trades)
	}
}

func threeVThree() Header {
	return Header{
		Teams: [2]Team{{Role: Attack}, {Role: Defense, Won: true}},
		Players: []Player{
			{Username: "a1", TeamIndex: 0}, {Username: "a2", TeamIndex: 0}, {Username: "a3", TeamIndex: 0},
			{Username: "d1", TeamIndex: 1}, {Username: "d2", TeamIndex: 1}, {Username: "d3", TeamIndex: 1},
		},
	}
}

func statsOf(stats []PlayerRoundStats, username string) PlayerRoundStats {
	for _, s := range stats {
		if s.Username == username {
			return s
		}
	}
	return PlayerRoundStats{}
}

// an opponent who dies without a killer during the clutch was still part of it.
func TestPlayerStats_OneVxMatchesClutch(t *testing.T) {
	r := &Reader{Header: threeVThree(), MatchFeedback: []MatchUpdate{
		kill("d1", "a1", 10), kill("a2", "d2", 20), kill("a3", "d3", 25),
		{Type: Death, Username: "a2", Phase: ActionPhase, ElapsedSeconds: 30},
		kill("d1", "a3", 40),
	}}
	s := statsOf(r.PlayerStats(), "d1")
	if s.ClutchDetail == nil || s.ClutchDetail.OpponentsAlive != 2 || s.OneVx != 2 {
		t.Fatalf("want 1vX 2 and opponentsAlive 2, got 1vX %d, clutch %+v", s.OneVx, s.ClutchDetail)
	}
}

func TestPlayerStats_OneVxNoClutchFallback(t *testing.T) {
	r := &Reader{Header: threeVThree(), MatchFeedback: []MatchUpdate{kill("d1", "a1", 10)}}
	for _, s := range r.PlayerStats() {
		if s.OneVx != 0 || s.ClutchDetail != nil {
			t.Fatalf("no clutch: want 1vX 0 for %s, got %d, clutch %+v", s.Username, s.OneVx, s.ClutchDetail)
		}
	}
}
