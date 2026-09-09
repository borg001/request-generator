package module

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/db"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/locale"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
)

func validationResponseMessage(fallback string, errs map[string]string) string {
	keys := make([]string, 0, len(errs))
	for key := range errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if message := strings.TrimSpace(errs[key]); message != "" {
			return message
		}
	}
	return fallback
}

func (generator *Generator) getPagination(page int64, size int64) (int64, int64, int64) {
	var limit int64
	if size <= 0 {
		limit = 10
	} else {
		limit = size
	}
	if page <= 0 {
		page = 0
	}
	offset := page * limit

	return limit, offset, page
}

func (generator *Generator) normalizeFilters(data map[string]string, filters map[string]fields.ModuleFilterField, lang locale.Lang) map[string]string {
	resultFilterMap := make(map[string]string)

parentLoop:
	for key, filter := range filters {
		filterValue, ok := data[key]
		if !ok || len(filterValue) == 0 {
			continue
		}

		for _, rule := range filter.Check {
			if err := rule.Validate(filterValue, string(lang)); err != nil {
				continue parentLoop
			}
		}

		// A filter on an array column is written as a Postgres array literal. A
		// single value - what a one-value pill sends - is the same filter with
		// one element in it, and reached the database as a malformed literal.
		if filter.Type == fields.ModuleFieldTypeArray && !strings.HasPrefix(filterValue, "{") {
			filterValue = "{" + filterValue + "}"
		}

		resultFilterMap[key] = filterValue
	}

	for key, value := range data {
		result := strings.Split(key, ".")
		if len(result) > 1 {
			resultFilterMap[key] = value
		}
	}

	return resultFilterMap
}

// effectiveListFilters is the sole typed registry for a list action. Virtual
// definitions intentionally override the same logical key for that action.
func (generator *Generator) effectiveListFilters(c *gin.Context, module *BaseModule, action actions.ListModuleAction, lang locale.Lang) map[string]fields.ModuleFilterField {
	allowedColumns := append([]pg.Column{}, action.Filter...)
	if action.FilterFunc != nil {
		allowedColumns = append(allowedColumns, action.FilterFunc(c)...)
	}
	registry := make(map[string]fields.ModuleFilterField)
	role := string(actions.GetRoleFromContext(c))
	for _, field := range module.Fields {
		if field.FilterCondition != nil && !field.FilterCondition(c) || !containsColumn(allowedColumns, field.Column) {
			continue
		}
		registry[field.ColumnName()] = fields.ModuleFilterField{
			Column: field.Column, Title: generator.Translate(lang, field.Title), Type: field.Type, FormType: field.FormType,
			Example: field.Example, AllLabel: generator.Translate(lang, field.AllLabel), Options: generator.fieldOptions(c, field, role, lang),
			OptionsSource: field.OptionsSource, Check: field.Check, Convert: field.Convert,
		}
	}
	for _, field := range action.VirtualFilters {
		if field.FilterCondition != nil && !field.FilterCondition(c) {
			continue
		}
		key := field.FieldName
		if key == "" && field.Column != nil {
			key = field.Column.Name()
		}
		if key == "" {
			continue
		}
		options := append([]fields.ModuleFieldOptions(nil), field.Options...)
		for i := range options {
			options[i].Label = generator.Translate(lang, options[i].Label)
		}
		field.Title = generator.Translate(lang, field.Title)
		field.AllLabel = generator.Translate(lang, field.AllLabel)
		field.Options = options
		registry[key] = field
	}
	return registry
}

func fieldFormTypeForRole(field fields.ModuleField, role string) fields.ModuleFieldFormType {
	if field.RoleFormType != nil {
		if formType, ok := field.RoleFormType[role]; ok {
			return formType
		}
	}
	return field.FormType
}

func (generator *Generator) checkRequest(
	context *gin.Context,
	data map[string]interface{},
	module *BaseModule,
	action actions.ModuleAction,
	scenario fields.Scenario,
	lang locale.Lang,
) map[string]string {
	errs := make(map[string]string)
	actionColumns := action.GetColumns(context)
	checked := make(map[string]struct{}, len(actionColumns))

	for _, col := range actionColumns {
		colName := col.Name()
		field := module.GetField(colName)
		if field == nil {
			continue
		}
		fieldKey := field.ColumnName()
		if field.Translatable {
			fieldKey = field.Name()
		}
		checked[fieldKey] = struct{}{}
		generator.validateRequestField(context, data, *field, scenario, lang, generator.db(module).RawDB(), errs)
	}

	for key := range data {
		if _, ok := checked[key]; ok {
			continue
		}
		field := module.GetField(key)
		if field == nil {
			continue
		}
		if len(module.GetRules(context, *field, scenario)) == 0 {
			continue
		}
		generator.validateRequestField(context, data, *field, scenario, lang, generator.db(module).RawDB(), errs)
	}

	return errs
}

func (generator *Generator) validationErrorMessage(field fields.ModuleField, lang locale.Lang, err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	label := strings.TrimSpace(generator.TranslateWithFallback(lang, field.Title, field.ColumnName()))
	if label == "" {
		return message
	}
	for _, name := range []string{field.ColumnName(), field.Name()} {
		if name != "" {
			message = strings.ReplaceAll(message, name, label)
		}
	}
	return message
}

func (generator *Generator) validateRequestField(
	context *gin.Context,
	data map[string]interface{},
	field fields.ModuleField,
	scenario fields.Scenario,
	lang locale.Lang,
	rawDB *sql.DB,
	errs map[string]string,
) {
	if field.Translatable {
		value := data[field.Name()]
		rules := contextFieldRules(context, field, scenario)
		if langMap, ok := value.(map[string]interface{}); ok {
			for langKey, langVal := range langMap {
				for _, rule := range rules {
					err := rule.Validate(langVal, string(lang))
					if err != nil {
						errs[field.Name()+"."+langKey] = generator.validationErrorMessage(field, lang, err)
					}
				}
			}
		} else {
			for _, rule := range rules {
				err := rule.Validate(value, string(lang))
				if err != nil {
					errs[field.Name()] = generator.validationErrorMessage(field, lang, err)
				}
			}
		}
		return
	}

	colName := field.ColumnName()
	value := data[colName]
	rules := contextFieldRules(context, field, scenario)
	for _, rule := range rules {
		if dr, ok := rule.(fields.DataCheckRule); ok {
			if err := dr.ValidateData(context, rawDB, data, string(lang)); err != nil {
				errs[colName] = generator.validationErrorMessage(field, lang, err)
			}
			continue
		}
		err := rule.Validate(value, string(lang))
		if err != nil {
			errs[colName] = generator.validationErrorMessage(field, lang, err)
		}
	}

	if field.Convert != nil && value != nil {
		_, err := field.Convert(context, value)
		if err != nil {
			errs[colName] = generator.validationErrorMessage(field, lang, err)
		}
	}
	if field.Type == fields.ModuleFieldTypeArray && field.ArrayStorage.Normalize() == fields.ModuleFieldArrayStorageJSON && value != nil {
		if _, err := fields.MarshalJSONArray(value); err != nil {
			errs[colName] = err.Error()
		}
	}
}

func contextFieldRules(context *gin.Context, field fields.ModuleField, scenario fields.Scenario) []fields.CheckRules {
	rules := make([]fields.CheckRules, 0, 10)
	if field.Check != nil {
		for _, rule := range field.Check {
			if scenarioMatches(rule.GetScenarios(), scenario) {
				rules = append(rules, rule)
			}
		}
	}
	if field.CheckFunc != nil {
		for _, rule := range field.CheckFunc(context) {
			if scenarioMatches(rule.GetScenarios(), scenario) {
				rules = append(rules, rule)
			}
		}
	}
	role := actions.GetRoleFromContext(context)
	for _, rc := range field.RoleCheck {
		if rc.Role == string(role) || rc.Role == string(actions.RoleAll) {
			for _, rule := range rc.Rules {
				if scenarioMatches(rule.GetScenarios(), scenario) {
					rules = append(rules, rule)
				}
			}
			break
		}
	}
	return rules
}

func scenarioMatches(scenarios []fields.Scenario, scenario fields.Scenario) bool {
	for _, current := range scenarios {
		if current == scenario {
			return true
		}
	}
	return false
}

func (generator *Generator) mapRequestInput(
	c *gin.Context,
	data map[string]interface{},
	module *BaseModule,
	actionColumns []pg.Column,
) map[string]interface{} {
	output := make(map[string]interface{})

	for _, field := range module.Fields {
		if field.Translatable {
			value, ok := data[field.Name()]
			if ok && containsColumn(actionColumns, field.Column) {
				if field.Convert != nil {
					convertedValue, err := field.Convert(c, value)
					if err != nil {
						continue
					}
					output[field.Name()] = convertedValue
				} else {
					output[field.Name()] = value
				}
			}
			continue
		}

		colName := field.ColumnName()
		value, ok := data[colName]
		if ok && containsColumn(actionColumns, field.Column) {
			if field.Convert != nil {
				convertedValue, err := field.Convert(c, value)
				if err != nil {
					continue
				}
				output[colName] = convertedValue
			} else {
				output[colName] = value
			}
		}
	}

	return output
}

func queryParam(c *gin.Context, param string) (interface{}, error) {
	result := c.Request.URL.Query().Get(param)
	if len(result) == 0 {
		return nil, fmt.Errorf("param %s incorrect", param)
	}
	return result, nil
}

func int64QueryParam(c *gin.Context, param string, defaultValue int64) int64 {
	resultInterface, err := queryParam(c, param)
	if err != nil {
		return defaultValue
	}

	resultString, ok := resultInterface.(string)
	if !ok {
		return defaultValue
	}

	result, err := strconv.ParseInt(resultString, 0, 10)
	if err != nil {
		_ = err
		return defaultValue
	}

	return result
}

func containsColumn(columns []pg.Column, target pg.Column) bool {
	return fields.ContainsColumn(columns, target)
}

func findViewAction(module *BaseModule) *actions.ViewModuleAction {
	for _, a := range module.Actions {
		if a.Action() == actions.ModuleActionNameView {
			if va, ok := a.(actions.ViewModuleAction); ok {
				return &va
			}
		}
	}
	return nil
}

func findUpdateAction(module *BaseModule) *actions.UpdateModuleAction {
	for _, a := range module.Actions {
		if a.Action() == actions.ModuleActionNameUpdate {
			if ua, ok := a.(actions.UpdateModuleAction); ok {
				return &ua
			}
		}
	}
	return nil
}

func (generator *Generator) buildTranslationContext(module *BaseModule) *db.TranslationContext {
	transFields := module.TranslatableFields()
	if len(transFields) == 0 {
		return nil
	}
	langs := make([]string, len(generator.Locales))
	for i, l := range generator.Locales {
		langs[i] = string(l)
	}
	fieldInfos := make([]db.TranslatableFieldInfo, len(transFields))
	for i, f := range transFields {
		fieldInfos[i] = db.TranslatableFieldInfo{FieldName: f.Name()}
	}
	return &db.TranslationContext{
		EntityName: module.GetEntityName(),
		Fields:     fieldInfos,
		Langs:      langs,
	}
}
