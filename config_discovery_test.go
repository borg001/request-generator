package module

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func discoveryContext(role, language string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/config?lang="+language, nil)
	c.Request = c.Request.WithContext(icontext.SetUser(c.Request.Context(), &icontext.UserInfo{ID: 1, Role: role}))
	return c, w
}

func TestConfigDiscoveryIsRequestScopedAndKeepsRuntimeRender(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configCalls, runtimeCalls, gates := 0, 0, 0
	m := &BaseModule{
		Name: "items", Path: "/api",
		Render:     renderer.Universal{List: &renderer.ListPage{ID: "items", Context: map[string]interface{}{"base": true}}},
		Navigation: []NavigationEntry{{ActionName: "list", Path: "/items", Show: true, Query: map[string]interface{}{"scope": "one"}}},
		Routes:     []RoutablePage{{ActionName: "list", Path: "/all-items"}},
		Actions: []actions.ModuleAction{actions.ListModuleAction{Widget: &actions.WidgetConfig{
			ID: "items-summary", Renderer: renderer.GlobalWidget{Surface: renderer.WidgetSurface{
				Kind: renderer.WidgetSurfaceDrawer, Placement: renderer.WidgetPlacementShellEnd, LoadPolicy: renderer.WidgetLoadOnOpen,
			}},
		}}},
		ConfigRenderFunc: func(c *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			configCalls++
			base.List.Context["request"] = c.Query("lang")
			if actions.GetRoleFromContext(c) == "editor" {
				base.Form = &renderer.FormPage{ID: "editor-form"}
			}
			return base, nil
		},
		RenderFunc: func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			runtimeCalls++
			base.List.Title = "runtime content"
			return base, nil
		},
	}
	g := &Generator{Modules: []*BaseModule{m}, Locales: []locale.Lang{locale.EN, locale.RU}, DefaultLocale: locale.EN,
		AccessGate: func(c *gin.Context, target AccessTarget) (bool, string) {
			gates++
			return actions.GetRoleFromContext(c) == "reader", "current request"
		},
	}
	for i, role := range []string{"reader", "editor"} {
		c, w := discoveryContext(role, []string{"en", "ru"}[i])
		g.actionConfigEndpoint()(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, i+1, configCalls, "navigation, routes and widgets share one descriptor")
		require.Zero(t, runtimeCalls)
		var response ConfigResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Len(t, response.Navigation, 1)
		require.Len(t, response.Routes, 2)
		require.Len(t, response.Widgets, 1)
		require.Equal(t, role == "reader", response.Navigation[0].Locked)
		require.Equal(t, renderer.PageTypeList, response.Navigation[0].Target.PageType)
		require.NotNil(t, response.Navigation[0].Target.Renderer)
		require.Nil(t, response.Routes[1].Target.Query.Params, "navigation query must not leak to another route")
		require.Nil(t, m.Render.Form)
		require.Equal(t, map[string]interface{}{"base": true}, m.Render.List.Context)

		// Resource discovery in a page response must still validate its full
		// renderer; the config cache is removed even when the context is reused.
		_, available, err := g.buildResourceLoad(c, m, m.Actions[0], nil, nil)
		require.NoError(t, err)
		require.True(t, available)
		require.Equal(t, 1, runtimeCalls)
		runtimeCalls = 0
		render, err := m.RenderFor(c)
		require.NoError(t, err)
		require.Equal(t, "runtime content", render.List.Title)
		runtimeCalls = 0
	}
	require.Equal(t, 8, gates, "access policy runs for every destination in every request")
}

func TestConfigDiscoveryPreservesDefaultDynamicPageTypes(t *testing.T) {
	m := &BaseModule{Name: "items", Path: "/api", Actions: []actions.ModuleAction{actions.ListModuleAction{}},
		Navigation: []NavigationEntry{{ActionName: "list", Show: true, Path: "/items"}},
		RenderFunc: func(c *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			if actions.GetRoleFromContext(c) == "editor" {
				base.ResourceGrid = &renderer.ResourceGridPage{Endpoint: "/api/items"}
			} else {
				base.List = &renderer.ListPage{ID: "items"}
			}
			return base, nil
		},
	}
	g := &Generator{Modules: []*BaseModule{m}}
	for _, role := range []string{"reader", "editor", "reader"} {
		c, w := discoveryContext(role, "en")
		g.actionConfigEndpoint()(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response ConfigResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		expected := renderer.PageTypeList
		if role == "editor" {
			expected = renderer.PageTypeResourceGrid
		}
		require.Equal(t, expected, response.Routes[0].Target.PageType)
	}
}

func TestConfigDiscoveryValidatesHookAndFieldReferences(t *testing.T) {
	for _, tc := range []struct {
		name    string
		hook    RenderFunc
		message string
	}{
		{"error", func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			return base, errors.New("discovery failed")
		}, "discovery failed"},
		{"shape", func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			base.List = &renderer.ListPage{}
			base.ResourceGrid = &renderer.ResourceGridPage{Endpoint: "/api/items"}
			return base, nil
		}, "mutually exclusive"},
		{"field", func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			base.Form = &renderer.FormPage{Sections: []renderer.FormSection{{ID: "bad", Renderer: renderer.RendererFieldMatrix, Matrix: &renderer.FieldMatrix{Type: renderer.FieldMatrixTypeList, List: &renderer.FieldMatrixList{Fields: []string{"unknown"}, Columns: renderer.FieldMatrixColumnsOne}}}}}
			return base, nil
		}, "unknown field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &BaseModule{Name: "items", Path: "/api", Fields: []fields.ModuleField{},
				Actions:    []actions.ModuleAction{actions.ListModuleAction{}},
				Navigation: []NavigationEntry{{ActionName: "list", Show: true, Path: "/items"}}, ConfigRenderFunc: tc.hook}
			c, w := discoveryContext("reader", "en")
			(&Generator{Modules: []*BaseModule{m}}).actionConfigEndpoint()(c)
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), tc.message)
		})
	}
}
