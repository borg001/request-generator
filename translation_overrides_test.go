package module

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
				_, _ = g.translationResponse(locale.RU)
			}
		}()
	}
	for i := 0; i < 50; i++ {
		g.SetTranslationOverrides(locale.RU, map[string]string{"k": "w"})
	}
	wait.Wait()
	require.Equal(t, "w", g.Translate(locale.RU, "k"))
}

func TestTranslationResponseRefreshAndIsolation(t *testing.T) {
	g := &Generator{translations: map[locale.Lang]map[string]string{
		locale.RU: {"word": "файл <&>"}, locale.EN: {"word": "file"},
	}}
	first, ok := g.translationResponse(locale.RU)
	require.True(t, ok)
	english, _ := g.translationResponse(locale.EN)
	for _, value := range []string{"новое", "", "ещё"} {
		overrides := map[string]string{"word": value}
		g.SetTranslationOverrides(locale.RU, overrides)
		overrides["word"] = "caller mutation"
		body, _ := g.translationResponse(locale.RU)
		var decoded map[string]string
		require.NoError(t, json.Unmarshal(body, &decoded))
		require.Equal(t, value, decoded["word"])
		again, _ := g.translationResponse(locale.EN)
		require.Equal(t, &english[0], &again[0], "unrelated language is retained")
		g.SetTranslationOverrides(locale.RU, map[string]string{"word": value})
		again, _ = g.translationResponse(locale.RU)
		require.Equal(t, &body[0], &again[0], "identical refresh does not re-encode")
	}
	g.SetTranslationOverrides(locale.RU, nil)
	body, _ := g.translationResponse(locale.RU)
	require.Equal(t, first, body)
	path := filepath.Join(t.TempDir(), "ru.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"word":"reloaded"}`), 0600))
	require.NoError(t, g.LoadTranslationsFile(locale.RU, path))
	body, _ = g.translationResponse(locale.RU)
	require.JSONEq(t, `{"word":"reloaded"}`, string(body))
	for i := 0; i < 100; i++ {
		_, ok = g.translationResponse(locale.Lang(fmt.Sprint(i)))
		require.False(t, ok)
	}
	responses := g.translationResponses.Load().(map[locale.Lang][]byte)
	require.Len(t, responses, 2, "unknown languages cannot grow the response cache")
}

func BenchmarkTranslationResponse(b *testing.B) {
	words := make(map[string]string, 10000)
	for i := 0; i < 10000; i++ {
		words[fmt.Sprintf("section.field.%d", i)] = "Перевод названия поля"
	}
	g := &Generator{translations: map[locale.Lang]map[string]string{locale.RU: words}}
	_, _ = g.translationResponse(locale.RU)
	b.Run("marshal", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(words)
		}
	})
	b.Run("published", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = g.translationResponse(locale.RU)
		}
	})
}
