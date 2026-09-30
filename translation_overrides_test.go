package module

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/darkrain/request-generator/locale"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Words replaced while the application runs are read before the files, per
// language, and a key left out reads the files again.
func TestTranslationOverridesAreReadBeforeTheFiles(t *testing.T) {
	g := &Generator{translations: map[locale.Lang]map[string]string{
		locale.RU: {"hints.a.text": "из файла", "hints.b.text": "тоже из файла"},
		locale.EN: {"hints.a.text": "from the file"},
	}}
	g.SetTranslationOverrides(locale.RU, map[string]string{"hints.a.text": "от администратора"})
	require.Equal(t, "от администратора", g.Translate(locale.RU, "hints.a.text"))
	require.Equal(t, "тоже из файла", g.Translate(locale.RU, "hints.b.text"))
	require.Equal(t, "from the file", g.Translate(locale.EN, "hints.a.text"))
	require.Equal(t, "от администратора", g.TranslateWithFallback(locale.RU, "hints.a.text", "fallback"))

	// A new set replaces the old one for its language only.
	g.SetTranslationOverrides(locale.EN, map[string]string{"hints.a.text": "by the administrator"})
	g.SetTranslationOverrides(locale.RU, map[string]string{})
	require.Equal(t, "из файла", g.Translate(locale.RU, "hints.a.text"))
	require.Equal(t, "by the administrator", g.Translate(locale.EN, "hints.a.text"))
}

// The words a client is handed carry the replaced ones.
func TestTranslationOverridesAreServedWithTheFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := &Generator{translations: map[locale.Lang]map[string]string{locale.RU: {"a": "1", "b": "2"}}}
	g.SetTranslationOverrides(locale.RU, map[string]string{"b": "3"})
	engine := gin.New()
	engine.GET("/api/lang/:key", g.handleLangTranslations())
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/lang/ru", nil))
	var words map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &words))
	require.Equal(t, map[string]string{"a": "1", "b": "3"}, words)
	require.Equal(t, "2", g.translations[locale.RU]["b"], "the files stay as they were")
}

// Readers and a writer can meet: go test -race holds that they do not race.
func TestTranslationOverridesCanBeReplacedWhileRead(t *testing.T) {
	g := &Generator{translations: map[locale.Lang]map[string]string{locale.RU: {"k": "v"}}}
	var wait sync.WaitGroup
	for reader := 0; reader < 4; reader++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := 0; i < 200; i++ {
				_ = g.Translate(locale.RU, "k")
			}
		}()
	}
	for i := 0; i < 50; i++ {
		g.SetTranslationOverrides(locale.RU, map[string]string{"k": "w"})
	}
	wait.Wait()
	require.Equal(t, "w", g.Translate(locale.RU, "k"))
}
