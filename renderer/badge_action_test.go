package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// A badge can say what pressing it does: a badge that names a case opens it,
// and the words of its action are translated with the badge.
func TestBadgeCarriesItsAction(t *testing.T) {
	badge := Badge{ID: "complaint", Field: "complaint_side", Action: &Action{ID: "open_complaint", Type: ActionRoute, LabelKey: "chat.open_complaint", Route: RouteAction{Path: "/support"}}}
	raw, err := json.Marshal(badge)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"action":{`)
	require.Contains(t, string(raw), `"open_complaint"`)

	localized := Localize(Universal{List: &ListPage{CardSchema: &CardSchema{Badges: []Badge{badge}}}}, func(value, key string) string {
		if key != "" {
			return "T:" + key
		}
		return value
	})
	action := localized.List.CardSchema.Badges[0].Action
	require.NotNil(t, action)
	require.Equal(t, "T:chat.open_complaint", action.Label)
	require.Empty(t, action.LabelKey)
	// The producer's own badge is left as it was.
	require.Equal(t, "chat.open_complaint", badge.Action.LabelKey)
}
