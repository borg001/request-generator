package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/darkrain/request-generator"
	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRouter создаёт тестовый роутер с мокированным AuthMiddleware
func setupTestRouter(authMiddleware func(actions.ModuleAction) gin.HandlerFunc) (*module.Generator, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	// Создаём простой тестовый модуль
	testModule := &module.BaseModule{
		Name: "users",
		Path: "/admin",
		Render: renderer.Universal{
			List: &renderer.ListPage{
				ID: "users",
			},
			Form: &renderer.FormPage{
				ID: "users-form",
			},
		},
		Navigation: []module.NavigationEntry{
			{
				ActionName: "list",
				Title:      "Пользователи",
				Group:      "Управление",
				Order:      1,
				Icon:       "user",
				Show:       true,
				Path:       "/admin/users",
				Query:      map[string]interface{}{"scope": "active"},
			},
			{
				ActionName: "add",
				Title:      "Добавить",
				Group:      "Управление",
				Order:      2,
				Icon:       "plus",
				Show:       true,
				Path:       "/admin/users/add",
			},
		},
		Routes: []module.RoutablePage{
			{ActionName: "defrec", Path: "/admin/users/create", Roles: []actions.Role{"admin"}},
		},
		Actions: []actions.ModuleAction{
			&actions.ListModuleAction{
				Permission: []actions.Role{"admin", "manager"},
				Label:      "Список пользователей",
			},
			&actions.AddModuleAction{
				Permission: []actions.Role{"admin"},
				Label:      "Добавить пользователя",
			},
			&actions.ViewModuleAction{
				Permission: []actions.Role{"admin", "manager", actions.RoleAll},
				Label:      "Просмотр пользователя",
			},
			&actions.UpdateModuleAction{
				Permission: []actions.Role{"admin"},
				Label:      "Обновить пользователя",
			},
		},
	}

	modules := []*module.BaseModule{testModule}

	// Мок PermissionMiddleware
	permissionMiddleware := func(action actions.ModuleAction, permissions []actions.Role) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	g := engine.Group("") // корневой роутер, т.к. Run() сам создаёт группу /api
	generator := module.NewGenerator(
		nil, // db функция не нужна для теста конфиг эндпоинта
		*g,
		modules,
		permissionMiddleware,
		authMiddleware,
	)

	// Регистрируем маршруты (это создаст конфиг эндпоинт)
	generator.Run()

	return generator, engine
}

// createMockAuthMiddleware создаёт мок AuthMiddleware, который устанавливает пользователя в контекст
func createMockAuthMiddleware(user *icontext.UserInfo) func(actions.ModuleAction) gin.HandlerFunc {
	return func(action actions.ModuleAction) gin.HandlerFunc {
		return func(c *gin.Context) {
			if user == nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			ctx := icontext.SetUser(c.Request.Context(), user)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		}
	}
}

func executeRequest(engine *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func executeJSONRequest(engine *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// TestConfigEndpoint_ValidToken — успешный запрос с валидным токеном
func TestConfigEndpoint_ValidToken(t *testing.T) {
	// Мокированный пользователь с ролью admin
	mockUser := &icontext.UserInfo{
		ID:   1,
		Role: "admin",
	}

	_, engine := setupTestRouter(createMockAuthMiddleware(mockUser))

	w := executeRequest(engine, "GET", "/api/config", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем, что ответ валидный JSON с правильной структурой
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "navigation")
	assert.Contains(t, response, "navigation_more_label")
	assert.Equal(t, "More", response["navigation_more_label"])
	assert.Contains(t, response, "widgets")
	assert.Contains(t, response, "role")
	assert.Equal(t, "admin", response["role"])
}

// TestConfigEndpoint_InvalidToken — 401 при невалидном токене
func TestConfigEndpoint_InvalidToken(t *testing.T) {
	// Мокированный middleware, который всегда возвращает 401
	invalidAuthMiddleware := func(action actions.ModuleAction) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}

	_, engine := setupTestRouter(invalidAuthMiddleware)

	w := executeRequest(engine, "GET", "/api/config", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestConfigEndpoint_MissingToken — 401 при отсутствии токена
func TestConfigEndpoint_MissingToken(t *testing.T) {
	// Мокированный middleware, который проверяет наличие токена
	noTokenMiddleware := func(action actions.ModuleAction) gin.HandlerFunc {
		return func(c *gin.Context) {
			// Имитируем отсутствие токена
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}

	_, engine := setupTestRouter(noTokenMiddleware)

	w := executeRequest(engine, "GET", "/api/config", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestConfigEndpoint_RoleFiltering — проверка что модули фильтруются по роли
func TestConfigEndpoint_RoleFiltering(t *testing.T) {
	// Создаём отдельный роутер для теста с модулем
	gin.SetMode(gin.TestMode)

	testModule := &module.BaseModule{
		Name: "restricted",
		Path: "/admin",
		Navigation: []module.NavigationEntry{
			{
				ActionName: "list",
				Title:      "Restricted Module",
				Group:      "Admin",
				Order:      1,
				Show:       true,
				Path:       "/admin/restricted",
			},
		},
		Actions: []actions.ModuleAction{
			&actions.ListModuleAction{
				Permission: []actions.Role{"admin"}, // Только админ
				Label:      "Restricted List",
			},
		},
	}

	modules := []*module.BaseModule{testModule}

	permissionMiddleware := func(action actions.ModuleAction, permissions []actions.Role) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	mockAdmin := &icontext.UserInfo{ID: 1, Role: "admin"}
	mockManager := &icontext.UserInfo{ID: 2, Role: "manager"}

	// Тестируем доступ для админа (должен видеть модуль)
	adminEngine := gin.New()
	authMiddlewareAdmin := createMockAuthMiddleware(mockAdmin)
	ag := adminEngine.Group("")
	adminGenerator := module.NewGenerator(nil, *ag, modules, permissionMiddleware, authMiddlewareAdmin)
	adminGenerator.Run()

	w := executeRequest(adminEngine, "GET", "/api/config", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var adminResponse module.ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &adminResponse)
	require.NoError(t, err)
	assert.NotEmpty(t, adminResponse.Navigation, "Admin should see navigation")

	// Тестируем доступ для менеджера (НЕ должен видеть модуль)
	managerEngine := gin.New()
	authMiddlewareManager := createMockAuthMiddleware(mockManager)
	mg := managerEngine.Group("")
	managerGenerator := module.NewGenerator(nil, *mg, modules, permissionMiddleware, authMiddlewareManager)
	managerGenerator.Run()

	w = executeRequest(managerEngine, "GET", "/api/config", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var managerResponse module.ConfigResponse
	err = json.Unmarshal(w.Body.Bytes(), &managerResponse)
	require.NoError(t, err)
	assert.Empty(t, managerResponse.Navigation, "Manager should not see admin-only navigation")
}

// TestConfigEndpoint_NavigationStructure — проверка структуры navigation
func TestConfigEndpoint_NavigationStructure(t *testing.T) {
	mockUser := &icontext.UserInfo{
		ID:   1,
		Role: "admin",
	}

	_, engine := setupTestRouter(createMockAuthMiddleware(mockUser))

	w := executeRequest(engine, "GET", "/api/config", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response module.ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.NotEmpty(t, response.Navigation, "Navigation should not be empty")

	for _, element := range response.Navigation {
		assert.Equal(t, "page", element.Target.Type, "Navigation item should default to page target")
		assert.NotEmpty(t, element.Path, "Page navigation item path should not be empty")
		assert.NotEmpty(t, element.Target.Query.Url, "Page navigation target query URL should not be empty")
		assert.NotEmpty(t, element.Target.Query.Method, "Page navigation target query method should not be empty")
		encoded, err := json.Marshal(element)
		require.NoError(t, err)
		assert.NotContains(t, string(encoded), `"data"`, "Navigation must not emit arbitrary legacy data")
		assert.NotContains(t, string(encoded), "view_adapter", "Navigation must not emit legacy view adapters")
	}
}

func TestConfigEndpoint_NavigationGroupTitleIsLocalized(t *testing.T) {
	generator, engine := setupTestRouter(createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}))
	generator.GroupTitles = map[string]string{"Управление": "navigation.groups.management"}
	translationsDir := t.TempDir()
	ruPath := filepath.Join(translationsDir, "ru.json")
	enPath := filepath.Join(translationsDir, "en.json")
	require.NoError(t, os.WriteFile(ruPath, []byte(`{"navigation":{"groups":{"management":"Управление"}}}`), 0o600))
	require.NoError(t, os.WriteFile(enPath, []byte(`{"navigation":{"groups":{"management":"Management"}}}`), 0o600))
	require.NoError(t, generator.LoadTranslationsFile(locale.RU, ruPath))
	require.NoError(t, generator.LoadTranslationsFile(locale.EN, enPath))
	generator.Locales = []locale.Lang{locale.RU, locale.EN}
	generator.DefaultLocale = locale.EN

	findTitle := func(language string) string {
		response := executeRequest(engine, http.MethodGet, "/api/config?lang="+language, nil)
		require.Equal(t, http.StatusOK, response.Code)
		var config module.ConfigResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &config))
		require.NotEmpty(t, config.Navigation)
		return config.Navigation[0].GroupTitle
	}
	require.Equal(t, "Управление", findTitle("ru"))
	require.Equal(t, "Management", findTitle("en"))

	generator.GroupTitles = nil
	response := executeRequest(engine, http.MethodGet, "/api/config?lang=en", nil)
	var config module.ConfigResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &config))
	require.Empty(t, config.Navigation[0].GroupTitle)
}

func TestConfigEndpoint_RouteRegistry(t *testing.T) {
	_, adminEngine := setupTestRouter(createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}))
	adminResponse := executeRequest(adminEngine, http.MethodGet, "/api/config", nil)
	require.Equal(t, http.StatusOK, adminResponse.Code)

	var adminConfig module.ConfigResponse
	require.NoError(t, json.Unmarshal(adminResponse.Body.Bytes(), &adminConfig))
	paths := make(map[string]module.ConfigRouteEntry, len(adminConfig.Routes))
	for _, route := range adminConfig.Routes {
		paths[route.Path] = route
	}
	require.Contains(t, paths, "/admin/users")
	require.Equal(t, map[string]interface{}{"scope": "active"}, paths["/admin/users"].Target.Query.Params)
	create, ok := paths["/admin/users/create"]
	require.True(t, ok)
	require.Equal(t, renderer.PageTypeForm, create.Target.PageType)
	require.NotNil(t, create.Target.Query)
	require.Equal(t, "/api/admin/users/defrec/", create.Target.Query.Url)
	require.Equal(t, http.MethodGet, create.Target.Query.Method)

	_, managerEngine := setupTestRouter(createMockAuthMiddleware(&icontext.UserInfo{ID: 2, Role: "manager"}))
	managerResponse := executeRequest(managerEngine, http.MethodGet, "/api/config", nil)
	require.Equal(t, http.StatusOK, managerResponse.Code)
	var managerConfig module.ConfigResponse
	require.NoError(t, json.Unmarshal(managerResponse.Body.Bytes(), &managerConfig))
	for _, route := range managerConfig.Routes {
		require.NotEqual(t, "/admin/users/create", route.Path)
	}
}

func TestConfigEndpoint_RouteRegistryRejectsDuplicatePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("")
	generator := module.NewGenerator(
		nil,
		*group,
		[]*module.BaseModule{{
			Name:    "duplicate-routes",
			Path:    "/admin",
			Render:  renderer.Universal{List: &renderer.ListPage{ID: "duplicate-routes"}},
			Actions: []actions.ModuleAction{actions.ListModuleAction{Permission: []actions.Role{"admin"}}},
			Navigation: []module.NavigationEntry{
				{ActionName: "list", Show: true, Path: "/admin/duplicate", Title: "One"},
				{ActionName: "list", Show: true, Path: "/admin/duplicate", Title: "Two"},
			},
		}},
		func(_ actions.ModuleAction, _ []actions.Role) gin.HandlerFunc {
			return func(c *gin.Context) { c.Next() }
		},
		createMockAuthMiddleware(&icontext.UserInfo{ID: 1, Role: "admin"}),
	)
	generator.Run()

	w := executeRequest(engine, http.MethodGet, "/api/config", nil)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "declared more than once")
}

func TestNavigationContract_HasNoArbitraryDataField(t *testing.T) {
	for _, value := range []interface{}{
		module.NavigationEntry{},
		module.ConfigNavigationEntry{},
		module.NavigationPageTarget{},
		module.RouteConfig{},
	} {
		_, exists := reflect.TypeOf(value).FieldByName("Data")
		assert.False(t, exists, "%T must not expose an arbitrary Data field", value)
	}
}

// TestConfigEndpoint_PageTargetStructure — проверка структуры page target
func TestConfigEndpoint_PageTargetStructure(t *testing.T) {
	mockUser := &icontext.UserInfo{
		ID:   1,
		Role: "admin",
	}

	_, engine := setupTestRouter(createMockAuthMiddleware(mockUser))

	w := executeRequest(engine, "GET", "/api/config", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var response module.ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	require.NotEmpty(t, response.Navigation, "Navigation should not be empty")

	var usersEntry *module.ConfigNavigationEntry
	for i := range response.Navigation {
		if response.Navigation[i].Path == "/admin/users" {
			usersEntry = &response.Navigation[i]
			break
		}
	}
	require.NotNil(t, usersEntry, "Should have navigation entry for /admin/users")

	require.NotNil(t, usersEntry.Target.Renderer, "Users page target should expose renderer discovery")
	assert.Equal(t, renderer.Name, usersEntry.Target.Renderer.Name)
	assert.Equal(t, renderer.Version, usersEntry.Target.Renderer.Version)
	assert.Equal(t, renderer.PageTypeList, usersEntry.Target.PageType)
	assert.Equal(t, "/api/admin/users", usersEntry.Target.Query.Url)
	assert.Equal(t, "GET", usersEntry.Target.Query.Method)
	encoded, err := json.Marshal(usersEntry)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"data"`, "Users page target must not emit arbitrary legacy data")
	assert.NotContains(t, string(encoded), "view_adapter", "Users page target must not emit legacy view adapters")
}
