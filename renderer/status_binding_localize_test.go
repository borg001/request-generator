package renderer

import "testing"

// The status chip on a card names states through its own map. A producer that
// merges columns cannot ship option metadata beside the value, so the words
// must be translated like every other card text instead of arriving as keys.
func TestStatusBindingLabelMapIsLocalized(t *testing.T) {
	source := Universal{List: &ListPage{
		Grid: &Grid{Mode: GridModeList},
		CardSchema: &CardSchema{
			Status: &StatusBinding{
				Field:    "card_status",
				LabelMap: map[string]string{"draft": "profiles.status.draft"},
				ToneMap:  map[string]string{"draft": "glass-slate"},
			},
		},
	}}

	if err := source.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	localized := Localize(source, func(value, _ string) string { return "localized:" + value })
	if got := localized.List.CardSchema.Status.LabelMap["draft"]; got != "localized:profiles.status.draft" {
		t.Fatalf("Status.LabelMap = %q", got)
	}

	cloned := source.Clone()
	cloned.List.CardSchema.Status.LabelMap["draft"] = "other"
	if source.List.CardSchema.Status.LabelMap["draft"] != "profiles.status.draft" {
		t.Fatal("Clone() shares status label map")
	}
}
