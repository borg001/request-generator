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

// A command applied to several rows speaks for them: its own short name on
// the bar and its own question, both producer keys like the single ones.
func TestLocalizeWorkspaceCommandMultiTexts(t *testing.T) {
	widget := GlobalWidget{
		Surface: WidgetSurface{Kind: WidgetSurfaceDrawer, Placement: WidgetPlacementShellEnd, LoadPolicy: WidgetLoadOnOpen},
		Workspace: &WorkspaceWidget{
			Commands: []WorkspaceCommand{{
				ID: "delete", Label: "chat.delete", Multi: true, MultiLabel: "chat.multi.delete",
				MultiConfirm: &Confirm{Title: "chat.confirm.many_title", Message: "chat.confirm.many_message", CancelLabel: "ui.cancel", ConfirmLabel: "chat.confirm.yes"},
			}},
		},
	}

	localized := LocalizeGlobalWidget(widget, func(key, _ string) string { return "ru:" + key })

	command := localized.Workspace.Commands[0]
	require.Equal(t, "ru:chat.multi.delete", command.MultiLabel)
	require.Equal(t, "ru:chat.confirm.many_title", command.MultiConfirm.Title)
	require.Equal(t, "ru:chat.confirm.many_message", command.MultiConfirm.Message)
	require.Equal(t, "chat.multi.delete", widget.Workspace.Commands[0].MultiLabel, "the declared widget is left as it was")
	require.Equal(t, "chat.confirm.many_title", widget.Workspace.Commands[0].MultiConfirm.Title, "the declared widget is left as it was")
}
