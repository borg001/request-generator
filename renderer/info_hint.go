package renderer

import (
	"fmt"
	"strings"
)

// InfoHint is a lasting explanation that a reader opens beside what it
// explains: a field, the heading of a section, a set of figures. The words are
// the producer's and are localized like any other; how the explanation opens
// is the consumer's - on hover or a click where there is a pointer, from the
// bottom of the screen where there is only a finger.
type InfoHint struct {
	// ID names the explanation apart from the place it stands in, so its
	// words can be told or replaced without touching that place.
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
	// Action leads to where the subject is told in full.
	Action *Action `json:"action,omitempty"`
}

// Validate refuses an explanation with nothing to say.
func (hint *InfoHint) Validate() error {
	if hint == nil {
		return nil
	}
	if strings.TrimSpace(hint.Text) == "" {
		return fmt.Errorf("info hint %q: text is required", hint.ID)
	}
	if hint.Action != nil {
		if err := hint.Action.Validate(); err != nil {
			return fmt.Errorf("info hint %q action: %w", hint.ID, err)
		}
	}
	return nil
}

// CloneInfoHint copies an explanation with its action: a reader's language
// written into the copy stays out of the producer's page.
func CloneInfoHint(value *InfoHint) *InfoHint {
	if value == nil {
		return nil
	}
	cp := *value
	cp.Action = cloneAction(value.Action)
	return &cp
}

// LocalizeInfoHint translates an explanation in place.
func LocalizeInfoHint(hint *InfoHint, resolve TextResolver) {
	if hint == nil || resolve == nil {
		return
	}
	(textLocalizer{resolve: resolve}).localizeInfoHint(hint)
}

func (localizer textLocalizer) localizeInfoHint(hint *InfoHint) {
	if hint == nil {
		return
	}
	localizer.localizeTextFields(&hint.Title, &hint.Text)
	localizer.localizeRendererAction(hint.Action)
}
