package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	module "github.com/darkrain/request-generator"
	"github.com/darkrain/request-generator/actions"
	dbpkg "github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A field can be shown per request: here the number of recipients a form may
// take depends on the role filling it in, while the shared metadata stays put.
func TestFieldPresentationFuncShowsTheFieldPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := pg.IntegerColumn("id")
	recipients := pg.StringColumn("recipients")
	title := pg.StringColumn("title")
	table := pg.NewTable("public", "presented_items", "", id, recipients, title)
	shared := &renderer.FieldPresentation{Renderer: renderer.RendererRecordSelect, MaxItems: 5, Hint: "Pick recipients"}
	testModule := &module.BaseModule{
		Name:       "presented-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
			{
				Column: recipients, Title: "Recipients", Type: fields.ModuleFieldTypeString, FormType: fields.ModuleFieldFormTypeMultiselect,
				Presentation: shared,
				PresentationFunc: func(c *gin.Context, presentation renderer.FieldPresentation) *renderer.FieldPresentation {
					if actions.GetRoleFromContext(c) != "client" {
						return nil
					}
					presentation.MaxItems = 3
					return &presentation
				},
			},
			{Column: title, Title: "Title", Type: fields.ModuleFieldTypeString, FormType: fields.ModuleFieldFormTypeText, Presentation: &renderer.FieldPresentation{Hint: "Name it"}},
		},
		Render: renderer.Universal{Form: &renderer.FormPage{ID: "presented-items-form"}},
		Actions: []actions.ModuleAction{
			actions.AddModuleAction{Columns: []pg.Column{recipients, title}, Permission: []actions.Role{actions.RoleAll}, Auth: true, Label: "Add"},
		},
	}
	engineFor := func(role string) *gin.Engine {
		engine := gin.New()
		group := engine.Group("")
		generator := module.NewGenerator(
			func(_ *module.BaseModule) dbpkg.DBExecutor { return fakeRendererDB{} },
			*group,
			[]*module.BaseModule{testModule},
			func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
				return func(c *gin.Context) { c.Next() }
			},
			createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: role}),
		)
		generator.Run()
		return engine
	}
	type defrec struct {
		Fields map[string]struct {
			Presentation *renderer.FieldPresentation `json:"presentation"`
		} `json:"fields"`
	}
	read := func(role string) defrec {
		w := executeRequest(engineFor(role), http.MethodGet, "/admin/presented-items/defrec/", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response defrec
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		return response
	}

	client := read("client")
	require.NotNil(t, client.Fields["recipients"].Presentation)
	assert.Equal(t, uint16(3), client.Fields["recipients"].Presentation.MaxItems)
	assert.Equal(t, "Pick recipients", client.Fields["recipients"].Presentation.Hint, "the rest of the presentation is kept")
	require.NotNil(t, client.Fields["title"].Presentation)
	assert.Equal(t, "Name it", client.Fields["title"].Presentation.Hint)

	agency := read("agency")
	require.NotNil(t, agency.Fields["recipients"].Presentation)
	assert.Equal(t, uint16(5), agency.Fields["recipients"].Presentation.MaxItems, "nil keeps the field's own presentation")
	assert.Equal(t, uint16(5), shared.MaxItems, "the shared metadata is never changed")
}
