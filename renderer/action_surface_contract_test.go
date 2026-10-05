package renderer

import (
	"encoding/json"
	"testing"
)

func TestActionSurfacePresentationSerializesAndClones(t *testing.T) {
	showHeader := false
	action := Action{
		ID:   "open",
		Type: ActionModal,
		ActionPresentation: ActionPresentation{
			Placement: ActionPlacementFilterFooter,
		},
		Modal: &ModalAction{Renderer: RendererUniversalDisplay, ShowHeader: &showHeader},
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("validate action: %v", err)
	}

	cloned := cloneActionValue(action)
	*cloned.Modal.ShowHeader = true
	if *action.Modal.ShowHeader {
		t.Fatal("clone must not share modal.show_header pointer")
	}

	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	const expected = `"placement":"filter_footer"`
	if !containsJSONFragment(string(payload), expected) {
		t.Fatalf("expected %s in %s", expected, payload)
	}
	if !containsJSONFragment(string(payload), `"show_header":false`) {
		t.Fatalf("expected explicit false show_header in %s", payload)
	}
}

func TestActionSurfacePresentationRejectsUnknownPlacement(t *testing.T) {
	action := Action{ActionPresentation: ActionPresentation{Placement: "sidebar"}}
	if err := action.Validate(); err == nil {
		t.Fatal("expected invalid placement error")
	}
}

func TestActionSurfacePresentationAcceptsBadgePlacement(t *testing.T) {
	action := Action{
		ID:   "owner",
		Type: ActionRoute,
		ActionPresentation: ActionPresentation{
			Placement: ActionPlacementBadge,
		},
		Route: RouteAction{Path: "/records/{id}", Params: map[string]string{"id": "record.id"}},
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("validate badge action: %v", err)
	}

	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal badge action: %v", err)
	}
	if !containsJSONFragment(string(payload), `"placement":"badge"`) {
		t.Fatalf("expected badge placement in %s", payload)
	}
}

func containsJSONFragment(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

// A list may hold one action in sight under its filters (theGHub1/api#429).
func TestActionPlacementStickyIsValid(t *testing.T) {
	action := Action{ID: "give_order", Type: ActionRoute, Label: "Order", ActionPresentation: ActionPresentation{Placement: ActionPlacementSticky}, Route: RouteAction{Path: "/orders/create"}}
	if err := action.Validate(); err != nil {
		t.Fatalf("validate sticky action: %v", err)
	}
	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	if !containsJSONFragment(string(payload), `"placement":"sticky"`) {
		t.Fatalf("expected the sticky placement in %s", payload)
	}
	if !ActionPlacementSticky.Valid() || ActionPlacement("floating").Valid() {
		t.Fatal("only the declared placements are valid")
	}
}

// A list's one action may float in the corner of the screen
// (theGHub1/api#440).
func TestActionPlacementCornerIsValid(t *testing.T) {
	action := Action{ID: "give_order", Type: ActionRoute, Label: "Order", ActionPresentation: ActionPresentation{Placement: ActionPlacementCorner, Icon: "plus"}, Route: RouteAction{Path: "/orders/create"}}
	if err := action.Validate(); err != nil {
		t.Fatalf("validate corner action: %v", err)
	}
	payload, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("marshal action: %v", err)
	}
	if !containsJSONFragment(string(payload), `"placement":"corner"`) {
		t.Fatalf("expected the corner placement in %s", payload)
	}
}

// An item that cannot be removed says why (theGHub1/api#449).
func TestMediaGalleryItemCarriesItsRemoveRefusal(t *testing.T) {
	plain, _ := json.Marshal(MediaGalleryItem{ID: "a"})
	if containsJSONFragment(string(plain), `"remove_refusal"`) {
		t.Fatalf("an item that can be removed says nothing: %s", plain)
	}
	kept, _ := json.Marshal(MediaGalleryItem{ID: "b", Cover: true, RemoveRefusal: "Replace it first"})
	if !containsJSONFragment(string(kept), `"remove_refusal":"Replace it first"`) {
		t.Fatalf("expected the refusal in %s", kept)
	}
}
