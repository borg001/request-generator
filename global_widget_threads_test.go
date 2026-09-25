package module

import (
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
)

// threadedGlobalWidgetModules turns the valid workspace into one whose
// selected master row holds threads; the detail reads the open thread.
func threadedGlobalWidgetModules() []*BaseModule {
	modules := validGlobalWidgetModules()
	threadKey := pg.IntegerColumn("thread_key")
	threadGroup := pg.StringColumn("thread_group")
	ownerID := pg.IntegerColumn("owner_id")
	threads := &BaseModule{
		Name: "thread_records",
		Path: "/workspace",
		Fields: []fields.ModuleField{
			{Column: threadKey, Type: fields.ModuleFieldTypeInt},
			{Column: threadGroup, Type: fields.ModuleFieldTypeString},
			{Column: ownerID, Type: fields.ModuleFieldTypeInt},
		},
		Render:  renderer.Universal{List: &renderer.ListPage{}},
		Actions: []actions.ModuleAction{actions.ListModuleAction{Filter: []pg.Column{ownerID}}},
	}
	workspace := threadedWorkspace(modules)
	workspace.Threads = &renderer.WorkspaceThreads{
		Resource: renderer.Resource{ActionResource: renderer.ActionResource{Module: "thread_records", Action: "list"}, Bindings: []renderer.RequestBinding{{
			Target: renderer.RequestBindingFilter,
			Field:  "owner_id",
			Source: selectionSource("id"),
		}}},
		Field:      "thread_key",
		GroupField: "thread_group",
		Groups: []renderer.WorkspaceThreadGroup{
			{Value: "first", Label: "workspace.threads.first", Single: true},
			{Value: "second", Label: "workspace.threads.second", EmptyLabel: "workspace.threads.second_empty"},
		},
		Badges: []renderer.Badge{{
			ID: "thread_state", Field: "thread_group", LabelMap: map[string]string{"first": "workspace.threads.state_first"},
			ToneMap: map[string]string{"first": "glass-glow-amber"},
		}},
	}
	workspace.Detail.Bindings[0].Source = selectionSource("thread_key")
	return append(modules, threads)
}

func threadedWorkspace(modules []*BaseModule) *renderer.WorkspaceWidget {
	return modules[2].Actions[0].(actions.ListModuleAction).Widget.Renderer.Workspace
}

func TestValidateGlobalWidgetsAcceptsThreads(t *testing.T) {
	require.NoError(t, (&Generator{Modules: threadedGlobalWidgetModules()}).validateGlobalWidgets())
}

func TestValidateGlobalWidgetsRejectsBrokenThreads(t *testing.T) {
	tests := []struct {
		name string
		edit func([]*BaseModule)
		err  string
	}{
		{
			name: "detail reads the row instead of the open thread",
			edit: func(modules []*BaseModule) {
				threadedWorkspace(modules).Detail.Bindings[0].Source = selectionSource("id")
			},
			err: `detail must bind thread field "thread_key"`,
		},
		{
			name: "threads ignore the selected row",
			edit: func(modules []*BaseModule) {
				threadedWorkspace(modules).Threads.Resource.Bindings = nil
			},
			err: `threads: resource must bind selection field "id"`,
		},
		{
			name: "group declared twice",
			edit: func(modules []*BaseModule) {
				threads := threadedWorkspace(modules).Threads
				threads.Groups = append(threads.Groups, renderer.WorkspaceThreadGroup{Value: "first", Label: "workspace.threads.again"})
			},
			err: `threads: group "first" is duplicated`,
		},
		{
			name: "badge declared twice",
			edit: func(modules []*BaseModule) {
				threads := threadedWorkspace(modules).Threads
				threads.Badges = append(threads.Badges, threads.Badges[0])
			},
			err: `threads: badge "thread_state" is duplicated`,
		},
		{
			name: "no group field",
			edit: func(modules []*BaseModule) {
				threadedWorkspace(modules).Threads.GroupField = ""
			},
			err: `threads: group field is required`,
		},
		{
			name: "threads are not a list",
			edit: func(modules []*BaseModule) {
				threadedWorkspace(modules).Threads.Resource.ActionResource = renderer.ActionResource{Module: "workspace_entry", Action: "add"}
			},
			err: `widget "work-area" threads action must be list`,
		},
		{
			name: "thread key the threads never return",
			edit: func(modules []*BaseModule) {
				workspace := threadedWorkspace(modules)
				workspace.Threads.Field = "participant_id"
				workspace.Detail.Bindings[0].Source = selectionSource("participant_id")
			},
			err: `widget "work-area" thread field "participant_id" is not returned by threads action`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules := threadedGlobalWidgetModules()
			test.edit(modules)
			err := (&Generator{Modules: modules}).validateGlobalWidgets()
			require.Error(t, err)
			require.Contains(t, err.Error(), test.err)
		})
	}
}

// Bindings read the open thread's fields as they read the selected row's.
func TestWorkspaceSelectionScopeReadsTheOpenThread(t *testing.T) {
	modules := threadedGlobalWidgetModules()
	scope, err := (&Generator{Modules: modules}).workspaceSelectionScope("work-area", *threadedWorkspace(modules))
	require.NoError(t, err)
	require.Equal(t, "id", scope.Field)
	require.Equal(t, renderer.TypedValueNumber, scope.Fields["participant_id"])
	require.Equal(t, renderer.TypedValueNumber, scope.Fields["thread_key"])
	require.Equal(t, renderer.TypedValueString, scope.Fields["thread_group"])
}

func TestWidgetLoadDescribesThreads(t *testing.T) {
	modules := threadedGlobalWidgetModules()
	master := modules[0]
	masterAction := master.Actions[0].(actions.ListModuleAction)
	masterAction.Columns = []pg.Column{master.Fields[0].Column, master.Fields[1].Column}
	master.Actions[0] = masterAction
	threads := modules[len(modules)-1]
	threadsAction := threads.Actions[0].(actions.ListModuleAction)
	threadsAction.Columns = []pg.Column{threads.Fields[0].Column, threads.Fields[1].Column}
	threads.Actions[0] = threadsAction

	context, _ := ginTestContext()
	owner := modules[2]
	load, available, err := (&Generator{Modules: modules}).buildWidgetLoad(context, owner, owner.Actions[0], *actionWidget(owner.Actions[0]), "")
	require.NoError(t, err)
	require.True(t, available)
	require.NotNil(t, load.Threads)
	require.Equal(t, "/api/workspace/thread_records", load.Threads.Request.Endpoint)
	require.Len(t, load.Threads.Bindings, 1)
	require.Equal(t, "owner_id", load.Threads.Bindings[0].Field)
	require.NotNil(t, load.Detail)
	require.Equal(t, "thread_key", load.Detail.Bindings[0].Source.Runtime.Field)

	cloned := load.Clone()
	cloned.Threads.Bindings[0].Field = "changed"
	require.Equal(t, "owner_id", load.Threads.Bindings[0].Field, "a cloned load keeps its own thread bindings")
}

func TestLocalizeGlobalWidgetNamesThreadGroups(t *testing.T) {
	modules := threadedGlobalWidgetModules()
	widget := modules[2].Actions[0].(actions.ListModuleAction).Widget.Renderer
	localized := renderer.LocalizeGlobalWidget(widget, func(key, fallback string) string { return "T:" + key })
	require.Equal(t, "T:workspace.threads.first", localized.Workspace.Threads.Groups[0].Label)
	require.Empty(t, localized.Workspace.Threads.Groups[0].EmptyLabel)
	require.Equal(t, "T:workspace.threads.second_empty", localized.Workspace.Threads.Groups[1].EmptyLabel)
	require.Equal(t, "T:workspace.threads.state_first", localized.Workspace.Threads.Badges[0].LabelMap["first"])
	require.Equal(t, "workspace.threads.first", widget.Workspace.Threads.Groups[0].Label, "localizing leaves the declared widget untouched")
	require.Equal(t, "workspace.threads.state_first", widget.Workspace.Threads.Badges[0].LabelMap["first"], "localizing leaves the declared badges untouched")
}

// A command about the open thread stands beside the threads.
func TestThreadPlacementIsAnActionSurface(t *testing.T) {
	require.True(t, renderer.ActionPlacementThread.Valid())
	require.Equal(t, renderer.ActionPlacement("thread"), renderer.ActionPlacementThread)
}
