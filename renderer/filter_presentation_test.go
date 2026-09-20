package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterPresentationUsesExistingPills(t *testing.T) {
	value := Universal{List: &ListPage{Filters: &Filters{
		Presentation: FilterPresentationToolbar,
		PillRows: [][]FilterPill{{
			{Label: "All", Presentation: FilterPillPresentationTabs, CountField: "all_count"},
			{Label: "Unread", Key: "read", Val: "false", Presentation: FilterPillPresentationToggle},
			{Label: "Important", Key: "priority", Val: "important", Presentation: FilterPillPresentationSummary, Tone: "cyan", CountField: "important_count"},
		}},
	}}}

	require.NoError(t, value.Validate())
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	require.JSONEq(t, `{"list_page":{"filters":{"presentation":"toolbar","enabled":false,"pill_rows":[[{"label":"All","count_field":"all_count","presentation":"tabs"},{"label":"Unread","key":"read","val":"false","presentation":"toggle"},{"label":"Important","key":"priority","val":"important","count_field":"important_count","presentation":"summary","tone":"cyan"}]]}}}`, string(payload))
}

func TestFilterPresentationValidation(t *testing.T) {
	tests := []struct {
		name  string
		value Filters
		want  string
	}{
		{
			name:  "unknown filter layout",
			value: Filters{Presentation: "drawer"},
			want:  `renderer.Universal: list page filters have invalid presentation "drawer"`,
		},
		{
			name:  "unknown pill presentation",
			value: Filters{PillRows: [][]FilterPill{{{Label: "All", Presentation: "button"}}}},
			want:  `renderer.Universal: list page filter pill "All" has invalid presentation "button"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Universal{List: &ListPage{Filters: &test.value}}).Validate()
			require.EqualError(t, err, test.want)
		})
	}
}

func TestSummaryItemsAreTypedAndLocalized(t *testing.T) {
	value := Universal{List: &ListPage{Summary: &Summary{
		Items: []SummaryItem{{ID: "all", Label: "summary.all", ValueField: "all_count"}},
	}}}
	require.NoError(t, value.Validate())
	localized := Localize(value, func(value, _ string) string { return "localized:" + value })
	require.Equal(t, "localized:summary.all", localized.List.Summary.Items[0].Label)
	require.Empty(t, localized.List.Summary.Items[0].LabelKey)

	clone := value.Clone()
	clone.List.Summary.Items[0].ValueField = "other_count"
	require.Equal(t, "all_count", value.List.Summary.Items[0].ValueField)
}

func TestSummaryItemValidation(t *testing.T) {
	err := (Universal{List: &ListPage{Summary: &Summary{Items: []SummaryItem{{ID: "all", Label: "All"}}}}}).Validate()
	require.EqualError(t, err, "renderer.Universal: list page: renderer.Summary: item \"all\" value field is required")
}

func TestFilterPillsCanBeOfferedAsOneMenu(t *testing.T) {
	value := Universal{List: &ListPage{Filters: &Filters{
		Presentation: FilterPresentationToolbar,
		PillRows: [][]FilterPill{
			{{Label: "Followers", Presentation: FilterPillPresentationTabs}, {Label: "Following", Key: "list_tag", Val: "following", Presentation: FilterPillPresentationTabs}},
			{{Label: "All roles", Presentation: FilterPillPresentationMenu}, {Label: "Agencies", Key: "profile_type", Val: "agency", Presentation: FilterPillPresentationMenu}},
		},
	}}}

	require.NoError(t, value.Validate())
	payload, err := json.Marshal(value.List.Filters.PillRows[1])
	require.NoError(t, err)
	require.JSONEq(t, `[{"label":"All roles","presentation":"menu"},{"label":"Agencies","key":"profile_type","val":"agency","presentation":"menu"}]`, string(payload))
}
