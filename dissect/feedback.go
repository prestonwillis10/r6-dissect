package dissect

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
)

type MatchUpdateType int

//go:generate stringer -type=MatchUpdateType
const (
	Kill MatchUpdateType = iota
	Death
	DefuserPlantStart
	DefuserPlantComplete
	DefuserDisableStart
	DefuserDisableComplete
	LocateObjective
	OperatorSwap
	Battleye
	PlayerLeave
	Other
	// DBNO and Revive are reserved and not currently emitted; they keep the numeric
	// IDs of DefuserPlantStop and DefuserDisableStop stable.
	DBNO
	Revive
	DefuserPlantStop
	DefuserDisableStop
)

type MatchUpdate struct {
	Type           MatchUpdateType `json:"type"`
	Username       string          `json:"username,omitempty"`
	Target         string          `json:"target,omitempty"`
	Headshot       *bool           `json:"headshot,omitempty"`
	Time           string          `json:"time"`
	TimeInSeconds  float64         `json:"timeInSeconds"`
	Phase          RoundPhase      `json:"phase,omitempty"`
	ElapsedSeconds float64         `json:"elapsedSeconds"`
	Message        string          `json:"message,omitempty"`
	Operator       Operator        `json:"operator,omitempty"`
	TeamKill       bool            `json:"teamKill,omitempty"`
	// Y10S4+: the player whose scoreboard kill counter increased right before this kill packet
	scoreboardKiller string
	// offset is the buffer offset of the listener that produced the event; -1 when inferred at round end
	offset int
	// derived is set when the event or its actor was inferred rather than read from a packet
	derived bool
}

// addUpdate stamps u with the current buffer offset and appends it to the match feedback.
// An offset already set (-1 for round-end inferences) is kept.
func (r *Reader) addUpdate(u MatchUpdate) {
	if u.offset == 0 {
		u.offset = r.offset
	}
	r.MatchFeedback = append(r.MatchFeedback, u)
}

func (i MatchUpdateType) MarshalJSON() (text []byte, err error) {
	return json.Marshal(stringerIntMarshal{
		Name: i.String(),
		ID:   int(i),
	})
}

func (i *MatchUpdateType) UnmarshalJSON(data []byte) (err error) {
	var x stringerIntMarshal
	if err = json.Unmarshal(data, &x); err != nil {
		return
	}
	*i = MatchUpdateType(x.ID)
	return
}

var activity2 = []byte{0x00, 0x00, 0x00, 0x22, 0xe3, 0x09, 0x00, 0x79}
var killIndicator = []byte{0x22, 0xd9, 0x13, 0x3c, 0xba}

func readMatchFeedback(r *Reader) error {
	if r.Header.CodeVersion >= Y9S1Update3 {
		if err := r.Skip(38); err != nil {
			return err
		}
	} else if r.Header.CodeVersion >= Y9S1 {
		if err := r.Skip(9); err != nil {
			return err
		}
		valid, err := r.Int()
		if err != nil {
			return err
		}
		if valid != 4 {
			return errors.New("match feedback failed valid check")
		}
		if err := r.Skip(24); err != nil {
			return err
		}
	} else {
		if err := r.Skip(1); err != nil {
			return err
		}
		if err := r.Seek(activity2); err != nil {
			return err
		}
	}
	size, err := r.Int()
	if err != nil {
		return err
	}
	if size == 0 { // kill or an unknown indicator at start of match
		killTrace, err := r.Bytes(5)
		if err != nil {
			return err
		}
		if !bytes.Equal(killTrace, killIndicator) {
			log.Debug().Hex("killTrace", killTrace).Send()
			return nil
		}
		username, err := r.String()
		if err != nil {
			return err
		}
		empty := len(username) == 0
		if empty {
			log.Debug().Str("warn", "kill username empty").Send()
		}
		if err = r.Skip(15); err != nil {
			return err
		}
		target, err := r.String()
		if err != nil {
			return err
		}
		log.Debug().Str("target", target).Msg("kill target parsed")
		if empty {
			if len(target) > 0 {
				if r.isDead(target) {
					log.Debug().Str("username", target).Msg("duplicate death filtered (player already dead)")
					return nil
				}
				u := MatchUpdate{
					Type:          Death,
					Username:      target,
					Time:          r.timeRaw,
					TimeInSeconds: r.time,
				}
				r.stamp(&u)
				r.addUpdate(u)
				log.Debug().Interface("match_update", u).Send()
				log.Debug().Msg("kill username empty because of death")
			}
			return nil
		}
		u := MatchUpdate{
			Type:          Kill,
			Username:      username,
			Target:        target,
			Time:          r.timeRaw,
			TimeInSeconds: r.time,
		}
		r.stamp(&u)
		if err = r.Skip(56); err != nil {
			return err
		}
		headshot, err := r.Int()
		if err != nil {
			return err
		}
		headshotPtr := new(bool)
		if headshot == 1 {
			*headshotPtr = true
		}
		u.Headshot = headshotPtr
		log.Debug().
			Str("killer", username).
			Str("target", target).
			Bool("headshot", headshot == 1).
			Msg("kill event parsed")
		killerIdx := r.PlayerIndexByUsername(u.Username)
		targetIdx := r.PlayerIndexByUsername(u.Target)
		if killerIdx >= 0 && targetIdx >= 0 {
			killerTeam := r.Header.Players[killerIdx].TeamIndex
			targetTeam := r.Header.Players[targetIdx].TeamIndex
			if killerTeam == targetTeam {
				log.Debug().
					Str("killer", u.Username).
					Str("target", u.Target).
					Int("team", killerTeam).
					Msg("team kill")
				u.TeamKill = true
			}
		}
		// The game re-sends kill packets for the same death (same killer, target and
		// headshot flag, up to ~90 seconds later). The first packet is the actual kill,
		// so any later packet for a target that is already dead is a duplicate.
		for _, val := range r.MatchFeedback {
			if (val.Type == Kill && val.Target == u.Target) || (val.Type == Death && val.Username == u.Target) {
				log.Debug().
					Str("killer", u.Username).
					Str("target", u.Target).
					Float64("existing_time", val.ElapsedSeconds).
					Float64("new_time", u.ElapsedSeconds).
					Msg("duplicate kill filtered (target already dead)")
				return nil
			}
		}
		u.scoreboardKiller = r.lastScoreboardKiller
		r.lastScoreboardKiller = ""
		r.addUpdate(u)
		log.Debug().Interface("match_update", u).Send()
		return nil
	}
	if r.Header.CodeVersion >= Y9S1 {
		return nil
	}
	b, err := r.Bytes(size)
	if err != nil {
		return err
	}
	msg := string(b)
	t := Other
	if strings.Contains(msg, "bombs") || strings.Contains(msg, "objective") {
		t = LocateObjective
	}
	if strings.Contains(msg, "BattlEye") {
		t = Battleye
	}
	if strings.Contains(msg, "left") {
		t = PlayerLeave
	}
	username := strings.Split(msg, " ")[0]
	if t == Other {
		username = ""
	} else {
		msg = ""
	}
	u := MatchUpdate{
		Type:          t,
		Username:      username,
		Target:        "",
		Time:          r.timeRaw,
		TimeInSeconds: r.time,
		Message:       msg,
	}
	r.addUpdate(u)
	log.Debug().Interface("match_update", u).Send()
	return nil
}

// isDead reports whether username already died in this round, either as the target of a
// Kill or through a Death event.
func (r *Reader) isDead(username string) bool {
	for _, u := range r.MatchFeedback {
		if (u.Type == Kill && u.Target == username) || (u.Type == Death && u.Username == username) {
			return true
		}
	}
	return false
}
