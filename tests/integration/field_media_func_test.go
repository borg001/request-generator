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

// A field's media control can be named per request: the way to a gallery
// reads as the model's when an agency edits her, while the shared metadata
// stays put.
func TestFieldMediaFuncNamesTheControlPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := pg.IntegerColumn("id")
	avatar := pg.StringColumn("avatar")
	table := pg.NewTable("public", "media_items", "", id, avatar)
	open := &renderer.Action{ID: "open_gallery", Label: "My gallery", Type: renderer.ActionRoute, Route: renderer.RouteAction{Path: "/my-gallery"}}
	shared := &renderer.FieldMediaConfig{Actions: &renderer.MediaGalleryActions{Open: open}}
	testModule := &module.BaseModule{
		Name:       "media-items",
		Path:       "/admin",
		Table:      table,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt, FormType: fields.ModuleFieldFormTypeNumber},
			{
				Column: avatar, Title: "Avatar", Type: fields.ModuleFieldTypeString, FormType: fields.ModuleFieldFormTypeText,
				Media: shared,
				MediaFunc: func(c *gin.Context, media renderer.FieldMediaConfig) *renderer.FieldMediaConfig {
					if actions.GetRoleFromContext(c) != "agency" {
						return nil
					}
					actionsCopy := *media.Actions
					openCopy := *actionsCopy.Open
					openCopy.Label = "Model gallery"
					actionsCopy.Open = &openCopy
					media.Actions = &actionsCopy
					return &media
				},
			},
		},
		Render: renderer.Universal{Form: &renderer.FormPage{ID: "media-items-form"}},
		Actions: []actions.ModuleAction{
			actions.AddModuleAction{Columns: []pg.Column{avatar}, Permission: []actions.Role{actions.RoleAll}, Auth: true, Label: "Add"},
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
			Media *renderer.FieldMediaConfig `json:"media"`
		} `json:"fields"`
	}
	read := func(role string) defrec {
		w := executeRequest(engineFor(role), http.MethodGet, "/admin/media-items/defrec/", nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response defrec
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		return response
	}

	agency := read("agency")
	require.NotNil(t, agency.Fields["avatar"].Media)
	assert.Equal(t, "Model gallery", agency.Fields["avatar"].Media.Actions.Open.Label)

	model := read("model")
	require.NotNil(t, model.Fields["avatar"].Media)
	assert.Equal(t, "My gallery", model.Fields["avatar"].Media.Actions.Open.Label, "nil keeps the field's own media")
	assert.Equal(t, "My gallery", open.Label, "the shared metadata is never changed")
}
