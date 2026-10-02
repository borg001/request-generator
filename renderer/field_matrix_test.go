package renderer

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A list says how many of its items stand in a row on a phone, within what a
// row can hold, and the reader gets the word.
func TestFieldMatrixListMobileColumns(t *testing.T) {
	list := func(mobile int) *FieldMatrix {
		return &FieldMatrix{Type: FieldMatrixTypeList, List: &FieldMatrixList{Fields: []string{"since"}, Columns: FieldMatrixColumnsThree, MobileColumns: mobile}}
	}
	require.NoError(t, list(0).Validate("timing"))
	require.NoError(t, list(1).Validate("timing"))
	require.EqualError(t, list(5).Validate("timing"), `renderer.Universal: matrix section "timing" list mobile columns must be 0-4`)
	require.EqualError(t, list(-1).Validate("timing"), `renderer.Universal: matrix section "timing" list mobile columns must be 0-4`)
	raw, err := json.Marshal(list(1).List)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"mobile_columns":1`)
	raw, err = json.Marshal(list(0).List)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "mobile_columns")
	copied := cloneFieldMatrix(list(1))
	assert.Equal(t, 1, copied.List.MobileColumns)
}

func TestFieldMatrixValidate(t *testing.T) {
	tests := []struct {
		name   string
		matrix *FieldMatrix
		valid  bool
	}{
		{
			name: "table",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Heads: []string{"matrix.duration", "matrix.incall", "matrix.outcall"},
					Rows:  []FieldMatrixRow{{Label: "matrix.1h", Cells: []FieldMatrixCell{{Field: "incall_1h_price"}, {Field: "outcall_1h_price"}}}},
				},
			},
			valid: true,
		},
		{
			name: "list",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeList,
				List: &FieldMatrixList{Fields: []string{"first", "second"}, Columns: FieldMatrixColumnsTwo},
			},
			valid: true,
		},
		{
			name: "table source",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Heads: []string{"Notification", "Toast"},
					Rows: []FieldMatrixRow{{
						ID:          "chat_messages",
						Label:       "Chat messages",
						Description: "Incoming chat alerts",
						Icon:        "chat",
						Tone:        "cyan",
						Cells:       []FieldMatrixCell{{Field: "toast_enabled", Label: "Toast", AvailableField: "toast_available"}},
					}},
					Source: &FieldMatrixDataSource{
						IDField:  "id",
						KeyField: "group_code",
						List:     ActionResource{Module: "notification_group_preferences", Action: "list"},
						Update:   ActionResource{Module: "notification_group_preferences", Action: "update"},
					},
				},
			},
			valid: true,
		},
		{
			name: "table source rows require ids",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Heads: []string{"Notification", "Toast"},
					Rows:  []FieldMatrixRow{{Label: "Chat messages", Cells: []FieldMatrixCell{{Field: "toast_enabled"}}}},
					Source: &FieldMatrixDataSource{
						IDField: "id", KeyField: "group_code",
						List:   ActionResource{Module: "notification_group_preferences", Action: "list"},
						Update: ActionResource{Module: "notification_group_preferences", Action: "update"},
					},
				},
			},
		},
		{
			name: "table source dynamic row",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Heads: []string{"Notification", "Toast"},
					Source: &FieldMatrixDataSource{
						IDField: "id", KeyField: "group_code",
						Row:  &FieldMatrixDataRow{LabelField: "label_key", DescriptionField: "description_key", IconField: "icon", ToneField: "tone", Cells: []FieldMatrixCell{{Field: "toast_enabled", Label: "Toast", AvailableField: "toast_available"}}},
						List: ActionResource{Module: "notification_group_preferences", Action: "list"}, Update: ActionResource{Module: "notification_group_preferences", Action: "update"},
					},
				},
			},
			valid: true,
		},
		{
			name: "accordion presentation",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Presentation: FieldMatrixTablePresentationAccordion,
					Heads:        []string{"Notification", "Toast"},
					Rows: []FieldMatrixRow{{
						Label: "Chat messages", Icon: "chat", Tone: "cyan",
						Cells: []FieldMatrixCell{{Field: "toast_enabled", Icon: "toast"}},
					}},
				},
			},
			valid: true,
		},
		{
			name: "table cell must select one value source",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeTable,
				Table: &FieldMatrixTable{
					Heads: []string{"matrix.duration"},
					Rows:  []FieldMatrixRow{{Cells: []FieldMatrixCell{{Field: "price", Text: "matrix.price"}}}},
				},
			},
		},
		{
			name: "list columns are closed enum",
			matrix: &FieldMatrix{
				Type: FieldMatrixTypeList,
				List: &FieldMatrixList{Fields: []string{"first"}, Columns: 5},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.matrix.Validate("rates")
			assert.Equal(t, test.valid, err == nil, err)
		})
	}
}

func TestUniversalValidateFieldMatrixRendererBinding(t *testing.T) {
	tests := []struct {
		name    string
		section FormSection
		valid   bool
	}{
		{
			name:    "matrix renderer without matrix",
			section: FormSection{ID: "rates", Renderer: RendererFieldMatrix},
		},
		{
			name: "matrix with another renderer",
			section: FormSection{ID: "rates", Renderer: RendererUniversalSection, Matrix: &FieldMatrix{
				Type: FieldMatrixTypeList,
				List: &FieldMatrixList{Fields: []string{"price"}, Columns: FieldMatrixColumnsOne},
			}},
		},
		{
			name: "matrix renderer with matrix",
			section: FormSection{ID: "rates", Renderer: RendererFieldMatrix, Matrix: &FieldMatrix{
				Type: FieldMatrixTypeList,
				List: &FieldMatrixList{Fields: []string{"price"}, Columns: FieldMatrixColumnsOne},
			}},
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := (Universal{Form: &FormPage{Sections: []FormSection{test.section}}}).Validate()
			assert.Equal(t, test.valid, err == nil, err)
		})
	}
}

func TestUniversalValidateFormSectionColumns(t *testing.T) {
	for _, columns := range []FieldMatrixColumnCount{0, FieldMatrixColumnsOne, FieldMatrixColumnsTwo, FieldMatrixColumnsThree, FieldMatrixColumnsFour} {
		t.Run("valid", func(t *testing.T) {
			err := (Universal{Form: &FormPage{Sections: []FormSection{{ID: "preferences", Columns: columns}}}}).Validate()
			require.NoError(t, err)
		})
	}

	err := (Universal{Form: &FormPage{Sections: []FormSection{{ID: "preferences", Columns: 5}}}}).Validate()
	require.EqualError(t, err, `renderer.Universal: form section "preferences" has unsupported columns`)
}

func TestFormSectionColumnsJSONAndClone(t *testing.T) {
	original := Universal{Form: &FormPage{Sections: []FormSection{{
		ID:      "payments",
		Fields:  []string{"accepted_payment", "commission_rate"},
		Columns: FieldMatrixColumnsOne,
	}}}}

	encoded, err := json.Marshal(original.Form.Sections[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"payments","fields":["accepted_payment","commission_rate"],"columns":1}`, string(encoded))

	cloned := original.Clone()
	require.Equal(t, FieldMatrixColumnsOne, cloned.Form.Sections[0].Columns)
}

func TestFormSectionResourceIsServerOnlyAndCloneSafe(t *testing.T) {
	source := Universal{Form: &FormPage{Sections: []FormSection{{
		ID:       "notifications",
		Renderer: RendererUniversalSection,
		Resource: &Resource{
			ActionResource: ActionResource{Module: "preferences", Action: "view"},
			Bindings: []RequestBinding{{
				Target: RequestBindingPathByKey,
				Source: ValueSource{Literal: &TypedValue{Type: TypedValueString, String: "user_id"}},
			}},
		},
		Load: &ResourceLoad{Request: APIAction{Method: "GET", Endpoint: "/api/preferences/view/:bykey/:value"}},
	}}}}

	encoded, err := json.Marshal(source.Form.Sections[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"notifications","renderer":"universal.section","load":{"request":{"method":"GET","endpoint":"/api/preferences/view/:bykey/:value"}}}`, string(encoded))

	cloned := source.Clone()
	cloned.Form.Sections[0].Resource.Bindings[0].Source.Literal.String = "changed"
	cloned.Form.Sections[0].Load.Request.Endpoint = "/changed"
	require.Equal(t, "user_id", source.Form.Sections[0].Resource.Bindings[0].Source.Literal.String)
	require.Equal(t, "/api/preferences/view/:bykey/:value", source.Form.Sections[0].Load.Request.Endpoint)
}

func TestFieldMatrixClone(t *testing.T) {
	original := Universal{Form: &FormPage{Sections: []FormSection{{
		ID: "rates",
		Matrix: &FieldMatrix{
			Type:  FieldMatrixTypeTable,
			Table: &FieldMatrixTable{Heads: []string{"matrix.duration"}, Rows: []FieldMatrixRow{{Cells: []FieldMatrixCell{{Field: "price"}}}}},
		},
	}}}}
	form := original.Clone().Form

	form.Sections[0].Matrix.Table.Heads[0] = "changed"
	assert.Equal(t, "matrix.duration", original.Form.Sections[0].Matrix.Table.Heads[0])
}

func TestFieldMatrixSourceCloneAndJSON(t *testing.T) {
	source := Universal{Form: &FormPage{Sections: []FormSection{{
		ID: "notifications",
		Matrix: &FieldMatrix{Type: FieldMatrixTypeTable, Table: &FieldMatrixTable{
			Heads: []string{"Type", "Toast"},
			Rows:  []FieldMatrixRow{{ID: "chat", Label: "Chat", Description: "Messages", Icon: "chat", Tone: "cyan", Cells: []FieldMatrixCell{{Field: "toast_enabled", Label: "Toast", AvailableField: "toast_available"}}}},
			Source: &FieldMatrixDataSource{
				IDField: "id", KeyField: "group_code",
				List: ActionResource{Module: "groups", Action: "list"}, Update: ActionResource{Module: "groups", Action: "update"},
				Load: &FieldMatrixDataSourceLoad{List: ResourceLoad{Request: APIAction{Method: "GET", Endpoint: "/api/groups"}}, Update: ResourceLoad{Request: APIAction{Method: "POST", Endpoint: "/api/groups/:bykey/:value"}}},
			},
		}},
	}}}}

	encoded, err := json.Marshal(source.Form.Sections[0].Matrix)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"table","table":{"heads":["Type","Toast"],"rows":[{"id":"chat","label":"Chat","description":"Messages","icon":"chat","tone":"cyan","cells":[{"field":"toast_enabled","label":"Toast","available_field":"toast_available"}]}],"source":{"id_field":"id","key_field":"group_code","load":{"list":{"request":{"method":"GET","endpoint":"/api/groups"}},"update":{"request":{"method":"POST","endpoint":"/api/groups/:bykey/:value"}}}}}}`, string(encoded))

	cloned := source.Clone()
	cloned.Form.Sections[0].Matrix.Table.Source.Load.Update.Request.Endpoint = "/changed"
	cloned.Form.Sections[0].Matrix.Table.Rows[0].Description = "changed"
	require.Equal(t, "/api/groups/:bykey/:value", source.Form.Sections[0].Matrix.Table.Source.Load.Update.Request.Endpoint)
	require.Equal(t, "Messages", source.Form.Sections[0].Matrix.Table.Rows[0].Description)
}

func TestFieldMatrixCellEnabledIf(t *testing.T) {
	on := true
	source := Universal{Form: &FormPage{Sections: []FormSection{{
		ID: "notifications",
		Matrix: &FieldMatrix{Type: FieldMatrixTypeTable, Table: &FieldMatrixTable{
			Heads: []string{"Type", "Push"},
			Source: &FieldMatrixDataSource{
				IDField: "id", KeyField: "group_code",
				List: ActionResource{Module: "groups", Action: "list"}, Update: ActionResource{Module: "groups", Action: "update"},
				Row: &FieldMatrixDataRow{LabelField: "label_key", Cells: []FieldMatrixCell{{
					Field: "push_enabled", AvailableField: "push_available",
					EnabledIf: &Condition{Path: "record.push_enabled", Truthy: &on},
				}}},
			},
		}},
	}}}}

	encoded, err := json.Marshal(source.Form.Sections[0].Matrix.Table.Source.Row.Cells[0])
	require.NoError(t, err)
	require.JSONEq(t, `{"field":"push_enabled","available_field":"push_available","enabled_if":{"path":"record.push_enabled","truthy":true}}`, string(encoded))

	cloned := source.Clone()
	cloned.Form.Sections[0].Matrix.Table.Source.Row.Cells[0].EnabledIf.Path = "record.changed"
	require.Equal(t, "record.push_enabled", source.Form.Sections[0].Matrix.Table.Source.Row.Cells[0].EnabledIf.Path)

	// A condition is about a cell's value, so a text cell may not carry one.
	err = validateFieldMatrixCells("notifications", 0, 2, true, []FieldMatrixCell{{Text: "matrix.note", EnabledIf: &Condition{Path: "record.push_enabled", Truthy: &on}}})
	require.ErrorContains(t, err, "enabled condition requires field")
}

func TestCardMediaPreviewIsSaid(t *testing.T) {
	encoded, err := json.Marshal(Media{Field: "identity_video_media", Size: MediaSizeThumb, Preview: true})
	require.NoError(t, err)
	require.JSONEq(t, `{"field":"identity_video_media","size":"thumb","preview":true}`, string(encoded))
	encoded, err = json.Marshal(Media{Field: "avatar"})
	require.NoError(t, err)
	require.JSONEq(t, `{"field":"avatar"}`, string(encoded))
}

func TestFieldPresentationCalendarMarks(t *testing.T) {
	presentation := &FieldPresentation{Renderer: RendererKey("calendar"), CalendarMarks: []CalendarMark{
		{Field: "expected_arrival_date", Label: "tours.calendar.expected_arrival", Tone: "cyan"},
		{Field: "other_tour_starts", Label: "tours.calendar.other_tours", Tone: "amber"},
	}}
	require.NoError(t, presentation.Validate())
	encoded, err := json.Marshal(presentation)
	require.NoError(t, err)
	require.JSONEq(t, `{"renderer":"calendar","calendar_marks":[{"field":"expected_arrival_date","label":"tours.calendar.expected_arrival","tone":"cyan"},{"field":"other_tour_starts","label":"tours.calendar.other_tours","tone":"amber"}]}`, string(encoded))

	cloned := CloneFieldPresentation(presentation)
	cloned.CalendarMarks[0].Label = "changed"
	require.Equal(t, "tours.calendar.expected_arrival", presentation.CalendarMarks[0].Label)

	require.ErrorContains(t, (&FieldPresentation{CalendarMarks: []CalendarMark{{Label: "x"}}}).Validate(), "calendar mark 0 needs a field")
}
