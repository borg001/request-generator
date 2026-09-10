package renderer

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFormNavigationTabs(t *testing.T) {
	value := Universal{Form: &FormPage{Navigation: &FormNavigation{Presentation: FormNavigationPresentationTabs}, Sections: []FormSection{{ID: "first", Title: "first.title", Renderer: RendererUniversalSection}}}}
	require.NoError(t, value.Validate())
	localized := Localize(value, func(value, _ string) string { return "localized:" + value })
	require.Equal(t, "localized:first.title", localized.Form.Sections[0].Title)
	require.Equal(t, FormNavigationPresentationTabs, localized.Form.Navigation.Presentation)
	clone := value.Clone()
	clone.Form.Navigation.Presentation = "other"
	require.Equal(t, FormNavigationPresentationTabs, value.Form.Navigation.Presentation)
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"navigation":{"presentation":"tabs"}`)
	require.Error(t, clone.Validate())
	value.Form.Workflow = &FormWorkflow{}
	require.Error(t, value.Validate())
	value.Form.Workflow = nil
	value.Form.Sections = append(value.Form.Sections, value.Form.Sections[0])
	require.Error(t, value.Validate())
	value.Form.Sections = nil
	require.Error(t, value.Validate())
	value.Form.Navigation = nil
	require.NoError(t, value.Validate())
	payload, err = json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"navigation"`)
}
