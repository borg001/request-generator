package module

import (
	"fmt"
	"strings"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
)

func (generator *Generator) validateGlobalWidgets() error {
	widgets := make(map[string]actions.WidgetConfig)
	for _, module := range generator.Modules {
		for _, action := range moduleActions(module) {
			widget := actionWidget(action)
			if widget == nil {
				continue
			}
			if err := widget.Validate(); err != nil {
				return fmt.Errorf("module %q action %q: %w", module.Name, action.Action(), err)
			}
			if _, exists := widgets[widget.ID]; exists {
				return fmt.Errorf("widget id %q is duplicated", widget.ID)
			}
			widgets[widget.ID] = *widget
			if err := validateNoWorkspaceInputBindings(widget.Bindings); err != nil {
				return fmt.Errorf("widget %q: %w", widget.ID, err)
			}
			if err := validateRequestBindingShape(module, action, widget.Bindings); err != nil {
				return fmt.Errorf("widget %q: %w", widget.ID, err)
			}
			if widget.Renderer.Workspace != nil {
				if err := generator.validateWorkspaceWidget(widget.ID, *widget.Renderer.Workspace); err != nil {
					return err
				}
			} else if err := validateWidgetResourcePresentation(module, action); err != nil {
				return fmt.Errorf("widget %q: %w", widget.ID, err)
			}
		}
	}
	for _, module := range generator.Modules {
		for _, action := range module.Render.Actions() {
			if err := generator.validateWidgetActionResult(module, action, action.AfterSuccess, true, widgets); err != nil {
				return err
			}
			if err := generator.validateWidgetActionResult(module, action, action.AfterError, false, widgets); err != nil {
				return err
			}
		}
	}
	for _, widget := range widgets {
		if widget.Renderer.Workspace == nil {
			continue
		}
		for _, command := range widget.Renderer.Workspace.Commands {
			if err := generator.validateWorkspaceCommandActionResult(widget.ID, command, widgets); err != nil {
				return err
			}
		}
	}
	return nil
}

func moduleActions(module *BaseModule) []actions.ModuleAction {
	result := make([]actions.ModuleAction, 0, len(module.Actions)+1)
	result = append(result, module.Actions...)
	if module.Defrec.Widget != nil {
		result = append(result, module.Defrec)
	}
	return result
}

func (generator *Generator) validateWorkspaceWidget(id string, workspace renderer.WorkspaceWidget) error {
	masterModule, masterAction, err := generator.validateResource(id, "master", workspace.Master)
	if err != nil {
		return err
	}
	if masterAction.Action() != actions.ModuleActionNameList {
		return fmt.Errorf("widget %q master action must be list", id)
	}
	selection, err := generator.workspaceSelectionScope(id, workspace)
	if err != nil {
		return err
	}
	if _, _, err := generator.validateResource(id, "detail", workspace.Detail); err != nil {
		return err
	}
	if workspace.Summary != nil {
		_, summaryAction, err := generator.validateResource(id, "summary", *workspace.Summary)
		if err != nil {
			return err
		}
		switch summaryAction.Action() {
		case actions.ModuleActionNameList, actions.ModuleActionNameView:
		default:
			return fmt.Errorf("widget %q summary action must be list or view", id)
		}
	}
	for _, command := range workspace.Commands {
		if err := generator.validateWorkspaceCommand(id, command, selection); err != nil {
			return err
		}
	}
	for _, subscription := range workspace.Subscriptions {
		module, ok := generator.moduleByName(subscription.Module)
		if !ok {
			return fmt.Errorf("widget %q subscription references unknown module %q", id, subscription.Module)
		}
		for _, name := range subscription.Actions {
			action, ok := findModuleAction(module, name)
			if !ok {
				return fmt.Errorf("widget %q subscription %q references unknown action %q", id, subscription.Module, name)
			}
			event := actions.RealtimeEvent(action)
			if event == nil {
				return fmt.Errorf("widget %q subscription %q action %q does not declare realtime event", id, subscription.Module, name)
			}
			if subscription.Correlation == nil {
				continue
			}
			if event.CorrelationField == "" {
				return fmt.Errorf("widget %q subscription %q action %q does not declare realtime correlation", id, subscription.Module, name)
			}
			if event.CorrelationField != subscription.Correlation.EventField {
				return fmt.Errorf("widget %q subscription %q action %q correlation field %q does not match declared field %q", id, subscription.Module, name, subscription.Correlation.EventField, event.CorrelationField)
			}
			eventField := module.GetField(event.CorrelationField)
			if eventField == nil {
				return fmt.Errorf("widget %q subscription %q action %q declares unknown correlation field %q", id, subscription.Module, name, event.CorrelationField)
			}
			if err := validateRuntimeValueTypeCompatibility(*eventField, *masterModule.GetField(selection.Field)); err != nil {
				return fmt.Errorf("widget %q subscription %q action %q correlation field %q: %w", id, subscription.Module, name, event.CorrelationField, err)
			}
		}
	}
	return nil
}

func (generator *Generator) validateWorkspaceCommand(widgetID string, command renderer.WorkspaceCommand, selection widgetSelectionScope) error {
	module, ok := generator.moduleByName(command.Module)
	if !ok {
		return fmt.Errorf("widget %q command %q references unknown module %q", widgetID, command.ID, command.Module)
	}
	action, ok := findModuleAction(module, command.Action)
	if !ok {
		return fmt.Errorf("widget %q command %q resource %q references unknown action %q", widgetID, command.ID, command.Module, command.Action)
	}
	switch action.Action() {
	case actions.ModuleActionNameAdd, actions.ModuleActionNameUpdate, actions.ModuleActionNameDelete:
	default:
		return fmt.Errorf("widget %q command %q action must be add, update or delete", widgetID, command.ID)
	}
	if err := validateWorkspaceCommandInput(module, action, command.Input); err != nil {
		return fmt.Errorf("widget %q command %q input: %w", widgetID, command.ID, err)
	}
	if err := validateRequestBindingShape(module, action, command.Bindings); err != nil {
		return fmt.Errorf("widget %q command %q: %w", widgetID, command.ID, err)
	}
	if err := validateWorkspaceCommandPresentation(command, selection); err != nil {
		return fmt.Errorf("widget %q command %q presentation: %w", widgetID, command.ID, err)
	}
	return nil
}

func (generator *Generator) validateWorkspaceCommandActionResult(widgetID string, command renderer.WorkspaceCommand, widgets map[string]actions.WidgetConfig) error {
	result := command.AfterSuccess
	if result == nil || result.Widget == nil {
		return nil
	}
	target := *result.Widget
	widget, exists := widgets[target.ID]
	if !exists {
		return fmt.Errorf("widget %q command %q references unknown widget %q", widgetID, command.ID, target.ID)
	}
	if target.Selection == nil {
		if len(target.Refresh) > 0 && widget.Renderer.Workspace == nil {
			return fmt.Errorf("widget %q command %q refreshes a non-workspace widget %q", widgetID, command.ID, target.ID)
		}
		return nil
	}
	if widget.Renderer.Workspace == nil {
		return fmt.Errorf("widget %q command %q sets selection on a non-workspace widget %q", widgetID, command.ID, target.ID)
	}

	source := target.Selection.Source
	if source.Resource.Module != command.Module || source.Resource.Action != command.Action {
		return fmt.Errorf("widget %q command %q selection source must reference its command action", widgetID, command.ID)
	}
	sourceModule, ok := generator.moduleByName(command.Module)
	if !ok {
		return fmt.Errorf("widget %q command %q source module %q is unavailable", widgetID, command.ID, command.Module)
	}
	sourceAction, ok := findModuleAction(sourceModule, command.Action)
	if !ok {
		return fmt.Errorf("widget %q command %q source action %q is unavailable", widgetID, command.ID, command.Action)
	}
	contract, ok := resolveStandardActionContract(sourceModule, sourceAction)
	if !ok {
		return fmt.Errorf("widget %q command %q action has no standard request", widgetID, command.ID)
	}
	sourceType, exists := contract.resultFieldType(source.Field)
	if !exists {
		return fmt.Errorf("widget %q command %q selection source field %q is not declared", widgetID, command.ID, source.Field)
	}
	selection, err := generator.workspaceSelectionScope(target.ID, *widget.Renderer.Workspace)
	if err != nil {
		return err
	}
	if sourceType != selection.Type {
		return fmt.Errorf("widget %q command %q selection source field %q type %q does not match target field %q type %q", widgetID, command.ID, source.Field, sourceType, selection.Field, selection.Type)
	}
	return nil
}

func validateWorkspaceCommandPresentation(command renderer.WorkspaceCommand, selection widgetSelectionScope) error {
	if command.Presentation == nil {
		return nil
	}
	if err := command.Presentation.Validate(); err != nil {
		return err
	}
	if command.Presentation.Active != "" {
		if _, exists := selection.Fields[command.Presentation.Active]; !exists {
			return fmt.Errorf("active field %q is not declared by master resource", command.Presentation.Active)
		}
	}
	for _, value := range []*renderer.Condition{
		command.Presentation.VisibleIf,
		command.Presentation.HiddenIf,
		command.Presentation.DisabledIf,
	} {
		if err := validateWorkspaceSelectionCondition(value, selection.Fields); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkspaceSelectionCondition(condition *renderer.Condition, fields map[string]renderer.TypedValueType) error {
	if condition == nil {
		return nil
	}
	if condition.Path != "" {
		if _, exists := fields[condition.Path]; !exists {
			return fmt.Errorf("condition field %q is not declared by master resource", condition.Path)
		}
	}
	for index := range condition.All {
		if err := validateWorkspaceSelectionCondition(&condition.All[index], fields); err != nil {
			return err
		}
	}
	for index := range condition.Any {
		if err := validateWorkspaceSelectionCondition(&condition.Any[index], fields); err != nil {
			return err
		}
	}
	switch value := condition.Not.(type) {
	case nil:
	case renderer.Condition:
		if err := validateWorkspaceSelectionCondition(&value, fields); err != nil {
			return err
		}
	case *renderer.Condition:
		if err := validateWorkspaceSelectionCondition(value, fields); err != nil {
			return err
		}
	default:
		return fmt.Errorf("condition not must be a condition")
	}
	return nil
}

func validateWorkspaceCommandInput(module *BaseModule, action actions.ModuleAction, input *renderer.WorkspaceCommandInput) error {
	if input == nil {
		return nil
	}
	if action.Action() != actions.ModuleActionNameAdd {
		return fmt.Errorf("is only supported by add action")
	}
	for _, fieldID := range input.Fields {
		field := module.GetField(fieldID)
		if field == nil {
			return fmt.Errorf("field %q is not declared", fieldID)
		}
		if !workspaceCommandActionMayWriteField(action, *field) {
			return fmt.Errorf("field %q is not declared by add action", fieldID)
		}
		switch field.FormType {
		case fields.ModuleFieldFormTypeHidden, fields.ModuleFieldFormTypeOnlyView:
			return fmt.Errorf("field %q is not editable", fieldID)
		}
	}
	return nil
}

func workspaceCommandActionMayWriteField(action actions.ModuleAction, field fields.ModuleField) bool {
	var columns []pg.Column
	var roleColumns []actions.RoleContext
	var hasDynamicColumns bool
	switch value := action.(type) {
	case actions.AddModuleAction:
		columns = value.Columns
		roleColumns = value.Fields
		hasDynamicColumns = value.ColumnsFunc != nil
	case *actions.AddModuleAction:
		columns = value.Columns
		roleColumns = value.Fields
		hasDynamicColumns = value.ColumnsFunc != nil
	default:
		return false
	}
	if hasDynamicColumns {
		return true
	}
	for _, column := range columns {
		if column.Name() == field.ColumnName() {
			return true
		}
	}
	for _, roleContext := range roleColumns {
		for _, column := range roleContext.Columns {
			if column.Name() == field.ColumnName() {
				return true
			}
		}
	}
	return false
}

func (generator *Generator) validateResource(widgetID, name string, resource renderer.Resource) (*BaseModule, actions.ModuleAction, error) {
	module, ok := generator.moduleByName(resource.Module)
	if !ok {
		return nil, nil, fmt.Errorf("widget %q %s resource references unknown module %q", widgetID, name, resource.Module)
	}
	action, ok := findModuleAction(module, resource.Action)
	if !ok {
		return nil, nil, fmt.Errorf("widget %q %s resource %q references unknown action %q", widgetID, name, resource.Module, resource.Action)
	}
	if err := validateNoWorkspaceInputBindings(resource.Bindings); err != nil {
		return nil, nil, fmt.Errorf("widget %q %s resource: %w", widgetID, name, err)
	}
	if err := validateRequestBindingShape(module, action, resource.Bindings); err != nil {
		return nil, nil, fmt.Errorf("widget %q %s resource: %w", widgetID, name, err)
	}
	if err := validateWidgetResourcePresentation(module, action); err != nil {
		return nil, nil, fmt.Errorf("widget %q %s resource: %w", widgetID, name, err)
	}
	return module, action, nil
}

func validateWidgetResourcePresentation(module *BaseModule, action actions.ModuleAction) error {
	switch action.Action() {
	case actions.ModuleActionNameList:
		if module.Render.List == nil {
			return fmt.Errorf("list action requires ListPage")
		}
	case actions.ModuleActionNameView:
		pageType := renderer.PageTypeRecord
		switch view := action.(type) {
		case actions.ViewModuleAction:
			pageType = viewActionPageType(view)
		case *actions.ViewModuleAction:
			pageType = viewActionPageType(*view)
		}
		switch pageType {
		case renderer.PageTypeRecord:
			if module.Render.Record == nil {
				return fmt.Errorf("view action requires RecordPage")
			}
		case renderer.PageTypeForm:
			if module.Render.Form == nil {
				return fmt.Errorf("view action requires FormPage")
			}
		default:
			return fmt.Errorf("view action has unsupported page type %q", pageType)
		}
	case actions.ModuleActionNameDefrec:
		if module.Render.Form == nil {
			return fmt.Errorf("defrec action requires FormPage")
		}
	default:
		return fmt.Errorf("action %q does not expose a readable renderer", action.Action())
	}
	return nil
}

type widgetSelectionScope struct {
	Field  string
	Type   renderer.TypedValueType
	Fields map[string]renderer.TypedValueType
}

type workspaceCommandInputScope struct {
	Fields map[string]fields.ModuleField
}

func validateNoWorkspaceInputBindings(bindings []renderer.RequestBinding) error {
	for _, binding := range bindings {
		if binding.Source.Runtime != nil && binding.Source.Runtime.Scope == renderer.RuntimeValueSourceInput {
			return fmt.Errorf("runtime input source is only supported by workspace commands")
		}
	}
	return nil
}

func validateRequestBindingShape(module *BaseModule, action actions.ModuleAction, bindings []renderer.RequestBinding) error {
	if err := renderer.ValidateRequestBindings(bindings); err != nil {
		return err
	}
	byTarget := make(map[renderer.RequestBindingTarget]renderer.RequestBinding, len(bindings))
	bodyBindings := make([]renderer.RequestBinding, 0)
	for _, binding := range bindings {
		if binding.Target == renderer.RequestBindingBody {
			bodyBindings = append(bodyBindings, binding)
			continue
		}
		byTarget[binding.Target] = binding
	}

	switch action.Action() {
	case actions.ModuleActionNameView:
		if err := validateWidgetPathBindings(module, action, byTarget, "view"); err != nil {
			return err
		}
		if len(bindings) != 2 {
			return fmt.Errorf("view action only supports path_by_key and path_value bindings")
		}
		return nil
	case actions.ModuleActionNameList:
		for _, binding := range bindings {
			if binding.Target != renderer.RequestBindingFilter {
				return fmt.Errorf("list action only supports filter bindings")
			}
		}
		return nil
	case actions.ModuleActionNameDefrec:
		if len(bindings) != 0 {
			return fmt.Errorf("defrec action does not accept bindings")
		}
		return nil
	case actions.ModuleActionNameAdd:
		if len(bodyBindings) == 0 || len(bodyBindings) != len(bindings) {
			return fmt.Errorf("add action requires body bindings only")
		}
		return validateWidgetBodyBindingFields(module, bodyBindings)
	case actions.ModuleActionNameUpdate:
		if len(bodyBindings) == 0 {
			return fmt.Errorf("update action requires body bindings")
		}
		if len(byTarget) != 2 || len(bindings) != len(bodyBindings)+2 {
			return fmt.Errorf("update action only supports path_by_key, path_value and body bindings")
		}
		if err := validateWidgetPathBindings(module, action, byTarget, "update"); err != nil {
			return err
		}
		return validateWidgetBodyBindingFields(module, bodyBindings)
	case actions.ModuleActionNameDelete:
		if len(bindings) != 2 {
			return fmt.Errorf("delete action only supports path_by_key and path_value bindings")
		}
		return validateWidgetPathBindings(module, action, byTarget, "delete")
	default:
		return fmt.Errorf("action %q does not support widget request bindings", action.Action())
	}
}

func validateWidgetPathBindings(module *BaseModule, action actions.ModuleAction, bindings map[renderer.RequestBindingTarget]renderer.RequestBinding, actionName string) error {
	byKey, hasByKey := bindings[renderer.RequestBindingPathByKey]
	if !hasByKey {
		return fmt.Errorf("%s action requires path_by_key and path_value bindings", actionName)
	}
	if _, hasValue := bindings[renderer.RequestBindingPathValue]; !hasValue {
		return fmt.Errorf("%s action requires path_by_key and path_value bindings", actionName)
	}
	if byKey.Source.Literal == nil || byKey.Source.Literal.Type != renderer.TypedValueString {
		return fmt.Errorf("path_by_key requires a string literal")
	}
	field := widgetActionByField(module, action, byKey.Source.Literal.String)
	if field == nil {
		return fmt.Errorf("path_by_key %q is not declared by %s action", byKey.Source.Literal.String, actionName)
	}
	return nil
}

func validateWidgetBodyBindingFields(module *BaseModule, bindings []renderer.RequestBinding) error {
	for _, binding := range bindings {
		field := module.GetField(binding.Field)
		if field == nil {
			return fmt.Errorf("body field %q is not declared", binding.Field)
		}
		if binding.Source.Runtime != nil && binding.Source.Runtime.Scope == renderer.RuntimeValueSourceInput {
			continue
		}
		fieldType, err := runtimeTypedValueType(*field)
		if err != nil {
			return fmt.Errorf("body field %q: %w", binding.Field, err)
		}
		if binding.Source.Literal != nil && binding.Source.Literal.Type != fieldType {
			return fmt.Errorf("body field %q literal type %q does not match expected type %q", binding.Field, binding.Source.Literal.Type, fieldType)
		}
	}
	return nil
}

// validateRequestBindingAvailability resolves the one source of truth
// for filter availability in the current request context. Static validation
// intentionally only checks binding shape because FilterFunc and
// FilterCondition are context-dependent.
func (generator *Generator) validateRequestBindingAvailability(c *gin.Context, module *BaseModule, action actions.ModuleAction, bindings []renderer.RequestBinding, selection *widgetSelectionScope, input *workspaceCommandInputScope) (bool, error) {
	if err := validateRequestBindingShape(module, action, bindings); err != nil {
		return false, err
	}

	switch action.Action() {
	case actions.ModuleActionNameView, actions.ModuleActionNameUpdate, actions.ModuleActionNameDelete:
		var byKey renderer.RequestBinding
		for _, binding := range bindings {
			if binding.Target == renderer.RequestBindingPathByKey {
				byKey = binding
				break
			}
		}
		for _, binding := range bindings {
			if binding.Target != renderer.RequestBindingPathValue {
				continue
			}
			field := widgetActionByField(module, action, byKey.Source.Literal.String)
			fieldType, err := runtimeTypedValueType(*field)
			if err != nil {
				return false, fmt.Errorf("path_by_key %q: %w", byKey.Source.Literal.String, err)
			}
			if err := validateWidgetRequestValueSource(binding.Source, fieldType, selection, input); err != nil {
				return false, err
			}
		}
		if action.Action() != actions.ModuleActionNameUpdate {
			break
		}
		fallthrough
	case actions.ModuleActionNameAdd:
		inputs, ok := widgetActionInputFields(c, module, action)
		if !ok {
			return false, fmt.Errorf("action %q has unsupported type %T", action.Action(), action)
		}
		for _, binding := range bindings {
			if binding.Target != renderer.RequestBindingBody {
				continue
			}
			field, exists := inputs[binding.Field]
			if !exists {
				return false, nil
			}
			if binding.Source.Runtime != nil && binding.Source.Runtime.Scope == renderer.RuntimeValueSourceInput {
				if input == nil {
					return false, fmt.Errorf("body field %q: input source is unavailable", binding.Field)
				}
				if _, exists := input.Fields[binding.Source.Runtime.Field]; !exists {
					return false, nil
				}
				continue
			}
			fieldType, err := runtimeTypedValueType(field)
			if err != nil {
				return false, fmt.Errorf("body field %q: %w", binding.Field, err)
			}
			if err := validateWidgetRequestValueSource(binding.Source, fieldType, selection, input); err != nil {
				return false, fmt.Errorf("body field %q: %w", binding.Field, err)
			}
		}
	case actions.ModuleActionNameList:
		list, ok := widgetListAction(action)
		if !ok {
			return false, fmt.Errorf("list action has unsupported type %T", action)
		}
		filters := generator.effectiveListFilters(c, module, list, generator.getLang(c))
		for _, binding := range bindings {
			field, exists := filters[binding.Field]
			if !exists {
				return false, nil
			}
			fieldType, ok := runtimeModuleFieldType(field.Type)
			if !ok {
				return false, fmt.Errorf("filter field %q type %q cannot be used as a runtime value", binding.Field, field.Type)
			}
			if err := validateWidgetRequestValueSource(binding.Source, fieldType, selection, input); err != nil {
				return false, fmt.Errorf("filter field %q: %w", binding.Field, err)
			}
		}
	}
	return true, nil
}

func widgetActionInputFields(c *gin.Context, module *BaseModule, action actions.ModuleAction) (map[string]fields.ModuleField, bool) {
	var columns []pg.Column
	switch value := action.(type) {
	case actions.AddModuleAction:
		columns = value.GetColumns(c)
	case *actions.AddModuleAction:
		columns = value.GetColumns(c)
	case actions.UpdateModuleAction:
		columns = value.GetColumns(c)
	case *actions.UpdateModuleAction:
		columns = value.GetColumns(c)
	default:
		return nil, false
	}
	result := make(map[string]fields.ModuleField, len(columns))
	for _, column := range columns {
		field := module.GetFieldByColumn(column)
		if field != nil {
			result[field.Name()] = *field
		}
	}
	return result, true
}

func workspaceCommandInputScopeForContext(c *gin.Context, module *BaseModule, action actions.ModuleAction, input *renderer.WorkspaceCommandInput) (*workspaceCommandInputScope, *renderer.WorkspaceCommandInputLoad, bool, error) {
	if input == nil {
		return nil, nil, true, nil
	}
	if action.Action() != actions.ModuleActionNameAdd {
		return nil, nil, false, fmt.Errorf("input is only supported by add action")
	}
	available, ok := widgetActionInputFields(c, module, action)
	if !ok {
		return nil, nil, false, fmt.Errorf("action %q has unsupported type %T", action.Action(), action)
	}
	role := actions.GetRoleFromContext(c)
	scope := &workspaceCommandInputScope{Fields: make(map[string]fields.ModuleField, len(input.Fields))}
	for _, fieldID := range input.Fields {
		field, exists := available[fieldID]
		if !exists {
			return nil, nil, false, nil
		}
		if field.RoleFormType != nil {
			if formType, exists := field.RoleFormType[string(role)]; exists {
				field.FormType = formType
			}
		}
		switch field.FormType {
		case fields.ModuleFieldFormTypeHidden, fields.ModuleFieldFormTypeOnlyView:
			return nil, nil, false, nil
		}
		scope.Fields[fieldID] = field
	}
	contract, ok := resolveStandardActionContract(module, module.Defrec)
	if !ok {
		return nil, nil, false, fmt.Errorf("module %q defrec has no standard request", module.Name)
	}
	return scope, &renderer.WorkspaceCommandInputLoad{
		Definition: renderer.ResourceLoad{Request: contract.Request},
	}, true, nil
}

func workspaceCommandPresentationAvailable(command renderer.WorkspaceCommand, selection widgetSelectionScope) bool {
	return validateWorkspaceCommandPresentation(command, selection) == nil
}

func widgetListAction(action actions.ModuleAction) (actions.ListModuleAction, bool) {
	switch value := action.(type) {
	case actions.ListModuleAction:
		return value, true
	case *actions.ListModuleAction:
		return *value, true
	default:
		return actions.ListModuleAction{}, false
	}
}

func validateWidgetRequestValueSource(source renderer.ValueSource, expected renderer.TypedValueType, selection *widgetSelectionScope, input *workspaceCommandInputScope) error {
	if source.Literal != nil {
		if source.Literal.Type != expected {
			return fmt.Errorf("literal type %q does not match expected type %q", source.Literal.Type, expected)
		}
		return nil
	}
	if source.Runtime == nil {
		return fmt.Errorf("source is required")
	}
	switch source.Runtime.Scope {
	case renderer.RuntimeValueSourceCurrentUser:
		if source.Runtime.Field != "id" {
			return fmt.Errorf("current_user field %q is not declared", source.Runtime.Field)
		}
		if expected != renderer.TypedValueNumber {
			return fmt.Errorf("current_user.id has type %q, expected %q", renderer.TypedValueNumber, expected)
		}
	case renderer.RuntimeValueSourceSelection:
		if selection == nil {
			return fmt.Errorf("selection source is unavailable")
		}
		actual, exists := selection.Fields[source.Runtime.Field]
		if !exists {
			return fmt.Errorf("selection field %q is not returned by master action", source.Runtime.Field)
		}
		if actual != expected {
			return fmt.Errorf("selection field %q has type %q, expected %q", source.Runtime.Field, actual, expected)
		}
	case renderer.RuntimeValueSourceInput:
		if input == nil {
			return fmt.Errorf("input source is unavailable")
		}
		if _, exists := input.Fields[source.Runtime.Field]; !exists {
			return fmt.Errorf("input field %q is not declared", source.Runtime.Field)
		}
	default:
		return fmt.Errorf("runtime scope %q is unsupported", source.Runtime.Scope)
	}
	return nil
}

func widgetActionByField(module *BaseModule, action actions.ModuleAction, name string) *fields.ModuleField {
	var by []pg.Column
	switch value := action.(type) {
	case actions.ViewModuleAction:
		by = value.By
	case *actions.ViewModuleAction:
		by = value.By
	case actions.UpdateModuleAction:
		by = value.By
	case *actions.UpdateModuleAction:
		by = value.By
	case actions.DeleteModuleAction:
		by = value.By
	case *actions.DeleteModuleAction:
		by = value.By
	default:
		return nil
	}
	for _, column := range by {
		if column.Name() == name {
			return module.GetField(name)
		}
	}
	return nil
}

func runtimeTypedValueType(field fields.ModuleField) (renderer.TypedValueType, error) {
	valueType, ok := runtimeModuleFieldType(field.Type)
	if !ok {
		return "", fmt.Errorf("field type %q cannot be used as a runtime value", field.Type)
	}
	return valueType, nil
}

func (generator *Generator) workspaceSelectionScope(widgetID string, workspace renderer.WorkspaceWidget) (widgetSelectionScope, error) {
	masterModule, ok := generator.moduleByName(workspace.Master.Module)
	if !ok {
		return widgetSelectionScope{}, fmt.Errorf("widget %q master resource references unknown module %q", widgetID, workspace.Master.Module)
	}
	selectionField := masterModule.GetField(workspace.Selection.Field)
	if selectionField == nil {
		return widgetSelectionScope{}, fmt.Errorf("widget %q selection field %q is not defined by master module %q", widgetID, workspace.Selection.Field, masterModule.Name)
	}
	selectionType, err := runtimeTypedValueType(*selectionField)
	if err != nil {
		return widgetSelectionScope{}, fmt.Errorf("widget %q selection field %q: %w", widgetID, workspace.Selection.Field, err)
	}
	selectionFields := make(map[string]renderer.TypedValueType)
	for _, field := range masterModule.Fields {
		fieldType, err := runtimeTypedValueType(field)
		if err != nil {
			continue
		}
		selectionFields[field.Name()] = fieldType
	}
	return widgetSelectionScope{Field: workspace.Selection.Field, Type: selectionType, Fields: selectionFields}, nil
}

func (generator *Generator) workspaceSelectionScopeForContext(c *gin.Context, widgetID string, workspace renderer.WorkspaceWidget) (widgetSelectionScope, error) {
	selection, err := generator.workspaceSelectionScope(widgetID, workspace)
	if err != nil {
		return widgetSelectionScope{}, err
	}
	masterModule, ok := generator.moduleByName(workspace.Master.Module)
	if !ok {
		return widgetSelectionScope{}, fmt.Errorf("widget %q master resource references unknown module %q", widgetID, workspace.Master.Module)
	}
	masterAction, ok := findModuleAction(masterModule, workspace.Master.Action)
	if !ok {
		return widgetSelectionScope{}, fmt.Errorf("widget %q master resource %q references unknown action %q", widgetID, workspace.Master.Module, workspace.Master.Action)
	}
	list, ok := widgetListAction(masterAction)
	if !ok {
		return widgetSelectionScope{}, fmt.Errorf("widget %q master action must be list", widgetID)
	}
	selection.Fields = make(map[string]renderer.TypedValueType)
	for _, column := range list.GetColumns(c) {
		field := masterModule.GetFieldByColumn(column)
		if field == nil {
			continue
		}
		fieldType, err := runtimeTypedValueType(*field)
		if err != nil {
			continue
		}
		selection.Fields[field.Name()] = fieldType
	}
	if _, exists := selection.Fields[selection.Field]; !exists {
		return widgetSelectionScope{}, fmt.Errorf("widget %q selection field %q is not returned by master action", widgetID, selection.Field)
	}
	return selection, nil
}

func (generator *Generator) validateWidgetActionResult(owner *BaseModule, action renderer.Action, result *renderer.ActionResult, afterSuccess bool, widgets map[string]actions.WidgetConfig) error {
	if result == nil || result.Widget == nil {
		return nil
	}
	target := *result.Widget
	widget, exists := widgets[target.ID]
	if !exists {
		return fmt.Errorf("module %q references unknown widget %q", owner.Name, target.ID)
	}
	if target.Selection == nil {
		if len(target.Refresh) == 0 {
			return nil
		}
		if widget.Renderer.Workspace == nil {
			return fmt.Errorf("module %q action target %q refreshes a non-workspace widget", owner.Name, target.ID)
		}
		return nil
	}
	if !afterSuccess {
		return fmt.Errorf("module %q action target %q selection is only allowed after success", owner.Name, target.ID)
	}
	if widget.Renderer.Workspace == nil {
		return fmt.Errorf("module %q action target %q sets selection on a non-workspace widget", owner.Name, target.ID)
	}
	return generator.validateWidgetTargetSelection(owner, action, target, widget)
}

func (generator *Generator) validateWidgetTargetSelection(owner *BaseModule, action renderer.Action, target renderer.WidgetTarget, widget actions.WidgetConfig) error {
	if action.Type != renderer.ActionAPI || action.API == nil {
		return fmt.Errorf("module %q action target %q selection requires an api action request", owner.Name, target.ID)
	}

	source := target.Selection.Source
	sourceModule, ok := generator.moduleByName(source.Resource.Module)
	if !ok {
		return fmt.Errorf("module %q action target %q selection source resource references unknown module %q", owner.Name, target.ID, source.Resource.Module)
	}
	sourceAction, ok := findModuleAction(sourceModule, source.Resource.Action)
	if !ok {
		return fmt.Errorf("module %q action target %q selection source resource %q references unknown action %q", owner.Name, target.ID, source.Resource.Module, source.Resource.Action)
	}
	contract, ok := resolveStandardActionContract(sourceModule, sourceAction)
	if !ok {
		return fmt.Errorf("module %q action target %q selection source resource %q action %q has no standard request", owner.Name, target.ID, source.Resource.Module, source.Resource.Action)
	}
	if action.API.Method != contract.Request.Method || !standardEndpointMatches(contract.Request.Endpoint, action.API.Endpoint) {
		return fmt.Errorf("module %q action target %q selection source resource %q action %q does not match action request %s %s", owner.Name, target.ID, source.Resource.Module, source.Resource.Action, contract.Request.Method, contract.Request.Endpoint)
	}
	sourceType, exists := contract.resultFieldType(source.Field)
	if !exists {
		return fmt.Errorf("module %q action target %q selection source resource %q action %q: response field %q is not declared", owner.Name, target.ID, source.Resource.Module, source.Resource.Action, source.Field)
	}
	selection, err := generator.workspaceSelectionScope(target.ID, *widget.Renderer.Workspace)
	if err != nil {
		return err
	}
	if sourceType != selection.Type {
		return fmt.Errorf("module %q action target %q selection source field %q has type %q, target field %q has type %q", owner.Name, target.ID, source.Field, sourceType, selection.Field, selection.Type)
	}
	return nil
}

func runtimeModuleFieldType(fieldType fields.ModuleFieldType) (renderer.TypedValueType, bool) {
	switch fieldType {
	case fields.ModuleFieldTypeString:
		return renderer.TypedValueString, true
	case fields.ModuleFieldTypeInt, fields.ModuleFieldTypeFloat:
		return renderer.TypedValueNumber, true
	case fields.ModuleFieldTypeBool:
		return renderer.TypedValueBool, true
	default:
		return "", false
	}
}

func validateRuntimeValueTypeCompatibility(source, target fields.ModuleField) error {
	sourceType, err := runtimeTypedValueType(source)
	if err != nil {
		return err
	}
	targetType, err := runtimeTypedValueType(target)
	if err != nil {
		return err
	}
	if sourceType != targetType {
		return fmt.Errorf("type %q does not match target type %q", sourceType, targetType)
	}
	return nil
}

// standardEndpointMatches reports whether an action request addresses the
// standard endpoint. A path parameter of the standard endpoint (":bykey",
// ":value") stands for any one segment, so "/api/items/id/{id}" is the update
// request of "/api/items/:bykey/:value".
func standardEndpointMatches(standard, endpoint string) bool {
	if standard == endpoint {
		return true
	}
	standardSegments := strings.Split(standard, "/")
	endpointSegments := strings.Split(endpoint, "/")
	if len(standardSegments) != len(endpointSegments) {
		return false
	}
	for index, segment := range standardSegments {
		if strings.HasPrefix(segment, ":") && len(segment) > 1 {
			if endpointSegments[index] == "" {
				return false
			}
			continue
		}
		if segment != endpointSegments[index] {
			return false
		}
	}
	return true
}
