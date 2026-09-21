package module

import (
	"errors"
	"testing"

	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPageRenderKeepsLegacyFullCallback(t *testing.T) {
	m := &BaseModule{Render: renderer.Universal{Form: &renderer.FormPage{ID: "edit"}}, RenderFunc: func(_ *gin.Context, base renderer.Universal) (renderer.Universal, error) {
		base.List = &renderer.ListPage{ID: base.Form.ID}
		return base, nil
	}}
	page, err := m.RenderPageFor(nil, renderer.PageTypeList)
	require.NoError(t, err)
	require.Equal(t, "edit", page.List.ID)
	require.NotNil(t, page.Form)
}

func TestPageRenderValidatesDynamicMetadataAndPropagatesErrors(t *testing.T) {
	m := &BaseModule{PageRenderFunc: func(_ *gin.Context, draft *renderer.Draft) error {
		return draft.Replace(renderer.Universal{Form: &renderer.FormPage{Sections: []renderer.FormSection{{ID: "matrix", Renderer: renderer.RendererFieldMatrix, Matrix: &renderer.FieldMatrix{Type: renderer.FieldMatrixTypeList, List: &renderer.FieldMatrixList{Fields: []string{"unknown"}, Columns: renderer.FieldMatrixColumnsOne}}}}}})
	}}
	_, err := m.RenderPageFor(nil, renderer.PageTypeForm)
	require.ErrorContains(t, err, "unknown field")
	m.PageRenderFunc = func(_ *gin.Context, _ *renderer.Draft) error { return errors.New("producer failed") }
	_, err = m.RenderPageFor(nil, renderer.PageTypeList)
	require.ErrorContains(t, err, "producer failed")
}
