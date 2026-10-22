package dissect

import (
	"encoding/json"
	"testing"
)

// TestMapNames pins worldid -> name for the modernized and new maps.
func TestMapNames(t *testing.T) {
	cases := []struct {
		id   int
		name string
	}{
		{398899676157, "FortressY10"},
		{418126004176, "ConsulateY10"},
		{409325881472, "VillaY10"},
		{423767322185, "TowerY10"},
		{436375283234, "CoastlineY10"},
		{419965653950, "CalypsoCasino"},
	}
	for _, c := range cases {
		m := Map(c.id)
		if got := m.String(); got != c.name {
			t.Errorf("Map(%d).String() = %q, want %q", c.id, got, c.name)
		}
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		var x struct {
			Name string `json:"name"`
			ID   int    `json:"id"`
		}
		if err := json.Unmarshal(b, &x); err != nil {
			t.Fatal(err)
		}
		if x.Name != c.name || x.ID != c.id {
			t.Errorf("Map(%d) JSON = %s, want name %q id %d", c.id, b, c.name, c.id)
		}
		var back Map
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if back != m {
			t.Errorf("Map(%d) JSON round trip = %d", c.id, back)
		}
	}
}
