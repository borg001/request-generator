package module

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/darkrain/request-generator/locale"
	"github.com/gin-gonic/gin"
)

const (
	translationLangContextKey = "request-generator.lang"
	translationFuncContextKey = "request-generator.translate"
)

type requestTranslationContextKey string

const (
	requestTranslationLangContextKey requestTranslationContextKey = "request-generator.request.lang"
	requestTranslationFuncContextKey requestTranslationContextKey = "request-generator.request.translate"
)

// Translator resolves a translation key with a caller-provided fallback.
type Translator func(key string, fallback string) string

// LoadTranslationsFile reads a nested JSON file, flattens it to dot-separated keys,
// and stores translations for the given locale.
//
// JSON structure:
//
//	{
//	  "users": {
//	    "label": "Users",
//	    "fields": { "email": "Email", "phone": "Phone" },
//	    "options": { "role": { "admin": "Admin" } },
//	    "actions": { "list": "User list" }
//	  }
//	}
//
// Produces keys: "users.label", "users.fields.email", "users.options.role.admin", etc.
func (g *Generator) LoadTranslationsFile(lang locale.Lang, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read translations file %s: %w", path, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse translations file %s: %w", path, err)
	}

	flat := make(map[string]string)
	flattenJSON(raw, "", flat)

	g.overridesMu.Lock()
	defer g.overridesMu.Unlock()
	if g.translations == nil {
		g.translations = make(map[locale.Lang]map[string]string)
	}
	g.translations[lang] = flat
	g.invalidateTranslationResponse(lang)
	return nil
}

func flattenJSON(data map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			flattenJSON(v, fullKey, result)
		}
	}
}

// Translate resolves a translation key for the given locale.
// Returns the translated text, or the key itself if not found.
// SetTranslationOverrides replaces, for one language, the words the
// application changes while it runs - text an administrator edits, say. They
// are read before the files and served with them; keys left out read the
// files again. Readers are never locked: every call publishes a new set.
func (g *Generator) SetTranslationOverrides(lang locale.Lang, overrides map[string]string) {
	g.overridesMu.Lock()
	defer g.overridesMu.Unlock()
	current, _ := g.translationOverrides.Load().(map[locale.Lang]map[string]string)
	if sameTranslations(current[lang], overrides) {
		return
	}
	next := make(map[locale.Lang]map[string]string, len(current)+1)
	for existing, words := range current {
		next[existing] = words
	}
	copied := make(map[string]string, len(overrides))
	for key, value := range overrides {
		copied[key] = value
	}
	next[lang] = copied
	g.translationOverrides.Store(next)
	g.invalidateTranslationResponse(lang)
}

func sameTranslations(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
}

// Caller holds overridesMu. Readers retain an immutable response already in
// flight; requests after publication see the replacement words immediately.
func (g *Generator) invalidateTranslationResponse(lang locale.Lang) {
	current, _ := g.translationResponses.Load().(map[locale.Lang][]byte)
	if _, ok := current[lang]; !ok {
		return
	}
	next := make(map[locale.Lang][]byte, len(current))
	for key, value := range current {
		if key != lang {
			next[key] = value
		}
	}
	g.translationResponses.Store(next)
}

func (g *Generator) translationResponse(lang locale.Lang) ([]byte, bool) {
	current, _ := g.translationResponses.Load().(map[locale.Lang][]byte)
	if value, ok := current[lang]; ok {
		return value, true
	}
	g.overridesMu.Lock()
	defer g.overridesMu.Unlock()
	current, _ = g.translationResponses.Load().(map[locale.Lang][]byte)
	if value, ok := current[lang]; ok {
		return value, true
	}
	words, ok := g.translations[lang]
	if !ok {
		return nil, false
	}
	overrides, _ := g.translationOverrides.Load().(map[locale.Lang]map[string]string)
	if len(overrides[lang]) > 0 {
		merged := make(map[string]string, len(words)+len(overrides[lang]))
		for key, value := range words {
			merged[key] = value
		}
		for key, value := range overrides[lang] {
			merged[key] = value
		}
		words = merged
	}
	// map[string]string always has a JSON representation, including invalid
	// UTF-8 replacement. No request-specific data enters this dictionary.
	encoded, _ := json.Marshal(words)
	next := make(map[locale.Lang][]byte, len(current)+1)
	for key, value := range current {
		next[key] = value
	}
	next[lang] = encoded
	g.translationResponses.Store(next)
	return encoded, true
}

func (g *Generator) translationOverride(lang locale.Lang, key string) (string, bool) {
	overrides, _ := g.translationOverrides.Load().(map[locale.Lang]map[string]string)
	text, ok := overrides[lang][key]
	return text, ok
}

func (g *Generator) Translate(lang locale.Lang, key string) string {
	if text, ok := g.translationOverride(lang, key); ok {
		return text
	}
	if g.translations != nil {
		if langMap, ok := g.translations[lang]; ok {
			if text, ok := langMap[key]; ok {
				return text
			}
		}
	}
	return key
}

// TranslateWithFallback resolves a translation key for the given locale and
// returns fallback when the key is empty or not found.
func (g *Generator) TranslateWithFallback(lang locale.Lang, key string, fallback string) string {
	if key == "" {
		return fallback
	}
	translated := g.Translate(lang, key)
	if translated == key {
		return fallback
	}
	return translated
}

// Lang returns the request locale selected by the generator.
func Lang(c *gin.Context) locale.Lang {
	if c == nil {
		return locale.EN
	}
	if value, ok := c.Get(translationLangContextKey); ok {
		if lang, ok := value.(locale.Lang); ok {
			return lang
		}
	}
	return locale.EN
}

// Translate resolves a translation key for the current request.
func Translate(c *gin.Context, key string, fallback string) string {
	if c != nil {
		if value, ok := c.Get(translationFuncContextKey); ok {
			if translate, ok := value.(Translator); ok {
				return translate(key, fallback)
			}
		}
	}
	if key == "" {
		return fallback
	}
	return fallback
}

// LangContext returns the locale attached to a request context by Generator.
// Atomic operations use context.Context rather than *gin.Context and can use
// this helper to build a localized server response before it is published.
func LangContext(ctx context.Context) locale.Lang {
	if ctx != nil {
		if lang, ok := ctx.Value(requestTranslationLangContextKey).(locale.Lang); ok {
			return lang
		}
	}
	return locale.EN
}

// TranslateContext resolves a translation key from a request context. It is
// the context.Context counterpart of Translate for operations that do not
// receive a Gin context.
func TranslateContext(ctx context.Context, key string, fallback string) string {
	if ctx != nil {
		if translate, ok := ctx.Value(requestTranslationFuncContextKey).(Translator); ok {
			return translate(key, fallback)
		}
	}
	return fallback
}

// Plural resolves one/few/many for Russian and one/other for other locales.
func Plural(c *gin.Context, baseKey string, count int, fallback string) string {
	lang := Lang(c)
	if lang == locale.RU {
		n := count
		if n < 0 {
			n = -n
		}
		mod10 := n % 10
		mod100 := n % 100
		if mod10 == 1 && mod100 != 11 {
			return Translate(c, baseKey+".one", fallback)
		}
		if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
			return Translate(c, baseKey+".few", fallback)
		}
		return Translate(c, baseKey+".many", fallback)
	}
	if count == 1 {
		return Translate(c, baseKey+".one", fallback)
	}
	return Translate(c, baseKey+".other", fallback)
}

func (g *Generator) setTranslationContext(c *gin.Context, lang locale.Lang) {
	translate := Translator(func(key string, fallback string) string {
		return g.TranslateWithFallback(lang, key, fallback)
	})
	c.Set(translationLangContextKey, lang)
	c.Set(translationFuncContextKey, translate)
	c.Request = c.Request.WithContext(context.WithValue(
		context.WithValue(c.Request.Context(), requestTranslationLangContextKey, lang),
		requestTranslationFuncContextKey,
		translate,
	))
}

// handleLangList returns the list of supported locales.
// GET /api/lang → [{"title":"English","key":"en"}, ...]
func (g *Generator) handleLangList() gin.HandlerFunc {
	type langItem struct {
		Title string `json:"title"`
		Key   string `json:"key"`
	}

	items := make([]langItem, 0, len(g.Locales))
	for _, l := range g.Locales {
		title := locale.LangTitles[l]
		if title == "" {
			title = string(l)
		}
		items = append(items, langItem{Title: title, Key: string(l)})
	}

	return func(c *gin.Context) {
		c.JSON(http.StatusOK, items)
	}
}

// handleLangTranslations returns all translations for a given locale.
// GET /api/lang/:key → {"users.label": "Users", ...}
func (g *Generator) handleLangTranslations() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := locale.Lang(c.Param("key"))

		if g.translations == nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "no translations loaded"})
			return
		}

		encoded, ok := g.translationResponse(key)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": fmt.Sprintf("locale %q not found", string(key))})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", encoded)
	}
}
