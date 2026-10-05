package module

import (
	"testing"

	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/stretchr/testify/require"
)

// A number raised to the field it stands above says so in the language of
// the reader (theGHub1/api#454).
func TestMinFieldNoticeIsSaidInTheReadersLanguage(t *testing.T) {
	generator := &Generator{translations: map[locale.Lang]map[string]string{
		locale.RU: {"order.price_raised": "Не ниже ставки — поставили {value}"},
	}}
	localized := generator.localizeFieldPresentation(locale.RU, &renderer.FieldPresentation{MinField: "price_floor", MinFieldNotice: "order.price_raised"})
	require.Equal(t, "Не ниже ставки — поставили {value}", localized.MinFieldNotice)
	require.Equal(t, "price_floor", localized.MinField)
}
