package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalizeGlobalWidgetSurfaceLabels(t *testing.T) {
	widget := GlobalWidget{Surface: WidgetSurface{
		Kind:       WidgetSurfaceDrawer,
		Placement:  WidgetPlacementShellEnd,
		LoadPolicy: WidgetLoadOnOpen,
		CloseLabel: "widget.close",
		BackLabel:  "widget.back",
		MoreLabel:  "widget.more",
	}}

	localized := LocalizeGlobalWidget(widget, func(key, _ string) string { return "ru:" + key })

	require.Equal(t, "ru:widget.close", localized.Surface.CloseLabel)
	require.Equal(t, "ru:widget.back", localized.Surface.BackLabel)
	require.Equal(t, "ru:widget.more", localized.Surface.MoreLabel)
}

// The words that name a selection of rows are a producer key like any other,
// and reached the screen untranslated while nothing localized them.
func TestLocalizeGlobalWidgetSelectionMultiLabel(t *testing.T) {
	widget := GlobalWidget{
		Surface: WidgetSurface{Kind: WidgetSurfaceDrawer, Placement: WidgetPlacementShellEnd, LoadPolicy: WidgetLoadOnOpen},
		Workspace: &WorkspaceWidget{
			Selection: WorkspaceSelection{Field: "id", MultiLabel: "chat.multi.selected"},
		},
	}

	localized := LocalizeGlobalWidget(widget, func(key, _ string) string { return "ru:" + key })

	require.Equal(t, "ru:chat.multi.selected", localized.Workspace.Selection.MultiLabel)
}
