package response

import (
	f "github.com/darkrain/request-generator/fields"
	"github.com/darkrain/request-generator/renderer"
)

type DefrecResponse struct {
	Renderer *renderer.Identity     `json:"renderer,omitempty"`
	FormPage *renderer.FormPage     `json:"form_page,omitempty"`
	Fields   map[string]interface{} `json:"fields"`
}

func NewDefrecResponse(fields []f.ModuleField) DefrecResponse {
	fieldsMap := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		item := map[string]interface{}{
			"title":     field.Title,
			"type":      string(field.Type),
			"form_type": string(field.FormType),
		}
		if field.Example != "" {
			item["example"] = field.Example
		}
		if len(field.Options) > 0 {
			item["options"] = field.Options
		}
		if field.OptionsSource != nil {
			item["options_source"] = field.OptionsSource
		}
		if field.Presentation != nil {
			item["presentation"] = field.Presentation
		}
		if field.Media != nil {
			item["media"] = field.Media
		}
		if field.Section != "" {
			item["section"] = field.Section
		}
		if len(field.Roles) > 0 {
			item["roles"] = field.Roles
		}
		for _, rule := range field.Check {
			if intr, ok := rule.(f.CheckRuleIntrospectable); ok {
				info := intr.RuleInfo()
				for _, scenario := range info.Scenarios {
					if scenario != f.ScenarioAdd {
						continue
					}
					switch info.Type {
					case "required":
						item["required"] = true
					case "length":
						item["min_length"] = info.Min
						item["max_length"] = info.Max
					case "range":
						// A control that knows the range it accepts can hold a
						// value inside it instead of refusing one outside it.
						item["min"] = info.Min
						item["max"] = info.Max
					}
					break
				}
			}
		}
		key := field.ColumnName()
		if field.Translatable {
			key = field.Name()
			item["translatable"] = true
		}
		fieldsMap[key] = item
	}

	return DefrecResponse{Fields: fieldsMap}
}

func (r *DefrecResponse) AttachRender(render renderer.Universal) {
	r.Renderer = render.FormIdentity()
	r.FormPage = render.Form
}
