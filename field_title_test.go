package module

import (
	"net/http/httptest"
	"testing"

	"github.com/darkrain/request-generator/fields"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The record a form reads names its fields as the field list does: TitleFunc
// first, the field's own title when it has no answer (theGHub1/api#367).
func TestFieldTitleForAsksTitleFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)

	field := fields.ModuleField{Title: "settings.privacy.blur_no_deals", TitleFunc: func(*gin.Context) string { return "settings.privacy.blur_no_deals_client" }}
	require.Equal(t, "settings.privacy.blur_no_deals_client", fieldTitleFor(c, field))
	field.TitleFunc = func(*gin.Context) string { return "" }
	require.Equal(t, "settings.privacy.blur_no_deals", fieldTitleFor(c, field))
	field.TitleFunc = nil
	require.Equal(t, "settings.privacy.blur_no_deals", fieldTitleFor(c, field))
}
