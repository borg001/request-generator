package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func infoHintPage() Universal {
	return Universal{Record: &RecordPage{Sections: []RecordSection{{
		ID:    "details",
		Title: "details.title",
		Info:  &InfoHint{ID: "details", Title: "hints.details.title", Text: "hints.details.text"},
		Components: []DisplayComponent{{
			ID:     "meetings",
			Type:   DisplayBadgeList,
			Fields: []string{"work_area"},
			Info:   &InfoHint{ID: "meetings", Text: "hints.meetings.text"},
		}},
	}}}}
}

// An explanation stands beside a section's title and a component's title,
// and is written out under "info".
func TestInfoHintIsWrittenBesideSectionsAndComponents(t *testing.T) {
	render := infoHintPage()
	require.NoError(t, render.Validate())
	encoded, err := json.Marshal(render.Record.Sections[0])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"info":{"id":"details","title":"hints.details.title","text":"hints.details.text"}`)
	require.Contains(t, string(encoded), `"info":{"id":"meetings","text":"hints.meetings.text"}`)
}

// An explanation with nothing to say is refused wherever it stands.
func TestInfoHintWithoutTextIsRefused(t *testing.T) {
	render := infoHintPage()
	render.Record.Sections[0].Info.Text = " "
	require.ErrorContains(t, render.Validate(), "text is required")

	render = infoHintPage()
	render.Record.Sections[0].Components[0].Info.Text = ""
	require.ErrorContains(t, render.Validate(), "text is required")

	presentation := &FieldPresentation{Info: &InfoHint{ID: "rate"}}
	require.ErrorContains(t, presentation.Validate(), "text is required")
}

// A reader's language is written into a copy of the page: the producer's page
// keeps its keys for the next reader.
func TestInfoHintIsLocalizedWithoutTouchingTheSource(t *testing.T) {
	source := infoHintPage()
	source.Record.Sections[0].Info.Action = &Action{ID: "faq", Type: ActionRoute, Label: "hints.more", Route: RouteAction{Path: "/support"}}
	localized := Localize(source, func(value, key string) string { return "ru:" + value })

	require.Equal(t, "ru:hints.details.title", localized.Record.Sections[0].Info.Title)
	require.Equal(t, "ru:hints.details.text", localized.Record.Sections[0].Info.Text)
	require.Equal(t, "ru:hints.more", localized.Record.Sections[0].Info.Action.Label)
	require.Equal(t, "ru:hints.meetings.text", localized.Record.Sections[0].Components[0].Info.Text)

	require.Equal(t, "hints.details.text", source.Record.Sections[0].Info.Text)
	require.Equal(t, "hints.more", source.Record.Sections[0].Info.Action.Label)
	require.Equal(t, "hints.meetings.text", source.Record.Sections[0].Components[0].Info.Text)
}

// A field's explanation is copied with its presentation and translated there.
func TestFieldPresentationCarriesItsOwnInfoHint(t *testing.T) {
	source := &FieldPresentation{Info: &InfoHint{ID: "rate", Title: "hints.rate.title", Text: "hints.rate.text"}}
	require.NoError(t, source.Validate())
	copied := CloneFieldPresentation(source)
	LocalizeInfoHint(copied.Info, func(value, key string) string { return "en:" + value })
	require.Equal(t, "en:hints.rate.text", copied.Info.Text)
	require.Equal(t, "hints.rate.text", source.Info.Text)
}

// A form section explains itself beside its title too: the explanation is
// written out, refused without words, and translated into a copy only.
func TestFormSectionCarriesItsOwnInfoHint(t *testing.T) {
	page := func() Universal {
		return Universal{Form: &FormPage{Sections: []FormSection{{
			ID:     "quiet_hours",
			Title:  "quiet_hours.title",
			Fields: []string{"quiet_hours_enabled"},
			Info:   &InfoHint{ID: "quiet_hours", Title: "hints.quiet_hours.title", Text: "hints.quiet_hours.text"},
		}}}}
	}
	source := page()
	require.NoError(t, source.Validate())
	encoded, err := json.Marshal(source.Form.Sections[0])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"info":{"id":"quiet_hours","title":"hints.quiet_hours.title","text":"hints.quiet_hours.text"}`)

	localized := Localize(source, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.quiet_hours.text", localized.Form.Sections[0].Info.Text)
	require.Equal(t, "hints.quiet_hours.text", source.Form.Sections[0].Info.Text)

	empty := page()
	empty.Form.Sections[0].Info.Text = ""
	require.ErrorContains(t, empty.Validate(), "text is required")
}

// A list page explains itself beside its title: the explanation is written
// out, refused without words, and translated into a copy only (#375).
func TestListPageCarriesItsOwnInfoHint(t *testing.T) {
	page := func() Universal {
		return Universal{List: &ListPage{
			ID:    "deals",
			Title: "deals.menu.list",
			Info:  &InfoHint{ID: "deals_tours_model", Title: "hints.deals_tours_model.title", Text: "hints.deals_tours_model.text"},
		}}
	}
	source := page()
	require.NoError(t, source.Validate())
	encoded, err := json.Marshal(source.List)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"info":{"id":"deals_tours_model","title":"hints.deals_tours_model.title","text":"hints.deals_tours_model.text"}`)

	localized := Localize(source, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.deals_tours_model.text", localized.List.Info.Text)
	require.Equal(t, "hints.deals_tours_model.text", source.List.Info.Text)

	empty := page()
	empty.List.Info.Text = ""
	require.ErrorContains(t, empty.Validate(), "text is required")
}

// A gallery item shown to its owner the way others see it carries the
// original beside it, and the gallery names both views (#375).
func TestMediaGalleryItemCarriesItsOriginal(t *testing.T) {
	item := MediaGalleryItem{ID: "7", Src: "storage://link/7/blur_faces", Thumbnail: "storage://link/7/blur_faces", OriginalSrc: "storage://link/7", OriginalThumbnail: "storage://link/7"}
	encoded, err := json.Marshal(item)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"original_src":"storage://link/7"`)
	require.Contains(t, string(encoded), `"original_thumbnail":"storage://link/7"`)
	labels, err := json.Marshal(MediaGalleryLabels{ViewMine: "As I see it", ViewOthers: "As others see it"})
	require.NoError(t, err)
	require.JSONEq(t, `{"view_mine":"As I see it","view_others":"As others see it"}`, string(labels))
}

// An overlay of badges may explain them beside them; the explanation is
// checked, copied and translated with the page.
func TestBlockOverlayCarriesItsOwnInfoHint(t *testing.T) {
	page := func() Universal {
		return Universal{Record: &RecordPage{Sections: []RecordSection{{
			ID: "gallery",
			Block: &Block{Overlays: []BlockOverlay{{
				Position: MediaOverlayTopRight,
				Badges:   []Badge{{ID: "affiliation", Field: "model_affiliation"}},
				Info:     &InfoHint{ID: "model_types", Title: "hints.model_types.title", Text: "hints.model_types.text"},
			}}},
		}}}}
	}
	source := page()
	require.NoError(t, source.Validate())
	encoded, err := json.Marshal(source.Record.Sections[0].Block.Overlays[0])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"info":{"id":"model_types","title":"hints.model_types.title","text":"hints.model_types.text"}`)

	localized := Localize(source, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.model_types.text", localized.Record.Sections[0].Block.Overlays[0].Info.Text)
	require.Equal(t, "hints.model_types.text", source.Record.Sections[0].Block.Overlays[0].Info.Text)

	cloned := source.Clone()
	cloned.Record.Sections[0].Block.Overlays[0].Info.Text = "changed"
	require.Equal(t, "hints.model_types.text", source.Record.Sections[0].Block.Overlays[0].Info.Text)

	empty := page()
	empty.Record.Sections[0].Block.Overlays[0].Info.Text = " "
	require.ErrorContains(t, empty.Validate(), "text is required")
}

// An action may explain itself beside it - a workspace command that waits, a
// button of a page; the explanation is checked, copied and translated with it.
func TestActionPresentationCarriesItsOwnInfoHint(t *testing.T) {
	hint := func() *InfoHint {
		return &InfoHint{ID: "chat_delete_wait", Title: "hints.chat_delete_wait.title", Text: "hints.chat_delete_wait.text"}
	}
	action := Action{ID: "delete", Type: ActionAPI, ActionPresentation: ActionPresentation{Info: hint()}}
	require.NoError(t, action.ActionPresentation.Validate())
	encoded, err := json.Marshal(action)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"info":{"id":"chat_delete_wait","title":"hints.chat_delete_wait.title","text":"hints.chat_delete_wait.text"}`)

	page := Universal{List: &ListPage{Actions: []Action{action}}}
	localized := Localize(page, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.chat_delete_wait.text", localized.List.Actions[0].Info.Text)
	require.Equal(t, "hints.chat_delete_wait.text", page.List.Actions[0].Info.Text)
	cloned := page.Clone()
	cloned.List.Actions[0].Info.Text = "changed"
	require.Equal(t, "hints.chat_delete_wait.text", page.List.Actions[0].Info.Text)

	empty := ActionPresentation{Info: &InfoHint{ID: "chat_delete_wait", Text: " "}}
	require.ErrorContains(t, empty.Validate(), "text is required")

	widget := GlobalWidget{Workspace: &WorkspaceWidget{Commands: []WorkspaceCommand{{ID: "delete_wait", Label: "chat.delete", Presentation: &ActionPresentation{Info: hint()}}}}}
	localizedWidget := LocalizeGlobalWidget(widget, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.chat_delete_wait.title", localizedWidget.Workspace.Commands[0].Presentation.Info.Title)
	require.Equal(t, "hints.chat_delete_wait.title", widget.Workspace.Commands[0].Presentation.Info.Title)
}
