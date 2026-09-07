package renderer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormSectionActionsValidateAndClone(t *testing.T) {
	render := Universal{Form: &FormPage{
		Actions:  []Action{{ID: "choose"}, {ID: "change"}},
		Sections: []FormSection{{ID: "recipients", Actions: []string{"choose", "change"}}},
	}}

	require.NoError(t, render.Validate())
	clone := render.Clone()
	clone.Form.Sections[0].Actions[0] = "changed"
	require.Equal(t, "choose", render.Form.Sections[0].Actions[0])
}

func TestFormSectionActionsRejectUnknownAndDuplicateReferences(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		section FormSection
		want    string
	}{
		{name: "unknown", section: FormSection{ID: "recipients", Actions: []string{"missing"}}, want: `action "missing" is not declared`},
		{name: "duplicate", section: FormSection{ID: "recipients", Action: "choose", Actions: []string{"choose"}}, want: `action "choose" is duplicated`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			render := Universal{Form: &FormPage{Actions: []Action{{ID: "choose"}}, Sections: []FormSection{testCase.section}}}
			require.ErrorContains(t, render.Validate(), testCase.want)
		})
	}
}
