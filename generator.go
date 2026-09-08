package module

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/icontext"
	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/darkrain/request-generator/response"
	"github.com/darkrain/request-generator/utils"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

const (
	GeneratorErrorAdd    string = "Cannot create record"
	GeneratorErrorUpdate string = "Cannot update record"
	GeneratorErrorDelete string = "Cannot delete record"
)

type Generator struct {
	db                   func(module *BaseModule) db.DBExecutor
	group                gin.RouterGroup
	Modules              []*BaseModule
	Features             []Features
	AuthMiddleware       func(module actions.ModuleAction) gin.HandlerFunc
	PermissionMiddleware func(action actions.ModuleAction, permissions []actions.Role) gin.HandlerFunc
	Locales              []locale.Lang
	DefaultLocale        locale.Lang
	translations         map[locale.Lang]map[string]string
	EnableOpenAPI        bool
	GroupTitles          map[string]string
	IconMap              map[string]string
	Realtime             RealtimeConfig
	realtimeHub          *realtimeHub
	// AccessGate lets the application close a destination for the current
	// actor without removing it from the configuration.
	AccessGate AccessGate
}

func NewGenerator(
	db func(module *BaseModule) db.DBExecutor,
	group gin.RouterGroup,
	modules []*BaseModule,
	permissionMiddleware func(action actions.ModuleAction, permissions []actions.Role) gin.HandlerFunc,
	authMiddleware func(action actions.ModuleAction) gin.HandlerFunc,
) *Generator {
	return &Generator{
		db:                   db,
		group:                group,
		Modules:              modules,
		Features:             []Features{},
		PermissionMiddleware: permissionMiddleware,
		AuthMiddleware:       authMiddleware,
		Locales:              []locale.Lang{locale.EN},
		DefaultLocale:        locale.EN,
	}
}

func (generator *Generator) getLang(c *gin.Context) locale.Lang {
	if lang := c.Query("lang"); lang != "" {
		l := locale.Lang(lang)
		for _, s := range generator.Locales {
			if s == l {
				return l
			}
		}
	}
	return locale.ParseAcceptLanguage(c.GetHeader("Accept-Language"), generator.Locales, generator.DefaultLocale)
}

func (generator *Generator) FeaturesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)

		localized := make([]Features, len(generator.Features))
		for i, f := range generator.Features {
			lf := Features{
				ModuleName: generator.Translate(lang, f.ModuleName),
				Actions:    make(map[string]FeaturesActions, len(f.Actions)),
			}
			for k, a := range f.Actions {
				lf.Actions[k] = FeaturesActions{
					Label: generator.Translate(lang, a.Label),
					Url:   a.Url,
					Type:  a.Type,
					Roles: a.Roles,
				}
			}
			localized[i] = lf
		}

		resp := FeaturesResponse{
			Modules: localized,
		}

		response.Response(l, c, resp)
	}
}

func (generator *Generator) Run() {

	featuresGroup := generator.group.Group("/api")
	featuresGroup.GET("/features", generator.FeaturesMiddleware())
	generator.initRealtime()
	if err := generator.validateRealtimeEvents(); err != nil {
		panic(fmt.Sprintf("invalid realtime event config: %v", err))
	}
	if err := generator.validateGlobalWidgets(); err != nil {
		panic(fmt.Sprintf("invalid global widget config: %v", err))
	}

	for _, module := range generator.Modules {
		if err := validateAtomicAddActions(module); err != nil {
			panic(fmt.Sprintf("invalid atomic add config in module %s: %v", module.Name, err))
		}
		if err := validateAtomicUpdateActions(module); err != nil {
			panic(fmt.Sprintf("invalid atomic update config in module %s: %v", module.Name, err))
		}
		if err := module.Render.Validate(); err != nil {
			if module.RenderFunc != nil {
				panic(fmt.Sprintf("invalid base renderer config in module %s: %v", module.Name, err))
			}
			panic(fmt.Sprintf("invalid renderer config in module %s: %v", module.Name, err))
		}
		if err := module.validateFieldMatrices(module.Render); err != nil {
			panic(fmt.Sprintf("invalid field matrix config in module %s: %v", module.Name, err))
		}
		if err := validateModuleFieldMedia(module); err != nil {
			panic(fmt.Sprintf("invalid field media config in module %s: %v", module.Name, err))
		}
		if err := validateModuleFieldPresentations(module); err != nil {
			panic(fmt.Sprintf("invalid field presentation config in module %s: %v", module.Name, err))
		}
		if err := validateModuleFieldArrayStorage(module); err != nil {
			panic(fmt.Sprintf("invalid array storage config in module %s: %v", module.Name, err))
		}
		if err := validateModuleFieldOptionsSources(module); err != nil {
			panic(fmt.Sprintf("invalid field options source in module %s: %v", module.Name, err))
		}
		if err := validateModuleVirtualFilterOptionsSources(module); err != nil {
			panic(fmt.Sprintf("invalid virtual filter options source in module %s: %v", module.Name, err))
		}
		if err := generator.validateCollectionRelations(module); err != nil {
			panic(fmt.Sprintf("invalid collection config in module %s: %v", module.Name, err))
		}

		featuresModule := Features{
			ModuleName:       module.Label,
			ModuleNameLabels: module.Labels,
			Actions:          make(map[string]FeaturesActions),
		}

		for _, action := range module.Actions {
			switch action.Action() {
			case actions.ModuleActionNameList:

				listAction, _ := action.(actions.ListModuleAction)
				featuresModule.Actions["list"] = FeaturesActions{
					Label:  listAction.Label,
					Labels: listAction.Labels,
					Url:    module.Path + "/" + module.Name,
					Type:   "GET",
					Roles:  listAction.Permission,
				}
				listGrpup := generator.group.Group(module.Path)
				if listAction.Auth {
					if generator.AuthMiddleware == nil {
						panic(fmt.Sprintf("auth middleware not implemented in module: %s", module.Name))
					}
					listGrpup.Use(generator.AuthMiddleware(listAction))
				}
				if len(listAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					listGrpup.Use(generator.PermissionMiddleware(listAction, listAction.Permission))
				}

				listGrpup.GET(module.Name, generator.actionList(module, listAction))
			case actions.ModuleActionNameAdd:
				addAction, _ := action.(actions.AddModuleAction)
				featuresModule.Actions["add"] = FeaturesActions{
					Label:  addAction.Label,
					Labels: addAction.Labels,
					Url:    module.Path + "/" + module.Name,
					Type:   "PUT",
					Roles:  addAction.Permission,
				}
				featuresModule.Actions["defrec"] = FeaturesActions{
					Label:  addAction.Label,
					Labels: addAction.Labels,
					Url:    fmt.Sprintf("%s/%s/defrec/", module.Path, module.Name),
					Type:   "GET",
					Roles:  addAction.Permission,
				}
				addGrpup := generator.group.Group(module.Path)
				if addAction.Auth {
					if generator.AuthMiddleware == nil {
						panic(fmt.Sprintf("auth middleware not implemented in module: %s", module.Name))
					}
					addGrpup.Use(generator.AuthMiddleware(addAction))
				}
				if len(addAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					addGrpup.Use(generator.PermissionMiddleware(addAction, addAction.Permission))
				}
				addGrpup.PUT(module.Name, generator.actionAdd(module, addAction))

				defrecGroup := generator.group.Group(fmt.Sprintf("%s/%s/defrec", module.Path, module.Name))
				if addAction.Auth && generator.AuthMiddleware != nil {
					defrecGroup.Use(generator.AuthMiddleware(addAction))
				}
				if len(addAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					defrecGroup.Use(generator.PermissionMiddleware(addAction, addAction.Permission))
				}
				defrecGroup.GET("/", generator.actionDefrec(module))

			case actions.ModuleActionNameView:
				viewAction, _ := action.(actions.ViewModuleAction)
				featuresModule.Actions["view"] = FeaturesActions{
					Label:  viewAction.Label,
					Labels: viewAction.Labels,
					Url:    module.Path + "/" + module.Name,
					Type:   "GET",
					Roles:  viewAction.Permission,
				}
				viewGrout := generator.group.Group(module.Path)
				if viewAction.Auth {
					if generator.AuthMiddleware == nil {
						panic(fmt.Sprintf("auth middleware not implemented in module: %s", module.Name))
					}
					viewGrout.Use(generator.AuthMiddleware(viewAction))
				}
				if len(viewAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					viewGrout.Use(generator.PermissionMiddleware(viewAction, viewAction.Permission))
				}

				viewGrout.GET(fmt.Sprintf("%s/view/:bykey/:value", module.Name), generator.actionView(module, viewAction))
			case actions.ModuleActionNameUpdate:
				updateAction, _ := action.(actions.UpdateModuleAction)
				featuresModule.Actions["update"] = FeaturesActions{
					Label:  updateAction.Label,
					Labels: updateAction.Labels,
					Url:    module.Path + "/" + module.Name,
					Type:   "POST",
					Roles:  updateAction.Permission,
				}
				updateGroup := generator.group.Group(module.Path)
				if updateAction.Auth {
					if generator.AuthMiddleware == nil {
						panic(fmt.Sprintf("auth middleware not implemented in module: %s", module.Name))
					}
					updateGroup.Use(generator.AuthMiddleware(updateAction))
				}
				if len(updateAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					updateGroup.Use(generator.PermissionMiddleware(updateAction, updateAction.Permission))
				}

				updateGroup.POST(fmt.Sprintf("%s/:bykey/:value", module.Name), generator.actionUpdate(module, updateAction))
			case actions.ModuleActionNameDelete:
				deleteAction, _ := action.(actions.DeleteModuleAction)
				featuresModule.Actions["delete"] = FeaturesActions{
					Label:  deleteAction.Label,
					Labels: deleteAction.Labels,
					Url:    module.Path + "/" + module.Name,
					Type:   "DELETE",
					Roles:  deleteAction.Permission,
				}
				deleteGroup := generator.group.Group(module.Path)
				if deleteAction.Auth {
					if generator.AuthMiddleware == nil {
						panic(fmt.Sprintf("auth middleware not implemented in module: %s", module.Name))
					}
					deleteGroup.Use(generator.AuthMiddleware(deleteAction))
				}
				if len(deleteAction.Permission) > 0 {
					if generator.PermissionMiddleware == nil {
						panic(fmt.Sprintf("permission middleware not implemented in module: %s", module.Name))
					}
					deleteGroup.Use(generator.PermissionMiddleware(deleteAction, deleteAction.Permission))
				}
				deleteGroup.DELETE(fmt.Sprintf("%s/delete/:bykey/:value", module.Name), generator.actionDelete(module, deleteAction))
			}
		}

		generator.Features = append(generator.Features, featuresModule)
	}

	// Locale endpoints
	featuresGroup.GET("/lang", generator.handleLangList())
	featuresGroup.GET("/lang/:key", generator.handleLangTranslations())

	// Config endpoint (protected by AuthMiddleware)
	if generator.AuthMiddleware != nil {
		configGroup := featuresGroup.Group("")
		// Dummy action for auth middleware (config doesn't map to a specific module action)
		dummyAction := actions.ListModuleAction{
			Permission: []actions.Role{},
			Auth:       true,
		}
		configGroup.Use(generator.AuthMiddleware(dummyAction))
		configGroup.GET("/config", generator.actionConfigEndpoint())
	}

	// Build and serve OpenAPI 3.0 spec (only when enabled)
	if generator.EnableOpenAPI {
		spec := generator.buildOpenAPISpec("Muta Alim API", "1.0.0")
		specJSON, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			panic(fmt.Sprintf("failed to marshal OpenAPI spec: %v", err))
		}

		featuresGroup.GET("/openapi.json", func(c *gin.Context) {
			c.Data(http.StatusOK, "application/json; charset=utf-8", specJSON)
		})
	}
}

func validateAtomicAddActions(module *BaseModule) error {
	for _, moduleAction := range module.Actions {
		var action actions.AddModuleAction
		switch value := moduleAction.(type) {
		case actions.AddModuleAction:
			action = value
		case *actions.AddModuleAction:
			if value == nil {
				continue
			}
			action = *value
		default:
			continue
		}
		switch action.Mode {
		case "", actions.AddModeStandard:
			continue
		case actions.AddModeAtomic:
			if action.Atomic == nil || action.Atomic.Operation == nil {
				return fmt.Errorf("atomic add action requires an operation")
			}
			if err := validateAtomicAddConfig(module, action); err != nil {
				return err
			}
			if action.BeforeAction != nil || action.AfterAction != nil {
				return fmt.Errorf("atomic add action cannot define before or after hooks")
			}
			if len(module.RoleBeforeHook) > 0 || len(module.RoleAfterHook) > 0 {
				return fmt.Errorf("atomic add action cannot be used by a module with role hooks")
			}
		default:
			return fmt.Errorf("unsupported add mode %q", action.Mode)
		}
	}
	return nil
}

func validateAtomicUpdateActions(module *BaseModule) error {
	for _, moduleAction := range module.Actions {
		var action actions.UpdateModuleAction
		switch value := moduleAction.(type) {
		case actions.UpdateModuleAction:
			action = value
		case *actions.UpdateModuleAction:
			if value == nil {
				continue
			}
			action = *value
		default:
			continue
		}
		switch action.Mode {
		case "", actions.UpdateModeStandard:
			continue
		case actions.UpdateModeAtomic:
			if action.Atomic == nil || action.Atomic.Operation == nil {
				return fmt.Errorf("atomic update action requires an operation")
			}
			if err := validateAtomicUpdateConfig(module, action); err != nil {
				return err
			}
			if action.BeforeAction != nil || action.AfterAction != nil {
				return fmt.Errorf("atomic update action cannot define before or after hooks")
			}
			if len(module.RoleBeforeHook) > 0 || len(module.RoleAfterHook) > 0 {
				return fmt.Errorf("atomic update action cannot be used by a module with role hooks")
			}
			if _, err := atomicPrimaryKeyKind(module); err != nil {
				return err
			}
			if len(action.By) == 0 {
				return fmt.Errorf("atomic update action requires selector fields")
			}
			for _, by := range action.By {
				field := module.GetFieldByColumn(by)
				if field == nil {
					return fmt.Errorf("atomic update selector field %q is not declared in module fields", by.Name())
				}
				switch field.Type {
				case fields.ModuleFieldTypeString, fields.ModuleFieldTypeInt, fields.ModuleFieldTypeFloat:
				default:
					return fmt.Errorf("atomic update selector field %q has unsupported type %q", by.Name(), field.Type)
				}
			}
		default:
			return fmt.Errorf("unsupported update mode %q", action.Mode)
		}
	}
	return nil
}

func validateModuleFieldMedia(module *BaseModule) error {
	for _, field := range module.Fields {
		if err := field.Media.Validate(); err != nil {
			return fmt.Errorf("field %q: %w", field.ColumnName(), err)
		}
	}
	return nil
}

func validateModuleFieldPresentations(module *BaseModule) error {
	for _, field := range module.Fields {
		if err := field.Presentation.Validate(); err != nil {
			return fmt.Errorf("field %q: %w", field.ColumnName(), err)
		}
	}
	return nil
}

func validateModuleFieldArrayStorage(module *BaseModule) error {
	for _, field := range module.Fields {
		if field.ArrayStorage == "" {
			continue
		}
		if field.Type != fields.ModuleFieldTypeArray {
			return fmt.Errorf("field %q configures array storage but is not an array", field.ColumnName())
		}
		if err := field.ArrayStorage.Validate(); err != nil {
			return fmt.Errorf("field %q: %w", field.ColumnName(), err)
		}
	}
	for _, moduleAction := range module.Actions {
		var filters []pg.Column
		switch action := moduleAction.(type) {
		case actions.ListModuleAction:
			filters = action.Filter
		case *actions.ListModuleAction:
			if action != nil {
				filters = action.Filter
			}
		default:
			continue
		}
		for _, column := range filters {
			field := module.GetFieldByColumn(column)
			if field != nil && field.ArrayStorage.Normalize() == fields.ModuleFieldArrayStorageJSON {
				return fmt.Errorf("field %q uses JSON array storage and cannot use the PostgreSQL array filter", field.ColumnName())
			}
		}
	}
	return nil
}

func validateModuleFieldOptionsSources(module *BaseModule) error {
	for _, field := range module.Fields {
		if err := field.OptionsSource.Validate(); err != nil {
			return fmt.Errorf("field %q: %w", field.ColumnName(), err)
		}
	}
	return nil
}

func validateModuleVirtualFilterOptionsSources(module *BaseModule) error {
	for _, moduleAction := range module.Actions {
		listAction, ok := moduleAction.(actions.ListModuleAction)
		if !ok {
			continue
		}
		for _, filter := range listAction.VirtualFilters {
			if err := filter.OptionsSource.Validate(); err != nil {
				return fmt.Errorf("filter %q: %w", filter.FieldName, err)
			}
		}
	}
	return nil
}

func (generator *Generator) fieldOptions(c *gin.Context, field fields.ModuleField, role string, lang locale.Lang) []fields.ModuleFieldOptions {
	options := make([]fields.ModuleFieldOptions, 0, len(field.Options))
	options = append(options, field.Options...)
	if field.OptionsFunc != nil {
		options = append(options, field.OptionsFunc(c)...)
	}
	for _, roleOptions := range field.RoleOptions {
		if roleOptions.Role == role || roleOptions.Role == string(actions.RoleAll) {
			options = append(options, roleOptions.Options...)
			break
		}
	}
	for i := range options {
		options[i].Label = generator.Translate(lang, options[i].Label)
		options[i].Description = generator.Translate(lang, options[i].Description)
	}
	return options
}

func validateListFilterAvailability(page *renderer.ListPage, filters map[string]fields.ModuleFilterField) error {
	if page == nil || page.Filters == nil {
		return nil
	}
	for _, placement := range [][]string{
		page.Filters.Primary,
		page.Filters.Secondary,
		page.Filters.More,
		page.Filters.Nested,
	} {
		for _, field := range placement {
			if _, ok := filters[field]; !ok {
				return fmt.Errorf("renderer filter %q is not available for the current request", field)
			}
		}
	}
	for _, group := range page.Filters.Groups {
		if err := validateFilterGroupAvailability(group, filters); err != nil {
			return err
		}
	}
	return nil
}

func validateFilterGroupAvailability(group renderer.FilterGroup, filters map[string]fields.ModuleFilterField) error {
	for _, field := range group.Fields {
		if _, ok := filters[field]; !ok {
			return fmt.Errorf("renderer filter group %q field %q is not available for the current request", group.ID, field)
		}
	}
	for _, section := range group.Sections {
		for _, field := range section.Fields {
			if _, ok := filters[field]; !ok {
				return fmt.Errorf("renderer filter group %q section %q field %q is not available for the current request", group.ID, section.ID, field)
			}
		}
	}
	for _, item := range group.Items {
		if item.Field != "" {
			if _, ok := filters[item.Field]; !ok {
				return fmt.Errorf("renderer filter group %q field %q is not available for the current request", group.ID, item.Field)
			}
		}
		if item.Group != nil {
			if err := validateFilterGroupAvailability(*item.Group, filters); err != nil {
				return err
			}
		}
	}
	return nil
}

func (generator *Generator) validateCollectionRelations(owner *BaseModule) error {
	if owner.Render.Form == nil {
		return nil
	}
	moduleByName := make(map[string]*BaseModule, len(generator.Modules))
	for _, candidate := range generator.Modules {
		moduleByName[candidate.Name] = candidate
	}
	for _, section := range owner.Render.Form.Sections {
		if section.Collection == nil {
			continue
		}
		collectionModule, ok := moduleByName[section.Collection.Module]
		if !ok {
			return fmt.Errorf("collection section %q references unknown module %q", section.ID, section.Collection.Module)
		}
		if section.Collection.Relation == "" {
			continue
		}
		relation, ok := findModuleRelation(collectionModule, section.Collection.Relation)
		if !ok {
			return fmt.Errorf("collection module %q must declare relation %q", section.Collection.Module, section.Collection.Relation)
		}
		if relation.TargetModule != owner.Name {
			return fmt.Errorf("collection module %q relation %q must target module %q", section.Collection.Module, section.Collection.Relation, owner.Name)
		}
		if relation.ScopeCheck == nil {
			return fmt.Errorf("collection module %q relation %q must declare ScopeCheck", section.Collection.Module, section.Collection.Relation)
		}
	}
	return nil
}

func (generator *Generator) actionList(module *BaseModule, action actions.ListModuleAction) func(c *gin.Context) {
	return func(c *gin.Context) {
		defer action.AfterRequest(c)

		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		role := actions.GetRoleFromContext(c)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)

		if hook := actions.ResolveRoleHook(module.RoleBeforeHook, role); hook != nil {
			if err := hook(c); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
				return
			}
		}
		defer func() {
			if hook := actions.ResolveRoleAfterHook(module.RoleAfterHook, role); hook != nil {
				hook(c)
			}
		}()

		err := action.BeforeRequest(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}

		page := int64QueryParam(c, "page", 0)
		defaultSize := int64(3000)
		if action.Size > 0 {
			defaultSize = action.Size
		}
		size := int64QueryParam(c, "size", defaultSize)
		if action.Maxsize > 0 && size > action.Maxsize {
			size = action.Maxsize
		}
		isCSV := int64QueryParam(c, "csv", 0)
		filter := generator.effectiveListFilters(c, module, action, lang)
		filters := generator.normalizeFilters(c.QueryMap("filter"), filter, lang)
		searchText := c.Query("search")
		addFilters := c.Query("addFilters")
		addHeads := c.Query("addHeads")

		columns := action.GetColumns(c)

		realFields := make([]fields.ModuleField, 0, 10)
		for _, realField := range module.Fields {
			if containsColumn(columns, realField.Column) {
				realFields = append(realFields, realField)
			}
		}
		realFields = fields.ResolveProjections(c, realFields)

		var where pg.BoolExpression
		if whereFn := actions.ResolveRoleWhere(module.RoleWhere, role); whereFn != nil {
			where = whereFn(c)
		}
		if action.Where != nil {
			actionWhere := action.Where(c)
			if where != nil {
				where = pg.AND(where, actionWhere)
			} else {
				where = actionWhere
			}
		}
		scope, status, err := generator.resolveRelationScope(c, module)
		if err != nil {
			response.ErrorResponse(l, c, status, err.Error(), nil)
			return
		}
		where = appendRelationScopeWhere(module, where, scope)

		joins := action.Join
		if roleJoins := actions.ResolveRoleJoin(module.RoleJoin, role); roleJoins != nil {
			joins = append(roleJoins, joins...)
		}

		var activeSort *actions.SortOption
		if sortParam := c.Query("sort"); sortParam != "" && len(action.Sort) > 0 {
			parts := strings.SplitN(sortParam, ":", 2)
			colName := parts[0]
			dir := actions.SortASC
			if len(parts) == 2 && parts[1] == "desc" {
				dir = actions.SortDESC
			}
			for _, col := range action.Sort {
				if col.Name() == colName {
					activeSort = &actions.SortOption{Column: col, Direction: dir}
					break
				}
			}
		} else if action.SortDefaultFunc != nil {
			activeSort = action.SortDefaultFunc(c)
		} else if action.SortDefault != nil {
			dir := action.SortDefaultDirection
			if dir == "" {
				dir = actions.SortASC
			}
			activeSort = &actions.SortOption{Column: action.SortDefault, Direction: dir}
		}

		tc := generator.buildTranslationContext(module)

		results, count, err := generator.db(module).List(
			l,
			module.Table,
			module.PrimaryKey,
			realFields,
			filter,
			page,
			size,
			action.Search,
			searchText,
			filters,
			where,
			joins,
			activeSort,
			tc,
		)

		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		results = generator.localizeResultList(lang, realFields, results)

		var heads map[string]interface{}
		if addHeads == "true" {
			heads = make(map[string]interface{})

			for _, realField := range module.Fields {
				if containsColumn(columns, realField.Column) {
					heads[realField.ColumnName()] = map[string]interface{}{
						"title": generator.Translate(lang, realField.Title),
					}
				}
			}
		}

		if len(results) == 0 {
			results = make([]interface{}, 0, 10)
		}

		render, err := module.RenderFor(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := generator.resolveListSummaryResource(c, &render, role); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := generator.resolveFormSectionResources(c, &render, role); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		render = generator.localizeRenderer(lang, render)
		if err := validateListFilterAvailability(render.List, filter); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		responseFilters := filter
		if addFilters != "true" {
			responseFilters = nil
		}

		if len(heads) == 0 {
			heads = make(map[string]interface{})
		}

		var sortOptions []actions.SortResponseItem
		if len(action.Sort) > 0 {
			for _, col := range action.Sort {
				field := module.GetFieldByColumn(col)
				label := col.Name()
				if field != nil {
					label = generator.Translate(lang, field.Title)
				}
				sortOptions = append(sortOptions,
					actions.SortResponseItem{Value: col.Name() + ":asc", Text: label + " ↑"},
					actions.SortResponseItem{Value: col.Name() + ":desc", Text: label + " ↓"},
				)
			}
		}

		output := struct {
			Count            int64                               `json:"count"`
			Size             int64                               `json:"size"`
			Page             int64                               `json:"page"`
			Renderer         *renderer.Identity                  `json:"renderer,omitempty"`
			ListPage         *renderer.ListPage                  `json:"list_page,omitempty"`
			ResourceGridPage *renderer.ResourceGridPage          `json:"resource_grid_page,omitempty"`
			Rows             []interface{}                       `json:"rows"`
			Heads            map[string]interface{}              `json:"heads"`
			Filters          map[string]fields.ModuleFilterField `json:"filters,omitempty"`
			Sort             []actions.SortResponseItem          `json:"sort,omitempty"`
		}{
			Count:            count,
			Size:             size,
			Page:             page,
			Renderer:         render.ListIdentity(),
			ListPage:         render.List,
			ResourceGridPage: render.ResourceGrid,
			Rows:             results,
			Heads:            heads,
			Filters:          responseFilters,
			Sort:             sortOptions,
		}

		if isCSV == 0 {
			response.Response(l, c, output)
		} else {
			resultJsonString, err := json.Marshal(results)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, err.Error(), nil)
				return
			}

			var d []map[string]interface{}
			err = json.Unmarshal(resultJsonString, &d)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, err.Error(), nil)
				return
			}

			csvResults := make([][]string, 0, 10)
			keys := make([]string, 0, 10)
			for _, v := range d {
				for key := range v {
					keys = append(keys, key)
				}
				break
			}
			sort.Strings(keys)
			csvResults = append(csvResults, keys)

			for _, v := range d {
				values := make([]string, 0, 10)
				for _, key := range keys {
					valueString, err := json.Marshal(v[key])
					if err != nil {
						continue
					}

					values = append(values, string(valueString))
				}
				csvResults = append(csvResults, values)
			}

			b := new(bytes.Buffer)
			w := csv.NewWriter(b)
			w.Comma = '\t'
			err = w.WriteAll(csvResults)

			response.ResponseCSV(l, c, b.Bytes())
		}
	}
}

func (generator *Generator) actionAdd(module *BaseModule, action actions.AddModuleAction) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		role := actions.GetRoleFromContext(c)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)
		ctx = c.Request.Context()

		if action.Mode != actions.AddModeAtomic {
			if hook := actions.ResolveRoleHook(module.RoleBeforeHook, role); hook != nil {
				if err := hook(c); err != nil {
					response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
					return
				}
			}
			defer func() {
				if hook := actions.ResolveRoleAfterHook(module.RoleAfterHook, role); hook != nil {
					hook(c)
				}
			}()

			err := action.BeforeRequest(c)
			if err != nil {
				if !c.Writer.Written() {
					response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), []string{
						err.Error(),
					})
				}
				return
			}
			if c.Writer.Written() {
				return
			}
		}

		var input map[string]interface{}
		err := utils.ParseJson(c.Request, &input)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{
				"Parse Input Error",
			})
			return
		}
		scope, status, err := generator.resolveRelationScope(c, module)
		if err != nil {
			response.ErrorResponse(l, c, status, GeneratorErrorAdd, []string{err.Error()})
			return
		}
		if err := rejectScopedSourceField(input, scope); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
			return
		}
		if _, err := injectRelationScopeInput(c, input, nil, nil, module, scope); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
			return
		}

		// Apply DefaultFunc for fields absent from the request body.
		for _, field := range module.Fields {
			if field.DefaultFunc == nil {
				continue
			}
			colName := field.ColumnName()
			if _, exists := input[colName]; !exists {
				input[colName] = field.DefaultFunc(c)
			}
		}

		errs := generator.checkRequest(c, input, module, action, fields.ScenarioAdd, lang)
		if len(errs) > 0 {
			response.ErrorResponse(l, c, http.StatusBadRequest, validationResponseMessage(GeneratorErrorAdd, errs), errs)
			return
		}

		columns := action.GetColumns(c)

		realFields := make([]fields.ModuleField, 0, 10)
		for _, realField := range module.Fields {
			inColumns := containsColumn(columns, realField.Column)
			hasDefault := realField.DefaultFunc != nil && input[realField.ColumnName()] != nil
			if inColumns || hasDefault {
				realFields = append(realFields, realField)
			}
		}

		tc := generator.buildTranslationContext(module)

		mapInput := generator.mapRequestInput(c, input, module, columns)
		// Include DefaultFunc fields that are not in the action columns,
		// always using the server-side DefaultFunc value to prevent client spoofing.
		for _, realField := range realFields {
			if !containsColumn(columns, realField.Column) && realField.DefaultFunc != nil {
				mapInput[realField.ColumnName()] = realField.DefaultFunc(c)
			}
		}
		realFields, err = injectRelationScopeInput(c, input, mapInput, realFields, module, scope)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
			return
		}
		if action.Mode == actions.AddModeAtomic {
			atomicInput, err := atomicInputFromFields(realFields, mapInput)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
				return
			}
			rawDB := generator.db(module).RawDB()
			if rawDB == nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorAdd, []string{"atomic add requires a SQL database"})
				return
			}
			tx, err := rawDB.BeginTx(ctx, nil)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorAdd, []string{err.Error()})
				return
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()
			output, err := action.Atomic.Operation(ctx, db.NewAtomicExecutor(tx), atomicInput)
			if err != nil {
				if rejection, ok := actions.AsAtomicCommittedRejection(err); ok {
					if err := tx.Commit(); err != nil {
						response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorAdd, []string{err.Error()})
						return
					}
					committed = true
					response.ErrorResponse(l, c, rejection.StatusCode(), rejection.Error(), []string{rejection.Error()})
					return
				}
				response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), []string{err.Error()})
				return
			}
			if output.PrimaryKey == "" {
				output.PrimaryKey = module.PrimaryKey.Name()
			}
			if err := validateAtomicResult(action.Atomic, output); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
				return
			}
			atomicPublishes, err := atomicRealtimePublishes(action.Atomic, atomicInput, output)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
				return
			}
			if tc != nil {
				if err := db.InsertTranslations(tx, tc, output.Value, realFields, mapInput); err != nil {
					response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
					return
				}
			}
			if err := tx.Commit(); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{err.Error()})
				return
			}
			committed = true
			response.Response(l, c, output)
			generator.publishAtomicRealtime(c, module, actions.ModuleActionNameAdd, output, atomicPublishes)
			return
		}

		output, err := generator.db(module).Add(l, module.Table, module.PrimaryKey, realFields, mapInput, tc)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorAdd, []string{
				err.Error(),
			})
			return
		}

		response.Response(l, c, output)

		action.AfterRequest(c)
		generator.publishRealtime(c, module, actions.ModuleActionNameAdd, output)
	}
}

func (generator *Generator) actionDefrec(module *BaseModule) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)

		err := module.Defrec.BeforeRequest(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}

		output := make([]fields.ModuleField, 0, 10)

		role := string(actions.GetRoleFromContext(c))

		for _, field := range module.Fields {
			// Skip fields that have role restrictions and current role is not in the list
			if len(field.Roles) > 0 {
				roleAllowed := false
				for _, r := range field.Roles {
					if r == role {
						roleAllowed = true
						break
					}
				}
				if !roleAllowed {
					continue
				}
			}

			checkItems := make([]fields.CheckRules, 0, 10)
			optionItems := generator.fieldOptions(c, field, role, lang)

			if field.Check != nil {
				for _, check := range field.Check {
					checkItems = append(checkItems, check)
				}
			}
			if field.CheckFunc != nil {
				for _, check := range field.CheckFunc(c) {
					checkItems = append(checkItems, check)
				}
			}
			for _, rc := range field.RoleCheck {
				if rc.Role == role || rc.Role == string(actions.RoleAll) {
					checkItems = append(checkItems, rc.Rules...)
					break
				}
			}

			if field.TitleFunc != nil {
				if title := field.TitleFunc(c); title != "" {
					field.Title = title
				}
			}
			field.Title = generator.Translate(lang, field.Title)
			field.Options = optionItems
			field.Check = checkItems
			field.Presentation = generator.localizeFieldPresentation(lang, field.Presentation)
			field.Media = generator.localizeFieldMedia(lang, field.Media, nil)

			if field.RoleSection != nil {
				if s, ok := field.RoleSection[role]; ok {
					field.Section = s
				}
			}
			field.FormType = fieldFormTypeForRole(field, role)

			output = append(output, field)
		}

		render, err := module.RenderFor(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := generator.resolveFormSectionResources(c, &render, actions.GetRoleFromContext(c)); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		render = generator.localizeRenderer(lang, render)
		defrecResponse := response.NewDefrecResponse(output)
		defrecResponse.AttachRender(render)
		response.Response(l, c, defrecResponse)

		module.Defrec.AfterRequest(c)
	}
}

func (generator *Generator) actionView(module *BaseModule, action actions.ViewModuleAction) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		role := actions.GetRoleFromContext(c)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)

		if hook := actions.ResolveRoleHook(module.RoleBeforeHook, role); hook != nil {
			if err := hook(c); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
				return
			}
		}
		defer func() {
			if hook := actions.ResolveRoleAfterHook(module.RoleAfterHook, role); hook != nil {
				hook(c)
			}
		}()

		pageType := viewActionPageTypeForContext(action, c)

		err := action.BeforeRequest(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}

		whereKey := c.Param("bykey")

		allowedKeys := make([]interface{}, 0, len(action.By))
		for _, col := range action.By {
			allowedKeys = append(allowedKeys, col.Name())
		}
		err = validation.In(allowedKeys...).Error(fmt.Sprintf(`allowed keys %v`, allowedKeys)).Validate(whereKey)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorDelete, []string{
				err.Error(),
			})
			return
		}

		whereValue := c.Param("value")
		if len(whereValue) == 0 {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorDelete, []string{
				"value param not found",
			})
			return
		}

		columns := action.GetColumns(c)

		realFields := make([]fields.ModuleField, 0, 10)
		for _, realField := range module.Fields {
			if containsColumn(columns, realField.Column) {
				realFields = append(realFields, realField)
			}
		}
		realFields = fields.ResolveProjections(c, realFields)

		pkWhere := pg.RawBool(
			fmt.Sprintf(`%s."%s" = #val`, module.Table.Alias(), whereKey),
			pg.RawArgs{"#val": whereValue},
		)
		if module.Table.Alias() == "" {
			pkWhere = pg.RawBool(
				fmt.Sprintf(`"%s"."%s" = #val`, module.Table.TableName(), whereKey),
				pg.RawArgs{"#val": whereValue},
			)
		}

		var where pg.BoolExpression = pkWhere
		if whereFn := actions.ResolveRoleWhere(module.RoleWhere, role); whereFn != nil {
			if roleWhere := whereFn(c); roleWhere != nil {
				where = pg.AND(pkWhere, roleWhere)
			}
		}

		joins := action.Join
		if roleJoins := actions.ResolveRoleJoin(module.RoleJoin, role); roleJoins != nil {
			joins = append(roleJoins, joins...)
		}

		tc := generator.buildTranslationContext(module)

		result, err := generator.db(module).View(l, module.Table, module.PrimaryKey, realFields, where, joins, tc)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		result = generator.localizeResultValue(lang, realFields, result)

		// Build rich view response with field metadata
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			response.Response(l, c, result)
			action.AfterRequest(c)
			return
		}

		// Determine editable columns from UpdateAction
		var editableColumns []pg.Column
		if updateAction := findUpdateAction(module); updateAction != nil {
			editableColumns = updateAction.GetColumns(c)
		}

		roleStr := string(role)

		item := make(map[string]interface{}, len(realFields))
		for _, field := range realFields {
			fieldKey := field.ColumnName()
			if field.Translatable {
				fieldKey = field.Name()
			}
			value := resultMap[fieldKey]

			fieldItem := map[string]interface{}{
				"title":     generator.Translate(lang, field.Title),
				"type":      string(field.Type),
				"form_type": string(fieldFormTypeForRole(field, roleStr)),
				"value":     value,
				"edit":      containsColumn(editableColumns, field.Column),
			}

			if field.Presentation != nil {
				fieldItem["presentation"] = generator.localizeFieldPresentation(lang, field.Presentation)
			}
			if field.Media != nil {
				fieldItem["media"] = generator.localizeFieldMedia(lang, field.Media, value)
			}

			if field.OptionsSource != nil {
				fieldItem["options_source"] = field.OptionsSource
			}

			options := generator.fieldOptions(c, field, roleStr, lang)
			if len(options) > 0 {
				fieldItem["options"] = options
			}

			item[fieldKey] = fieldItem
		}

		render, err := module.RenderFor(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		if err := generator.resolveFormSectionResources(c, &render, role); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
			return
		}
		render = generator.localizeRenderer(lang, render)

		output := struct {
			Renderer   *renderer.Identity     `json:"renderer,omitempty"`
			RecordPage *renderer.RecordPage   `json:"record_page,omitempty"`
			FormPage   *renderer.FormPage     `json:"form_page,omitempty"`
			Item       map[string]interface{} `json:"item"`
		}{
			Renderer: viewRouteIdentity(render, pageType),
			Item:     item,
		}
		switch pageType {
		case renderer.PageTypeForm:
			output.FormPage = render.Form
		case renderer.PageTypeRecord:
			output.RecordPage = render.Record
		}

		response.Response(l, c, output)

		action.AfterRequest(c)
	}
}

func (generator *Generator) actionUpdate(module *BaseModule, action actions.UpdateModuleAction) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		role := actions.GetRoleFromContext(c)
		lang := generator.getLang(c)
		generator.setTranslationContext(c, lang)
		ctx = c.Request.Context()
		var err error

		if action.Mode != actions.UpdateModeAtomic {
			if hook := actions.ResolveRoleHook(module.RoleBeforeHook, role); hook != nil {
				if err := hook(c); err != nil {
					response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
					return
				}
			}
			defer func() {
				if hook := actions.ResolveRoleAfterHook(module.RoleAfterHook, role); hook != nil {
					hook(c)
				}
			}()

			err = action.BeforeRequest(c)
			if err != nil {
				if !c.Writer.Written() {
					response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), []string{err.Error()})
				}
				return
			}
		}

		whereKey := c.Param("bykey")
		allowedKeys := make([]interface{}, 0, len(action.By))
		for _, col := range action.By {
			allowedKeys = append(allowedKeys, col.Name())
		}
		err = validation.In(allowedKeys...).Error(fmt.Sprintf(`allowed keys %v`, allowedKeys)).Validate(whereKey)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{
				err.Error(),
			})
			return
		}

		whereValue := c.Param("value")
		if len(whereValue) == 0 {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{
				"value param not found",
			})
			return
		}
		var atomicSelector actions.AtomicSelector
		if action.Mode == actions.UpdateModeAtomic {
			atomicSelector, err = atomicUpdateSelectorFromRoute(c, module, action.By, whereKey, whereValue)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
		}

		var input map[string]interface{}
		err = utils.ParseJson(c.Request, &input)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, nil)
			return
		}
		scope, status, err := generator.resolveRelationScope(c, module)
		if err != nil {
			response.ErrorResponse(l, c, status, GeneratorErrorUpdate, []string{err.Error()})
			return
		}
		if err := rejectScopedSourceField(input, scope); err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
			return
		}

		errs := generator.checkRequest(c, input, module, action, fields.ScenarioUpdate, lang)
		if len(errs) > 0 {
			response.ErrorResponse(l, c, http.StatusBadRequest, validationResponseMessage(GeneratorErrorUpdate, errs), errs)
			return
		}

		columns := action.GetColumns(c)

		realFields := make([]fields.ModuleField, 0, 10)
		for _, realField := range module.Fields {
			if containsColumn(columns, realField.Column) {
				realFields = append(realFields, realField)
			}
		}

		tc := generator.buildTranslationContext(module)
		if tc != nil && action.Mode != actions.UpdateModeAtomic {
			tc.EntityID = whereValue
		}

		mapInput := generator.mapRequestInput(c, input, module, columns)

		// Build WHERE condition: primary key + optional role/action filters
		selectorValue := interface{}(whereValue)
		if action.Mode == actions.UpdateModeAtomic {
			selectorValue = atomicSelector.Value.Interface()
		}
		where := pg.BoolExpression(pg.RawBool(
			fmt.Sprintf(`"%s" = #val`, whereKey),
			pg.RawArgs{"#val": selectorValue},
		))
		if whereFn := actions.ResolveRoleWhere(module.RoleWhere, role); whereFn != nil {
			if roleWhere := whereFn(c); roleWhere != nil {
				where = pg.AND(where, roleWhere)
			}
		}
		if action.Where != nil {
			if actionWhere := action.Where(c); actionWhere != nil {
				where = pg.AND(where, actionWhere)
			}
		}
		where = appendRelationScopeWhere(module, where, scope)

		if action.Mode == actions.UpdateModeAtomic {
			atomicInput, err := atomicInputFromFields(realFields, mapInput)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
			rawDB := generator.db(module).RawDB()
			if rawDB == nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorUpdate, []string{"atomic update requires a SQL database"})
				return
			}
			tx, err := rawDB.BeginTx(ctx, nil)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback()
				}
			}()
			executor := db.NewAtomicExecutor(tx)
			recordID, err := atomicUpdateSubject(ctx, executor, module, where)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{"atomic update target is unavailable"})
				return
			}
			output, err := action.Atomic.Operation(ctx, executor, actions.AtomicUpdateInput{
				Input:    atomicInput,
				Selector: atomicSelector,
			})
			if err != nil {
				if rejection, ok := actions.AsAtomicCommittedRejection(err); ok {
					if err := tx.Commit(); err != nil {
						response.ErrorResponse(l, c, http.StatusInternalServerError, GeneratorErrorUpdate, []string{err.Error()})
						return
					}
					committed = true
					response.ErrorResponse(l, c, rejection.StatusCode(), rejection.Error(), []string{rejection.Error()})
					return
				}
				response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), []string{err.Error()})
				return
			}
			if output.PrimaryKey == "" {
				output.PrimaryKey = module.PrimaryKey.Name()
			}
			if err := validateAtomicUpdateResult(action.Atomic, output); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
			atomicPublishes, err := atomicUpdateRealtimePublishes(action.Atomic, atomicInput, output)
			if err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
			if tc != nil {
				if err := db.UpsertTranslations(tx, tc, recordID, realFields, mapInput); err != nil {
					response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
					return
				}
			}
			if err := tx.Commit(); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
				return
			}
			committed = true
			response.Response(l, c, output)
			generator.publishAtomicRealtime(c, module, actions.ModuleActionNameUpdate, output, atomicPublishes)
			return
		}

		_, err = generator.db(module).Update(l, module.Table, module.PrimaryKey, realFields, mapInput, where, tc)
		if err != nil {
			l.Errorln("UPDATE ERR: ", err)
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, []string{err.Error()})
			return
		}

		// Return view response if ViewAfterUpdate is enabled (default true) and ViewAction exists
		useView := action.ViewAfterUpdate == nil || *action.ViewAfterUpdate
		if useView {
			if viewAction := findViewAction(module); viewAction != nil {
				viewColumns := viewAction.GetColumns(c)
				viewFields := make([]fields.ModuleField, 0, len(module.Fields))
				for _, f := range module.Fields {
					if containsColumn(viewColumns, f.Column) {
						viewFields = append(viewFields, f)
					}
				}

				// A field whose projection depends on the caller has to be
				// resolved here too: the record returned after an update is
				// read with the same expressions a view is.
				viewFields = fields.ResolveProjections(c, viewFields)

				viewJoins := viewAction.Join
				if roleJoins := actions.ResolveRoleJoin(module.RoleJoin, role); roleJoins != nil {
					viewJoins = append(roleJoins, viewJoins...)
				}

				viewResult, viewErr := generator.db(module).View(l, module.Table, module.PrimaryKey, viewFields, where, viewJoins, tc)
				if viewErr == nil {
					response.Response(l, c, viewResult)
					action.AfterRequest(c)
					generator.publishRealtime(c, module, actions.ModuleActionNameUpdate, viewResult)
					return
				}
			}
		}

		// Fallback: re-fetch with update columns
		fallbackResult, fallbackErr := generator.db(module).View(l, module.Table, module.PrimaryKey, fields.ResolveProjections(c, realFields), where, nil, tc)
		if fallbackErr != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorUpdate, nil)
			return
		}

		response.Response(l, c, fallbackResult)

		action.AfterRequest(c)
		generator.publishRealtime(c, module, actions.ModuleActionNameUpdate, fallbackResult)
	}
}

func (generator *Generator) actionDelete(module *BaseModule, action actions.DeleteModuleAction) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		l, _ := icontext.GetLogger(ctx)
		role := actions.GetRoleFromContext(c)

		if hook := actions.ResolveRoleHook(module.RoleBeforeHook, role); hook != nil {
			if err := hook(c); err != nil {
				response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), nil)
				return
			}
		}
		defer func() {
			if hook := actions.ResolveRoleAfterHook(module.RoleAfterHook, role); hook != nil {
				hook(c)
			}
		}()

		err := action.BeforeRequest(c)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, err.Error(), []string{err.Error()})
			return
		}

		whereKey := c.Param("bykey")
		allowedKeys := make([]interface{}, 0, len(action.By))
		for _, col := range action.By {
			allowedKeys = append(allowedKeys, col.Name())
		}
		err = validation.In(allowedKeys...).Error(fmt.Sprintf(`allowed keys %v`, allowedKeys)).Validate(whereKey)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorDelete, []string{
				err.Error(),
			})
			return
		}

		whereValue := c.Param("value")
		if len(whereValue) == 0 {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorDelete, nil)
			return
		}

		tc := generator.buildTranslationContext(module)
		if tc != nil {
			tc.EntityID = whereValue
		}

		// Build WHERE condition: primary key + optional role/action filters
		where := pg.BoolExpression(pg.RawBool(
			fmt.Sprintf(`"%s" = #val`, whereKey),
			pg.RawArgs{"#val": whereValue},
		))
		if whereFn := actions.ResolveRoleWhere(module.RoleWhere, role); whereFn != nil {
			if roleWhere := whereFn(c); roleWhere != nil {
				where = pg.AND(where, roleWhere)
			}
		}
		if action.Where != nil {
			if actionWhere := action.Where(c); actionWhere != nil {
				where = pg.AND(where, actionWhere)
			}
		}
		scope, status, err := generator.resolveRelationScope(c, module)
		if err != nil {
			response.ErrorResponse(l, c, status, GeneratorErrorDelete, []string{err.Error()})
			return
		}
		where = appendRelationScopeWhere(module, where, scope)

		err = generator.db(module).Delete(l, module.Table, where, tc)
		if err != nil {
			response.ErrorResponse(l, c, http.StatusBadRequest, GeneratorErrorDelete, []string{
				err.Error(),
			})
			return
		}

		output := struct {
			Delete bool `json:"delete"`
		}{
			Delete: true,
		}
		response.Response(l, c, output)

		action.AfterRequest(c)
		generator.publishRealtime(c, module, actions.ModuleActionNameDelete, output)
	}
}
