package fields

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/darkrain/request-generator/locale"
	"github.com/darkrain/request-generator/renderer"
	"github.com/gin-gonic/gin"
	pg "github.com/go-jet/jet/v2/postgres"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

const (
	ErrorUnknownType     string = "Unknown type"
	ErrorUnknownFormType string = "Unknown formType"
)

type ModuleFieldType string

const (
	ModuleFieldTypeString ModuleFieldType = "string"
	ModuleFieldTypeInt    ModuleFieldType = "int"
	ModuleFieldTypeFloat  ModuleFieldType = "float"
	ModuleFieldTypeBool   ModuleFieldType = "bool"
	ModuleFieldTypeArray  ModuleFieldType = "array"
	ModuleFieldTypeObject ModuleFieldType = "object"
)

func ModuleFieldTypeOf(value string) (ModuleFieldType, error) {
	switch value {
	case string(ModuleFieldTypeString):
		return ModuleFieldTypeString, nil
	case string(ModuleFieldTypeInt):
		return ModuleFieldTypeInt, nil
	case string(ModuleFieldTypeFloat):
		return ModuleFieldTypeFloat, nil
	case string(ModuleFieldTypeBool):
		return ModuleFieldTypeBool, nil
	case string(ModuleFieldTypeArray):
		return ModuleFieldTypeArray, nil
	case string(ModuleFieldTypeObject):
		return ModuleFieldTypeObject, nil
	}
	return ModuleFieldTypeString, errors.New(ErrorUnknownType)
}

// ModuleFieldArrayStorage controls how a typed array is persisted. It does
// not affect the renderer contract: both variants are returned as `array`.
type ModuleFieldArrayStorage string

const (
	ModuleFieldArrayStoragePostgres ModuleFieldArrayStorage = "postgres_array"
	ModuleFieldArrayStorageJSON     ModuleFieldArrayStorage = "json"
)

func (storage ModuleFieldArrayStorage) Normalize() ModuleFieldArrayStorage {
	if storage == "" {
		return ModuleFieldArrayStoragePostgres
	}
	return storage
}

func (storage ModuleFieldArrayStorage) Validate() error {
	switch storage.Normalize() {
	case ModuleFieldArrayStoragePostgres, ModuleFieldArrayStorageJSON:
		return nil
	default:
		return fmt.Errorf("unsupported array storage %q", storage)
	}
}

// MarshalJSONArray validates and serializes a value for a JSON/JSONB array
// column. Keeping this beside the field contract prevents producer-level
// converters from defining incompatible array encodings.
func MarshalJSONArray(value interface{}) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(string(encoded)), "[") {
		return nil, fmt.Errorf("expected JSON array")
	}
	return encoded, nil
}

// MarshalJSONObject validates and serializes a value for a JSON/JSONB object
// column. A JSON object provided as a string stays JSON rather than becoming
// a quoted JSON string, which preserves defaults such as "{}".
func MarshalJSONObject(value interface{}) ([]byte, error) {
	if raw, ok := value.(string); ok {
		encoded := []byte(strings.TrimSpace(raw))
		if !strings.HasPrefix(string(encoded), "{") {
			return nil, fmt.Errorf("expected JSON object")
		}
		var object map[string]interface{}
		if err := json.Unmarshal(encoded, &object); err != nil {
			return nil, err
		}
		return encoded, nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(string(encoded)), "{") {
		return nil, fmt.Errorf("expected JSON object")
	}
	return encoded, nil
}

type ModuleFieldFormType string

const (
	ModuleFieldFormTypeText        ModuleFieldFormType = "text"
	ModuleFieldFormTypeTime        ModuleFieldFormType = "time"
	ModuleFieldFormTypeNumber      ModuleFieldFormType = "number"
	ModuleFieldFormTypeTextArea    ModuleFieldFormType = "textarea"
	ModuleFieldFormTypeSelect      ModuleFieldFormType = "select"
	ModuleFieldFormTypeCheckBox    ModuleFieldFormType = "checkbox"
	ModuleFieldFormTypeMultiselect ModuleFieldFormType = "multiselect"
	ModuleFieldFormTypeMap         ModuleFieldFormType = "map"
	ModuleFieldFormTypeHidden      ModuleFieldFormType = "hidden"
	ModuleFieldFormTypeOnlyView    ModuleFieldFormType = "onlyview"
)

func ModuleFieldFormTypeOf(value string) (ModuleFieldFormType, error) {
	switch value {
	case string(ModuleFieldFormTypeText):
		return ModuleFieldFormTypeMap, nil
	case string(ModuleFieldFormTypeTime):
		return ModuleFieldFormTypeTime, nil
	case string(ModuleFieldFormTypeNumber):
		return ModuleFieldFormTypeNumber, nil
	case string(ModuleFieldFormTypeTextArea):
		return ModuleFieldFormTypeTextArea, nil
	case string(ModuleFieldFormTypeSelect):
		return ModuleFieldFormTypeSelect, nil
	case string(ModuleFieldFormTypeCheckBox):
		return ModuleFieldFormTypeCheckBox, nil
	case string(ModuleFieldFormTypeMultiselect):
		return ModuleFieldFormTypeMultiselect, nil
	case string(ModuleFieldFormTypeMap):
		return ModuleFieldFormTypeMap, nil
	case string(ModuleFieldFormTypeHidden):
		return ModuleFieldFormTypeHidden, nil
	case string(ModuleFieldFormTypeOnlyView):
		return ModuleFieldFormTypeOnlyView, nil
	}
	return ModuleFieldFormTypeMap, errors.New(ErrorUnknownFormType)
}

type Scenario string

const (
	ScenarioAdd    Scenario = "add"
	ScenarioUpdate Scenario = "update"
)

type RoleCheck struct {
	Role  string
	Rules []CheckRules
}

type RoleOptions struct {
	Role    string
	Options []ModuleFieldOptions
}

type ModuleField struct {
	Column           pg.Column     `json:"-"`
	SelectExpression pg.Projection `json:"-"`
	// SelectExpressionFunc resolves a request-scoped projection without
	// mutating shared module metadata. It is useful for computed fields whose
	// value depends on the authenticated caller.
	SelectExpressionFunc func(c *gin.Context) pg.Projection              `json:"-"`
	Title                string                                          `json:"title"`
	Titles               map[string]string                               `json:"-"`
	Type                 ModuleFieldType                                 `json:"type"`
	FormType             ModuleFieldFormType                             `json:"form_type,omitempty"`
	Example              string                                          `json:"example,omitempty"`
	AllLabel             string                                          `json:"all_label,omitempty"`
	ArrayStorage         ModuleFieldArrayStorage                         `json:"-"`
	Presentation         *renderer.FieldPresentation                     `json:"presentation,omitempty"`
	Media                *renderer.FieldMediaConfig                      `json:"media,omitempty"`
	Options              []ModuleFieldOptions                            `json:"options,omitempty"`
	OptionsSource        *FieldOptionsSource                             `json:"options_source,omitempty"`
	OptionsFunc          func(context *gin.Context) []ModuleFieldOptions `json:"-"`
	RoleOptions          []RoleOptions                                   `json:"-"`
	Check                []CheckRules                                    `json:"-"`
	CheckFunc            func(context *gin.Context) []CheckRules         `json:"-"`
	RoleCheck            []RoleCheck                                     `json:"-"`
	// DefaultFunc is called during Add when the field is absent from the request body.
	// The returned value is injected into the input before validation and DB insert.
	DefaultFunc          func(c *gin.Context) interface{}                             `json:"-"`
	Convert              func(c *gin.Context, value interface{}) (interface{}, error) `json:"-"`
	ResultValueConverter func(value interface{}) interface{}                          `json:"-"`
	// ResultValueLocalizer converts a parsed result value into the language of
	// the current request. It runs after the database layer and therefore does
	// not make DB executors depend on request or locale state.
	ResultValueLocalizer func(value interface{}, translate func(key string, fallback string) string) interface{} `json:"-"`
	Translatable         bool                                                                                    `json:"-"`
	// Group and Order are producer-only inputs used to build typed renderer
	// composition. They are never serialized as field metadata.
	Group           string                         `json:"-"`
	Order           int                            `json:"-"`
	FieldName       string                         `json:"-"`
	FilterCondition func(c *gin.Context) bool      `json:"-"`
	Roles           []string                       `json:"roles,omitempty"`
	Section         string                         `json:"section,omitempty"`
	RoleSection     map[string]string              `json:"-"`
	RoleFormType    map[string]ModuleFieldFormType `json:"-"`
	// TitleFunc names the field for the request at hand. The same column can
	// mean different things to different subjects - a person has a name, an
	// organisation has a title - and only the module knows which is being
	// edited. It returns a translation key; an empty result keeps Title.
	TitleFunc func(c *gin.Context) string `json:"-"`
}

// ColumnName returns the database column name from the Jet column.
func (f ModuleField) ColumnName() string {
	if f.Column != nil {
		return f.Column.Name()
	}
	return ""
}

// Name returns the logical field name. For translatable fields this is
// FieldName (e.g. "name"); for regular fields it falls back to ColumnName().
func (f ModuleField) Name() string {
	if f.FieldName != "" {
		return f.FieldName
	}
	return f.ColumnName()
}

// GetProjection returns the SELECT expression for this field.
// If SelectExpression is set (e.g. a function wrapper), it is used instead of the raw column.
func (f ModuleField) GetProjection() pg.Projection {
	if f.SelectExpression != nil {
		return f.SelectExpression
	}
	return f.Column
}

// ResolveProjection returns an independent field value with a request-scoped
// projection, when configured. The original module field remains immutable and
// can therefore be reused safely by concurrent requests.
func (f ModuleField) ResolveProjection(c *gin.Context) ModuleField {
	if f.SelectExpressionFunc != nil {
		f.SelectExpression = f.SelectExpressionFunc(c)
	}
	return f
}

// ResolveProjections returns request-scoped copies of module fields. It is
// called by read actions immediately before building a database query.
func ResolveProjections(c *gin.Context, values []ModuleField) []ModuleField {
	if len(values) == 0 {
		return nil
	}
	out := make([]ModuleField, len(values))
	for i, value := range values {
		out[i] = value.ResolveProjection(c)
	}
	return out
}

// NewScanValue returns a fresh sql scan destination appropriate for this column's type.
func (f ModuleField) NewScanValue() interface{} {
	switch f.Column.(type) {
	case pg.ColumnBool:
		return &sql.NullBool{}
	case pg.ColumnInteger:
		return &sql.NullInt64{}
	case pg.ColumnFloat:
		return &sql.NullFloat64{}
	case pg.ColumnTimestamp, pg.ColumnTimestampz, pg.ColumnDate, pg.ColumnTime, pg.ColumnTimez:
		return &sql.NullTime{}
	default:
		return &sql.NullString{}
	}
}

type ModuleFilterField struct {
	Column          pg.Column                                                    `json:"-"`
	FieldName       string                                                       `json:"-"`
	Title           string                                                       `json:"title"`
	Titles          map[string]string                                            `json:"-"`
	Type            ModuleFieldType                                              `json:"type"`
	FormType        ModuleFieldFormType                                          `json:"form_type,omitempty"`
	Example         string                                                       `json:"example,omitempty"`
	AllLabel        string                                                       `json:"all_label,omitempty"`
	Options         []ModuleFieldOptions                                         `json:"options,omitempty"`
	OptionsSource   *FieldOptionsSource                                          `json:"options_source,omitempty"`
	Check           []CheckRules                                                 `json:"-"`
	Convert         func(c *gin.Context, value interface{}) (interface{}, error) `json:"-"`
	FilterCondition func(c *gin.Context) bool                                    `json:"-"`
}

func (f ModuleFilterField) ColumnName() string {
	if f.Column != nil {
		return f.Column.Name()
	}
	return ""
}

type ModuleFieldOptions struct {
	Value       interface{}       `json:"value"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Labels      map[string]string `json:"-"`
}

type FieldOptionsSourceMode string

const (
	FieldOptionsSourceModeList FieldOptionsSourceMode = "list"
	FieldOptionsSourceModeTree FieldOptionsSourceMode = "tree"
)

type FieldOptionsQueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type FieldOptionsSource struct {
	Endpoint    string                   `json:"endpoint"`
	Query       []FieldOptionsQueryParam `json:"query,omitempty"`
	SearchParam string                   `json:"search_param,omitempty"`
	Mode        FieldOptionsSourceMode   `json:"mode,omitempty"`
}

func (source *FieldOptionsSource) Validate() error {
	if source == nil {
		return nil
	}
	if source.Endpoint == "" {
		return fmt.Errorf("field options source endpoint is required")
	}
	switch source.Mode {
	case "", FieldOptionsSourceModeList, FieldOptionsSourceModeTree:
	default:
		return fmt.Errorf("field options source has unsupported mode %q", source.Mode)
	}
	for _, param := range source.Query {
		if param.Key == "" {
			return fmt.Errorf("field options source query key is required")
		}
	}
	return nil
}

type CheckRules interface {
	Validate(obj interface{}, lang string) error
	GetScenarios() []Scenario
}

// DataCheckRule is a CheckRules variant that has access to the full input map,
// gin.Context, and the raw *sql.DB. Use it for composite validation spanning
// multiple fields (e.g. uniqueness across (who_add, whom_add, tag)).
// Implementors must also satisfy CheckRules to be stored in a Check slice;
// the Validate method can be a no-op since the generator calls ValidateData instead.
type DataCheckRule interface {
	CheckRules
	ValidateData(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error
}

// dataRule is a function-based DataCheckRule for inline use.
type dataRule struct {
	fn        func(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error
	scenarios []Scenario
}

func (r dataRule) Validate(_ interface{}, _ string) error { return nil }
func (r dataRule) GetScenarios() []Scenario               { return r.scenarios }
func (r dataRule) ValidateData(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error {
	return r.fn(c, db, data, lang)
}

// DataRule creates a CheckRules entry for composite, context-aware validation.
// The fn receives gin.Context, *sql.DB, and the full input map — suitable for
// multi-field uniqueness checks or any validation that needs DB access.
func DataRule(fn func(c *gin.Context, db *sql.DB, data map[string]interface{}, lang string) error, scenarios []Scenario) CheckRules {
	return dataRule{fn: fn, scenarios: scenarios}
}

// RuleInfo holds validation rule metadata for OpenAPI spec generation.
type RuleInfo struct {
	Type      string // "required", "in", "length", "url", "email"
	Field     string
	Values    []interface{} // for "in" rules
	Min       int           // for "length" rules
	Max       int           // for "length" rules
	Scenarios []Scenario
}

// CheckRuleIntrospectable exposes rule metadata for documentation generation.
type CheckRuleIntrospectable interface {
	RuleInfo() RuleInfo
}

func RequiredRule(field pg.Column, scenarios []Scenario) requiredRule {
	return requiredRule{
		Field:     field,
		Scenarios: scenarios,
	}
}

func InRule(field pg.Column, values []interface{}, scenarios []Scenario) inRule {
	return inRule{
		Field:     field,
		Values:    values,
		Scenarios: scenarios,
	}
}

func InDBRule(field pg.Column, values func() []interface{}, scenarios []Scenario) inRule {
	return inRule{
		Field:     field,
		Values:    values(),
		Scenarios: scenarios,
	}
}

func LenRule(field pg.Column, min int, max int, scenarios []Scenario) lengthRule {
	return lengthRule{
		Min:       min,
		Max:       max,
		Field:     field,
		Scenarios: scenarios,
	}
}

func UrlRule(field pg.Column, scenarios []Scenario) urlRule {
	return urlRule{
		Field:     field,
		Scenarios: scenarios,
	}
}

type requiredRule struct {
	CheckRules `json:"-"`
	Field      pg.Column  `json:"-"`
	Scenarios  []Scenario `json:"scenarios"`
}

type inRule struct {
	Field     pg.Column     `json:"-"`
	Values    []interface{} `json:"values"`
	Scenarios []Scenario    `json:"scenarios"`
}

type emailRule struct {
	CheckRules `json:"-"`
	Type       string     `json:"type"`
	Field      pg.Column  `json:"-"`
	Scenarios  []Scenario `json:"scenarios"`
}

type urlRule struct {
	CheckRules `json:"-"`
	Type       string     `json:"type"`
	Field      pg.Column  `json:"-"`
	Scenarios  []Scenario `json:"scenarios"`
}

type lengthRule struct {
	CheckRules `json:"-"`
	Type       string     `json:"type"`
	Min        int        `json:"min"`
	Max        int        `json:"max"`
	Field      pg.Column  `json:"-"`
	Scenarios  []Scenario `json:"scenarios"`
}

func (rule requiredRule) GetScenarios() []Scenario {
	return rule.Scenarios
}

func (rule inRule) GetScenarios() []Scenario {
	return rule.Scenarios
}

func (rule emailRule) GetScenarios() []Scenario {
	return rule.Scenarios
}

func (rule urlRule) GetScenarios() []Scenario {
	return rule.Scenarios
}

func (rule lengthRule) GetScenarios() []Scenario {
	return rule.Scenarios
}

func (rule requiredRule) RuleInfo() RuleInfo {
	return RuleInfo{Type: "required", Field: rule.Field.Name(), Scenarios: rule.Scenarios}
}

func (rule requiredRule) Validate(obj interface{}, lang string) error {
	return validation.Required.Error(fmt.Sprintf(locale.Message(locale.Lang(lang), "required"), rule.Field.Name())).Validate(obj)
}

func (rule inRule) RuleInfo() RuleInfo {
	return RuleInfo{Type: "in", Field: rule.Field.Name(), Values: rule.Values, Scenarios: rule.Scenarios}
}

func (rule inRule) Validate(obj interface{}, lang string) error {
	if obj == nil {
		return nil
	}
	stringValues := make([]interface{}, 0, 10)
	for _, validationVal := range rule.Values {
		stringValues = append(stringValues, fmt.Sprintf("%v", validationVal))
	}
	return validation.In(stringValues...).Error(fmt.Sprintf(locale.Message(locale.Lang(lang), "in"), rule.Field.Name(), rule.Values)).Validate(fmt.Sprintf("%v", obj))
}

func (rule emailRule) RuleInfo() RuleInfo {
	return RuleInfo{Type: "email", Field: rule.Field.Name(), Scenarios: rule.Scenarios}
}

func (rule emailRule) Validate(obj interface{}, lang string) error {
	return is.Email.Error(fmt.Sprintf(locale.Message(locale.Lang(lang), "email"), rule.Field.Name())).Validate(obj)
}

func (rule urlRule) RuleInfo() RuleInfo {
	return RuleInfo{Type: "url", Field: rule.Field.Name(), Scenarios: rule.Scenarios}
}

func (rule urlRule) Validate(obj interface{}, lang string) error {
	return is.URL.Error(fmt.Sprintf(locale.Message(locale.Lang(lang), "url"), rule.Field.Name())).Validate(obj)
}

func (rule lengthRule) RuleInfo() RuleInfo {
	return RuleInfo{Type: "length", Field: rule.Field.Name(), Min: rule.Min, Max: rule.Max, Scenarios: rule.Scenarios}
}

func (rule lengthRule) Validate(obj interface{}, lang string) error {
	return validation.Length(
		rule.Min,
		rule.Max,
	).Error(
		fmt.Sprintf(locale.Message(locale.Lang(lang), "length"), rule.Field.Name(), rule.Min, rule.Max),
	).Validate(obj)
}

// ContainsColumn checks if a column is present in the list by name.
func ContainsColumn(columns []pg.Column, target pg.Column) bool {
	targetName := target.Name()
	for _, c := range columns {
		if c.Name() == targetName {
			return true
		}
	}
	return false
}
