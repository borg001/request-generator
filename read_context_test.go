package module

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/icontext"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestGeneratedReadsBindHTTPRequestCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, view := range []bool{false, true} {
		pool, mock, err := sqlmock.New()
		require.NoError(t, err)
		executor := db.NewDBWithPreparedViews(pool, 8)
		id := pg.IntegerColumn("id")
		mod := &BaseModule{Name: "items", Path: "/api", Table: pg.NewTable("public", "items", "", id), PrimaryKey: id}
		engine := gin.New()
		gen := NewGenerator(func(*BaseModule) db.DBExecutor { return executor }, *engine.Group(""), []*BaseModule{mod}, nil, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.Request = httptest.NewRequest("GET", "/api/items", nil).WithContext(context.WithValue(ctx, icontext.LoggerContextKey, log.NewEntry(log.New())))
		if view {
			c.Params = gin.Params{{Key: "bykey", Value: "id"}, {Key: "value", Value: "1"}}
			gen.actionView(mod, actions.ViewModuleAction{By: []pg.Column{id}})(c)
		} else {
			gen.actionList(mod, actions.ListModuleAction{})(c)
		}
		require.Equal(t, 400, w.Code)
		require.Contains(t, w.Body.String(), "context canceled")
		require.NoError(t, mock.ExpectationsWereMet())
		pool.Close()
	}
}
