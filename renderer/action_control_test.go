package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// An action can be drawn as a switch that stands at the position of a field;
// a control the consumer does not know is refused.
func TestActionControlSwitch(t *testing.T) {
	presentation := ActionPresentation{Control: ActionControlSwitch, Active: "agency_access"}
	require.NoError(t, presentation.Validate())
	encoded, err := json.Marshal(presentation)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"control":"switch"`)
	presentation.Control = "slider"
	require.ErrorContains(t, presentation.Validate(), `unsupported control "slider"`)
}
