package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	module "github.com/darkrain/request-generator"
	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/gin-gonic/gin"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationScopeList_CanonicalizesIntegerIDAndQualifiesSourceColumn(t *testing.T) {
	cleanTable(t)
	_, err := sqlDB.Exec(`INSERT INTO test_items (name, email, age, role) VALUES
		('One', 'one@example.test', 30, 'user'),
		('Two', 'two@example.test', 40, 'user')`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`INSERT INTO test_tags (item_id, tag) VALUES (1, 'alpha'), (2, 'beta')`)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("")

	id := postgres.IntegerColumn("id")
	ownersTable := postgres.NewTable("public", "owners", "", id)
	tagID := postgres.IntegerColumn("id")
	tagItemID := postgres.IntegerColumn("item_id")
	tagCol := postgres.StringColumn("tag")
	tagsTable := postgres.NewTable("public", "test_tags", "tags", tagID, tagItemID, tagCol)

	var checkedScope module.RelationScope
	itemsModule := &module.BaseModule{
		Name:       "items",
		Path:       "/api",
		Table:      tbl,
		PrimaryKey: tbl.ID,
		Fields: []fields.ModuleField{
			{Column: tbl.ID, Title: "ID", Type: fields.ModuleFieldTypeInt},
			{Column: tbl.Name, Title: "Name", Type: fields.ModuleFieldTypeString},
			{Column: tbl.Email, Title: "Email", Type: fields.ModuleFieldTypeString},
			{Column: tbl.Age, Title: "Age", Type: fields.ModuleFieldTypeInt},
			{Column: tbl.Role, Title: "Role", Type: fields.ModuleFieldTypeString},
		},
		Relations: []module.ModuleRelation{{
			Name:         "owner",
			TargetModule: "owners",
			SourceField:  tbl.ID,
			TargetField:  id,
			ScopeCheck: func(_ *gin.Context, scope module.RelationScope) error {
				checkedScope = scope
				return nil
			},
		}},
		Actions: []actions.ModuleAction{
			actions.ListModuleAction{
				Columns:    []postgres.Column{tbl.ID, tbl.Name, tbl.Email, tbl.Age, tbl.Role},
				Join:       []actions.ModuleActionJoin{actions.NewJoin(tagsTable, actions.JoinTypeLeft, postgres.RawBool(`test_items."id" = tags."item_id"`, nil), []postgres.Column{tagCol}, "tags")},
				Permission: []actions.Role{actions.RoleAll},
				Auth:       true,
			},
		},
	}
	ownersModule := &module.BaseModule{
		Name:       "owners",
		Path:       "/api",
		Table:      ownersTable,
		PrimaryKey: id,
		Fields: []fields.ModuleField{
			{Column: id, Title: "ID", Type: fields.ModuleFieldTypeInt},
		},
	}

	generator := module.NewGenerator(
		func(_ *module.BaseModule) db.DBExecutor { return testDB },
		*group,
		[]*module.BaseModule{itemsModule, ownersModule},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		func(_ actions.ModuleAction) gin.HandlerFunc {
			return func(c *gin.Context) {
				ctx := context.WithValue(c.Request.Context(), icontext.LoggerContextKey, testLog)
				ctx = icontext.SetUser(ctx, &icontext.UserInfo{ID: 1, Role: "admin"})
				c.Request = c.Request.WithContext(ctx)
				c.Next()
			}
		},
	)
	generator.Run()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items?scope[relation]=owner&scope[id]=1", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, module.RelationScope{Relation: "owner", ID: int64(1)}, checkedScope)

	var response struct {
		Count int64                    `json:"count"`
		Rows  []map[string]interface{} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, int64(1), response.Count)
	require.Len(t, response.Rows, 1)
	assert.Equal(t, float64(1), response.Rows[0]["id"])

	checkedScope = module.RelationScope{}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/items?scope[relation]=owner&scope[id]=bad", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, module.RelationScope{}, checkedScope)
}
