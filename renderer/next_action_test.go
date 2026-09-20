package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func nextActionForm(next string) Universal {
	return Universal{Form: &FormPage{
		ID: "order", Layout: LayoutOneColumn, Fields: []string{"name"},
		Actions: []Action{
			{ID: "create", Type: ActionAPI, Behavior: ActionBehaviorSubmit, Label: "Send", API: &APIAction{Method: "PUT", Endpoint: "/api/orders"}, AfterSuccess: &ActionResult{NextAction: next}},
			{ID: "save_template", Type: ActionModal, Label: "Save", Modal: &ModalAction{Renderer: RendererUniversalSection, Title: "Save"}, Route: RouteAction{Path: "/templates/create"}},
		},
	}}
}

func TestFormActionMayHandOverToAnotherActionOfThePage(t *testing.T) {
	require.NoError(t, nextActionForm("save_template").Validate())
	require.NoError(t, nextActionForm("").Validate())
}

func TestFormActionCannotHandOverToAnUndeclaredActionOrItself(t *testing.T) {
	require.EqualError(t, nextActionForm("missing").Validate(), `renderer.Universal: form action "create" hands over to undeclared action "missing"`)
	require.EqualError(t, nextActionForm("create").Validate(), `renderer.Universal: form action "create" cannot hand over to itself`)
}
