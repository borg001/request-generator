package module

import (
	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
)

func (generator *Generator) localizeFieldPresentation(lang locale.Lang, value *renderer.FieldPresentation) *renderer.FieldPresentation {
	localized := renderer.CloneFieldPresentation(value)
	if localized == nil {
		return nil
	}
	resolver := generator.rendererTextResolver(lang)
	for _, field := range []*string{&localized.Prefix, &localized.Suffix, &localized.Hint, &localized.Placeholder, &localized.Description} {
		*field = resolver(*field, "")
	}
	return localized
}

func (generator *Generator) localizeFieldMedia(lang locale.Lang, value *renderer.FieldMediaConfig, fieldValue interface{}) *renderer.FieldMediaConfig {
	localized := renderer.CloneFieldMediaConfig(value)
	if localized == nil {
		return nil
	}
	if localized.Item != nil && localized.Item.Src == "" {
		if src, ok := fieldValue.(string); ok {
			localized.Item.Src = src
		}
	}
	return renderer.LocalizeFieldMedia(localized, generator.rendererTextResolver(lang))
}

func (generator *Generator) localizeRenderer(lang locale.Lang, value renderer.Universal) renderer.Universal {
	return renderer.Localize(value, generator.rendererTextResolver(lang))
}

func (generator *Generator) rendererTextResolver(lang locale.Lang) renderer.TextResolver {
	return func(value string, key string) string {
		if key != "" {
			return generator.TranslateWithFallback(lang, key, value)
		}
		return generator.Translate(lang, value)
	}
}
