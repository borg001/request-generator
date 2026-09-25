package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A range can go without an end: the form sends a flag instead, and the
// switch that sets it is named and explained in the reader's language.
func TestDateRangeOpenEndIsDeclaredAndLocalized(t *testing.T) {
	value := Universal{Form: &FormPage{
		Fields: []string{"from", "to", "open_ended"},
		Sections: []FormSection{{
			ID: "dates", Renderer: RendererDateRange, Fields: []string{"from", "to", "open_ended"},
			DateRange: &DateRangeConfig{StartField: "from", EndField: "to", OpenEndField: "open_ended", OpenEndLabel: "dates.open", OpenEndHint: "dates.open_hint"},
		}},
	}}
	require.NoError(t, value.Validate())
	localized := Localize(value, func(value, _ string) string { return "localized:" + value })
	require.Equal(t, "localized:dates.open", localized.Form.Sections[0].DateRange.OpenEndLabel)
	require.Equal(t, "localized:dates.open_hint", localized.Form.Sections[0].DateRange.OpenEndHint)
}

func TestDateRangeOpenEndRejectsInvalidContracts(t *testing.T) {
	base := func() Universal {
		return Universal{Form: &FormPage{Fields: []string{"from", "to", "open_ended"}, Sections: []FormSection{{
			ID: "dates", Renderer: RendererDateRange, Fields: []string{"from", "to", "open_ended"},
			DateRange: &DateRangeConfig{StartField: "from", EndField: "to", OpenEndField: "open_ended", OpenEndLabel: "dates.open"},
		}}}}
	}
	tests := []struct {
		name string
		edit func(*Universal)
		want string
	}{
		{name: "same as a date", edit: func(value *Universal) { value.Form.Sections[0].DateRange.OpenEndField = "to" }, want: `renderer.Universal: date range section "dates" open end field must differ from its dates`},
		{name: "no label", edit: func(value *Universal) { value.Form.Sections[0].DateRange.OpenEndLabel = "" }, want: `renderer.Universal: date range section "dates" open end field needs a label`},
		{name: "not in the form", edit: func(value *Universal) { value.Form.Fields = []string{"from", "to"} }, want: `renderer.Universal: date range section "dates" field "open_ended" is not declared by the form`},
		{name: "not in the section", edit: func(value *Universal) { value.Form.Sections[0].Fields = []string{"from", "to"} }, want: `renderer.Universal: date range section "dates" field "open_ended" is not declared by the section`},
	}
	for _, current := range tests {
		t.Run(current.name, func(t *testing.T) {
			value := base()
			current.edit(&value)
			require.EqualError(t, value.Validate(), current.want)
		})
	}
}
