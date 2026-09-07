package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
)

// AtomicCommittedRejection is a business rejection returned after an atomic
// operation has intentionally persisted state such as an attempt counter.
// It is created through constructors so producers cannot define invalid HTTP
// statuses or empty client messages.
type AtomicCommittedRejection struct {
	status  int
	message string
}

func NewAtomicCommittedRejection(message string) error {
	return NewAtomicCommittedRejectionWithStatus(http.StatusBadRequest, message)
}

func NewAtomicCommittedRejectionWithStatus(status int, message string) error {
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		status = http.StatusBadRequest
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &AtomicCommittedRejection{status: status, message: message}
}

func (rejection *AtomicCommittedRejection) Error() string   { return rejection.message }
func (rejection *AtomicCommittedRejection) StatusCode() int { return rejection.status }

func AsAtomicCommittedRejection(err error) (*AtomicCommittedRejection, bool) {
	var rejection *AtomicCommittedRejection
	if !errors.As(err, &rejection) {
		return nil, false
	}
	return rejection, true
}

// AddMode selects the persistence path for an add action.
type AddMode string

const (
	AddModeStandard AddMode = "standard"
	AddModeAtomic   AddMode = "atomic"
)

// AtomicValue is the explicit value union passed to an atomic operation.
// It avoids exposing the request's untyped JSON map to domain code.
type AtomicValue struct {
	String  *string    `json:"string,omitempty"`
	Int     *int64     `json:"int,omitempty"`
	Float   *float64   `json:"float,omitempty"`
	Bool    *bool      `json:"bool,omitempty"`
	Time    *time.Time `json:"time,omitempty"`
	Strings []string   `json:"strings,omitempty"`
	Ints    []int64    `json:"ints,omitempty"`
	JSON    []byte     `json:"json,omitempty"`
}

func AtomicString(value string) AtomicValue  { return AtomicValue{String: &value} }
func AtomicInt(value int64) AtomicValue      { return AtomicValue{Int: &value} }
func AtomicFloat(value float64) AtomicValue  { return AtomicValue{Float: &value} }
func AtomicBool(value bool) AtomicValue      { return AtomicValue{Bool: &value} }
func AtomicTime(value time.Time) AtomicValue { return AtomicValue{Time: &value} }

func (value AtomicValue) Validate() error {
	variants := 0
	if value.String != nil {
		variants++
	}
	if value.Int != nil {
		variants++
	}
	if value.Float != nil {
		variants++
	}
	if value.Bool != nil {
		variants++
	}
	if value.Time != nil {
		variants++
	}
	if value.Strings != nil {
		variants++
	}
	if value.Ints != nil {
		variants++
	}
	if value.JSON != nil {
		variants++
		if !json.Valid(value.JSON) {
			return fmt.Errorf("atomic JSON value is invalid")
		}
	}
	if variants != 1 {
		return fmt.Errorf("atomic value must contain exactly one variant")
	}
	return nil
}

func (value AtomicValue) Interface() interface{} {
	switch {
	case value.String != nil:
		return *value.String
	case value.Int != nil:
		return *value.Int
	case value.Float != nil:
		return *value.Float
	case value.Bool != nil:
		return *value.Bool
	case value.Time != nil:
		return *value.Time
	case value.Strings != nil:
		return value.Strings
	case value.Ints != nil:
		return value.Ints
	case value.JSON != nil:
		return value.JSON
	default:
		return nil
	}
}

type AtomicField struct {
	Name  string      `json:"name"`
	Value AtomicValue `json:"value"`
}

// AtomicInput contains normalized and validated add fields.
type AtomicInput struct {
	Fields []AtomicField `json:"fields"`
}

func (input AtomicInput) Field(name string) (AtomicValue, bool) {
	for _, field := range input.Fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return AtomicValue{}, false
}

func (input AtomicInput) String(name string) (string, bool) {
	value, ok := input.Field(name)
	if !ok || value.String == nil {
		return "", false
	}
	return *value.String, true
}

func (input AtomicInput) Int(name string) (int64, bool) {
	value, ok := input.Field(name)
	if !ok || value.Int == nil {
		return 0, false
	}
	return *value.Int, true
}

func (input AtomicInput) RequireString(name string) (string, error) {
	value, ok := input.String(name)
	if !ok {
		return "", fmt.Errorf("atomic input field %q is not a string", name)
	}
	return value, nil
}

// AtomicInsert is a single INSERT statement executed within the generator-owned
// transaction. The operation never receives the underlying transaction type.
type AtomicInsert struct {
	Table      pg.Table            `json:"-"`
	PrimaryKey pg.Column           `json:"-"`
	Fields     []AtomicInsertField `json:"-"`
}

type AtomicInsertField struct {
	Column pg.Column   `json:"-"`
	Value  AtomicValue `json:"value"`
}

// AtomicUpsert is a conflict-safe INSERT. With no UpdateFields it uses
// ON CONFLICT DO NOTHING; otherwise it updates the declared fields and
// returns the primary key of either path.
type AtomicUpsert struct {
	Insert          AtomicInsert        `json:"-"`
	ConflictColumns []pg.Column         `json:"-"`
	UpdateFields    []AtomicInsertField `json:"-"`
}

// AtomicUpsertResult separates a newly inserted record from a conflict path.
// A do-nothing conflict has no record; callers can then SelectOne inside the
// same generator-owned transaction.
type AtomicUpsertResult struct {
	Record   AtomicRecord `json:"record"`
	Inserted bool         `json:"inserted"`
}

// AtomicValueKind declares the SQL result shape requested by AtomicSelect.
// Keeping it explicit avoids leaking driver scan values into domain operations.
type AtomicValueKind string

const (
	AtomicValueKindString         AtomicValueKind = "string"
	AtomicValueKindNullableString AtomicValueKind = "nullable_string"
	AtomicValueKindInt            AtomicValueKind = "int"
	AtomicValueKindNullableInt    AtomicValueKind = "nullable_int"
	AtomicValueKindFloat          AtomicValueKind = "float"
	AtomicValueKindBool           AtomicValueKind = "bool"
	AtomicValueKindTime           AtomicValueKind = "time"
	AtomicValueKindStrings        AtomicValueKind = "strings"
	AtomicValueKindInts           AtomicValueKind = "ints"
)

func (kind AtomicValueKind) valid() bool {
	switch kind {
	case AtomicValueKindString, AtomicValueKindNullableString, AtomicValueKindInt, AtomicValueKindNullableInt, AtomicValueKindFloat, AtomicValueKindBool, AtomicValueKindTime, AtomicValueKindStrings, AtomicValueKindInts:
		return true
	default:
		return false
	}
}

type AtomicSelectField struct {
	Name   string          `json:"name"`
	Column pg.Column       `json:"-"`
	Kind   AtomicValueKind `json:"kind"`
}

// AtomicSelect describes a typed read executed inside the same transaction as
// the following atomic writes.
type AtomicSelect struct {
	Table  pg.Table            `json:"-"`
	Fields []AtomicSelectField `json:"-"`
	Where  pg.BoolExpression   `json:"-"`
}

// AtomicSelectMany describes a bounded, deterministic typed read executed
// inside the same transaction as the following atomic writes.
type AtomicSelectMany struct {
	AtomicSelect
	OrderBy []pg.OrderByClause `json:"-"`
	Limit   int                `json:"-"`
}

type AtomicUpdateOperation string

const (
	AtomicUpdateSet       AtomicUpdateOperation = "set"
	AtomicUpdateIncrement AtomicUpdateOperation = "increment"
)

// AtomicUpdateField is a closed assignment used by AtomicUpdate. Increment is
// deliberately restricted to numeric values so modules cannot inject SQL expressions.
type AtomicUpdateField struct {
	Column    pg.Column             `json:"-"`
	Operation AtomicUpdateOperation `json:"operation"`
	Value     AtomicValue           `json:"value"`
}

// AtomicUpdate describes typed changes to rows selected by a mandatory Jet
// predicate. The executor returns the exact number of affected rows.
type AtomicUpdate struct {
	Table  pg.Table            `json:"-"`
	Fields []AtomicUpdateField `json:"-"`
	Where  pg.BoolExpression   `json:"-"`
}

// AtomicRecord is both the atomic add response and the source for route
// interpolation. Fields are serialized at the top level of the response, so a
// renderer can resolve routes such as /profiles/{nick} from the HTTP result.
type AtomicRecord struct {
	Value      int64         `json:"value"`
	PrimaryKey string        `json:"primary_key"`
	Fields     []AtomicField `json:"fields,omitempty"`
}

func (record AtomicRecord) Validate() error {
	seen := map[string]struct{}{"value": {}, "primary_key": {}}
	for _, field := range record.Fields {
		if field.Name == "" {
			return fmt.Errorf("atomic record field name is required")
		}
		if _, exists := seen[field.Name]; exists {
			return fmt.Errorf("atomic record field %q is duplicated or reserved", field.Name)
		}
		seen[field.Name] = struct{}{}
		if err := field.Value.Validate(); err != nil {
			return fmt.Errorf("atomic record field %q: %w", field.Name, err)
		}
	}
	return nil
}

func (record AtomicRecord) MarshalJSON() ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"value":       record.Value,
		"primary_key": record.PrimaryKey,
	}
	for _, field := range record.Fields {
		result[field.Name] = atomicResponseValue(field.Value)
	}
	return json.Marshal(result)
}

func atomicResponseValue(value AtomicValue) interface{} {
	if value.JSON != nil {
		return json.RawMessage(value.JSON)
	}
	return value.Interface()
}

func (record AtomicRecord) Field(name string) (AtomicValue, bool) {
	for _, field := range record.Fields {
		if field.Name == name {
			return field.Value, true
		}
	}
	return AtomicValue{}, false
}

func (record AtomicRecord) String(name string) (string, bool) {
	value, ok := record.Field(name)
	if !ok || value.String == nil {
		return "", false
	}
	return *value.String, true
}

func (record AtomicRecord) Int(name string) (int64, bool) {
	value, ok := record.Field(name)
	if !ok || value.Int == nil {
		return 0, false
	}
	return *value.Int, true
}

// AtomicResultField declares a typed field an atomic operation may use as a
// source for generator-owned side effects. It does not add a project DTO: the
// field is still returned by AtomicRecord.Fields.
type AtomicResultField struct {
	Name string          `json:"name"`
	Kind AtomicValueKind `json:"kind"`
}

func (field AtomicResultField) Validate() error {
	if field.Name == "" {
		return fmt.Errorf("atomic result field name is required")
	}
	if field.Name == "value" || field.Name == "primary_key" {
		return fmt.Errorf("atomic result field %q is reserved", field.Name)
	}
	if !field.Kind.valid() {
		return fmt.Errorf("atomic result field %q has unsupported kind %q", field.Name, field.Kind)
	}
	return nil
}

// AtomicValueSource references a normalized input or a declared atomic result
// field. The closed source union prevents callbacks and map-based side effects.
type AtomicValueSourceScope string

const (
	AtomicValueSourceInput  AtomicValueSourceScope = "input"
	AtomicValueSourceResult AtomicValueSourceScope = "result"
)

type AtomicValueSource struct {
	Scope AtomicValueSourceScope `json:"scope"`
	Field string                 `json:"field"`
}

func (source AtomicValueSource) Validate() error {
	if source.Field == "" {
		return fmt.Errorf("atomic value source field is required")
	}
	switch source.Scope {
	case AtomicValueSourceInput, AtomicValueSourceResult:
		return nil
	default:
		return fmt.Errorf("atomic value source scope %q is unsupported", source.Scope)
	}
}

// AtomicRealtimeRecipient maps a trusted numeric result field to the
// generator-owned user topic. Input fields are intentionally not accepted so
// a request cannot select its own recipients.
type AtomicRealtimeRecipient struct {
	UserID AtomicValueSource `json:"user_id"`
}

type AtomicRealtimeCorrelation struct {
	Field  string            `json:"field"`
	Source AtomicValueSource `json:"source"`
}

// AtomicRealtimePayloadField copies a server-computed atomic result field
// into the realtime event payload. Sources are intentionally result-only: a
// producer must validate or derive the value before it reaches another user.
type AtomicRealtimePayloadField struct {
	Key    string            `json:"key"`
	Source AtomicValueSource `json:"source"`
}

// AtomicRealtimePublishConfig declares a realtime event emitted after a
// successful atomic transaction commit.
type AtomicRealtimePublishConfig struct {
	Recipients []AtomicRealtimeRecipient `json:"recipients"`
	// Roles publishes a refresh-only event to every currently connected user
	// with one of the declared roles. The realtime transport authorizes these
	// topics from the authenticated connection; a client cannot subscribe to a
	// different role. It is intended for shared resources where enumerating all
	// users on each write would be both expensive and unnecessary.
	Roles               []Role                       `json:"roles,omitempty"`
	Correlation         *AtomicRealtimeCorrelation   `json:"correlation,omitempty"`
	Payload             []AtomicRealtimePayloadField `json:"payload,omitempty"`
	SkipEmptyRecipients bool                         `json:"skip_empty_recipients,omitempty"`
}

// AtomicExecutor deliberately exposes only the operations needed by domain
// creation logic, not a driver transaction or connection.
type AtomicExecutor interface {
	Insert(context.Context, AtomicInsert) (AtomicRecord, error)
	Upsert(context.Context, AtomicUpsert) (AtomicUpsertResult, error)
	SelectOne(context.Context, AtomicSelect) (AtomicRecord, error)
	SelectMany(context.Context, AtomicSelectMany) ([]AtomicRecord, error)
	Update(context.Context, AtomicUpdate) (int64, error)
}

type AtomicAddOperation func(context.Context, AtomicExecutor, AtomicInput) (AtomicRecord, error)

type AtomicAddConfig struct {
	Operation    AtomicAddOperation            `json:"-"`
	ResultFields []AtomicResultField           `json:"result_fields,omitempty"`
	Publish      []AtomicRealtimePublishConfig `json:"publish,omitempty"`
}
