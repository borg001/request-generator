package renderer

import (
	"encoding/json"
	"testing"
)

func TestLayoutMobileSlotsAreSerializedAndCloned(t *testing.T) {
	layout := &Layout{
		Type:        LayoutThreeColumn,
		Slots:       []string{"left", "center", "right"},
		MobileSlots: []string{"center", "left", "right"},
	}

	encoded, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal layout: %v", err)
	}
	if got, want := string(encoded), `{"type":"three_column","slots":["left","center","right"],"mobile_slots":["center","left","right"]}`; got != want {
		t.Fatalf("layout JSON = %s, want %s", got, want)
	}

	cloned := cloneLayout(layout)
	cloned.MobileSlots[0] = "left"
	if layout.MobileSlots[0] != "center" {
		t.Fatalf("clone must not share mobile slots: %#v", layout.MobileSlots)
	}
}

func TestRecordSectionMobileOrderIsSerialized(t *testing.T) {
	section := RecordSection{ID: "summary", MobileOrder: 20}

	encoded, err := json.Marshal(section)
	if err != nil {
		t.Fatalf("marshal record section: %v", err)
	}
	if got, want := string(encoded), `{"id":"summary","mobile_order":20}`; got != want {
		t.Fatalf("record section JSON = %s, want %s", got, want)
	}
}
