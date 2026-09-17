package module

import (
	"testing"

	"github.com/darkrain/request-generator/actions"
	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/stretchr/testify/require"
)

func TestAtomicUpdateContractDeclaresTypedResult(t *testing.T) {
	contract, ok := resolveStandardActionContract(&BaseModule{Path: "/api", Name: "entries"}, actions.UpdateModuleAction{Mode: actions.UpdateModeAtomic})
	require.True(t, ok)
	require.Equal(t, "POST", contract.Request.Method)
	require.Equal(t, "/api/entries/:bykey/:value", contract.Request.Endpoint)
	require.Equal(t, renderer.TypedValueNumber, mustActionResultType(t, contract, renderer.ActionResultFieldValue))
	require.Equal(t, renderer.TypedValueString, mustActionResultType(t, contract, renderer.ActionResultFieldPrimaryKey))
}

func TestStandardUpdateContractDoesNotDeclareAtomicResult(t *testing.T) {
	contract, ok := resolveStandardActionContract(&BaseModule{Path: "/api", Name: "entries"}, actions.UpdateModuleAction{})
	require.True(t, ok)
	_, exists := contract.resultFieldType(renderer.ActionResultFieldValue)
	require.False(t, exists)
}

func mustActionResultType(t *testing.T, contract standardActionContract, field renderer.ActionResultField) renderer.TypedValueType {
	t.Helper()
	value, ok := contract.resultFieldType(field)
	require.True(t, ok)
	return value
}

func TestAtomicActionContractCarriesDeclaredResultFields(t *testing.T) {
	id := pg.IntegerColumn("id")
	module := &BaseModule{Name: "invitations", Path: "/api", Fields: []fields.ModuleField{{Column: id, Type: fields.ModuleFieldTypeInt}}}
	declared := []actions.AtomicResultField{
		{Name: "chat_id", Kind: actions.AtomicValueKindInt},
		{Name: "title", Kind: actions.AtomicValueKindString},
		{Name: "recipients", Kind: actions.AtomicValueKindInts},
		{Name: "value", Kind: actions.AtomicValueKindString},
	}

	update, ok := resolveStandardActionContract(module, actions.UpdateModuleAction{Mode: actions.UpdateModeAtomic, By: []pg.Column{id}, Atomic: &actions.AtomicUpdateConfig{ResultFields: declared}})
	require.True(t, ok)
	for field, want := range map[renderer.ActionResultField]renderer.TypedValueType{
		renderer.ActionResultFieldValue: renderer.TypedValueNumber,
		"chat_id":                       renderer.TypedValueNumber,
		"title":                         renderer.TypedValueString,
	} {
		got, exists := update.resultFieldType(field)
		require.True(t, exists, field)
		require.Equal(t, want, got, field)
	}
	_, exists := update.resultFieldType("recipients")
	require.False(t, exists, "a list is not a scalar result")

	add, ok := resolveStandardActionContract(module, &actions.AddModuleAction{Mode: actions.AddModeAtomic, Atomic: &actions.AtomicAddConfig{ResultFields: declared}})
	require.True(t, ok)
	got, exists := add.resultFieldType("chat_id")
	require.True(t, exists)
	require.Equal(t, renderer.TypedValueNumber, got)

	plain, ok := resolveStandardActionContract(module, actions.UpdateModuleAction{By: []pg.Column{id}})
	require.True(t, ok)
	_, exists = plain.resultFieldType("chat_id")
	require.False(t, exists, "a plain update declares no result")
}

func TestStandardEndpointMatchesPathParameters(t *testing.T) {
	require.True(t, standardEndpointMatches("/api/items/:bykey/:value", "/api/items/id/{id}"))
	require.True(t, standardEndpointMatches("/api/items/:bykey/:value", "/api/items/id/7"))
	require.True(t, standardEndpointMatches("/api/items", "/api/items"))
	require.False(t, standardEndpointMatches("/api/items/:bykey/:value", "/api/other/id/{id}"))
	require.False(t, standardEndpointMatches("/api/items/:bykey/:value", "/api/items/id"))
	require.False(t, standardEndpointMatches("/api/items/:bykey/:value", "/api/items/id/"))
	require.False(t, standardEndpointMatches("/api/items", "/api/items/extra"))
}
