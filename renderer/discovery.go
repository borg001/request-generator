package renderer

import "fmt"

// Discovery describes route capabilities without page contents. It is a value
// (no pointers, slices or maps), so request callbacks can safely change it.
// Renderer identity/version is always supplied by the generator.
type Discovery struct {
	List   PageType
	Form   bool
	Record bool
}

func (r Universal) Discovery() Discovery {
	return Discovery{List: r.ListRoutePageType(), Form: r.Form != nil, Record: r.Record != nil}
}

func (d Discovery) Validate() error {
	if d.List != "" && d.List != PageTypeList && d.List != PageTypeResourceGrid {
		return fmt.Errorf("renderer.Discovery: invalid list page type %q", d.List)
	}
	return nil
}

func (d Discovery) ListIdentity() *Identity     { return discoveryIdentity(d.List != "") }
func (d Discovery) FormIdentity() *Identity     { return discoveryIdentity(d.Form) }
func (d Discovery) RecordIdentity() *Identity   { return discoveryIdentity(d.Record) }
func (d Discovery) ListRoutePageType() PageType { return d.List }
func (d Discovery) FormRoutePageType() PageType {
	if d.Form {
		return PageTypeForm
	}
	return ""
}
func (d Discovery) RecordRoutePageType() PageType {
	if d.Record {
		return PageTypeRecord
	}
	return ""
}
func discoveryIdentity(present bool) *Identity {
	if !present {
		return nil
	}
	identity := UniversalIdentity()
	return &identity
}
