package renderer

import "testing"

// A component can say how many cells a phone row holds, within what a grid
// holds at all; and can read a filled form back in the form's look.
func TestDisplayComponentMobileColumnsAndFormLook(t *testing.T) {
	for _, columns := range []int{0, 1, 3, 4} {
		component := DisplayComponent{ID: "figures", Type: DisplayDataList, MobileColumns: columns, FormLook: true}
		if err := component.Validate(); err != nil {
			t.Fatalf("%d columns: %v", columns, err)
		}
	}
	for _, columns := range []int{-1, 5} {
		component := DisplayComponent{ID: "figures", Type: DisplayDataList, MobileColumns: columns}
		if err := component.Validate(); err == nil {
			t.Fatalf("%d columns should be refused", columns)
		}
	}
}
