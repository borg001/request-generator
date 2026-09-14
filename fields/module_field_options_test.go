package fields

import (
	"encoding/json"
	"testing"
)

// An option offered as a card carries its mark, trailing figure and note to
// the application; an option without them stays as short as before.
func TestModuleFieldOptionsCarryCardDecorations(t *testing.T) {
	decorated, err := json.Marshal(ModuleFieldOptions{Value: "a", Label: "A", Badge: "Best", BadgeTone: "amber", Trailing: "$9.99", TrailingNote: "per unit", Note: "bonus"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(decorated, &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"badge": "Best", "badge_tone": "amber", "trailing": "$9.99", "trailing_note": "per unit", "note": "bonus"} {
		if got[key] != want {
			t.Fatalf("%s = %v, want %q", key, got[key], want)
		}
	}
	plain, err := json.Marshal(ModuleFieldOptions{Value: "b", Label: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"value":"b","label":"B"}` {
		t.Fatalf("plain option = %s", plain)
	}
}
