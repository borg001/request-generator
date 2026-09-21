package renderer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTemplateIsolatesDeclarationAndConcurrentPages(t *testing.T) {
	base := Universal{List: &ListPage{ID: "items", Context: map[string]interface{}{"nested": map[string]interface{}{"title": "original"}}}, Form: &FormPage{ID: "edit"}}
	template, err := Compile(base)
	require.NoError(t, err)
	base.List.Context["nested"].(map[string]interface{})["title"] = "changed after compile"
	for i := 0; i < 32; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			draft, err := template.Draft(PageTypeList)
			require.NoError(t, err)
			page, err := draft.Edit()
			require.NoError(t, err)
			require.Nil(t, page.Form)
			require.Equal(t, "original", page.List.Context["nested"].(map[string]interface{})["title"])
			page.List.Context["nested"].(map[string]interface{})["title"] = t.Name()
			result, err := draft.Build()
			require.NoError(t, err)
			require.Same(t, page.List, result.List)
			require.Equal(t, t.Name(), result.List.Context["nested"].(map[string]interface{})["title"])
			_, err = draft.Edit()
			require.Error(t, err)
			require.Error(t, draft.Replace(Universal{}))
			_, err = draft.Build()
			require.Error(t, err)
		})
	}
}

func TestDraftReplacementAndValidation(t *testing.T) {
	template, err := Compile(Universal{List: &ListPage{ID: "base"}})
	require.NoError(t, err)
	_, err = template.Draft("invalid")
	require.Error(t, err)
	for _, pageType := range []PageType{PageTypeList, PageTypeResourceGrid, PageTypeForm, PageTypeRecord} {
		draft, err := template.Draft(pageType)
		require.NoError(t, err)
		require.Error(t, draft.Replace(Universal{List: &ListPage{}, Form: &FormPage{}}))
		require.NoError(t, draft.Replace(Universal{}))
		result, err := draft.Build()
		require.NoError(t, err)
		require.True(t, result.IsZero(), "explicit removal must not resurrect the template")
	}
	draft, err := template.Draft(PageTypeList)
	require.NoError(t, err)
	fresh := &ListPage{ID: "fresh"}
	require.NoError(t, draft.Replace(Universal{List: fresh}))
	result, err := draft.Build()
	require.NoError(t, err)
	require.Same(t, fresh, result.List, "replacement must not be cloned")
	draft, err = template.Draft(PageTypeList)
	require.NoError(t, err)
	require.NoError(t, draft.Replace(Universal{List: fresh, ResourceGrid: &ResourceGridPage{}}))
	_, err = draft.Build()
	require.ErrorContains(t, err, "mutually exclusive")
	_, err = Compile(Universal{List: fresh, ResourceGrid: &ResourceGridPage{}})
	require.Error(t, err)
}
