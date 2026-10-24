package wire

// Edge is one ownership reference: Owner's Field points at Target.
type Edge struct {
	Owner    Handle
	Field    FieldID
	Target   Handle
	Class    ClassID
	Index    uint32
	HasIndex bool
	Tick     uint32
}

// Graph indexes the reference tokens of a replay so an object can be resolved
// to the objects that own it.
type Graph struct {
	parents  map[Handle]Edge // last edge that targeted the handle
	classes  map[Handle]ClassID
	children map[Handle][]Edge
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{
		parents:  map[Handle]Edge{},
		classes:  map[Handle]ClassID{},
		children: map[Handle][]Edge{},
	}
}

// Add records t when t.Op.IsRef() and !t.Target.IsNull(); other tokens are ignored.
func (g *Graph) Add(tick uint32, t Token) {
	if !t.Op.IsRef() || t.Target.IsNull() {
		return
	}
	e := Edge{Owner: t.Object, Field: t.Field, Target: t.Target, Class: t.Class,
		Index: t.Index, HasIndex: t.HasIndex, Tick: tick}
	g.parents[t.Target] = e
	g.classes[t.Target] = t.Class
	g.children[t.Object] = append(g.children[t.Object], e)
}

// Class returns the class last seen for h as a reference target.
func (g *Graph) Class(h Handle) (ClassID, bool) {
	c, ok := g.classes[h]
	return c, ok
}

// Parent returns the last edge that targeted h.
func (g *Graph) Parent(h Handle) (Edge, bool) {
	e, ok := g.parents[h]
	return e, ok
}

// Children returns the edges whose owner is h, in the order they were added.
func (g *Graph) Children(h Handle) []Edge { return g.children[h] }

const maxAncestorSteps = 16

// Ancestor walks Parent links from h (at most 16 steps) and returns the first handle for which stop returns true.
func (g *Graph) Ancestor(h Handle, stop func(Handle) bool) (Handle, bool) {
	cur := h
	for i := 0; i < maxAncestorSteps; i++ {
		e, ok := g.parents[cur]
		if !ok {
			return Handle{}, false
		}
		cur = e.Owner
		if stop(cur) {
			return cur, true
		}
	}
	return Handle{}, false
}
