package module

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryValueIsolatesConcurrentRolesAndPreservesGates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := &BaseModule{Name: "items", Path: "/api", Render: renderer.Universal{List: &renderer.ListPage{ID: "items"}},
		Actions:    []actions.ModuleAction{actions.ListModuleAction{}, actions.AddModuleAction{}},
		Navigation: []NavigationEntry{{ActionName: "list", Path: "/items", Show: true, Query: map[string]interface{}{"scope": "one"}}},
		Routes:     []RoutablePage{{ActionName: "list", Path: "/all-items"}},
		RenderFunc: func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			return base, errors.New("runtime must not run during compact discovery")
		},
		ConfigRenderFunc: func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
			return base, errors.New("old discovery must not run")
		},
		DiscoveryFunc: func(c *gin.Context, base renderer.Discovery) (renderer.Discovery, error) {
			calls, _ := c.Get("discovery-calls")
			if calls != nil {
				return base, errors.New("discovery called twice in one config")
			}
			c.Set("discovery-calls", 1)
			if actions.GetRoleFromContext(c) == "editor" {
				base.List = renderer.PageTypeResourceGrid
				base.Form = true
			}
			return base, nil
		},
	}
	g := &Generator{Modules: []*BaseModule{m}, AccessGate: func(c *gin.Context, _ AccessTarget) (bool, string) {
		return actions.GetRoleFromContext(c) == "reader", c.Query("lang")
	}}
	for i := 0; i < 32; i++ {
		i := i
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			role, lang := "reader", "en"
			if i%2 == 1 {
				role, lang = "editor", "ru"
			}
			c, w := discoveryContext(role, lang)
			g.actionConfigEndpoint()(c)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var result ConfigResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
			expected := renderer.PageTypeList
			if role == "editor" {
				expected = renderer.PageTypeResourceGrid
			}
			require.Equal(t, expected, result.Navigation[0].Target.PageType)
			require.Equal(t, role == "reader", result.Navigation[0].Locked)
			require.Nil(t, result.Routes[1].Target.Query.Params)
			if role == "editor" {
				require.Equal(t, renderer.PageTypeForm, result.Routes[0].Target.Children["add"].PageType)
			}
			require.Nil(t, m.Render.Form)
			require.NotNil(t, m.Render.List)
			// Outside /config the existing full runtime validation remains.
			_, err := renderForDiscovery(c, m)
			require.ErrorContains(t, err, "runtime must not run")
		})
	}
}

func TestDiscoveryValueRejectsInvalidCapabilitiesAndErrors(t *testing.T) {
	for _, hook := range []DiscoveryFunc{
		func(_ *gin.Context, d renderer.Discovery) (renderer.Discovery, error) {
			d.List = renderer.PageTypeForm
			return d, nil
		},
		func(_ *gin.Context, d renderer.Discovery) (renderer.Discovery, error) {
			return d, errors.New("unavailable")
		},
	} {
		m := &BaseModule{Name: "items", DiscoveryFunc: hook, Actions: []actions.ModuleAction{actions.ListModuleAction{}}, Navigation: []NavigationEntry{{ActionName: "list", Show: true, Path: "/items"}}}
		c, w := discoveryContext("reader", "en")
		(&Generator{Modules: []*BaseModule{m}}).actionConfigEndpoint()(c)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func BenchmarkConfigDiscoveryValue(b *testing.B) {
	gin.SetMode(gin.TestMode)
	m := &BaseModule{Render: renderer.Universal{List: &renderer.ListPage{ID: "items"}, Form: &renderer.FormPage{ID: "edit"}}}
	for i := 0; i < 80; i++ {
		m.Render.Form.Sections = append(m.Render.Form.Sections, renderer.FormSection{ID: fmt.Sprint(i), Title: "title"})
	}
	for _, compact := range []bool{false, true} {
		b.Run(fmt.Sprintf("compact=%v", compact), func(b *testing.B) {
			m.DiscoveryFunc = nil
			m.ConfigRenderFunc = func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) { return base, nil }
			if compact {
				m.DiscoveryFunc = func(_ *gin.Context, base renderer.Discovery) (renderer.Discovery, error) { return base, nil }
			}
			c, _ := discoveryContext("reader", "en")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.Set(configRenderCacheKey, make(map[*BaseModule]renderer.Discovery))
				if _, err := renderForDiscovery(c, m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
