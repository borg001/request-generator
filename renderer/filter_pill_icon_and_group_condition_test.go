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

// A list can open with filters of its own choosing: served as defaults, kept
// apart by a clone, and refused when they name a filter the list does not have.
func TestFilterDefaultsAreServedAndCloned(t *testing.T) {
	filters := Filters{Defaults: map[string]interface{}{"location_ids": []int{1, 2}}}
	raw, err := json.Marshal(filters)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"defaults":{"location_ids":[1,2]}`) {
		t.Fatalf("filters JSON %s lacks the defaults", raw)
	}
	clone := cloneFilters(&filters)
	clone.Defaults["location_ids"] = "changed"
	if _, same := filters.Defaults["location_ids"].(string); same {
		t.Fatal("a clone shares its defaults with the original")
	}
}
