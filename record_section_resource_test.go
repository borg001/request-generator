package module

import (
	"encoding/json"
	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestRecordResourceResolutionAndPermissions(t *testing.T) {
	id := pg.IntegerColumn("id")
	table := pg.NewTable("public", "entries", "", id)
	target := &BaseModule{Name: "entries", Path: "/api", Table: table, PrimaryKey: id, Fields: []fields.ModuleField{{Column: id, Type: fields.ModuleFieldTypeInt}}, Actions: []actions.ModuleAction{actions.ListModuleAction{Columns: []pg.Column{id}, Permission: []actions.Role{"editor"}, Auth: true, Filter: []pg.Column{id}}}, Render: renderer.Universal{List: &renderer.ListPage{ID: "entries"}}}
	generator := &Generator{Modules: []*BaseModule{target}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/parent/view/id/7", nil)
	original := renderer.Universal{Record: &renderer.RecordPage{Sections: []renderer.RecordSection{{ID: "children", Renderer: renderer.RendererUniversalSection, Resource: &renderer.Resource{ActionResource: renderer.ActionResource{Module: "entries", Action: "list"}, Bindings: []renderer.RequestBinding{{Target: renderer.RequestBindingFilter, Field: "id", Source: renderer.ValueSource{Literal: &renderer.TypedValue{Type: renderer.TypedValueNumber, Number: 7}}}}}}}}}
	allowed := original.Clone()
	require.NoError(t, generator.resolveFormSectionResources(c, &allowed, "editor"))
	require.NotNil(t, allowed.Record.Sections[0].Load)
	encoded, err := json.Marshal(allowed)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "/api/entries")
	require.NotContains(t, string(encoded), `"resource"`)
	denied := original.Clone()
	require.NoError(t, generator.resolveFormSectionResources(c, &denied, "viewer"))
	require.Empty(t, denied.Record.Sections)
	require.Nil(t, original.Record.Sections[0].Load)
}
