package module

import (
	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/renderer"
)

// standardActionContract is the single request and scalar-result definition
// for a generator action. Routes, widget loads and action-result references
// resolve through this contract instead of repeating endpoint rules.
type standardActionContract struct {
	Request      renderer.APIAction
	ResultFields []standardActionResultField
}

type standardActionResultField struct {
	Field renderer.ActionResultField
	Type  renderer.TypedValueType
}

func (contract standardActionContract) resultFieldType(field renderer.ActionResultField) (renderer.TypedValueType, bool) {
	for _, result := range contract.ResultFields {
		if result.Field == field {
			return result.Type, true
		}
	}
	return "", false
}

func resolveStandardActionContract(module *BaseModule, action actions.ModuleAction) (standardActionContract, bool) {
	base := apiQueryURL(module.Path + "/" + module.Name)
	switch action.Action() {
	case actions.ModuleActionNameList:
		return standardActionContract{Request: renderer.APIAction{Method: "GET", Endpoint: base}}, true
	case actions.ModuleActionNameAdd:
		contract := standardActionContract{
			Request: renderer.APIAction{Method: "PUT", Endpoint: base},
			ResultFields: []standardActionResultField{
				{Field: renderer.ActionResultFieldValue, Type: renderer.TypedValueNumber},
				{Field: renderer.ActionResultFieldPrimaryKey, Type: renderer.TypedValueString},
			},
		}
		switch value := action.(type) {
		case actions.AddModuleAction:
			if value.Mode == actions.AddModeAtomic && value.Atomic != nil {
				contract.ResultFields = appendAtomicResultFields(contract.ResultFields, value.Atomic.ResultFields)
			}
		case *actions.AddModuleAction:
			if value != nil && value.Mode == actions.AddModeAtomic && value.Atomic != nil {
				contract.ResultFields = appendAtomicResultFields(contract.ResultFields, value.Atomic.ResultFields)
			}
		}
		return contract, true
	case actions.ModuleActionNameDefrec:
		return standardActionContract{Request: renderer.APIAction{Method: "GET", Endpoint: base + "/defrec/"}}, true
	case actions.ModuleActionNameView:
		return standardActionContract{Request: renderer.APIAction{Method: "GET", Endpoint: base + "/view/:bykey/:value"}}, true
	case actions.ModuleActionNameUpdate:
		contract := standardActionContract{Request: renderer.APIAction{Method: "POST", Endpoint: base + "/:bykey/:value"}}
		switch value := action.(type) {
		case actions.UpdateModuleAction:
			if value.Mode == actions.UpdateModeAtomic {
				contract.ResultFields = atomicUpdateActionResultFields(value.Atomic)
			}
		case *actions.UpdateModuleAction:
			if value != nil && value.Mode == actions.UpdateModeAtomic {
				contract.ResultFields = atomicUpdateActionResultFields(value.Atomic)
			}
		}
		return contract, true
	case actions.ModuleActionNameDelete:
		return standardActionContract{
			Request:      renderer.APIAction{Method: "DELETE", Endpoint: base + "/delete/:bykey/:value"},
			ResultFields: []standardActionResultField{{Field: renderer.ActionResultFieldDelete, Type: renderer.TypedValueBool}},
		}, true
	default:
		return standardActionContract{}, false
	}
}

func atomicUpdateActionResultFields(config *actions.AtomicUpdateConfig) []standardActionResultField {
	fields := []standardActionResultField{
		{Field: renderer.ActionResultFieldValue, Type: renderer.TypedValueNumber},
		{Field: renderer.ActionResultFieldPrimaryKey, Type: renderer.TypedValueString},
	}
	if config != nil {
		fields = appendAtomicResultFields(fields, config.ResultFields)
	}
	return fields
}

// appendAtomicResultFields adds the scalar fields an atomic operation declares
// it returns. The runtime refuses a result that lacks one of them, so an
// action result may name them the same way it names the record value.
func appendAtomicResultFields(fields []standardActionResultField, declared []actions.AtomicResultField) []standardActionResultField {
	for _, result := range declared {
		kind, ok := atomicKindTypedValueType(result.Kind)
		if !ok {
			continue
		}
		field := renderer.ActionResultField(result.Name)
		if field == renderer.ActionResultFieldValue || field == renderer.ActionResultFieldPrimaryKey {
			continue
		}
		fields = append(fields, standardActionResultField{Field: field, Type: kind})
	}
	return fields
}

func standardActionRouteQuery(module *BaseModule, action actions.ModuleAction) *RouteQuery {
	contract, ok := resolveStandardActionContract(module, action)
	if !ok {
		return nil
	}
	return &RouteQuery{Url: contract.Request.Endpoint, Method: contract.Request.Method}
}
