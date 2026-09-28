package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A set of chips names the field it reads and how many of it the card shows;
// a copy of the card does not share it, and its name reaches the reader
// translated.
func TestCardChips(t *testing.T) {
	schema := &CardSchema{Chips: &CardChips{Field: "countries", Icon: "pin", Tone: "pink", MaxVisible: 3, Label: "cards.countries"}}
	require.NoError(t, schema.Validate())

	copied := cloneCardSchema(schema)
	copied.Chips.Label = "changed"
	require.Equal(t, "cards.countries", schema.Chips.Label)

	localizer := textLocalizer{resolve: func(value, _ string) string { return "T:" + value }}
	localizer.localizeCardSchema(copied)
	require.Equal(t, "T:changed", copied.Chips.Label)

	require.EqualError(t, (&CardSchema{Chips: &CardChips{}}).Validate(), "renderer.CardSchema: chips field is required")
	require.EqualError(t, (&CardSchema{Chips: &CardChips{Field: "countries", MaxVisible: -1}}).Validate(), "renderer.CardSchema: chips max_visible cannot be negative")
}
