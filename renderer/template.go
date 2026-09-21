package renderer

import "fmt"

// Template owns a private snapshot of a validated declaration. It can be shared
// between requests. As with Clone, custom objects inside interface{} must be
// immutable; only the supported JSON-like containers are copied.
type Template struct{ base Universal }

func Compile(base Universal) (*Template, error) {
	base = base.Clone()
	if err := base.Validate(); err != nil {
		return nil, err
	}
	return &Template{base: base}, nil
}

// Draft lazily copies a single response branch. List requests select either
// List or ResourceGrid. Drafts and any pointers obtained from them belong to
// one request and must not be retained or used after Build.
type Draft struct {
	template     *Template
	pageType     PageType
	value        Universal
	ready, built bool
}

func (t *Template) Draft(pageType PageType) (*Draft, error) {
	switch pageType {
	case PageTypeList, PageTypeResourceGrid, PageTypeForm, PageTypeRecord:
	default:
		return nil, fmt.Errorf("renderer: unsupported page type %q", pageType)
	}
	return &Draft{template: t, pageType: pageType}, nil
}

func (d *Draft) PageType() PageType { return d.pageType }

// Edit returns the request-owned selected branch, copying it at most once.
// Changes to its pointer fields are retained; use Replace to replace a root.
func (d *Draft) Edit() (Universal, error) {
	if d.built {
		return Universal{}, fmt.Errorf("renderer: draft already built")
	}
	if !d.ready {
		switch d.pageType {
		case PageTypeList, PageTypeResourceGrid:
			d.value.List = cloneListPage(d.template.base.List)
			d.value.ResourceGrid = cloneResourceGridPage(d.template.base.ResourceGrid)
		case PageTypeForm:
			d.value.Form = cloneFormPage(d.template.base.Form)
		case PageTypeRecord:
			d.value.Record = cloneRecordPage(d.template.base.Record)
		}
		d.ready = true
	}
	return d.value, nil
}

// Replace transfers a freshly built request-owned branch without copying the
// template first. It must not contain unrelated pages or shared mutable data.
func (d *Draft) Replace(value Universal) error {
	if d.built {
		return fmt.Errorf("renderer: draft already built")
	}
	allowed := value
	switch d.pageType {
	case PageTypeList, PageTypeResourceGrid:
		allowed.Form, allowed.Record = nil, nil
	case PageTypeForm:
		allowed.List, allowed.Record, allowed.ResourceGrid = nil, nil, nil
	case PageTypeRecord:
		allowed.List, allowed.Form, allowed.ResourceGrid = nil, nil, nil
	}
	if allowed != value {
		return fmt.Errorf("renderer: draft for %q contains unrelated pages", d.pageType)
	}
	d.value, d.ready = value, true
	return nil
}

// Build consumes the draft and validates even an unmodified selected branch.
func (d *Draft) Build() (Universal, error) {
	value, err := d.Edit()
	if err != nil {
		return Universal{}, err
	}
	d.built = true
	d.template, d.value = nil, Universal{}
	if err := value.Validate(); err != nil {
		return Universal{}, err
	}
	return value, nil
}
