package module

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/locale"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// A delete refused before it runs says why in the language of the request,
// like an update or an add does (theGHub1/api#449).
func TestDeleteRefusalSpeaksTheRequestLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generator := &Generator{
		Locales:       []locale.Lang{locale.EN, locale.RU},
		DefaultLocale: locale.EN,
		translations:  map[locale.Lang]map[string]string{locale.RU: {"test.keep": "Это фото нельзя удалить"}},
	}
	action := actions.DeleteModuleAction{BeforeAction: func(c *gin.Context) error {
		return errors.New(Translate(c, "test.keep", "This photo stays"))
	}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx := context.WithValue(context.Background(), icontext.LoggerContextKey, log.NewEntry(log.New()))
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/items/delete/id/1?lang=ru", nil).WithContext(ctx)
	generator.actionDelete(&BaseModule{Name: "items"}, action)(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Это фото нельзя удалить")
}
