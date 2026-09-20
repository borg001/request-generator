package renderer

import "testing"

// Localizing a widget must not touch the configuration it was given. The
// composer badges used to share their maps with the module, so the first
// request after a restart replaced the keys with one language and every later
// request - in any language - was answered in that first one.
func TestLocalizeGlobalWidgetLeavesTheSourceUntouched(t *testing.T) {
	source := GlobalWidget{
		Surface: WidgetSurface{Kind: WidgetSurfaceDrawer, Placement: WidgetPlacementShellEnd, LoadPolicy: WidgetLoadOnOpen},
		Workspace: &WorkspaceWidget{
			ComposerBadges: []Badge{{
				ID:       "deal_state",
				Field:    "deal_status",
				LabelMap: map[string]string{"finished_cancelled": "deals.status_cancelled"},
				ToneMap:  map[string]string{"finished_cancelled": "glass-slate"},
			}},
		},
	}

	ru := LocalizeGlobalWidget(source, func(value, key string) string {
		if value == "deals.status_cancelled" {
			return "Отменён"
		}
		return value
	})
	if got := ru.Workspace.ComposerBadges[0].LabelMap["finished_cancelled"]; got != "Отменён" {
		t.Fatalf("russian label = %q", got)
	}
	if got := source.Workspace.ComposerBadges[0].LabelMap["finished_cancelled"]; got != "deals.status_cancelled" {
		t.Fatalf("source was rewritten to %q", got)
	}

	en := LocalizeGlobalWidget(source, func(value, key string) string {
		if value == "deals.status_cancelled" {
			return "Cancelled"
		}
		return value
	})
	if got := en.Workspace.ComposerBadges[0].LabelMap["finished_cancelled"]; got != "Cancelled" {
		t.Fatalf("english label = %q", got)
	}
	if got := ru.Workspace.ComposerBadges[0].LabelMap["finished_cancelled"]; got != "Отменён" {
		t.Fatalf("the russian copy changed to %q", got)
	}
}
