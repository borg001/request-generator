package renderer

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func ownershipFixture() Universal {
	return Universal{
		List: &ListPage{ID: "items", Title: "items.title", CardSchema: &CardSchema{
			Badges:  []Badge{{ID: "state", LabelMap: map[string]string{"ready": "state.ready"}}},
			Actions: []Action{{ID: "open", Type: ActionRoute, LabelKey: "open", Route: RouteAction{Path: "/items", Query: map[string]interface{}{"nested": map[string]interface{}{"id": "record.id"}}}}},
		}},
		Form:   &FormPage{ID: "edit", Title: "edit.title", Sections: []FormSection{{ID: "details", Title: "details.title"}}},
		Record: &RecordPage{ID: "item", Title: "item.title"},
	}
}

func TestLocalizeOwnedMatchesCopyAndIsolatesRequests(t *testing.T) {
	base := ownershipFixture()
	before, err := json.Marshal(base)
	require.NoError(t, err)
	for _, language := range []string{"en", "ru", "en", "ru"} {
		language := language
		t.Run(language, func(t *testing.T) {
			t.Parallel()
			resolve := func(value, key string) string {
				if key != "" {
					value = key
				}
				if value == "" {
					return ""
				}
				return language + ":" + value
			}
			owned := base.Clone()
			localized := LocalizeOwned(owned, resolve)
			require.Equal(t, Localize(base, resolve), localized)
			require.Same(t, owned.List, localized.List, "owned localization must not create another page tree")
			localized.List.CardSchema.Actions[0].Route.(RouteAction).Query["nested"].(map[string]interface{})["id"] = language
			after, err := json.Marshal(base)
			require.NoError(t, err)
			require.Equal(t, string(before), string(after))
		})
	}
}

func TestLocalizeNilResolverKeepsPublicCopyContract(t *testing.T) {
	base := ownershipFixture()
	copy := Localize(base, nil)
	copy.Form.Title = "changed"
	require.Equal(t, "edit.title", base.Form.Title)
	require.Same(t, base.List, LocalizeOwned(base, nil).List)
}

func TestLocalizeOwnedVisitsAliasedTextOnce(t *testing.T) {
	confirm := &Confirm{Title: "confirm.title", ConfirmLabel: "confirm.yes"}
	labels := map[string]string{"ready": "state.ready"}
	action := Action{ID: "save", Type: ActionAPI, LabelKey: "save", Confirm: confirm, AfterSuccess: &ActionResult{Toast: "saved"}}
	base := Universal{
		List:   &ListPage{ID: "items", CardSchema: &CardSchema{Actions: []Action{action}, Badges: []Badge{{ID: "first", LabelMap: labels}, {ID: "second", LabelMap: labels}}}},
		Record: &RecordPage{Actions: []Action{action}},
	}
	resolve := func(value, key string) string {
		if key != "" {
			value = key
		}
		return "translated:" + value // intentionally not idempotent
	}
	expected := Localize(base, resolve)
	actual := LocalizeOwned(base, resolve)
	require.Equal(t, expected, actual)
}

func BenchmarkRendererOwnership(b *testing.B) {
	base := ownershipFixture()
	for i := 0; i < 80; i++ {
		base.Form.Sections = append(base.Form.Sections, FormSection{ID: fmt.Sprint(i), Title: "section.title", Fields: []string{"name", "description", "status"}})
	}
	resolve := func(value, key string) string {
		if key != "" {
			return key
		}
		return value
	}
	for _, owned := range []bool{false, true} {
		b.Run(fmt.Sprintf("owned=%v", owned), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				render := base.Clone()
				if owned {
					LocalizeOwned(render, resolve)
				} else {
					Localize(render, resolve)
				}
			}
		})
	}
}

func TestDiscoveryMatchesPageIdentities(t *testing.T) {
	for mask := 0; mask < 12; mask++ {
		var render Universal
		switch mask % 3 {
		case 1:
			render.List = &ListPage{ID: "items"}
		case 2:
			render.ResourceGrid = &ResourceGridPage{Endpoint: "/items"}
		}
		if mask/3&1 != 0 {
			render.Form = &FormPage{}
		}
		if mask/3&2 != 0 {
			render.Record = &RecordPage{}
		}
		d := render.Discovery()
		require.NoError(t, d.Validate())
		require.Equal(t, render.ListIdentity(), d.ListIdentity())
		require.Equal(t, render.FormIdentity(), d.FormIdentity())
		require.Equal(t, render.RecordIdentity(), d.RecordIdentity())
		require.Equal(t, render.ListRoutePageType(), d.ListRoutePageType())
		require.Equal(t, render.FormRoutePageType(), d.FormRoutePageType())
		require.Equal(t, render.RecordRoutePageType(), d.RecordRoutePageType())
	}
	require.Error(t, (Discovery{List: PageTypeForm}).Validate())
}
