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
