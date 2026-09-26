package renderer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The words of a component's filter and selection are translated for each
// reader. Shared with the producer's page, the first reader's translation
// stayed in it, and every reader after read the tabs in that language.
func TestLocalizingARecordPageLeavesTheProducersFilterAsItWas(t *testing.T) {
	page := &RecordPage{Sections: []RecordSection{{ID: "people", Components: []DisplayComponent{{
		ID:            "applications",
		ItemFilter:    &ItemFilter{Field: "group", AllLabel: "list.all", Options: []ItemFilterOption{{Value: "applied", Label: "tabs.applied"}}},
		ItemSelection: &ItemSelection{Field: "state", Value: "open", ActionID: "pay", IDsKey: "ids", CountLabel: "picked.count"},
	}}}}}
	// A text field carries its key as its value, the way a producer writes it.
	translate := func(language string) func(value, key string) string {
		return func(value, key string) string {
			if key == "" {
				key = value
			}
			if strings.Contains(key, ".") {
				return language + ":" + key
			}
			return value
		}
	}

	first := Localize(Universal{Record: page}, translate("ru"))
	second := Localize(Universal{Record: page}, translate("en"))

	require.Equal(t, "ru:tabs.applied", first.Record.Sections[0].Components[0].ItemFilter.Options[0].Label)
	require.Equal(t, "en:tabs.applied", second.Record.Sections[0].Components[0].ItemFilter.Options[0].Label)
	require.Equal(t, "en:picked.count", second.Record.Sections[0].Components[0].ItemSelection.CountLabel)
	// The producer's own page is left as it was.
	require.Equal(t, "tabs.applied", page.Sections[0].Components[0].ItemFilter.Options[0].Label)
	require.Equal(t, "list.all", page.Sections[0].Components[0].ItemFilter.AllLabel)
	require.Equal(t, "picked.count", page.Sections[0].Components[0].ItemSelection.CountLabel)
}
