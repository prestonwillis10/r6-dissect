package wire_test

import (
	"testing"

	wire "github.com/redraskal/r6-dissect/dissect/wire"
)

func TestGraph(t *testing.T) {
	g := wire.NewGraph()
	player, pawn, inv := h("a07305f0"), h("303b04f0"), h("c4d404f0")
	g.Add(1, wire.Token{Op: wire.OpRefNamed, Object: player, Field: fid("27c08dca"), Target: pawn, Class: cid("b2216bf3")})
	g.Add(2, wire.Token{Op: wire.OpRef, Object: pawn, Field: fid("e8d1e539"), Target: inv, Class: cid("579fb57b")})
	g.Add(3, wire.Token{Op: wire.OpRef, Object: pawn, Field: fid("1ed17fd6"), Target: wire.Handle{}, Class: wire.ClassID{}}) // null: ignored
	g.Add(4, wire.Token{Op: wire.OpValue, Object: pawn, Field: fid("e58c06e9"), Value: []byte{2, 0, 0, 0}})                  // value: ignored
	if c, ok := g.Class(pawn); !ok || c != cid("b2216bf3") {
		t.Fatal("class of pawn", c, ok)
	}
	if e, ok := g.Parent(inv); !ok || e.Owner != pawn || e.Field != fid("e8d1e539") || e.Tick != 2 {
		t.Fatal("parent of inv", e, ok)
	}
	g.Add(5, wire.Token{Op: wire.OpRef, Object: player, Field: fid("e8d1e539"), Target: inv, Class: cid("579fb57b")})
	if e, ok := g.Parent(inv); !ok || e.Owner != player || e.Tick != 5 {
		t.Fatal("parent must be the last edge", e, ok)
	}
	g.Add(6, wire.Token{Op: wire.OpRef, Object: pawn, Field: fid("11111111"), Target: h("0102030405060708"), Class: cid("579fb57b")})
	if ch := g.Children(pawn); len(ch) != 2 || ch[0].Tick != 2 || ch[1].Tick != 6 {
		t.Fatal("children must keep add order", ch)
	}
	if n := len(g.Children(pawn)); n != 2 {
		t.Fatal("children of pawn", n)
	}
	isPlayer := func(x wire.Handle) bool { return x == player }
	if a, ok := g.Ancestor(inv, isPlayer); !ok || a != player {
		t.Fatal("ancestor", a, ok)
	}
	if _, ok := g.Ancestor(player, func(wire.Handle) bool { return false }); ok {
		t.Fatal("no ancestor must be reported as !ok")
	}
}

func TestGraph_AncestorCycle(t *testing.T) {
	g := wire.NewGraph()
	a, b := h("a07305f0"), h("303b04f0")
	g.Add(1, wire.Token{Op: wire.OpRef, Object: a, Field: fid("27c08dca"), Target: b, Class: cid("b2216bf3")})
	g.Add(2, wire.Token{Op: wire.OpRef, Object: b, Field: fid("27c08dca"), Target: a, Class: cid("b2216bf3")})
	calls := 0
	if _, ok := g.Ancestor(a, func(wire.Handle) bool { calls++; return calls > 16 }); ok {
		t.Fatal("a cycle with no match must terminate as !ok")
	}
	if calls != 16 {
		t.Fatalf("stop called %d times, want 16", calls)
	}
}

func TestHandleOf(t *testing.T) {
	if wire.HandleOf(0x00000000f00573a0) != h("a07305f0") {
		t.Fatal(wire.HandleOf(0x00000000f00573a0))
	}
}

func TestGraph_Fixture(t *testing.T) {
	b := decompress(t, fixture(t, "Y11S3", "custom_fortress_plant.rec"))
	l, err := wire.Walk(b)
	if err != nil {
		t.Fatal(err)
	}
	g := wire.NewGraph()
	var owners = map[wire.Handle]bool{}
	_, err = wire.DecodeTLV(b, l, func(tick uint32, tk wire.Token) error {
		g.Add(tick, tk)
		if tk.Op.IsRef() && !tk.Target.IsNull() {
			owners[tk.Object] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var pawnEdges int
	for o := range owners {
		for _, e := range g.Children(o) {
			if e.Field == fid("27c08dca") {
				pawnEdges++
			}
		}
	}
	if pawnEdges < 10 {
		t.Errorf("pawn edges = %d", pawnEdges)
	}
}
