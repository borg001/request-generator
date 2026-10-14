package renderer

import (
	"encoding/json"
	"strings"
	"testing"
)

func mediaSelectionSection(selection *MediaGallerySelection, items ...MediaGalleryItem) Universal {
	return Universal{Form: &FormPage{Sections: []FormSection{{
		ID:             "media",
		Renderer:       RendererMediaGallery,
		MediaItems:     items,
		MediaSelection: selection,
	}}}}
}

func TestMediaGallerySelectionIsCarriedAsDeclared(t *testing.T) {
	selection := &MediaGallerySelection{
		Actions: []Action{
			{ID: "send_a", Type: ActionEmit, Label: "Send to A"},
			{ID: "send_b", Type: ActionEmit, Label: "Send to B"},
		},
		Limit:       10,
		StartLabel:  "Select",
		CancelLabel: "Cancel",
		CountLabel:  "Selected: {count}",
		LimitLabel:  "No more than 10",
		Note:        "What is sent to A also stands in B",
	}
	render := mediaSelectionSection(selection,
		MediaGalleryItem{ID: "one", Tone: "amber", SelectActions: []string{"send_a", "send_b"}},
		MediaGalleryItem{ID: "two", SelectActions: []string{"send_a"}},
		MediaGalleryItem{ID: "three"},
	)
	if err := render.Validate(); err != nil {
		t.Fatalf("a declared selection must be valid: %v", err)
	}
	encoded, err := json.Marshal(render.Form.Sections[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"media_selection":{"actions":[{"id":"send_a"`,
		`"limit":10`,
		`"start_label":"Select"`,
		`"cancel_label":"Cancel"`,
		`"count_label":"Selected: {count}"`,
		`"limit_label":"No more than 10"`,
		`"note":"What is sent to A also stands in B"`,
		`"tone":"amber"`,
		`"select_actions":["send_a","send_b"]`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("section JSON must carry %s, got %s", want, encoded)
		}
	}
	plain, _ := json.Marshal(MediaGalleryItem{ID: "three"})
	if strings.Contains(string(plain), "tone") || strings.Contains(string(plain), "select_actions") {
		t.Fatalf("an item that says neither must carry neither, got %s", plain)
	}
}

func TestMediaGallerySelectionRefusesWhatCannotBeDrawn(t *testing.T) {
	cases := map[string]Universal{
		"an action without an id": mediaSelectionSection(&MediaGallerySelection{Actions: []Action{{Type: ActionEmit}}}),
		"the same action twice": mediaSelectionSection(&MediaGallerySelection{Actions: []Action{
			{ID: "send", Type: ActionEmit}, {ID: "send", Type: ActionEmit},
		}}),
		"a negative limit": mediaSelectionSection(&MediaGallerySelection{Limit: -1}),
		"an item picked for an undeclared action": mediaSelectionSection(
			&MediaGallerySelection{Actions: []Action{{ID: "send", Type: ActionEmit}}},
			MediaGalleryItem{ID: "one", SelectActions: []string{"other"}},
		),
		"an item picked where nothing is declared": mediaSelectionSection(nil, MediaGalleryItem{ID: "one", SelectActions: []string{"send"}}),
		"an action with an unknown placement": mediaSelectionSection(&MediaGallerySelection{Actions: []Action{
			{ID: "send", Type: ActionEmit, ActionPresentation: ActionPresentation{Placement: ActionPlacement("nowhere")}},
		}}),
	}
	for name, render := range cases {
		if err := render.Validate(); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
}

func TestMediaGallerySelectionIsClonedAndListedWithTheActions(t *testing.T) {
	render := mediaSelectionSection(
		&MediaGallerySelection{Actions: []Action{{ID: "send", Type: ActionEmit, Label: "Send"}}},
		MediaGalleryItem{ID: "one", SelectActions: []string{"send"}},
	)
	cloned := render.Clone()
	cloned.Form.Sections[0].MediaSelection.Actions[0].Label = "Changed"
	cloned.Form.Sections[0].MediaItems[0].SelectActions[0] = "changed"
	if render.Form.Sections[0].MediaSelection.Actions[0].Label != "Send" {
		t.Fatal("a clone must not share the selection's actions with its source")
	}
	if render.Form.Sections[0].MediaItems[0].SelectActions[0] != "send" {
		t.Fatal("a clone must not share an item's select actions with its source")
	}
	listed := false
	for _, action := range render.Actions() {
		if action.ID == "send" {
			listed = true
		}
	}
	if !listed {
		t.Fatal("the selection's actions must be listed among the renderer's actions")
	}
}

// A gallery item says which group it belongs to, and the gallery names the two
// ways it is read (theGHub1/api#502).
func TestMediaGalleryItemsCarryTheirGroup(t *testing.T) {
	encoded, err := json.Marshal(MediaGalleryItem{ID: "media-1", Group: "publication-7"})
	if err != nil || !strings.Contains(string(encoded), `"group":"publication-7"`) {
		t.Fatalf("group is not carried: %s %v", encoded, err)
	}
	labels, err := json.Marshal(MediaGalleryLabels{GroupItems: "Photos", GroupGroups: "Publications"})
	if err != nil || !strings.Contains(string(labels), `"group_items":"Photos"`) || !strings.Contains(string(labels), `"group_groups":"Publications"`) {
		t.Fatalf("group labels are not carried: %s %v", labels, err)
	}
	clone := cloneMediaGalleryItems([]MediaGalleryItem{{ID: "media-1", Group: "publication-7"}})
	if clone[0].Group != "publication-7" {
		t.Fatalf("group is lost by a clone")
	}
}
