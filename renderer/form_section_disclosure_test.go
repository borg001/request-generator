package renderer

import "testing"

// A block that folds away is opened by its own head, so it needs something to
// be opened by: a fold with no title could never be reopened.
func TestFoldingSectionNeedsATitleToOpenItBy(t *testing.T) {
	section := FormSection{
		ID:       "model-data",
		Renderer: RendererUniversalSection,
		Block:    &Block{Type: BlockPanel, Disclosure: DisclosureClosed},
	}
	if err := validateSectionDisclosure(section); err == nil {
		t.Fatal("a folding section with no title was accepted")
	}
	section.PanelTitle = "settings.nav.data"
	if err := validateSectionDisclosure(section); err != nil {
		t.Fatalf("a named folding section was refused: %v", err)
	}
}

func TestOnlyTheKnownFoldStatesAreAccepted(t *testing.T) {
	section := FormSection{
		ID:         "rates",
		PanelTitle: "settings.rates.title",
		Block:      &Block{Type: BlockPanel, Disclosure: DisclosureToken("half-open")},
	}
	if err := validateSectionDisclosure(section); err == nil {
		t.Fatal("an unknown fold state was accepted")
	}
	for _, state := range []DisclosureToken{"", DisclosureOpen, DisclosureClosed} {
		section.Block.Disclosure = state
		if err := validateSectionDisclosure(section); err != nil {
			t.Fatalf("fold state %q was refused: %v", state, err)
		}
	}
}

// The blocks of a section fold on their own, so they are checked as well.
func TestTheBlocksOfASectionAreCheckedToo(t *testing.T) {
	section := FormSection{
		ID:         "profile",
		PanelTitle: "settings.profile.title",
		Sections: []FormSection{{
			ID:    "model-location",
			Block: &Block{Type: BlockPanel, Disclosure: DisclosureClosed},
		}},
	}
	if err := validateSectionDisclosure(section); err == nil {
		t.Fatal("a nameless folding block inside a section was accepted")
	}
}
