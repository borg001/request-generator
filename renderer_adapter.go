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
	for _, field := range []*string{&localized.Prefix, &localized.Suffix, &localized.Hint, &localized.Placeholder, &localized.Description, &localized.MinFieldNotice} {
		*field = resolver(*field, "")
	}
	renderer.LocalizeInfoHint(localized.Info, resolver)
	if len(localized.CalendarMarks) > 0 {
		marks := append([]renderer.CalendarMark(nil), localized.CalendarMarks...)
		for index := range marks {
			marks[index].Label = resolver(marks[index].Label, "")
		}
		localized.CalendarMarks = marks
	}
	if len(localized.NoticeByValue) > 0 {
		notices := append([]renderer.FieldValueNotice(nil), localized.NoticeByValue...)
		for index := range notices {
			for _, field := range []*string{&notices[index].Title, &notices[index].Message, &notices[index].ConfirmLabel} {
				*field = resolver(*field, "")
			}
		}
		localized.NoticeByValue = notices
	}
	return localized
}

func (generator *Generator) localizeFieldMedia(lang locale.Lang, value *renderer.FieldMediaConfig, fieldValue interface{}) *renderer.FieldMediaConfig {
	localized := renderer.LocalizeFieldMedia(value, generator.rendererTextResolver(lang))
	if localized == nil {
		return nil
	}
	if localized.Item != nil && localized.Item.Src == "" {
		if src, ok := fieldValue.(string); ok {
			localized.Item.Src = src
		}
	}
	return localized
}

func (generator *Generator) localizeRenderer(lang locale.Lang, value renderer.Universal) renderer.Universal {
	return renderer.Localize(value, generator.rendererTextResolver(lang))
}

func (generator *Generator) localizeOwnedRenderer(lang locale.Lang, value renderer.Universal) renderer.Universal {
	return renderer.LocalizeOwned(value, generator.rendererTextResolver(lang))
}

func (generator *Generator) rendererTextResolver(lang locale.Lang) renderer.TextResolver {
	return func(value string, key string) string {
		if key != "" {
			return generator.TranslateWithFallback(lang, key, value)
		}
		return generator.Translate(lang, value)
	}
}
