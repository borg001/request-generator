package renderer

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRecordResourceContract(t *testing.T) {
	resource := &Resource{ActionResource: ActionResource{Module: "entries", Action: "list"}}
	original := Universal{Record: &RecordPage{Sections: []RecordSection{{ID: "history", Renderer: RendererUniversalSection, Resource: resource, LoadingLabel: "loading", RetryLabel: "retry"}}}}
	require.NoError(t, original.Validate())
	cloned := original.Clone()
	cloned.Record.Sections[0].Resource.Module = "other"
	require.Equal(t, "entries", original.Record.Sections[0].Resource.Module)
	data, err := json.Marshal(original)
	require.NoError(t, err)
	require.NotContains(t, string(data), "entries")
	require.Contains(t, string(data), "loading_label")
	for _, action := range []string{"add", "update", "delete"} {
		original.Record.Sections[0].Resource.Action = action
		require.Error(t, original.Validate())
	}
	original.Record.Sections[0].Resource.Action = "list"
	original.Record.Sections[0].Renderer = RendererUniversalDisplay
	require.Error(t, original.Validate())
}
