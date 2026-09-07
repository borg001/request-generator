package module

import (
	"fmt"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
)

type NavigationEntry struct {
	ActionName  string                 `json:"action"`
	ID          string                 `json:"id,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Title       string                 `json:"title"`
	Icon        string                 `json:"icon,omitempty"`
	Show        bool                   `json:"show"`
	Order       int                    `json:"order"`
	MobileOrder int                    `json:"mobile_order,omitempty"`
	MobileTitle string                 `json:"mobile_title,omitempty"`
	Group       string                 `json:"group"`
	Target      NavigationTarget       `json:"target,omitempty"`
	Roles       []actions.Role         `json:"roles,omitempty"`
	Query       map[string]interface{} `json:"query,omitempty"`
}

type NavigationTarget struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	PageType renderer.PageType      `json:"page_type,omitempty"`
}

// RoutablePage declares a page that can be opened by a typed route action but
// is not necessarily visible in sidebar navigation.
type RoutablePage struct {
	ActionName string           `json:"action"`
	Path       string           `json:"path"`
	Target     NavigationTarget `json:"target,omitempty"`
	Roles      []actions.Role   `json:"roles,omitempty"`
}

type RenderFunc func(c *gin.Context, base renderer.Universal) (renderer.Universal, error)

type RelationScope struct {
	Relation string
	ID       interface{}
}

type ModuleRelation struct {
	Name         string                                          `json:"name,omitempty"`
	TargetModule string                                          `json:"target_module,omitempty"`
	SourceField  pg.Column                                       `json:"-"`
	TargetField  pg.Column                                       `json:"-"`
	ScopeCheck   func(c *gin.Context, scope RelationScope) error `json:"-"`
}

type BaseModule struct {
	Name           string                     `json:"name"`
	Label          string                     `json:"label"`
	Labels         map[string]string          `json:"-"`
	Table          pg.Table                   `json:"-"`
	PrimaryKey     pg.Column                  `json:"-"`
	Path           string                     `json:"path"`
	Fields         []fields.ModuleField       `json:"fields"`
	Defrec         actions.DefrecModuleAction `json:"defrec"`
	Actions        []actions.ModuleAction     `json:"actions"`
	RoleWhere      []actions.RoleWhere        `json:"-"`
	RoleJoin       []actions.RoleJoin         `json:"-"`
	RoleBeforeHook []actions.RoleHook         `json:"-"`
	RoleAfterHook  []actions.RoleAfterHook    `json:"-"`
	EntityName     string                     `json:"-"`
	Navigation     []NavigationEntry          `json:"navigation,omitempty"`
	Routes         []RoutablePage             `json:"routes,omitempty"`
	Render         renderer.Universal         `json:"-"`
	RenderFunc     RenderFunc                 `json:"-"`
	Relations      []ModuleRelation           `json:"-"`
}

func (module *BaseModule) RenderFor(c *gin.Context) (renderer.Universal, error) {
	render := module.Render.Clone()
	if module.RenderFunc == nil {
		return render, nil
	}
	var err error
	render, err = module.RenderFunc(c, render)
	if err != nil {
		return renderer.Universal{}, err
	}
	if err := render.Validate(); err != nil {
		return renderer.Universal{}, err
	}
	if err := module.validateFieldMatrices(render); err != nil {
		return renderer.Universal{}, err
	}
	return render, nil
}

func (module *BaseModule) validateFieldMatrices(render renderer.Universal) error {
	if render.Form == nil {
		return nil
	}
	for _, section := range render.Form.Sections {
		matrix := section.Matrix
		if matrix == nil {
			continue
		}
		fieldIDs := make([]string, 0)
		switch matrix.Type {
		case renderer.FieldMatrixTypeList:
			fieldIDs = append(fieldIDs, matrix.List.Fields...)
		case renderer.FieldMatrixTypeTable:
			for _, row := range matrix.Table.Rows {
				for _, cell := range row.Cells {
					if cell.Field != "" {
						fieldIDs = append(fieldIDs, cell.Field)
					}
				}
			}
		}
		for _, fieldID := range fieldIDs {
			if module.GetField(fieldID) == nil {
				return fmt.Errorf("renderer.Universal: matrix section %q references unknown field %q", section.ID, fieldID)
			}
		}
	}
	return nil
}

func (module BaseModule) GetEntityName() string {
	if module.EntityName != "" {
		return module.EntityName
	}
	return module.Table.TableName()
}

func (module BaseModule) TranslatableFields() []fields.ModuleField {
	var result []fields.ModuleField
	for _, f := range module.Fields {
		if f.Translatable {
			result = append(result, f)
		}
	}
	return result
}

func (module BaseModule) GetField(columnName string) *fields.ModuleField {
	for i, field := range module.Fields {
		if field.ColumnName() == columnName {
			return &module.Fields[i]
		}
		if field.Translatable && field.FieldName == columnName {
			return &module.Fields[i]
		}
	}
	return nil
}

func (module BaseModule) GetFieldByColumn(col pg.Column) *fields.ModuleField {
	return module.GetField(col.Name())
}

func (module BaseModule) GetRules(context *gin.Context, field fields.ModuleField, scenario fields.Scenario) []fields.CheckRules {
	checkRules := make([]fields.CheckRules, 0, 10)
	if field.Check != nil {
		for _, rule := range field.Check {
			for _, checkScenario := range rule.GetScenarios() {
				if checkScenario == scenario {
					checkRules = append(checkRules, rule)
				}
			}
		}
	}
	if field.CheckFunc != nil {
		for _, rule := range field.CheckFunc(context) {
			for _, checkScenario := range rule.GetScenarios() {
				if checkScenario == scenario {
					checkRules = append(checkRules, rule)
				}
			}
		}
	}
	if len(field.RoleCheck) > 0 {
		role := string(actions.GetRoleFromContext(context))
		for _, rc := range field.RoleCheck {
			if rc.Role == role || rc.Role == string(actions.RoleAll) {
				for _, rule := range rc.Rules {
					for _, checkScenario := range rule.GetScenarios() {
						if checkScenario == scenario {
							checkRules = append(checkRules, rule)
						}
					}
				}
				break
			}
		}
	}
	return checkRules
}
