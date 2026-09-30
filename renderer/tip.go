package renderer

import (
	"fmt"
	"strings"
)

// TipDevice names the screen a tip is told on. A wide screen with a pointer
// and a phone are met in different places and told differently, so a tip is
// written for one of them.
type TipDevice string

const (
	TipDeviceDesktop TipDevice = "desktop"
	TipDeviceMobile  TipDevice = "mobile"
)

// Tip is a temporary hint: said once to a reader at a place of the page and
// gone once the reader has answered it. The producer decides who is told what
// and when; the consumer shows one at a time, on the screen it names.
type Tip struct {
	ID     string    `json:"id"`
	Device TipDevice `json:"device"`
	// Anchor names the section, component, field or action of the page the
	// tip points at. Without one the tip stands on its own.
	Anchor  string `json:"anchor,omitempty"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text"`
	Version int    `json:"version,omitempty"`
	// Dismiss records that the reader answered the tip; it is what the tip's
	// closing button does.
	Dismiss *Action `json:"dismiss"`
	// Action is a step the tip offers besides closing.
	Action *Action `json:"action,omitempty"`
	// Steps make the tip an introduction: its own title and text open it,
	// and the steps follow one after another, each at a place of the screen.
	// The reader goes on, or skips the rest; either way is its answer.
	Steps     []TipStep `json:"steps,omitempty"`
	NextLabel string    `json:"next_label,omitempty"`
	SkipLabel string    `json:"skip_label,omitempty"`
	DoneLabel string    `json:"done_label,omitempty"`
	// Brand puts the application's mark on top of the tip's opening card: a
	// welcome says whose place the reader has come to.
	Brand bool `json:"brand,omitempty"`
}

// TipStep is one step of an introduction.
type TipStep struct {
	Anchor string `json:"anchor,omitempty"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text"`
}

func validateTips(scope string, tips []Tip) error {
	seen := make(map[string]struct{}, len(tips))
	for _, tip := range tips {
		if strings.TrimSpace(tip.ID) == "" {
			return fmt.Errorf("%s tip: id is required", scope)
		}
		key := tip.ID + "/" + string(tip.Device)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s tip %q: told twice on %s", scope, tip.ID, tip.Device)
		}
		seen[key] = struct{}{}
		if tip.Device != TipDeviceDesktop && tip.Device != TipDeviceMobile {
			return fmt.Errorf("%s tip %q: device must be desktop or mobile", scope, tip.ID)
		}
		if strings.TrimSpace(tip.Text) == "" {
			return fmt.Errorf("%s tip %q: text is required", scope, tip.ID)
		}
		if tip.Dismiss == nil {
			return fmt.Errorf("%s tip %q: dismiss is required", scope, tip.ID)
		}
		if err := tip.Dismiss.Validate(); err != nil {
			return fmt.Errorf("%s tip %q dismiss: %w", scope, tip.ID, err)
		}
		if tip.Action != nil {
			if err := tip.Action.Validate(); err != nil {
				return fmt.Errorf("%s tip %q action: %w", scope, tip.ID, err)
			}
		}
		for index, step := range tip.Steps {
			if strings.TrimSpace(step.Text) == "" {
				return fmt.Errorf("%s tip %q step %d: text is required", scope, tip.ID, index+1)
			}
		}
		if len(tip.Steps) > 0 && (strings.TrimSpace(tip.NextLabel) == "" || strings.TrimSpace(tip.DoneLabel) == "") {
			return fmt.Errorf("%s tip %q: an introduction names its next and done buttons", scope, tip.ID)
		}
	}
	return nil
}

func (r Universal) validateTips() error {
	if r.List != nil {
		if err := validateTips("list page", r.List.Tips); err != nil {
			return err
		}
	}
	if r.Record != nil {
		if err := validateTips("record page", r.Record.Tips); err != nil {
			return err
		}
	}
	if r.Form != nil {
		if err := validateTips("form page", r.Form.Tips); err != nil {
			return err
		}
	}
	return nil
}

func cloneTips(values []Tip) []Tip {
	if values == nil {
		return nil
	}
	out := make([]Tip, len(values))
	for i, tip := range values {
		out[i] = tip
		out[i].Dismiss = cloneAction(tip.Dismiss)
		out[i].Action = cloneAction(tip.Action)
		out[i].Steps = cloneSlice(tip.Steps)
	}
	return out
}

func (localizer textLocalizer) localizeTips(tips []Tip) {
	for i := range tips {
		localizer.localizeTextFields(&tips[i].Title, &tips[i].Text, &tips[i].NextLabel, &tips[i].SkipLabel, &tips[i].DoneLabel)
		for step := range tips[i].Steps {
			localizer.localizeTextFields(&tips[i].Steps[step].Title, &tips[i].Steps[step].Text)
		}
		localizer.localizeRendererAction(tips[i].Dismiss)
		localizer.localizeRendererAction(tips[i].Action)
	}
}
