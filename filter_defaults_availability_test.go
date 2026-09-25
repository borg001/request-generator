package module

import (
	"testing"

	"github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
	"github.com/stretchr/testify/require"
)

// A default names a filter the list offers; one it does not offer is an error
// in the page, not a filter silently dropped.
func TestFilterDefaultsMustNameAvailableFilters(t *testing.T) {
	page := &renderer.ListPage{Filters: &renderer.Filters{Defaults: map[string]interface{}{"location_ids": []int{1}}}}
	require.EqualError(t, validateListFilterAvailability(page, map[string]fields.ModuleFilterField{}), `renderer filter default "location_ids" is not available for the current request`)
	require.NoError(t, validateListFilterAvailability(page, map[string]fields.ModuleFilterField{"location_ids": {}}))
}
