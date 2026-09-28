package module

import (
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
)

// variantGlobalWidgetModules gives the valid workspace a list of its own for
// the pill kind=work: its rows are the works, and a row unfolds into the
// threads it holds.
func variantGlobalWidgetModules() []*BaseModule {
	modules := validGlobalWidgetModules()
	works := &BaseModule{
		Name: "work_records",
		Path: "/workspace",
		Fields: []fields.ModuleField{
			{Column: pg.IntegerColumn("id"), Type: fields.ModuleFieldTypeInt},
			{Column: pg.StringColumn("threads"), Type: fields.ModuleFieldTypeString},
		},
		Render:  renderer.Universal{List: &renderer.ListPage{}},
		Actions: []actions.ModuleAction{actions.ListModuleAction{}},
	}
	workspace := threadedWorkspace(modules)
	workspace.MasterVariants = []renderer.WorkspaceMasterVariant{{
		Key: "kind", Val: "work",
		Master: renderer.Resource{ActionResource: renderer.ActionResource{Module: "work_records", Action: "list"}},
		Unfold: "threads",
	}}
	return append(modules, works)
}

func TestValidateGlobalWidgetsAcceptsMasterVariants(t *testing.T) {
	require.NoError(t, (&Generator{Modules: variantGlobalWidgetModules()}).validateGlobalWidgets())
}

func TestValidateGlobalWidgetsRejectsBrokenMasterVariants(t *testing.T) {
	tests := []struct {
		name string
		edit func([]*BaseModule)
		err  string
	}{
		{
			name: "no pill",
			edit: func(modules []*BaseModule) { threadedWorkspace(modules).MasterVariants[0].Key = "" },
			err:  "master variant 0: key is required",
		},
		{
			name: "the same pill twice",
			edit: func(modules []*BaseModule) {
				workspace := threadedWorkspace(modules)
				workspace.MasterVariants = append(workspace.MasterVariants, workspace.MasterVariants[0])
			},
			err: `master variant "kind=work" is duplicated`,
		},
		{
			name: "rows without the selection field",
			edit: func(modules []*BaseModule) {
				modules[len(modules)-1].Fields = modules[len(modules)-1].Fields[1:]
			},
			err: `master variant kind=work does not define selection field "id"`,
		},
		{
			name: "a variant that is not a list",
			edit: func(modules []*BaseModule) {
				threadedWorkspace(modules).MasterVariants[0].Master.ActionResource = renderer.ActionResource{Module: "workspace_entry", Action: "add"}
			},
			err: "master variant",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules := variantGlobalWidgetModules()
			test.edit(modules)
			err := (&Generator{Modules: modules}).validateGlobalWidgets()
			require.Error(t, err)
			require.Contains(t, err.Error(), test.err)
		})
	}
}

// A copy of the widget owns its variants: filling in one reader's bindings
// must not reach the module's own.
func TestCloneWorkspaceWidgetOwnsItsMasterVariants(t *testing.T) {
	source := &renderer.WorkspaceWidget{MasterVariants: []renderer.WorkspaceMasterVariant{{
		Key: "kind", Val: "work", Unfold: "threads",
		Master: renderer.Resource{ActionResource: renderer.ActionResource{Module: "work_records", Action: "list"}, Bindings: []renderer.RequestBinding{{Target: renderer.RequestBindingFilter, Field: "owner_id"}}},
	}}}
	cloned := renderer.GlobalWidget{Workspace: source}.Clone().Workspace
	cloned.MasterVariants[0].Master.Bindings[0].Field = "changed"
	cloned.MasterVariants[0].Val = "changed"
	require.Equal(t, "owner_id", source.MasterVariants[0].Master.Bindings[0].Field)
	require.Equal(t, "work", source.MasterVariants[0].Val)
}
