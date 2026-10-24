package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/redraskal/r6-dissect/dissect"
	"github.com/redraskal/r6-dissect/dissect/wire"
	"github.com/rs/zerolog"
)

type lab struct {
	out bytes.Buffer
}

// errLimit stops a decode once --limit lines are written.
var errLimit = errors.New("limit reached")

func (l *lab) emit(v any) error {
	enc := json.NewEncoder(&l.out)
	return enc.Encode(v)
}

// parse handles one subcommand's flags and returns the single path argument. Flags may precede or follow the path.
func (l *lab) parse(name string, args []string, setup func(*flag.FlagSet)) (string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the CLI prints one line itself; the flag package's own report and usage stay silent
	if setup != nil {
		setup(fs)
	}
	if err := fs.Parse(args); err != nil {
		return "", flagError(fs, name, err)
	}
	if fs.NArg() == 0 {
		return "", usageError("missing <round.rec>")
	}
	path := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", flagError(fs, name, err)
	}
	if fs.NArg() != 0 {
		return "", usageError("expected one round file")
	}
	return path, nil
}

// flagError turns a flag-parse failure into the CLI's one-line usage error; a
// help request (-h, --help) answers with the subcommand's one-line usage so the
// flags stay discoverable without the flag package's multi-line output.
func flagError(fs *flag.FlagSet, name string, err error) error {
	if !errors.Is(err, flag.ErrHelp) {
		return usageError(err.Error())
	}
	u := "usage: dissect-lab " + name + " <round.rec>"
	fs.VisitAll(func(f *flag.Flag) {
		if f.DefValue == "false" {
			u += " [--" + f.Name + "]"
		} else {
			u += " [--" + f.Name + " N]"
		}
	})
	return usageError(u)
}

// load reads the decompressed buffer of one replay.
func load(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, usageError("dissect-lab works on one round file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	r, err := dissect.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r.Data(), nil
}

// loadLayout loads the buffer and walks it.
func loadLayout(path string) ([]byte, wire.Layout, error) {
	b, err := load(path)
	if err != nil {
		return nil, wire.Layout{}, err
	}
	lay, err := wire.Walk(b)
	if err != nil {
		return nil, lay, fmt.Errorf("%s: %w", path, err)
	}
	return b, lay, nil
}

type channelLine struct {
	Channel   int    `json:"channel"`
	HasHeader bool   `json:"hasHeader"`
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Envelopes int    `json:"envelopes"`
	TickFirst uint32 `json:"tickFirst"`
	TickLast  uint32 `json:"tickLast"`
}

func channelLines(lay wire.Layout) []channelLine {
	out := make([]channelLine, 0, len(lay.Channels))
	for i, c := range lay.Channels {
		cl := channelLine{
			Channel: i, HasHeader: c.HasHeader, ID: hex.EncodeToString(c.ID[:]), Kind: c.Kind.String(),
			Start: c.Start, End: c.End, Envelopes: len(c.Envelopes),
		}
		if n := len(c.Envelopes); n > 0 {
			cl.TickFirst, cl.TickLast = c.Envelopes[0].Tick, c.Envelopes[n-1].Tick
		}
		out = append(out, cl)
	}
	return out
}

func unplacedFraction(lay wire.Layout) float64 {
	if lay.Size == 0 {
		return 0
	}
	return float64(lay.UnplacedBytes()) / float64(lay.Size)
}

func (l *lab) segments(args []string) error {
	path, err := l.parse("segments", args, nil)
	if err != nil {
		return err
	}
	_, lay, err := loadLayout(path)
	if err != nil {
		return err
	}
	for _, c := range channelLines(lay) {
		if err := l.emit(c); err != nil {
			return err
		}
	}
	for _, g := range lay.Gaps {
		if err := l.emit(struct {
			Gap   bool `json:"gap"`
			Start int  `json:"start"`
			End   int  `json:"end"`
		}{true, g.Start, g.End}); err != nil {
			return err
		}
	}
	return l.emit(struct {
		Summary          bool    `json:"summary"`
		Size             int     `json:"size"`
		Prelude          int     `json:"prelude"`
		UnplacedBytes    int     `json:"unplacedBytes"`
		UnplacedFraction float64 `json:"unplacedFraction"`
	}{true, lay.Size, lay.Prelude, lay.UnplacedBytes(), unplacedFraction(lay)})
}

type tlvLine struct {
	Tick   uint32  `json:"tick"`
	Offset int     `json:"offset"`
	Op     string  `json:"op"`
	Object string  `json:"object"`
	Field  string  `json:"field"`
	Index  *uint32 `json:"index,omitempty"`
	Len    *int    `json:"len,omitempty"`
	Value  *string `json:"value,omitempty"`
	Target *string `json:"target,omitempty"`
	Class  *string `json:"class,omitempty"`
	Count  *uint32 `json:"count,omitempty"`
}

func newTLVLine(tick uint32, t wire.Token) tlvLine {
	ln := tlvLine{Tick: tick, Offset: t.Offset, Op: t.Op.String(), Object: t.Object.String(), Field: t.Field.String()}
	if t.HasIndex {
		ln.Index = &t.Index
	}
	switch {
	case t.Op.IsValue():
		n, v := len(t.Value), hex.EncodeToString(t.Value)
		ln.Len, ln.Value = &n, &v
	case t.Op.IsRef():
		tg, cl := t.Target.String(), t.Class.String()
		ln.Target, ln.Class = &tg, &cl
	case t.Op == wire.OpBatch:
		ln.Count = &t.Count
	}
	return ln
}

func (l *lab) tlv(args []string) error {
	var limit int
	path, err := l.parse("tlv", args, func(fs *flag.FlagSet) {
		fs.IntVar(&limit, "limit", 0, "stop after N tokens (0 = unlimited)")
	})
	if err != nil {
		return err
	}
	if limit < 0 {
		return usageError("--limit must be 0 or more")
	}
	b, lay, err := loadLayout(path)
	if err != nil {
		return err
	}
	n := 0
	_, err = wire.DecodeTLV(b, lay, func(tick uint32, t wire.Token) error {
		if err := l.emit(newTLVLine(tick, t)); err != nil {
			return err
		}
		if n++; limit > 0 && n >= limit {
			return errLimit
		}
		return nil
	})
	if err != nil && err != errLimit {
		return err
	}
	return nil
}

type movementLine struct {
	Tick    uint32  `json:"tick"`
	Offset  int     `json:"offset"`
	Entity  string  `json:"entity"`
	Tag     string  `json:"tag"`
	Len     int     `json:"len"`
	Payload *string `json:"payload,omitempty"`
}

func (l *lab) movement(args []string) error {
	var limit int
	var noPayload bool
	path, err := l.parse("movement", args, func(fs *flag.FlagSet) {
		fs.IntVar(&limit, "limit", 0, "stop after N records (0 = unlimited)")
		fs.BoolVar(&noPayload, "no-payload", false, "omit the payload hex")
	})
	if err != nil {
		return err
	}
	if limit < 0 {
		return usageError("--limit must be 0 or more")
	}
	b, lay, err := loadLayout(path)
	if err != nil {
		return err
	}
	n := 0
	_, err = wire.DecodeMovement(b, lay, func(g wire.Group) error {
		for _, r := range g.Records {
			ln := movementLine{
				Tick: g.Envelope.Tick, Offset: r.Offset, Entity: r.Entity.String(),
				Tag: hex.EncodeToString(r.Tag[:]), Len: len(r.Payload) + len(r.Tag),
			}
			if !noPayload {
				p := hex.EncodeToString(r.Payload)
				ln.Payload = &p
			}
			if err := l.emit(ln); err != nil {
				return err
			}
			if n++; limit > 0 && n >= limit {
				return errLimit
			}
		}
		return nil
	})
	if err != nil && err != errLimit {
		return err
	}
	return nil
}

func (l *lab) coverage(args []string) error {
	path, err := l.parse("coverage", args, nil)
	if err != nil {
		return err
	}
	b, lay, err := loadLayout(path)
	if err != nil {
		return err
	}
	ts, err := wire.DecodeTLV(b, lay, func(uint32, wire.Token) error { return nil })
	if err != nil {
		return err
	}
	ms, err := wire.DecodeMovement(b, lay, nil)
	if err != nil {
		return err
	}
	return l.emit(map[string]any{
		"size":             lay.Size,
		"unplacedFraction": unplacedFraction(lay),
		"channels":         channelLines(lay),
		"tlv": map[string]int{
			"envelopes": ts.Envelopes, "blocks": ts.Blocks, "tokens": ts.Tokens, "errors": len(ts.Errors),
		},
		"movement": map[string]any{
			"envelopes": ms.Envelopes, "groups": ms.Groups, "records": ms.Records,
			"badBytes": ms.BadBytes, "ticksMonotonic": ms.TicksMonotonic,
		},
	})
}

// readTimeline opens one round file, builds its timeline and returns it with the decompressed buffer.
func readTimeline(path string) ([]byte, *dissect.Timeline, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if st.IsDir() {
		return nil, nil, usageError("dissect-lab works on one round file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	zerolog.SetGlobalLevel(zerolog.Disabled)
	r, err := dissect.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	r.EnableTimeline()
	b := r.Data() // only available before Read
	if err := r.Read(); !dissect.Ok(err) {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	tl, err := r.Timeline()
	if err != nil {
		var te *dissect.TimelineError
		if errors.As(err, &te) {
			return nil, nil, fmt.Errorf("%s: timeline: %s", path, te.Reason)
		}
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, tl, nil
}

type joinLine struct {
	Seq          int                `json:"seq"`
	Type         string             `json:"type"`
	Actor        string             `json:"actor,omitempty"`
	Target       string             `json:"target,omitempty"`
	Tick         uint32             `json:"tick"`
	ElapsedMs    int64              `json:"elapsedMs"`
	RoundClockMs uint32             `json:"roundClockMs"`
	Phase        dissect.RoundPhase `json:"phase"`
	Status       string             `json:"status"`
}

func (l *lab) join(args []string) error {
	path, err := l.parse("join", args, nil)
	if err != nil {
		return err
	}
	_, tl, err := readTimeline(path)
	if err != nil {
		return err
	}
	for _, e := range tl.Events {
		if err := l.emit(joinLine{
			Seq: e.Seq, Type: e.Type.String(), Actor: e.Actor, Target: e.Target, Tick: e.Tick, ElapsedMs: e.ElapsedMs,
			RoundClockMs: e.RoundClockMs, Phase: e.Phase, Status: e.Status,
		}); err != nil {
			return err
		}
	}
	return nil
}

type graphLine struct {
	Tick        uint32 `json:"tick"`
	Owner       string `json:"owner"`
	Field       string `json:"field"`
	Target      string `json:"target"`
	Class       string `json:"class"`
	Index       uint32 `json:"index"`
	HasIndex    bool   `json:"hasIndex"`
	OwnerPlayer string `json:"ownerPlayer,omitempty"`
}

func (l *lab) graph(args []string) error {
	path, err := l.parse("graph", args, nil)
	if err != nil {
		return err
	}
	b, tl, err := readTimeline(path)
	if err != nil {
		return err
	}
	lay, err := wire.Walk(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	players := map[string]string{}
	for _, en := range tl.Entities {
		if en.Kind == "player" && en.Owner != "" {
			players[en.ID] = en.Owner
		}
	}
	g := wire.NewGraph()
	var edges []wire.Edge
	_, err = wire.DecodeTLV(b, lay, func(tick uint32, t wire.Token) error {
		g.Add(tick, t)
		if t.Op.IsRef() && !t.Target.IsNull() {
			edges = append(edges, wire.Edge{Owner: t.Object, Field: t.Field, Target: t.Target, Class: t.Class,
				Index: t.Index, HasIndex: t.HasIndex, Tick: tick})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	isPlayer := func(h wire.Handle) bool { _, ok := players[h.String()]; return ok }
	for _, e := range edges {
		ln := graphLine{
			Tick: e.Tick, Owner: e.Owner.String(), Field: e.Field.String(), Target: e.Target.String(),
			Class: e.Class.String(), Index: e.Index, HasIndex: e.HasIndex,
		}
		if isPlayer(e.Owner) {
			ln.OwnerPlayer = players[e.Owner.String()]
		} else if h, ok := g.Ancestor(e.Owner, isPlayer); ok {
			ln.OwnerPlayer = players[h.String()]
		}
		if err := l.emit(ln); err != nil {
			return err
		}
	}
	return nil
}
