package module

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/darkrain/request-generator/response"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
)

// ConfigResponse структурирует ответ эндпоинта /api/config
type ConfigResponse struct {
	Navigation          []ConfigNavigationEntry `json:"navigation"`
	NavigationMoreLabel string                  `json:"navigation_more_label"`
	Routes              []ConfigRouteEntry      `json:"routes"`
	Widgets             []ConfigWidget          `json:"widgets"`
	Role                string                  `json:"role"`
}

type ConfigRouteEntry struct {
	Path   string               `json:"path"`
	Target NavigationPageTarget `json:"target"`
}

type ConfigNavigationEntry struct {
	ID          string                 `json:"id,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Target      NavigationPageTarget   `json:"target"`
	Title       string                 `json:"title"`
	Icon        string                 `json:"icon,omitempty"`
	Order       int                    `json:"order,omitempty"`
	MobileOrder int                    `json:"mobile_order,omitempty"`
	MobileTitle string                 `json:"mobile_title,omitempty"`
	Group       string                 `json:"group,omitempty"`
	GroupTitle  string                 `json:"group_title,omitempty"`
	Query       map[string]interface{} `json:"query,omitempty"`
}

type NavigationPageTarget struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Renderer *renderer.Identity     `json:"renderer,omitempty"`
	PageType renderer.PageType      `json:"page_type,omitempty"`
	Query    *RouteQuery            `json:"query,omitempty"`
	Children map[string]RouteConfig `json:"children,omitempty"`
}

type ConfigWidget struct {
	ID       string                `json:"id"`
	Order    int                   `json:"order,omitempty"`
	Renderer renderer.Identity     `json:"renderer"`
	Widget   renderer.GlobalWidget `json:"widget"`
	Load     renderer.WidgetLoad   `json:"load"`
}

// RouteConfig конфигурирует маршрут
type RouteConfig struct {
	Title     string                 `json:"title"`
	MenuTitle string                 `json:"menuTitle,omitempty"`
	Renderer  *renderer.Identity     `json:"renderer,omitempty"`
	PageType  renderer.PageType      `json:"page_type,omitempty"`
	Query     *RouteQuery            `json:"query,omitempty"`
	Children  map[string]RouteConfig `json:"children,omitempty"`
}

// RouteQuery описывает параметры запроса для маршрута
type RouteQuery struct {
	Url    string                 `json:"url"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// actionConfigEndpoint генерирует конфиг для webapp
func (generator *Generator) actionConfigEndpoint() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)

		user, ok := icontext.GetUser(ctx)
		if !ok {
			response.ErrorResponse(l, c, http.StatusUnauthorized, "Unauthorized", nil)
			return
		}

		role := user.Role
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)

		navigation, err := generator.buildNavigation(c, role, lang)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		routes, err := generator.buildRouteRegistry(c, role)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		widgets, err := generator.buildWidgets(c, role)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}

		config := ConfigResponse{
			Navigation:          navigation,
			NavigationMoreLabel: generator.TranslateWithFallback(lang, "nav.more", "More"),
			Routes:              routes,
			Widgets:             widgets,
			Role:                role,
		}

		response.Response(l, c, config)
	}
}

// hasPermission проверяет, есть ли у роли доступ к действию
func hasPermission(action actions.ModuleAction, role string) bool {
	var permissions []actions.Role

	switch a := action.(type) {
	case actions.ListModuleAction:
		permissions = a.Permission
	case *actions.ListModuleAction:
		permissions = a.Permission
	case actions.AddModuleAction:
		permissions = a.Permission
	case *actions.AddModuleAction:
		permissions = a.Permission
	case actions.ViewModuleAction:
		permissions = a.Permission
	case *actions.ViewModuleAction:
		permissions = a.Permission
	case actions.UpdateModuleAction:
		permissions = a.Permission
	case *actions.UpdateModuleAction:
		permissions = a.Permission
	case actions.DeleteModuleAction:
		permissions = a.Permission
	case *actions.DeleteModuleAction:
		permissions = a.Permission
	case actions.DefrecModuleAction:
		permissions = a.Permission
	case *actions.DefrecModuleAction:
		permissions = a.Permission
	default:
		return false
	}

	if len(permissions) == 0 {
		return true
	}

	for _, perm := range permissions {
		if string(perm) == role || perm == actions.RoleAll {
			return true
		}
	}

	return false
}

func navigationID(module *BaseModule, entry NavigationEntry) string {
	if entry.ID != "" {
		return entry.ID
	}
	if entry.Group != "" || entry.ActionName != "" {
		return entry.Group + "." + module.Name + "." + entry.ActionName
	}
	return module.Name
}

func navigationPath(module *BaseModule, entry NavigationEntry, targetType string) string {
	if entry.Path != "" {
		return entry.Path
	}
	if targetType != "page" {
		return ""
	}
	return module.Path + "/" + module.Name
}

func navigationTarget(entry NavigationEntry) NavigationPageTarget {
	target := NavigationPageTarget{
		Type:   entry.Target.Type,
		Name:   entry.Target.Name,
		Params: entry.Target.Params,
	}
	if target.Type == "" {
		target.Type = "page"
	}
	return target
}

func (generator *Generator) buildNavigation(c *gin.Context, role string, lang locale.Lang) ([]ConfigNavigationEntry, error) {
	var result []ConfigNavigationEntry

	for _, module := range generator.Modules {
		for _, entry := range module.Navigation {
			if !entry.Show {
				continue
			}
			action, ok := findModuleAction(module, entry.ActionName)
			if !ok || !hasPermission(action, role) || !navigationRoleAllowed(entry, role) {
				continue
			}

			target, err := generator.buildPageTarget(c, module, action, entry.Target, entry.Query, role)
			if err != nil {
				return nil, err
			}

			configEntry := ConfigNavigationEntry{
				ID:          navigationID(module, entry),
				Path:        navigationPath(module, entry, target.Type),
				Title:       generator.Translate(lang, entry.Title),
				Icon:        entry.Icon,
				Group:       entry.Group,
				Order:       entry.Order,
				MobileOrder: entry.MobileOrder,
				MobileTitle: generator.TranslateWithFallback(lang, entry.MobileTitle, ""),
				Target:      target,
				Query:       entry.Query,
			}
			if titleKey, ok := generator.GroupTitles[entry.Group]; ok {
				configEntry.GroupTitle = generator.Translate(lang, titleKey)
			}
			result = append(result, configEntry)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Group == result[j].Group {
			return result[i].Order < result[j].Order
		}
		return result[i].Group < result[j].Group
	})

	return result, nil
}

func (generator *Generator) buildRouteRegistry(c *gin.Context, role string) ([]ConfigRouteEntry, error) {
	result := make([]ConfigRouteEntry, 0)
	seen := make(map[string]struct{})
	appendRoute := func(module *BaseModule, path string, action actions.ModuleAction, targetConfig NavigationTarget, queryParams map[string]interface{}) error {
		if path == "" || targetConfig.Type != "" && targetConfig.Type != "page" {
			return nil
		}
		target, err := generator.buildPageTarget(c, module, action, targetConfig, queryParams, role)
		if err != nil {
			return err
		}
		if _, exists := seen[path]; exists {
			return fmt.Errorf("config route path %q is declared more than once", path)
		}
		seen[path] = struct{}{}
		result = append(result, ConfigRouteEntry{Path: path, Target: target})
		return nil
	}

	for _, module := range generator.Modules {
		for _, entry := range module.Navigation {
			if !entry.Show || !navigationRoleAllowed(entry, role) {
				continue
			}
			action, ok := findModuleAction(module, entry.ActionName)
			if !ok || !hasPermission(action, role) {
				continue
			}
			if err := appendRoute(module, navigationPath(module, entry, "page"), action, entry.Target, entry.Query); err != nil {
				return nil, err
			}
		}
		for _, page := range module.Routes {
			action, ok := findModuleAction(module, page.ActionName)
			if !ok || !hasPermission(action, role) || !routeRolesAllowed(page, role) {
				continue
			}
			if err := appendRoute(module, page.Path, action, page.Target, nil); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (generator *Generator) buildPageTarget(c *gin.Context, module *BaseModule, action actions.ModuleAction, targetConfig NavigationTarget, queryParams map[string]interface{}, role string) (NavigationPageTarget, error) {
	target := navigationTarget(NavigationEntry{Target: targetConfig})
	if target.Type != "page" {
		return target, nil
	}
	render, err := module.RenderFor(c)
	if err != nil {
		return NavigationPageTarget{}, err
	}
	route := generator.buildRouteForAction(module, render, action, role)
	if targetConfig.PageType != "" {
		route.Renderer = viewRouteIdentity(render, targetConfig.PageType)
		route.PageType = viewRoutePageType(render, targetConfig.PageType)
	}
	target.Renderer = route.Renderer
	target.PageType = route.PageType
	target.Query = route.Query
	target.Children = route.Children
	if target.Query != nil && queryParams != nil {
		target.Query.Params = queryParams
	}
	return target, nil
}

func navigationRoleAllowed(entry NavigationEntry, role string) bool {
	if len(entry.Roles) == 0 {
		return true
	}
	for _, r := range entry.Roles {
		if string(r) == role || r == actions.RoleAll {
			return true
		}
	}
	return false
}

func routeRolesAllowed(route RoutablePage, role string) bool {
	if len(route.Roles) == 0 {
		return true
	}
	for _, allowed := range route.Roles {
		if string(allowed) == role || allowed == actions.RoleAll {
			return true
		}
	}
	return false
}

func findModuleAction(module *BaseModule, actionName string) (actions.ModuleAction, bool) {
	if actionName == string(actions.ModuleActionNameDefrec) {
		return module.Defrec, true
	}
	for _, action := range module.Actions {
		if string(action.Action()) == actionName {
			return action, true
		}
	}
	return nil, false
}

func (generator *Generator) buildRouteForAction(module *BaseModule, render renderer.Universal, action actions.ModuleAction, role string) RouteConfig {
	switch a := action.(type) {
	case actions.ListModuleAction:
		return generator.buildListRoute(module, render, a, role)
	case *actions.ListModuleAction:
		return generator.buildListRoute(module, render, *a, role)
	case actions.ViewModuleAction:
		return generator.buildViewRoute(module, render, a)
	case *actions.ViewModuleAction:
		return generator.buildViewRoute(module, render, *a)
	case actions.AddModuleAction:
		return generator.buildAddRoute(module, render, a)
	case *actions.AddModuleAction:
		return generator.buildAddRoute(module, render, *a)
	case actions.DefrecModuleAction:
		return generator.buildDefrecRoute(module, render, a)
	case *actions.DefrecModuleAction:
		return generator.buildDefrecRoute(module, render, *a)
	default:
		return RouteConfig{}
	}
}

func (generator *Generator) buildWidgets(c *gin.Context, role string) ([]ConfigWidget, error) {
	var result []ConfigWidget

	for _, module := range generator.Modules {
		for _, action := range module.Actions {
			widget := actionWidget(action)
			if widget == nil || !hasPermission(action, role) {
				continue
			}
			config, available, err := generator.buildWidget(c, module, action, *widget, role)
			if err != nil {
				return nil, err
			}
			if available {
				result = append(result, config)
			}
		}

		if module.Defrec.Widget != nil && hasPermission(module.Defrec, role) {
			config, available, err := generator.buildWidget(c, module, module.Defrec, *module.Defrec.Widget, role)
			if err != nil {
				return nil, err
			}
			if available {
				result = append(result, config)
			}
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Widget.Surface.Placement == result[j].Widget.Surface.Placement {
			return result[i].Order < result[j].Order
		}
		return result[i].Widget.Surface.Placement < result[j].Widget.Surface.Placement
	})

	return result, nil
}

func (generator *Generator) buildWidget(c *gin.Context, owner *BaseModule, action actions.ModuleAction, widget actions.WidgetConfig, role string) (ConfigWidget, bool, error) {
	load, available, err := generator.buildWidgetLoad(c, owner, action, widget, role)
	if err != nil || !available {
		return ConfigWidget{}, available, err
	}
	localized := renderer.LocalizeGlobalWidget(widget.Renderer, generator.rendererTextResolver(generator.getLang(c)))
	filterWorkspaceCommands(&localized, load.Commands)
	return ConfigWidget{
		ID:       widget.ID,
		Order:    widget.Order,
		Renderer: localized.Identity(),
		Widget:   localized,
		Load:     load,
	}, true, nil
}

func filterWorkspaceCommands(widget *renderer.GlobalWidget, loads []renderer.WorkspaceCommandLoad) {
	if widget == nil || widget.Workspace == nil || len(widget.Workspace.Commands) == 0 {
		return
	}
	available := make(map[string]struct{}, len(loads))
	for _, load := range loads {
		available[load.ID] = struct{}{}
	}
	commands := make([]renderer.WorkspaceCommand, 0, len(loads))
	for _, command := range widget.Workspace.Commands {
		if _, exists := available[command.ID]; exists {
			commands = append(commands, command)
		}
	}
	widget.Workspace.Commands = commands
}

func (generator *Generator) buildWidgetLoad(c *gin.Context, owner *BaseModule, action actions.ModuleAction, widget actions.WidgetConfig, role string) (renderer.WidgetLoad, bool, error) {
	if widget.Renderer.Workspace == nil {
		resource, available, err := generator.buildResourceLoad(c, owner, action, widget.Bindings, nil)
		if err != nil || !available {
			return renderer.WidgetLoad{}, available, err
		}
		return renderer.WidgetLoad{Resource: &resource}, true, nil
	}

	workspace := widget.Renderer.Workspace
	selection, err := generator.workspaceSelectionScopeForContext(c, widget.ID, *workspace)
	if err != nil {
		return renderer.WidgetLoad{}, false, err
	}
	var summary *renderer.ResourceLoad
	if workspace.Summary != nil {
		resource, available, err := generator.buildReferencedResourceLoad(c, *workspace.Summary, role, &selection)
		if err != nil || !available {
			return renderer.WidgetLoad{}, available, err
		}
		summary = &resource
	}
	master, available, err := generator.buildReferencedResourceLoad(c, workspace.Master, role, nil)
	if err != nil || !available {
		return renderer.WidgetLoad{}, available, err
	}
	detail, available, err := generator.buildReferencedResourceLoad(c, workspace.Detail, role, &selection)
	if err != nil || !available {
		return renderer.WidgetLoad{}, available, err
	}
	commands, err := generator.buildWorkspaceCommandLoads(c, workspace.Commands, role, &selection)
	if err != nil {
		return renderer.WidgetLoad{}, false, err
	}
	return renderer.WidgetLoad{Summary: summary, Master: &master, Detail: &detail, Commands: commands}, true, nil
}

func (generator *Generator) buildWorkspaceCommandLoads(c *gin.Context, commands []renderer.WorkspaceCommand, role string, selection *widgetSelectionScope) ([]renderer.WorkspaceCommandLoad, error) {
	loads := make([]renderer.WorkspaceCommandLoad, 0, len(commands))
	for _, command := range commands {
		load, available, err := generator.buildWorkspaceCommandLoad(c, command, role, selection)
		if err != nil {
			return nil, fmt.Errorf("workspace command %q: %w", command.ID, err)
		}
		if !available {
			continue
		}
		loads = append(loads, load)
	}
	return loads, nil
}

func (generator *Generator) buildReferencedResourceLoad(c *gin.Context, resource renderer.Resource, role string, selection *widgetSelectionScope) (renderer.ResourceLoad, bool, error) {
	module, ok := generator.moduleByName(resource.Module)
	if !ok {
		return renderer.ResourceLoad{}, false, fmt.Errorf("widget resource references unknown module %q", resource.Module)
	}
	action, ok := findModuleAction(module, resource.Action)
	if !ok {
		return renderer.ResourceLoad{}, false, fmt.Errorf("widget resource %q references unknown action %q", resource.Module, resource.Action)
	}
	if !hasPermission(action, role) {
		return renderer.ResourceLoad{}, false, nil
	}
	load, available, err := generator.buildResourceLoad(c, module, action, resource.Bindings, selection)
	if err != nil || !available {
		return renderer.ResourceLoad{}, available, err
	}
	return load, true, nil
}

// resolveListSummaryResource converts a producer-owned record reference into
// the standard request contract used by summary-bound list controls.
func (generator *Generator) resolveListSummaryResource(c *gin.Context, render *renderer.Universal, role actions.Role) error {
	if render == nil || render.List == nil || render.List.Summary == nil || render.List.Summary.Resource == nil {
		return nil
	}

	load, available, err := generator.buildReferencedResourceLoad(c, *render.List.Summary.Resource, string(role), nil)
	if err != nil {
		return fmt.Errorf("list summary resource: %w", err)
	}
	if !available {
		render.List.Summary.Load = nil
		return nil
	}
	render.List.Summary.Load = &load
	return nil
}

func (generator *Generator) buildWorkspaceCommandLoad(c *gin.Context, command renderer.WorkspaceCommand, role string, selection *widgetSelectionScope) (renderer.WorkspaceCommandLoad, bool, error) {
	module, ok := generator.moduleByName(command.Module)
	if !ok {
		return renderer.WorkspaceCommandLoad{}, false, fmt.Errorf("references unknown module %q", command.Module)
	}
	action, ok := findModuleAction(module, command.Action)
	if !ok {
		return renderer.WorkspaceCommandLoad{}, false, fmt.Errorf("resource %q references unknown action %q", command.Module, command.Action)
	}
	if !hasPermission(action, role) {
		return renderer.WorkspaceCommandLoad{}, false, nil
	}
	if !workspaceCommandPresentationAvailable(command, *selection) {
		return renderer.WorkspaceCommandLoad{}, false, nil
	}
	inputScope, inputLoad, available, err := workspaceCommandInputScopeForContext(c, module, action, command.Input)
	if err != nil || !available {
		return renderer.WorkspaceCommandLoad{}, available, err
	}
	resource, available, err := generator.buildWorkspaceCommandResourceLoad(c, module, action, command.Bindings, selection, inputScope)
	if err != nil || !available {
		return renderer.WorkspaceCommandLoad{}, available, err
	}
	return renderer.WorkspaceCommandLoad{
		ID:           command.ID,
		Request:      resource.Request,
		Bindings:     resource.Bindings,
		Input:        inputLoad,
		AfterSuccess: cloneWorkspaceCommandActionResult(command.AfterSuccess),
	}, true, nil
}

func cloneWorkspaceCommandActionResult(value *renderer.ActionResult) *renderer.ActionResult {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Widget != nil {
		widget := *value.Widget
		if value.Widget.Selection != nil {
			selection := *value.Widget.Selection
			widget.Selection = &selection
		}
		widget.Refresh = append([]renderer.WorkspaceRefreshTarget(nil), value.Widget.Refresh...)
		cloned.Widget = &widget
	}
	return &cloned
}

func (generator *Generator) buildResourceLoad(c *gin.Context, module *BaseModule, action actions.ModuleAction, bindings []renderer.RequestBinding, selection *widgetSelectionScope) (renderer.ResourceLoad, bool, error) {
	available, err := generator.validateRequestBindingAvailability(c, module, action, bindings, selection, nil)
	if err != nil || !available {
		return renderer.ResourceLoad{}, available, err
	}
	if _, err := module.RenderFor(c); err != nil {
		return renderer.ResourceLoad{}, false, err
	}
	contract, ok := resolveStandardActionContract(module, action)
	if !ok {
		return renderer.ResourceLoad{}, false, fmt.Errorf("resource %q action %q has no request", module.Name, action.Action())
	}
	return renderer.ResourceLoad{
		Request:  contract.Request,
		Bindings: append([]renderer.RequestBinding(nil), bindings...),
	}, true, nil
}

// resolveFormSectionResources turns server-only section resource references
// into executable standard-action requests for the current principal. A form
// section never publishes a producer-defined endpoint or request body.
func (generator *Generator) resolveFormSectionResources(c *gin.Context, render *renderer.Universal, role actions.Role) error {
	if render == nil || render.Form == nil {
		return nil
	}

	sections := make([]renderer.FormSection, 0, len(render.Form.Sections))
	for _, section := range render.Form.Sections {
		if section.Resource != nil {
			if section.Resource.Action != string(actions.ModuleActionNameView) {
				return fmt.Errorf("form section %q resource action must be view", section.ID)
			}

			targetModule, ok := generator.moduleByName(section.Resource.Module)
			if !ok {
				return fmt.Errorf("form section %q resource references unknown module %q", section.ID, section.Resource.Module)
			}
			targetRender, err := targetModule.RenderFor(c)
			if err != nil {
				return fmt.Errorf("form section %q resource render: %w", section.ID, err)
			}
			if targetRender.Form == nil {
				return fmt.Errorf("form section %q resource %q must render a form", section.ID, section.Resource.Module)
			}

			load, available, err := generator.buildReferencedResourceLoad(c, *section.Resource, string(role), nil)
			if err != nil {
				return fmt.Errorf("form section %q resource: %w", section.ID, err)
			}
			if !available {
				continue
			}
			section.Load = &load
		}

		if err := generator.resolveFieldMatrixSource(c, &section, role); err != nil {
			return err
		}
		sections = append(sections, section)
	}
	render.Form.Sections = sections
	return nil
}

func (generator *Generator) resolveFieldMatrixSource(c *gin.Context, section *renderer.FormSection, role actions.Role) error {
	if section == nil || section.Matrix == nil || section.Matrix.Table == nil || section.Matrix.Table.Source == nil {
		return nil
	}
	source := section.Matrix.Table.Source
	if source.List.Action != string(actions.ModuleActionNameList) {
		return fmt.Errorf("form section %q matrix source list action must be list", section.ID)
	}
	if source.Update.Action != string(actions.ModuleActionNameUpdate) {
		return fmt.Errorf("form section %q matrix source update action must be update", section.ID)
	}

	listLoad, listModule, listAction, available, err := generator.resolveFieldMatrixAction(c, source.List, role)
	if err != nil {
		return fmt.Errorf("form section %q matrix source list: %w", section.ID, err)
	}
	if !available {
		return fmt.Errorf("form section %q matrix source list is unavailable", section.ID)
	}
	if listAction.Action() != actions.ModuleActionNameList {
		return fmt.Errorf("form section %q matrix source list must reference list", section.ID)
	}

	updateLoad, updateModule, updateAction, available, err := generator.resolveFieldMatrixAction(c, source.Update, role)
	if err != nil {
		return fmt.Errorf("form section %q matrix source update: %w", section.ID, err)
	}
	if !available {
		return fmt.Errorf("form section %q matrix source update is unavailable", section.ID)
	}
	update, ok := updateAction.(actions.UpdateModuleAction)
	if !ok {
		return fmt.Errorf("form section %q matrix source update must reference update", section.ID)
	}
	if listModule.Name != updateModule.Name {
		return fmt.Errorf("form section %q matrix source list and update must reference one module", section.ID)
	}
	if listModule.GetField(source.IDField) == nil {
		return fmt.Errorf("form section %q matrix source id field %q is unknown", section.ID, source.IDField)
	}
	if !containsColumn(update.By, listModule.GetField(source.IDField).Column) {
		return fmt.Errorf("form section %q matrix source id field %q is not an update selector", section.ID, source.IDField)
	}
	if listModule.GetField(source.KeyField) == nil {
		return fmt.Errorf("form section %q matrix source key field %q is unknown", section.ID, source.KeyField)
	}
	for _, row := range section.Matrix.Table.Rows {
		for _, cell := range row.Cells {
			if cell.Field == "" {
				continue
			}
			field := listModule.GetField(cell.Field)
			if field == nil {
				return fmt.Errorf("form section %q matrix source field %q is unknown", section.ID, cell.Field)
			}
			if field.Type != fields.ModuleFieldTypeBool || !containsColumn(update.Columns, field.Column) {
				return fmt.Errorf("form section %q matrix source field %q must be an editable bool", section.ID, cell.Field)
			}
			if cell.AvailableField != "" && listModule.GetField(cell.AvailableField) == nil {
				return fmt.Errorf("form section %q matrix source availability field %q is unknown", section.ID, cell.AvailableField)
			}
		}
	}
	source.Load = &renderer.FieldMatrixDataSourceLoad{List: listLoad, Update: updateLoad}
	return nil
}

func (generator *Generator) resolveFieldMatrixAction(c *gin.Context, resource renderer.ActionResource, role actions.Role) (renderer.ResourceLoad, *BaseModule, actions.ModuleAction, bool, error) {
	targetModule, ok := generator.moduleByName(resource.Module)
	if !ok {
		return renderer.ResourceLoad{}, nil, nil, false, fmt.Errorf("references unknown module %q", resource.Module)
	}
	action, ok := findModuleAction(targetModule, resource.Action)
	if !ok {
		return renderer.ResourceLoad{}, nil, nil, false, fmt.Errorf("references unknown action %q", resource.Action)
	}
	if !hasPermission(action, string(role)) {
		return renderer.ResourceLoad{}, targetModule, action, false, nil
	}
	// A field matrix supplies the selector from source.id_field and exactly one
	// editable boolean cell at interaction time. It cannot use the static body
	// bindings required by a general widget update resource.
	if action.Action() == actions.ModuleActionNameUpdate {
		if _, err := targetModule.RenderFor(c); err != nil {
			return renderer.ResourceLoad{}, targetModule, action, false, err
		}
		contract, ok := resolveStandardActionContract(targetModule, action)
		if !ok {
			return renderer.ResourceLoad{}, targetModule, action, false, fmt.Errorf("update action has no standard request")
		}
		return renderer.ResourceLoad{Request: contract.Request}, targetModule, action, true, nil
	}
	load, available, err := generator.buildResourceLoad(c, targetModule, action, nil, nil)
	if err != nil || !available {
		return renderer.ResourceLoad{}, targetModule, action, available, err
	}
	return load, targetModule, action, true, nil
}

func (generator *Generator) buildWorkspaceCommandResourceLoad(c *gin.Context, module *BaseModule, action actions.ModuleAction, bindings []renderer.RequestBinding, selection *widgetSelectionScope, input *workspaceCommandInputScope) (renderer.ResourceLoad, bool, error) {
	available, err := generator.validateRequestBindingAvailability(c, module, action, bindings, selection, input)
	if err != nil || !available {
		return renderer.ResourceLoad{}, available, err
	}
	contract, ok := resolveStandardActionContract(module, action)
	if !ok {
		return renderer.ResourceLoad{}, false, fmt.Errorf("workspace command resource %q action %q has no request", module.Name, action.Action())
	}
	return renderer.ResourceLoad{
		Request:  contract.Request,
		Bindings: append([]renderer.RequestBinding(nil), bindings...),
	}, true, nil
}

func (generator *Generator) moduleByName(name string) (*BaseModule, bool) {
	for _, candidate := range generator.Modules {
		if candidate.Name == name {
			return candidate, true
		}
	}
	return nil, false
}

func actionWidget(action actions.ModuleAction) *actions.WidgetConfig {
	switch a := action.(type) {
	case actions.ListModuleAction:
		return a.Widget
	case *actions.ListModuleAction:
		return a.Widget
	case actions.ViewModuleAction:
		return a.Widget
	case *actions.ViewModuleAction:
		return a.Widget
	case actions.AddModuleAction:
		return a.Widget
	case *actions.AddModuleAction:
		return a.Widget
	case actions.UpdateModuleAction:
		return a.Widget
	case *actions.UpdateModuleAction:
		return a.Widget
	case actions.DeleteModuleAction:
		return a.Widget
	case *actions.DeleteModuleAction:
		return a.Widget
	case actions.DefrecModuleAction:
		return a.Widget
	case *actions.DefrecModuleAction:
		return a.Widget
	default:
		return nil
	}
}

func (generator *Generator) buildListRoute(module *BaseModule, render renderer.Universal, action actions.ListModuleAction, role string) RouteConfig {
	route := RouteConfig{
		Title:     action.Label,
		MenuTitle: action.Label,
		Renderer:  render.ListIdentity(),
		PageType:  render.ListRoutePageType(),
		Query:     standardActionRouteQuery(module, action),
	}

	route.Children = generator.buildRouteChildren(module, render, role)

	return route
}

func (generator *Generator) buildViewRoute(module *BaseModule, render renderer.Universal, action actions.ViewModuleAction) RouteConfig {
	pageType := viewActionPageType(action)

	return RouteConfig{
		Title:    action.Label,
		Renderer: viewRouteIdentity(render, pageType),
		PageType: viewRoutePageType(render, pageType),
		Query:    standardActionRouteQuery(module, action),
	}
}

func viewActionPageType(action actions.ViewModuleAction) renderer.PageType {
	if action.PageType != "" {
		return action.PageType
	}
	return renderer.PageTypeRecord
}

func viewActionPageTypeForContext(action actions.ViewModuleAction, c *gin.Context) renderer.PageType {
	if action.PageTypeFunc != nil {
		if pageType := action.PageTypeFunc(c); pageType != "" {
			return pageType
		}
	}
	return viewActionPageType(action)
}

func viewRouteIdentity(render renderer.Universal, pageType renderer.PageType) *renderer.Identity {
	switch pageType {
	case renderer.PageTypeForm:
		return render.FormIdentity()
	case renderer.PageTypeRecord:
		return render.RecordIdentity()
	default:
		return nil
	}
}

func viewRoutePageType(render renderer.Universal, pageType renderer.PageType) renderer.PageType {
	switch pageType {
	case renderer.PageTypeForm:
		return render.FormRoutePageType()
	case renderer.PageTypeRecord:
		return render.RecordRoutePageType()
	default:
		return ""
	}
}

func (generator *Generator) buildAddRoute(module *BaseModule, render renderer.Universal, action actions.AddModuleAction) RouteConfig {
	return RouteConfig{
		Title:    action.Label,
		Renderer: render.FormIdentity(),
		PageType: render.FormRoutePageType(),
		Query:    standardActionRouteQuery(module, action),
	}
}

func (generator *Generator) buildDefrecRoute(module *BaseModule, render renderer.Universal, action actions.DefrecModuleAction) RouteConfig {
	return RouteConfig{
		Title:    action.Label,
		Renderer: render.FormIdentity(),
		PageType: render.FormRoutePageType(),
		Query:    standardActionRouteQuery(module, action),
	}
}

func apiQueryURL(path string) string {
	if path == "/api" || strings.HasPrefix(path, "/api/") {
		return path
	}
	return "/api" + path
}

// buildRouteChildren формирует children маршруты (view, edit, add)
func (generator *Generator) buildRouteChildren(module *BaseModule, render renderer.Universal, role string) map[string]RouteConfig {
	children := make(map[string]RouteConfig)

	for _, action := range module.Actions {
		switch a := action.(type) {
		case actions.ViewModuleAction:
			if child, ok := generator.buildViewChild(module, render, a, role); ok {
				children[":id"] = child
			}
		case *actions.ViewModuleAction:
			if child, ok := generator.buildViewChild(module, render, *a, role); ok {
				children[":id"] = child
			}
		case actions.UpdateModuleAction:
			if child, ok := generator.buildUpdateChild(module, render, a, role); ok {
				children[":id/edit"] = child
			}
		case *actions.UpdateModuleAction:
			if child, ok := generator.buildUpdateChild(module, render, *a, role); ok {
				children[":id/edit"] = child
			}
		case actions.AddModuleAction:
			if child, ok := generator.buildAddChild(module, render, a, role); ok {
				children["add"] = child
			}
		case *actions.AddModuleAction:
			if child, ok := generator.buildAddChild(module, render, *a, role); ok {
				children["add"] = child
			}
		}
	}

	return children
}

func (generator *Generator) buildViewChild(module *BaseModule, render renderer.Universal, a actions.ViewModuleAction, role string) (RouteConfig, bool) {
	if !hasPermission(a, role) {
		return RouteConfig{}, false
	}

	return RouteConfig{
		Title:    a.Label,
		Renderer: viewRouteIdentity(render, viewActionPageType(a)),
		PageType: viewRoutePageType(render, viewActionPageType(a)),
		Query:    standardRecordActionRouteQuery(module, a, a.By),
	}, true
}

func (generator *Generator) buildUpdateChild(module *BaseModule, render renderer.Universal, a actions.UpdateModuleAction, role string) (RouteConfig, bool) {
	if !hasPermission(a, role) {
		return RouteConfig{}, false
	}

	return RouteConfig{
		Title:    a.Label,
		Renderer: render.FormIdentity(),
		PageType: render.FormRoutePageType(),
		Query:    standardRecordActionRouteQuery(module, a, a.By),
	}, true
}

// standardRecordActionRouteQuery binds the generated :id child route to the
// standard selector placeholders. The first allowed selector is used, except
// that a module primary key wins when it is explicitly allowed.
func standardRecordActionRouteQuery(module *BaseModule, action actions.ModuleAction, by []pg.Column) *RouteQuery {
	query := standardActionRouteQuery(module, action)
	if query == nil || len(by) == 0 {
		return query
	}
	byKey := by[0].Name()
	for _, column := range by {
		if column.Name() == module.PrimaryKey.Name() {
			byKey = column.Name()
			break
		}
	}
	params := make(map[string]interface{}, len(query.Params)+2)
	for key, value := range query.Params {
		params[key] = value
	}
	params["bykey"] = byKey
	params["value"] = "{id}"
	query.Params = params
	return query
}

func (generator *Generator) buildAddChild(module *BaseModule, render renderer.Universal, a actions.AddModuleAction, role string) (RouteConfig, bool) {
	if !hasPermission(a, role) {
		return RouteConfig{}, false
	}

	return RouteConfig{
		Title:    a.Label,
		Renderer: render.FormIdentity(),
		PageType: render.FormRoutePageType(),
		Query:    standardActionRouteQuery(module, a),
	}, true
}
