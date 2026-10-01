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
	// BackLabel names the button that turns a story back a window.
	BackLabel string `json:"back_label,omitempty"`
	// Brand puts the application's mark on top of the tip's opening card: a
	// welcome says whose place the reader has come to.
	Brand bool `json:"brand,omitempty"`
	// Presentation says how the tip is told. By default it stands by the
	// places it names. A story tells the way something goes, window by
	// window, with a scene playing over each step's words.
	Presentation TipPresentation `json:"presentation,omitempty"`
	// Demo is a sample the page shows while the tip is told, so the steps can
	// point at the parts of something the reader does not have yet - a
	// picture in a gallery. It answers its controls on the page alone and is
	// gone once the tip is answered.
	Demo *TipDemo `json:"demo,omitempty"`
	// Cast is who the reader is in a story's scenes - a client, a model, an
	// agency or a manager - so the one drawn for the reader looks like the
	// reader: an agency sending an order is not drawn as a client.
	Cast string `json:"cast,omitempty"`
}

// TipCasts are the people a story can cast its reader as.
var TipCasts = map[string]bool{"client": true, "model": true, "agency": true, "manager": true}

// TipDemo is the sample a tip brings to its page.
type TipDemo struct {
	Kind TipDemoKind `json:"kind"`
	// Label is the badge on the sample: it says the thing is not the reader's.
	Label string `json:"label,omitempty"`
	// Menu is what the sample's own menu shows - words only; none of it runs.
	Menu []string `json:"menu,omitempty"`
	// People are the persons a list of people shows while its walk is told:
	// drawn, nobody's profile, answering nothing (theGHub1/api#360).
	People []TipDemoPerson `json:"people,omitempty"`
	// Picture names which of the application's sample pictures a media item
	// shows - the application holds the files, with a twin whose face is
	// hidden. Without one the kit draws its own (theGHub1/api#372).
	Picture string `json:"picture,omitempty"`
}

// TipDemoPerson is one sample person: a name, who the person is - a
// client, a model, an agency or a manager - and how the two follow each
// other: the person follows the reader (follower), or both do (mutual).
type TipDemoPerson struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	Relation string `json:"relation,omitempty"`
}

// TipDemoKind is what the sample is.
type TipDemoKind string

const (
	// TipDemoMediaItem is a picture standing in a media gallery.
	TipDemoMediaItem TipDemoKind = "media_item"
	// TipDemoPeople are people standing at the head of a list of people.
	TipDemoPeople TipDemoKind = "people"
)

// TipPresentation is how a tip is told.
type TipPresentation string

const (
	// TipPresentationStory is a window of steps, each with its scene.
	TipPresentationStory TipPresentation = "story"
)

// TipStep is one step of an introduction.
type TipStep struct {
	Anchor string `json:"anchor,omitempty"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text"`
	// Scene names the moving picture over the step's words in a story; the
	// consumer draws the scenes it knows by these names.
	Scene string `json:"scene,omitempty"`
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
		if tip.Presentation != "" && tip.Presentation != TipPresentationStory {
			return fmt.Errorf("%s tip %q: presentation %q is unknown", scope, tip.ID, tip.Presentation)
		}
		if tip.Presentation == TipPresentationStory && len(tip.Steps) == 0 {
			return fmt.Errorf("%s tip %q: a story is told in steps", scope, tip.ID)
		}
		if tip.Demo != nil && tip.Demo.Kind != TipDemoMediaItem && tip.Demo.Kind != TipDemoPeople {
			return fmt.Errorf("%s tip %q: demo kind %q is unknown", scope, tip.ID, tip.Demo.Kind)
		}
		if tip.Demo != nil && tip.Demo.Kind == TipDemoPeople {
			if len(tip.Demo.People) == 0 {
				return fmt.Errorf("%s tip %q: people demo has no people", scope, tip.ID)
			}
			for _, person := range tip.Demo.People {
				if !TipCasts[person.Role] {
					return fmt.Errorf("%s tip %q: demo person role %q is unknown", scope, tip.ID, person.Role)
				}
				if person.Relation != "" && person.Relation != "follower" && person.Relation != "mutual" {
					return fmt.Errorf("%s tip %q: demo person relation %q is unknown", scope, tip.ID, person.Relation)
				}
			}
		}
		if tip.Cast != "" && !TipCasts[tip.Cast] {
			return fmt.Errorf("%s tip %q: cast %q is unknown", scope, tip.ID, tip.Cast)
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
	if r.ResourceGrid != nil {
		if err := validateTips("resource grid page", r.ResourceGrid.Tips); err != nil {
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
		if tip.Demo != nil {
			demo := *tip.Demo
			demo.Menu = cloneSlice(tip.Demo.Menu)
			demo.People = cloneSlice(tip.Demo.People)
			out[i].Demo = &demo
		}
	}
	return out
}

func (localizer textLocalizer) localizeTips(tips []Tip) {
	for i := range tips {
		localizer.localizeTextFields(&tips[i].Title, &tips[i].Text, &tips[i].NextLabel, &tips[i].SkipLabel, &tips[i].DoneLabel, &tips[i].BackLabel)
		for step := range tips[i].Steps {
			localizer.localizeTextFields(&tips[i].Steps[step].Title, &tips[i].Steps[step].Text)
		}
		localizer.localizeRendererAction(tips[i].Dismiss)
		localizer.localizeRendererAction(tips[i].Action)
		if demo := tips[i].Demo; demo != nil {
			localizer.localizeTextFields(&demo.Label)
			for item := range demo.Menu {
				localizer.localizeTextFields(&demo.Menu[item])
			}
			for person := range demo.People {
				localizer.localizeTextFields(&demo.People[person].Name)
			}
		}
	}
}
