package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisplayComponentJSONAndLocalization(t *testing.T) {
	source := displayComponentsUniversal()

	encoded, err := json.Marshal(source.Record.Sections[0].Components)
	require.NoError(t, err)
	require.JSONEq(t, `[
		{
			"id":"rates",
			"type":"data_list",
			"display_type":"tile_grid",
			"items":[
				{"field":"incall","label":"display.incall","label_fallback":"Incall"},
				{"field":"outcall","label":"display.outcall","label_fallback":"Outcall"}
			]
		},
		{
			"id":"offers",
			"type":"accordion_groups",
			"collection_groups":{
				"source_field":"offers",
				"groups":[{
					"id":"available",
					"label":"group.available",
					"label_fallback":"Available",
					"tone":"rect-cyan",
					"item_condition":{"path":"status","equals":"available"}
				}]
			}
		}
	]`, string(encoded))

	encodedBlock, err := json.Marshal(source.Record.Sections[0].Block)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"overlays":[{
			"id":"availability",
			"position":"top-left",
			"badges":[{"id":"status","field":"status","tone":"glass-success"}],
			"size":"sm"
		}]
	}`, string(encodedBlock))

	localized := Localize(source, func(value, key string) string {
		translations := map[string]string{
			"display.incall":  "Incall price",
			"group.available": "Available offers",
		}
		if text, ok := translations[value]; ok {
			return text
		}
		return value
	})

	require.Equal(t, "Incall price", localized.Record.Sections[0].Components[0].Items[0].Label)
	require.Equal(t, "Outcall", localized.Record.Sections[0].Components[0].Items[1].Label)
	require.Empty(t, localized.Record.Sections[0].Components[0].Items[0].LabelFallback)
	require.Equal(t, "Available offers", localized.Record.Sections[0].Components[1].CollectionGroups.Groups[0].Label)
	require.Empty(t, localized.Record.Sections[0].Components[1].CollectionGroups.Groups[0].LabelFallback)
	require.Equal(t, "display.incall", source.Record.Sections[0].Components[0].Items[0].Label)
	require.Equal(t, "group.available", source.Record.Sections[0].Components[1].CollectionGroups.Groups[0].Label)
}

func TestDisplayComponentValidation(t *testing.T) {
	tests := []struct {
		name      string
		component DisplayComponent
		valid     bool
	}{
		{
			name:      "valid typed tile grid",
			component: DisplayComponent{Type: DisplayDataList, DisplayType: ComponentDisplayTileGrid, Items: []DisplayFieldRef{{Field: "incall"}}},
			valid:     true,
		},
		{
			name: "valid accordion groups",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{
				SourceField: "offers",
				Groups:      []DisplayCollectionGroup{{ID: "available", ItemCondition: &Condition{Path: "status", Equals: "available"}}},
			}},
			valid: true,
		},
		{
			name:      "unsupported display type",
			component: DisplayComponent{Type: DisplayDataList, DisplayType: ComponentDisplayType("cards")},
		},
		{
			name:      "items on another component",
			component: DisplayComponent{Type: DisplayMediaGallery, Items: []DisplayFieldRef{{Field: "photos"}}},
		},
		{
			name:      "empty item field",
			component: DisplayComponent{Type: DisplayDataList, Items: []DisplayFieldRef{{}}},
		},
		{
			name:      "duplicate item field",
			component: DisplayComponent{Type: DisplayDataList, Items: []DisplayFieldRef{{Field: "incall"}, {Field: "incall"}}},
		},
		{
			name:      "accordion misses groups",
			component: DisplayComponent{Type: DisplayAccordionGroups},
		},
		{
			name:      "empty group source",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{Groups: []DisplayCollectionGroup{{ID: "available", ItemCondition: &Condition{Path: "status"}}}}},
		},
		{
			name: "duplicate group identifier",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{
				SourceField: "offers",
				Groups: []DisplayCollectionGroup{
					{ID: "available", ItemCondition: &Condition{Path: "status"}},
					{ID: "available", ItemCondition: &Condition{Path: "status"}},
				},
			}},
		},
		{
			name:      "empty group condition",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{SourceField: "offers", Groups: []DisplayCollectionGroup{{ID: "available", ItemCondition: &Condition{}}}}},
		},
		{
			name:      "group condition with path but no predicate",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{SourceField: "offers", Groups: []DisplayCollectionGroup{{ID: "available", ItemCondition: &Condition{Path: "status"}}}}},
		},
		{
			name:      "group condition with unsupported not value",
			component: DisplayComponent{Type: DisplayAccordionGroups, CollectionGroups: &DisplayCollectionGroups{SourceField: "offers", Groups: []DisplayCollectionGroup{{ID: "available", ItemCondition: &Condition{Not: "invalid"}}}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Universal{Record: &RecordPage{Sections: []RecordSection{{ID: "details", Components: []DisplayComponent{test.component}}}}}).Validate()
			assert.Equal(t, test.valid, err == nil, err)
		})
	}
}

func TestDisplayComponentActionReferenceValidation(t *testing.T) {
	valid := Universal{Record: &RecordPage{
		Actions: []Action{{ID: "open_story", Type: ActionEmit}},
		Sections: []RecordSection{{ID: "identity", Components: []DisplayComponent{{
			ID: "identity", Type: DisplayIdentity, ActionID: "open_story",
		}}}},
	}}
	require.NoError(t, valid.Validate())

	invalid := valid.Clone()
	invalid.Record.Sections[0].Components[0].ActionID = "unknown"
	require.EqualError(t, invalid.Validate(), `renderer.Universal: record section "identity" component "identity" action_id "unknown" is not declared in record page actions`)
}

func TestBlockOverlayValidation(t *testing.T) {
	tests := []struct {
		name  string
		block *Block
		valid bool
	}{
		{name: "valid", block: &Block{Overlays: []BlockOverlay{{Position: MediaOverlayTopLeft, Badges: []Badge{{Field: "status"}}}}}, valid: true},
		{name: "without badges", block: &Block{Overlays: []BlockOverlay{{Position: MediaOverlayTopLeft}}}},
		{name: "unsupported position", block: &Block{Overlays: []BlockOverlay{{Position: MediaOverlayPosition("corner"), Badges: []Badge{{Field: "status"}}}}}},
		{name: "duplicate position", block: &Block{Overlays: []BlockOverlay{{Position: MediaOverlayTopLeft, Badges: []Badge{{Field: "status"}}}, {Position: MediaOverlayTopLeft, Badges: []Badge{{Field: "type"}}}}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Universal{Record: &RecordPage{Sections: []RecordSection{{ID: "details", Block: test.block}}}}).Validate()
			assert.Equal(t, test.valid, err == nil, err)
		})
	}
}

func TestDisplayComponentClone(t *testing.T) {
	original := displayComponentsUniversal()
	cloned := original.Clone()

	cloned.Record.Sections[0].Components[0].Items[0].Label = "changed"
	cloned.Record.Sections[0].Components[1].CollectionGroups.Groups[0].ItemCondition.Path = "changed"
	cloned.Record.Sections[0].Components[1].CollectionGroups.Groups[0].Tone = "changed"
	cloned.Record.Sections[0].Block.Overlays[0].Badges[0].Tone = "changed"

	assert.Equal(t, "display.incall", original.Record.Sections[0].Components[0].Items[0].Label)
	assert.Equal(t, "status", original.Record.Sections[0].Components[1].CollectionGroups.Groups[0].ItemCondition.Path)
	assert.Equal(t, "rect-cyan", original.Record.Sections[0].Components[1].CollectionGroups.Groups[0].Tone)
	assert.Equal(t, "glass-success", original.Record.Sections[0].Block.Overlays[0].Badges[0].Tone)
}

func displayComponentsUniversal() Universal {
	return Universal{Record: &RecordPage{Sections: []RecordSection{{
		ID: "details",
		Block: &Block{Overlays: []BlockOverlay{{
			ID:       "availability",
			Position: MediaOverlayTopLeft,
			Badges:   []Badge{{ID: "status", Field: "status", Tone: "glass-success"}},
			Size:     SizeSM,
		}}},
		Components: []DisplayComponent{
			{
				ID:          "rates",
				Type:        DisplayDataList,
				DisplayType: ComponentDisplayTileGrid,
				Items: []DisplayFieldRef{
					{Field: "incall", Label: "display.incall", LabelFallback: "Incall"},
					{Field: "outcall", Label: "display.outcall", LabelFallback: "Outcall"},
				},
			},
			{
				ID:   "offers",
				Type: DisplayAccordionGroups,
				CollectionGroups: &DisplayCollectionGroups{
					SourceField: "offers",
					Groups: []DisplayCollectionGroup{{
						ID:            "available",
						Label:         "group.available",
						LabelFallback: "Available",
						Tone:          "rect-cyan",
						ItemCondition: &Condition{Path: "status", Equals: "available"},
					}},
				},
			},
		},
	}}}}
}

// A preview borrows the actions the page already declares, so a name that is
// not there has to fail where every other action reference does.
func TestDisplayComponentPreviewValidation(t *testing.T) {
	valid := Universal{Record: &RecordPage{
		Actions: []Action{{ID: "favorite", Type: ActionAPI}, {ID: "share_profile", Type: ActionEmit}},
		Sections: []RecordSection{{ID: "hero", Components: []DisplayComponent{{
			ID:      "identity",
			Type:    DisplayIdentity,
			Preview: &DisplayPreview{Label: "Profile photo", CloseLabel: "Close", Actions: []string{"favorite", "share_profile"}},
		}}}},
	}}
	require.NoError(t, valid.Validate())

	cloned := valid.Clone()
	cloned.Record.Sections[0].Components[0].Preview.Actions[0] = "unknown"
	require.EqualError(t, cloned.Validate(), `renderer.Universal: record section "hero" component "identity" preview action "unknown" is not declared in record page actions`)
	require.Equal(t, "favorite", valid.Record.Sections[0].Components[0].Preview.Actions[0])

	duplicated := valid.Clone()
	duplicated.Record.Sections[0].Components[0].Preview.Actions = []string{"favorite", "favorite"}
	require.Error(t, duplicated.Validate())

	wrongType := valid.Clone()
	wrongType.Record.Sections[0].Components[0].Type = DisplayDataList
	require.Error(t, wrongType.Validate())
}
