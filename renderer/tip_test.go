package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func tipPage() Universal {
	return Universal{Record: &RecordPage{
		Sections: []RecordSection{{ID: "hero"}},
		Tips: []Tip{{
			ID: "status", Device: TipDeviceDesktop, Anchor: "hero", Title: "hints.status.title", Text: "hints.status.text", Version: 2,
			Dismiss: &Action{ID: "dismiss", Type: ActionAPI, Label: "hints.got_it", API: &APIAction{Method: "POST", Endpoint: "/api/hint-states"}},
		}, {
			ID: "status", Device: TipDeviceMobile, Text: "hints.status.mobile_text",
			Dismiss: &Action{ID: "dismiss", Type: ActionAPI, Label: "hints.got_it", API: &APIAction{Method: "POST", Endpoint: "/api/hint-states"}},
		}},
	}}
}

// A tip is written for a screen, with the answer that closes it.
func TestTipsAreWrittenForAScreen(t *testing.T) {
	render := tipPage()
	require.NoError(t, render.Validate())
	encoded, err := json.Marshal(render.Record.Tips[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"status","device":"desktop","anchor":"hero","title":"hints.status.title","text":"hints.status.text","version":2,"dismiss":{"id":"dismiss","type":"api","label":"hints.got_it","api":{"method":"POST","endpoint":"/api/hint-states"}}}`, string(encoded))
}

func TestTipsWithoutScreenTextOrAnswerAreRefused(t *testing.T) {
	cases := map[string]func(*Tip){
		"device must be desktop or mobile": func(tip *Tip) { tip.Device = "tablet" },
		"text is required":                 func(tip *Tip) { tip.Text = " " },
		"dismiss is required":              func(tip *Tip) { tip.Dismiss = nil },
		"id is required":                   func(tip *Tip) { tip.ID = "" },
	}
	for want, spoil := range cases {
		render := tipPage()
		spoil(&render.Record.Tips[0])
		require.ErrorContains(t, render.Validate(), want)
	}
	render := tipPage()
	render.Record.Tips[1].Device = TipDeviceDesktop
	require.ErrorContains(t, render.Validate(), "told twice on desktop")
}

// A reader's language is written into a copy: the producer's page keeps its
// keys, on a list, a record and a form alike.
func TestTipsAreLocalizedWithoutTouchingTheSource(t *testing.T) {
	source := tipPage()
	source.List = &ListPage{Tips: cloneTips(source.Record.Tips)}
	source.Form = &FormPage{Tips: cloneTips(source.Record.Tips)}
	localized := Localize(source, func(value, key string) string { return "ru:" + value })
	for _, tips := range [][]Tip{localized.Record.Tips, localized.List.Tips, localized.Form.Tips} {
		require.Equal(t, "ru:hints.status.text", tips[0].Text)
		require.Equal(t, "ru:hints.got_it", tips[0].Dismiss.Label)
	}
	require.Equal(t, "hints.status.text", source.Record.Tips[0].Text)
	require.Equal(t, "hints.got_it", source.Record.Tips[0].Dismiss.Label)
	require.Equal(t, "hints.got_it", source.List.Tips[0].Dismiss.Label)
}

// A grid of cards carries tips as a list does: checked, copied and put into
// the reader's language (theGHub1/api#359).
func TestResourceGridTipsAreCheckedAndLocalized(t *testing.T) {
	source := Universal{ResourceGrid: &ResourceGridPage{Endpoint: "/api/agency_models", Tips: cloneTips(tipPage().Record.Tips)}}
	require.NoError(t, source.validateTips())
	localized := Localize(source, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.status.text", localized.ResourceGrid.Tips[0].Text)
	require.Equal(t, "hints.status.text", source.ResourceGrid.Tips[0].Text)
	source.ResourceGrid.Tips[1].Device = TipDeviceDesktop
	require.ErrorContains(t, source.validateTips(), "told twice on desktop")
}

// An introduction is a tip with steps: each step says something, and the
// reader is told how to go on and how to finish.
func TestAnIntroductionIsATipWithSteps(t *testing.T) {
	render := tipPage()
	render.Record.Tips[0].Steps = []TipStep{{Anchor: "/deals", Title: "hints.intro_1.title", Text: "hints.intro_1.text"}}
	render.Record.Tips[0].NextLabel, render.Record.Tips[0].SkipLabel, render.Record.Tips[0].DoneLabel = "hints.next", "hints.skip", "hints.done"
	require.NoError(t, render.Validate())

	localized := Localize(render, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.intro_1.text", localized.Record.Tips[0].Steps[0].Text)
	require.Equal(t, "ru:hints.next", localized.Record.Tips[0].NextLabel)
	require.Equal(t, "hints.intro_1.text", render.Record.Tips[0].Steps[0].Text)

	render.Record.Tips[0].Steps[0].Text = ""
	require.ErrorContains(t, render.Validate(), "step 1: text is required")
	render.Record.Tips[0].Steps[0].Text = "hints.intro_1.text"
	render.Record.Tips[0].DoneLabel = ""
	require.ErrorContains(t, render.Validate(), "names its next and done buttons")
}

// A welcome carries the application's mark on its opening card, and says so
// under "brand"; a tip without it says nothing.
func TestTipBrandIsWrittenOnlyWhenAsked(t *testing.T) {
	render := tipPage()
	render.Record.Tips[0].Brand = true
	require.NoError(t, render.Validate())
	encoded, err := json.Marshal(render.Record.Tips[0])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"brand":true`)
	plain, err := json.Marshal(render.Record.Tips[1])
	require.NoError(t, err)
	require.NotContains(t, string(plain), `"brand"`)
}

// A story is told in steps, each with its scene; an unknown way of telling
// and a story without steps are refused.
func TestTipStoryIsToldInStepsWithScenes(t *testing.T) {
	render := tipPage()
	story := &render.Record.Tips[0]
	story.Presentation = TipPresentationStory
	require.ErrorContains(t, render.Validate(), "a story is told in steps")
	story.Steps = []TipStep{{Title: "hints.story_1.title", Text: "hints.story_1.text", Scene: "order_fill"}}
	story.NextLabel, story.DoneLabel, story.BackLabel = "hints.next", "hints.done", "hints.back"
	require.NoError(t, render.Validate())
	encoded, err := json.Marshal(*story)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"presentation":"story"`)
	require.Contains(t, string(encoded), `"scene":"order_fill"`)
	require.Contains(t, string(encoded), `"back_label":"hints.back"`)
	// The story's back button is spoken in the reader's language too.
	localized := Localize(render, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.back", localized.Record.Tips[0].BackLabel)
	story.Presentation = "carousel"
	require.ErrorContains(t, render.Validate(), `presentation "carousel" is unknown`)
}

// A tip can bring a sample to its page - a picture in a gallery - whose badge
// and menu are spoken in the reader's language, and only known samples pass.
func TestTipDemoIsASampleOfAKnownKind(t *testing.T) {
	render := tipPage()
	tip := &render.Record.Tips[0]
	tip.Demo = &TipDemo{Kind: TipDemoMediaItem, Label: "hints.demo", Menu: []string{"settings.media.publish_post"}}
	require.NoError(t, render.Validate())
	encoded, err := json.Marshal(*tip)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"demo":{"kind":"media_item","label":"hints.demo","menu":["settings.media.publish_post"]}`)
	localized := Localize(render, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.demo", localized.Record.Tips[0].Demo.Label)
	require.Equal(t, "ru:settings.media.publish_post", localized.Record.Tips[0].Demo.Menu[0])
	require.Equal(t, "hints.demo", render.Record.Tips[0].Demo.Label, "the source is not touched")
	tip.Demo.Kind = "hologram"
	require.ErrorContains(t, render.Validate(), `demo kind "hologram" is unknown`)
}

// A story casts its reader as one of its people; anyone else is refused.
func TestTipCastIsOneOfTheStoryPeople(t *testing.T) {
	render := tipPage()
	render.Record.Tips[0].Cast = "agency"
	require.NoError(t, render.Validate())
	render.Record.Tips[0].Cast = "admin"
	require.ErrorContains(t, render.Validate(), `cast "admin" is unknown`)
}

// A list of contacts is walked through on sample people (theGHub1/api#360):
// each is someone of a known kind, following the reader or followed back,
// and is put into the reader's language on a copy.
func TestTipDemoPeopleArePeopleOfAKnownKind(t *testing.T) {
	render := tipPage()
	render.Record.Tips[0].Demo = &TipDemo{Kind: TipDemoPeople, Label: "hints.demo", People: []TipDemoPerson{
		{Name: "hints.demo_model", Role: "model", Relation: "follower"},
		{Name: "hints.demo_client", Role: "client", Relation: "mutual"},
	}}
	require.NoError(t, render.Validate())
	localized := Localize(render, func(value, key string) string { return "ru:" + value })
	require.Equal(t, "ru:hints.demo_model", localized.Record.Tips[0].Demo.People[0].Name)
	require.Equal(t, "hints.demo_model", render.Record.Tips[0].Demo.People[0].Name)
	render.Record.Tips[0].Demo.People[1].Role = "admin"
	require.ErrorContains(t, render.Validate(), `demo person role "admin" is unknown`)
	render.Record.Tips[0].Demo.People = nil
	require.ErrorContains(t, render.Validate(), "people demo has no people")
}
