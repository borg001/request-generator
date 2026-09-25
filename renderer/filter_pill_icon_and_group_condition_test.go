package renderer

import (
	"encoding/json"
	"strings"
	"testing"
)

// A pill that switches the list to a kind of result can carry a mark, and a
// group can stand in the row only while that pill is on.
func TestFilterPillIconAndGroupConditionAreServed(t *testing.T) {
	filters := Filters{
		PillRows: [][]FilterPill{{{Label: "Tours", Key: "open_tour", Val: "1", Tone: "amber", Icon: "ref_plane"}}},
		Groups: []FilterGroup{{
			ID: "tour_commission", Label: "Commission", Placement: FilterGroupPlacementPrimary, Fields: []string{"tour_commission"},
			VisibleIf: &Condition{Path: "filters.open_tour", Equals: "1"},
		}},
	}
	raw, err := json.Marshal(filters)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"icon":"ref_plane"`, `"tone":"amber"`, `"visible_if":{"path":"filters.open_tour","equals":"1"}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("filters JSON %s lacks %s", raw, want)
		}
	}
	if err := validateFilterGroups("list", &filters); err != nil {
		t.Fatalf("a group with a condition is a valid group: %v", err)
	}
}
