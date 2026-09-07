package renderer

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type Universal struct {
	List         *ListPage         `json:"list_page,omitempty"`
	Form         *FormPage         `json:"form_page,omitempty"`
	Record       *RecordPage       `json:"record_page,omitempty"`
	ResourceGrid *ResourceGridPage `json:"resource_grid_page,omitempty"`
}

func (r Universal) Identity() *Identity {
	if r.IsZero() {
		return nil
	}
	identity := UniversalIdentity()
	return &identity
}

func (r Universal) ListIdentity() *Identity {
	if r.List == nil && r.ResourceGrid == nil {
		return nil
	}
	return r.Identity()
}

func (r Universal) FormIdentity() *Identity {
	if r.Form == nil {
		return nil
	}
	return r.Identity()
}

func (r Universal) RecordIdentity() *Identity {
	if r.Record == nil {
		return nil
	}
	return r.Identity()
}

func (r Universal) ListRoutePageType() PageType {
	if r.List != nil {
		return PageTypeList
	}
	if r.ResourceGrid != nil {
		return PageTypeResourceGrid
	}
	return ""
}

func (r Universal) FormRoutePageType() PageType {
	if r.Form != nil {
		return PageTypeForm
	}
	return ""
}

func (r Universal) RecordRoutePageType() PageType {
	if r.Record != nil {
		return PageTypeRecord
	}
	return ""
}

func (r Universal) IsZero() bool {
	return r.List == nil && r.Form == nil && r.Record == nil && r.ResourceGrid == nil
}

func (r Universal) Validate() error {
	if r.List != nil && r.ResourceGrid != nil {
		return fmt.Errorf("renderer.Universal: List and ResourceGrid are mutually exclusive for one list route")
	}
	if r.Form != nil {
		if err := validateActions("form page", r.Form.Actions); err != nil {
			return err
		}
		if err := validateFormWorkflow(r.Form); err != nil {
			return err
		}
		for _, section := range r.Form.Sections {
			if err := validateFormSectionActions(r.Form, section); err != nil {
				return err
			}
			if err := validateFormSectionColumns(section); err != nil {
				return err
			}
			if err := validateDateRangeSection(r.Form, section); err != nil {
				return err
			}
			if err := section.Prompts.Validate(); err != nil {
				return fmt.Errorf("renderer.Universal: form section %q prompts: %w", section.ID, err)
			}
			if section.Resource != nil {
				if section.Renderer != RendererUniversalSection {
					return fmt.Errorf("renderer.Universal: resource section %q requires renderer %q", section.ID, RendererUniversalSection)
				}
				if err := section.Resource.Validate("resource"); err != nil {
					return fmt.Errorf("renderer.Universal: resource section %q: %w", section.ID, err)
				}
			}
			if section.ListPage != nil {
				if err := validateListPage("form section list page", section.ListPage); err != nil {
					return err
				}
			}
			if section.Renderer == RendererFieldMatrix && section.Matrix == nil {
				return fmt.Errorf("renderer.Universal: field matrix section %q must define matrix", section.ID)
			}
			if section.Renderer != RendererFieldMatrix && section.Matrix != nil {
				return fmt.Errorf("renderer.Universal: section %q matrix requires renderer %q", section.ID, RendererFieldMatrix)
			}
			if section.Matrix != nil {
				if err := section.Matrix.Validate(section.ID); err != nil {
					return err
				}
			}
			if err := validateMediaGalleryItems(fmt.Sprintf("form section %q", section.ID), section.MediaItems); err != nil {
				return err
			}
			if section.Collection == nil {
				if err := validateMediaActions(section.MediaActions); err != nil {
					return err
				}
				continue
			}
			if section.Collection.Module == "" {
				return fmt.Errorf("renderer.Universal: collection section %q must define module", section.ID)
			}
			for _, bucket := range section.Collection.Buckets {
				if err := validateActions("collection bucket", bucket.Actions); err != nil {
					return err
				}
				if bucket.Predicate == nil {
					continue
				}
				if bucket.Predicate.Field == "" {
					return fmt.Errorf("renderer.Universal: collection bucket %q predicate must define field", bucket.ID)
				}
				if bucket.Predicate.Operator == "" {
					return fmt.Errorf("renderer.Universal: collection bucket %q predicate must define operator", bucket.ID)
				}
				if len(bucket.Predicate.Values) > 0 && bucket.Predicate.Value != nil {
					return fmt.Errorf("renderer.Universal: collection bucket %q predicate must not define both value and values", bucket.ID)
				}
			}
			if err := validateActions("collection", section.Collection.Actions); err != nil {
				return err
			}
			if err := validateMediaActions(section.MediaActions); err != nil {
				return err
			}
		}
	}
	if r.List != nil {
		if err := validateListPage("list page", r.List); err != nil {
			return err
		}
	}
	if r.Record != nil {
		if err := validateActions("record page", r.Record.Actions); err != nil {
			return err
		}
		if err := validateRecordComponents(r.Record); err != nil {
			return err
		}
	}
	if r.ResourceGrid != nil {
		if err := validateAction("resource grid create", r.ResourceGrid.Create); err != nil {
			return err
		}
		if err := validateAction("resource grid delete", r.ResourceGrid.Delete); err != nil {
			return err
		}
		if err := validateAction("resource grid update", r.ResourceGrid.Update); err != nil {
			return err
		}
		if r.ResourceGrid.Card != nil {
			if err := r.ResourceGrid.Card.Validate(); err != nil {
				return err
			}
			if err := validateActions("resource grid card", r.ResourceGrid.Card.Actions); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRecordComponents(page *RecordPage) error {
	for _, section := range page.Sections {
		if err := section.Block.Validate(); err != nil {
			return fmt.Errorf("renderer.Universal: record section %q block: %w", section.ID, err)
		}
		for _, component := range section.Components {
			if err := component.Validate(); err != nil {
				return fmt.Errorf("renderer.Universal: record section %q component %q: %w", section.ID, component.ID, err)
			}
			if component.ActionID != "" && !recordPageHasAction(page, component.ActionID) {
				return fmt.Errorf("renderer.Universal: record section %q component %q action_id %q is not declared in record page actions", section.ID, component.ID, component.ActionID)
			}
		}
	}
	return nil
}

func recordPageHasAction(page *RecordPage, id string) bool {
	for index := range page.Actions {
		if page.Actions[index].ID == id {
			return true
		}
	}
	return false
}

func (component DisplayComponent) Validate() error {
	if err := validateMediaGalleryItems(fmt.Sprintf("display component %q", component.ID), component.MediaItems); err != nil {
		return err
	}
	if component.Type == DisplayStatusTimeline && len(component.Fields) != 1 {
		return fmt.Errorf("status timeline requires exactly one field")
	}
	if component.DisplayType != "" {
		if component.Type != DisplayDataList {
			return fmt.Errorf("display type requires component type %q", DisplayDataList)
		}
		switch component.DisplayType {
		case ComponentDisplayKeyValueGrid, ComponentDisplayTileGrid:
		default:
			return fmt.Errorf("unsupported display type %q", component.DisplayType)
		}
	}
	if len(component.Items) > 0 {
		if component.Type != DisplayDataList {
			return fmt.Errorf("items require component type %q", DisplayDataList)
		}
		seen := make(map[string]struct{}, len(component.Items))
		for _, item := range component.Items {
			if item.Field == "" {
				return fmt.Errorf("item field is required")
			}
			if _, exists := seen[item.Field]; exists {
				return fmt.Errorf("item field %q is duplicated", item.Field)
			}
			seen[item.Field] = struct{}{}
		}
	}
	if component.CollectionGroups != nil {
		if component.Type != DisplayAccordionGroups {
			return fmt.Errorf("collection groups require component type %q", DisplayAccordionGroups)
		}
		if component.CollectionGroups.SourceField == "" {
			return fmt.Errorf("collection groups source field is required")
		}
		if len(component.CollectionGroups.Groups) == 0 {
			return fmt.Errorf("collection groups are required")
		}
		seen := make(map[string]struct{}, len(component.CollectionGroups.Groups))
		for _, group := range component.CollectionGroups.Groups {
			if group.ID == "" {
				return fmt.Errorf("collection group id is required")
			}
			if _, exists := seen[group.ID]; exists {
				return fmt.Errorf("collection group id %q is duplicated", group.ID)
			}
			seen[group.ID] = struct{}{}
			if !hasCondition(group.ItemCondition) {
				return fmt.Errorf("collection group %q item condition is required", group.ID)
			}
		}
	}
	if component.Type == DisplayAccordionGroups && component.CollectionGroups == nil {
		return fmt.Errorf("accordion groups require collection groups")
	}
	return nil
}

func (block *Block) Validate() error {
	if block == nil {
		return nil
	}
	seen := make(map[MediaOverlayPosition]struct{}, len(block.Overlays))
	for _, overlay := range block.Overlays {
		if !validMediaOverlayPosition(overlay.Position) {
			return fmt.Errorf("unsupported block overlay position %q", overlay.Position)
		}
		if _, exists := seen[overlay.Position]; exists {
			return fmt.Errorf("block overlay position %q is duplicated", overlay.Position)
		}
		seen[overlay.Position] = struct{}{}
		if len(overlay.Badges) == 0 {
			return fmt.Errorf("block overlay %q badges are required", overlay.Position)
		}
	}
	return nil
}

func validMediaOverlayPosition(position MediaOverlayPosition) bool {
	switch position {
	case MediaOverlayTopLeft, MediaOverlayTopRight, MediaOverlayBottomLeft, MediaOverlayBottomRight:
		return true
	default:
		return false
	}
}

func hasCondition(condition *Condition) bool {
	if condition == nil {
		return false
	}
	hasDirectPredicate := condition.Equals != nil ||
		condition.NotEquals != nil ||
		len(condition.In) > 0 ||
		len(condition.NotIn) > 0 ||
		condition.Empty != nil ||
		condition.NotEmpty != nil ||
		condition.Truthy != nil ||
		condition.Falsy != nil
	if hasDirectPredicate && condition.Path == "" {
		return false
	}

	hasPredicate := hasDirectPredicate
	for index := range condition.All {
		if !hasCondition(&condition.All[index]) {
			return false
		}
		hasPredicate = true
	}
	for index := range condition.Any {
		if !hasCondition(&condition.Any[index]) {
			return false
		}
		hasPredicate = true
	}
	if condition.Not != nil {
		switch value := condition.Not.(type) {
		case Condition:
			if !hasCondition(&value) {
				return false
			}
		case *Condition:
			if !hasCondition(value) {
				return false
			}
		default:
			return false
		}
		hasPredicate = true
	}
	return hasPredicate
}

func validateListPage(scope string, page *ListPage) error {
	if page == nil {
		return nil
	}
	if err := page.Grid.Validate(); err != nil {
		return fmt.Errorf("renderer.Universal: %s: %w", scope, err)
	}
	if err := page.GroupBy.Validate(); err != nil {
		return fmt.Errorf("renderer.Universal: %s: %w", scope, err)
	}
	if err := validateActions(scope, page.Actions); err != nil {
		return err
	}
	if page.Summary != nil {
		if err := page.Summary.Validate(); err != nil {
			return fmt.Errorf("renderer.Universal: %s: %w", scope, err)
		}
	}
	if page.CardSchema != nil {
		if err := page.CardSchema.Validate(); err != nil {
			return err
		}
		if err := validateActions(scope+" card schema", page.CardSchema.Actions); err != nil {
			return err
		}
	}
	if err := validateListSelection(scope, page.Selection, page.CardSchema); err != nil {
		return err
	}
	if err := validateFilterRangePresets(scope, page.Filters); err != nil {
		return err
	}
	if err := validateFilterGroups(scope, page.Filters); err != nil {
		return err
	}
	if err := validateFilterPills(scope, page.Filters); err != nil {
		return err
	}
	if page.Filters != nil {
		if err := page.Filters.DateRange.Validate(scope + " filters"); err != nil {
			return err
		}
	}
	return nil
}

func validateListSelection(scope string, selection *ListSelection, card *CardSchema) error {
	if selection == nil {
		return nil
	}
	if selection.KeyField == "" {
		return fmt.Errorf("renderer.ListPage: selection.key_field is required")
	}
	if selection.ToggleAction == "" {
		return fmt.Errorf("renderer.ListPage: selection.toggle_action is required")
	}
	if selection.ValuesField == "" {
		return fmt.Errorf("renderer.ListPage: selection.values_field is required")
	}
	if selection.Limit < 1 {
		return fmt.Errorf("renderer.ListPage: selection.limit must be greater than zero")
	}
	if selection.Source == nil || selection.Source.Method == "" || selection.Source.Endpoint == "" {
		return fmt.Errorf("renderer.ListPage: selection.source method and endpoint are required")
	}
	if card == nil {
		return fmt.Errorf("renderer.ListPage: selection requires card_schema")
	}
	foundToggle := false
	for i := range card.Actions {
		if card.Actions[i].ID == selection.ToggleAction {
			foundToggle = true
			break
		}
	}
	if !foundToggle {
		return fmt.Errorf("renderer.ListPage: selection.toggle_action %q is not declared in card_schema.actions", selection.ToggleAction)
	}
	if err := validateAction(scope+" selection clear", selection.Clear); err != nil {
		return err
	}
	if err := validateAction(scope+" selection proceed", selection.Proceed); err != nil {
		return err
	}
	if selection.Clear == nil || selection.Clear.Type != ActionAPI || selection.Clear.API == nil {
		return fmt.Errorf("renderer.ListPage: selection.clear must be an api action")
	}
	if selection.Proceed == nil || (selection.Proceed.Type != ActionRoute && selection.Proceed.Type != ActionModal) {
		return fmt.Errorf("renderer.ListPage: selection.proceed must be a route or modal action")
	}
	return nil
}

func filterFields(filters *Filters) map[string]struct{} {
	declared := make(map[string]struct{})
	if filters == nil {
		return declared
	}
	for _, placement := range [][]string{filters.Primary, filters.Secondary, filters.More, filters.Nested} {
		for _, field := range placement {
			declared[field] = struct{}{}
		}
	}
	for _, group := range filters.Groups {
		appendFilterGroupFields(declared, group)
	}
	return declared
}

func appendFilterGroupFields(declared map[string]struct{}, group FilterGroup) {
	for _, field := range group.Fields {
		declared[field] = struct{}{}
	}
	for _, section := range group.Sections {
		for _, field := range section.Fields {
			declared[field] = struct{}{}
		}
	}
	for _, item := range group.Items {
		if item.Field != "" {
			declared[item.Field] = struct{}{}
		}
		if item.Group != nil {
			appendFilterGroupFields(declared, *item.Group)
		}
	}
}

func validateFilterRangePresets(scope string, filters *Filters) error {
	if filters == nil || len(filters.RangePresets) == 0 {
		return nil
	}
	declared := filterFields(filters)
	seen := make(map[string]struct{}, len(filters.RangePresets))
	for _, group := range filters.RangePresets {
		if group.Field == "" {
			return fmt.Errorf("renderer.Universal: %s range presets field is required", scope)
		}
		if _, exists := seen[group.Field]; exists {
			return fmt.Errorf("renderer.Universal: %s range presets field %q is duplicated", scope, group.Field)
		}
		seen[group.Field] = struct{}{}
		if _, exists := declared[group.Field]; !exists {
			return fmt.Errorf("renderer.Universal: %s range presets field %q is not declared in filters", scope, group.Field)
		}
		if len(group.Presets) == 0 {
			return fmt.Errorf("renderer.Universal: %s range presets field %q must have at least one preset", scope, group.Field)
		}
		for _, preset := range group.Presets {
			if preset.Min > preset.Max {
				return fmt.Errorf("renderer.Universal: %s range preset for field %q has min greater than max", scope, group.Field)
			}
		}
	}
	return nil
}

func validateFilterGroups(scope string, filters *Filters) error {
	if filters == nil {
		return nil
	}
	fieldOwners := make(map[string]string)
	for _, placement := range []struct {
		name   string
		fields []string
	}{
		{name: "primary", fields: filters.Primary},
		{name: "secondary", fields: filters.Secondary},
		{name: "more", fields: filters.More},
		{name: "nested", fields: filters.Nested},
	} {
		for _, field := range placement.fields {
			if owner, exists := fieldOwners[field]; exists {
				return fmt.Errorf("renderer.Universal: %s filter field %q is declared in both %s and %s", scope, field, owner, placement.name)
			}
			fieldOwners[field] = placement.name
		}
	}
	ids := make(map[string]struct{}, len(filters.Groups))
	for _, group := range filters.Groups {
		if group.ID == "" {
			return fmt.Errorf("renderer.Universal: %s filter group id is required", scope)
		}
		if _, exists := ids[group.ID]; exists {
			return fmt.Errorf("renderer.Universal: %s filter group %q is duplicated", scope, group.ID)
		}
		ids[group.ID] = struct{}{}
		if group.Label == "" && group.LabelKey == "" {
			return fmt.Errorf("renderer.Universal: %s filter group %q label is required", scope, group.ID)
		}
		if !group.Placement.Valid() {
			return fmt.Errorf("renderer.Universal: %s filter group %q has invalid placement %q", scope, group.ID, group.Placement)
		}
		if err := validateFilterGroupContent(scope, group, fieldOwners); err != nil {
			return err
		}
	}
	return nil
}

func validateFilterPills(scope string, filters *Filters) error {
	if filters == nil {
		return nil
	}
	if !filters.Presentation.Valid() {
		return fmt.Errorf("renderer.Universal: %s filters have invalid presentation %q", scope, filters.Presentation)
	}
	for _, row := range append(append([][]FilterPill{}, filters.PillRows...), filters.SecondaryPillRows...) {
		for _, pill := range row {
			if !pill.Presentation.Valid() {
				return fmt.Errorf("renderer.Universal: %s filter pill %q has invalid presentation %q", scope, pill.Label, pill.Presentation)
			}
		}
	}
	return nil
}

func validateFilterGroupContent(scope string, group FilterGroup, fieldOwners map[string]string) error {
	if !group.Presentation.Valid() {
		return fmt.Errorf("renderer.Universal: %s filter group %q has invalid presentation %q", scope, group.ID, group.Presentation)
	}
	if group.Presentation == "" {
		if len(group.Sections) != 0 {
			return fmt.Errorf("renderer.Universal: %s filter group %q sections require a presentation", scope, group.ID)
		}
		if len(group.Fields) != 0 && len(group.Items) != 0 {
			return fmt.Errorf("renderer.Universal: %s filter group %q must use either fields or items", scope, group.ID)
		}
		if len(group.Fields) == 0 && len(group.Items) == 0 {
			return fmt.Errorf("renderer.Universal: %s filter group %q must contain at least one field", scope, group.ID)
		}
		for _, field := range group.Fields {
			if err := claimFilterGroupField(scope, group.ID, "", field, fieldOwners); err != nil {
				return err
			}
		}
		if err := validateFilterGroupItems(scope, group.ID, group.Items, fieldOwners); err != nil {
			return err
		}
		return nil
	}
	if len(group.Fields) != 0 || len(group.Items) != 0 {
		return fmt.Errorf("renderer.Universal: %s filter group %q with presentation %q must use sections instead of fields", scope, group.ID, group.Presentation)
	}
	if len(group.Sections) == 0 {
		return fmt.Errorf("renderer.Universal: %s filter group %q with presentation %q must contain at least one section", scope, group.ID, group.Presentation)
	}
	sectionIDs := make(map[string]struct{}, len(group.Sections))
	for _, section := range group.Sections {
		if section.ID == "" {
			return fmt.Errorf("renderer.Universal: %s filter group %q section id is required", scope, group.ID)
		}
		if _, exists := sectionIDs[section.ID]; exists {
			return fmt.Errorf("renderer.Universal: %s filter group %q section %q is duplicated", scope, group.ID, section.ID)
		}
		sectionIDs[section.ID] = struct{}{}
		if section.Label == "" && section.LabelKey == "" {
			return fmt.Errorf("renderer.Universal: %s filter group %q section %q label is required", scope, group.ID, section.ID)
		}
		if len(section.Fields) == 0 {
			return fmt.Errorf("renderer.Universal: %s filter group %q section %q must contain at least one field", scope, group.ID, section.ID)
		}
		for _, field := range section.Fields {
			if err := claimFilterGroupField(scope, group.ID, section.ID, field, fieldOwners); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFilterGroupItems(scope, parentID string, items []FilterGroupItem, fieldOwners map[string]string) error {
	childIDs := make(map[string]struct{})
	for _, item := range items {
		if (item.Field == "") == (item.Group == nil) {
			return fmt.Errorf("renderer.Universal: %s filter group %q item must contain exactly one field or group", scope, parentID)
		}
		if item.Field != "" {
			if err := claimFilterGroupField(scope, parentID, "", item.Field, fieldOwners); err != nil {
				return err
			}
			continue
		}
		child := *item.Group
		if child.Placement != "" {
			return fmt.Errorf("renderer.Universal: %s nested filter group %q must not declare placement", scope, child.ID)
		}
		if child.ID == "" {
			return fmt.Errorf("renderer.Universal: %s filter group %q nested group id is required", scope, parentID)
		}
		if _, exists := childIDs[child.ID]; exists {
			return fmt.Errorf("renderer.Universal: %s filter group %q nested group %q is duplicated", scope, parentID, child.ID)
		}
		childIDs[child.ID] = struct{}{}
		if child.Label == "" && child.LabelKey == "" {
			return fmt.Errorf("renderer.Universal: %s filter group %q nested group %q label is required", scope, parentID, child.ID)
		}
		if err := validateFilterGroupContent(scope, child, fieldOwners); err != nil {
			return err
		}
	}
	return nil
}

func claimFilterGroupField(scope, groupID, sectionID, field string, fieldOwners map[string]string) error {
	owner := fmt.Sprintf("group %q", groupID)
	if sectionID != "" {
		owner = fmt.Sprintf("group %q section %q", groupID, sectionID)
	}
	if field == "" {
		return fmt.Errorf("renderer.Universal: %s %s contains an empty field", scope, owner)
	}
	if previous, exists := fieldOwners[field]; exists {
		return fmt.Errorf("renderer.Universal: %s filter field %q is declared in both %s and %s", scope, field, previous, owner)
	}
	fieldOwners[field] = owner
	return nil
}

func validateActions(scope string, actions []Action) error {
	for i := range actions {
		if err := validateAction(scope, &actions[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateMediaActions(actions *MediaGalleryActions) error {
	if actions == nil {
		return nil
	}
	for scope, action := range map[string]*Action{
		"media upload": actions.Upload, "media link": actions.Link, "media update": actions.Update,
		"media reorder":  actions.Reorder,
		"media recenter": actions.Recenter, "media crop": actions.Crop, "media remove": actions.Remove,
	} {
		if err := validateAction(scope, action); err != nil {
			return err
		}
	}
	return nil
}

func validateMediaGalleryItems(scope string, items []MediaGalleryItem) error {
	for index := range items {
		if err := validateActions(fmt.Sprintf("%s media item %q", scope, items[index].ID), items[index].Actions); err != nil {
			return err
		}
	}
	return nil
}

func validateAction(scope string, action *Action) error {
	if action == nil {
		return nil
	}
	if err := action.Validate(); err != nil {
		return fmt.Errorf("renderer.Universal: %s action %q: %w", scope, action.ID, err)
	}
	return nil
}

type Layout struct {
	Type        LayoutType   `json:"type,omitempty"`
	Mode        string       `json:"mode,omitempty"`
	Slots       []string     `json:"slots,omitempty"`
	MobileSlots []string     `json:"mobile_slots,omitempty"`
	Left        SizeToken    `json:"left,omitempty"`
	Center      SizeToken    `json:"center,omitempty"`
	Right       SizeToken    `json:"right,omitempty"`
	Align       AlignToken   `json:"align,omitempty"`
	MaxWidth    MaxWidth     `json:"max_width,omitempty"`
	Gap         SpacingToken `json:"gap,omitempty"`
}

type Filters struct {
	Renderer          RendererKey          `json:"renderer,omitempty"`
	Presentation      FilterPresentation   `json:"presentation,omitempty"`
	Enabled           bool                 `json:"enabled"`
	PrimaryPlacement  string               `json:"primary_placement,omitempty"`
	SecondaryEnabled  *bool                `json:"secondary_enabled,omitempty"`
	ResetPlacement    string               `json:"reset_placement,omitempty"`
	Levels            []string             `json:"levels,omitempty"`
	Primary           []string             `json:"primary,omitempty"`
	Secondary         []string             `json:"secondary,omitempty"`
	More              []string             `json:"more,omitempty"`
	Nested            []string             `json:"nested,omitempty"`
	Groups            []FilterGroup        `json:"groups,omitempty"`
	PillRows          [][]FilterPill       `json:"pill_rows,omitempty"`
	SecondaryPillRows [][]FilterPill       `json:"secondary_pill_rows,omitempty"`
	Reset             *FilterReset         `json:"reset,omitempty"`
	Text              *FilterText          `json:"text,omitempty"`
	RangePresets      []FilterRangePresets `json:"range_presets,omitempty"`
	DateRange         *DateRangeToolbar    `json:"date_range,omitempty"`
}

// FilterPresentation selects a reusable arrangement of the controls declared
// by Filters. It does not alter their request semantics.
type FilterPresentation string

const (
	FilterPresentationToolbar FilterPresentation = "toolbar"
)

func (presentation FilterPresentation) Valid() bool {
	return presentation == "" || presentation == FilterPresentationToolbar
}

type FilterGroupPlacement string

const (
	FilterGroupPlacementPrimary   FilterGroupPlacement = "primary"
	FilterGroupPlacementSecondary FilterGroupPlacement = "secondary"
	FilterGroupPlacementMore      FilterGroupPlacement = "more"
	FilterGroupPlacementNested    FilterGroupPlacement = "nested"
)

func (placement FilterGroupPlacement) Valid() bool {
	switch placement {
	case FilterGroupPlacementPrimary, FilterGroupPlacementSecondary, FilterGroupPlacementMore, FilterGroupPlacementNested:
		return true
	default:
		return false
	}
}

type FilterGroupPresentation string

const (
	FilterGroupPresentationTabs FilterGroupPresentation = "tabs"
)

func (presentation FilterGroupPresentation) Valid() bool {
	return presentation == "" || presentation == FilterGroupPresentationTabs
}

// FilterGroup describes one named filter control and the fields it owns.
// Placement determines the typed level in which the control is rendered.
type FilterGroup struct {
	ID           string                  `json:"id"`
	Label        string                  `json:"label,omitempty"`
	LabelKey     string                  `json:"label_key,omitempty"`
	Placement    FilterGroupPlacement    `json:"placement,omitempty"`
	Presentation FilterGroupPresentation `json:"presentation,omitempty"`
	Fields       []string                `json:"fields,omitempty"`
	Sections     []FilterGroupSection    `json:"sections,omitempty"`
	Items        []FilterGroupItem       `json:"items,omitempty"`
}

// FilterGroupSection describes an ordered typed section inside a presented group.
type FilterGroupSection struct {
	ID       string   `json:"id"`
	Label    string   `json:"label,omitempty"`
	LabelKey string   `json:"label_key,omitempty"`
	Fields   []string `json:"fields"`
}

// FilterGroupItem preserves the order of direct fields and nested groups.
// Exactly one of Field or Group must be set.
type FilterGroupItem struct {
	Field string       `json:"field,omitempty"`
	Group *FilterGroup `json:"group,omitempty"`
}

type FilterRangePresets struct {
	Field   string              `json:"field"`
	Presets []FilterRangePreset `json:"presets"`
}

type FilterRangePreset struct {
	Label string  `json:"label"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// FilterText contains all text rendered by built-in filter controls.
// Values are translation keys in module configuration and localized before
// the renderer response is serialized.
type FilterText struct {
	SearchPlaceholder string `json:"search_placeholder,omitempty"`
	ResetLabel        string `json:"reset_label,omitempty"`
	ResetAllLabel     string `json:"reset_all_label,omitempty"`
	ApplyLabel        string `json:"apply_label,omitempty"`
	LoadingLabel      string `json:"loading_label,omitempty"`
	EmptyLabel        string `json:"empty_label,omitempty"`
	EmptyDescription  string `json:"empty_description,omitempty"`
	EmptyIcon         string `json:"empty_icon,omitempty"`
	NoResultsLabel    string `json:"no_results_label,omitempty"`
	CancelLabel       string `json:"cancel_label,omitempty"`
	CloseLabel        string `json:"close_label,omitempty"`
	RangeMinLabel     string `json:"range_min_label,omitempty"`
	RangeMaxLabel     string `json:"range_max_label,omitempty"`
}

type FilterPill struct {
	Label         string                 `json:"label,omitempty"`
	LabelKey      string                 `json:"label_key,omitempty"`
	GroupLabel    string                 `json:"group_label,omitempty"`
	GroupLabelKey string                 `json:"group_label_key,omitempty"`
	Key           string                 `json:"key,omitempty"`
	Val           string                 `json:"val,omitempty"`
	CountField    string                 `json:"count_field,omitempty"`
	Dot           bool                   `json:"dot,omitempty"`
	Presentation  FilterPillPresentation `json:"presentation,omitempty"`
	Tone          string                 `json:"tone,omitempty"`
}

// FilterPillPresentation describes the visual control for an existing filter
// pill. The key and val still fully define the generated list query.
type FilterPillPresentation string

const (
	FilterPillPresentationTabs    FilterPillPresentation = "tabs"
	FilterPillPresentationToggle  FilterPillPresentation = "toggle"
	FilterPillPresentationSummary FilterPillPresentation = "summary"
)

func (presentation FilterPillPresentation) Valid() bool {
	switch presentation {
	case "", FilterPillPresentationTabs, FilterPillPresentationToggle, FilterPillPresentationSummary:
		return true
	default:
		return false
	}
}

type FilterReset struct {
	Preserve []string `json:"preserve,omitempty"`
}

type Grid struct {
	Enabled bool         `json:"enabled"`
	Mode    GridMode     `json:"mode,omitempty"`
	Columns *GridColumns `json:"columns,omitempty"`
}

type GridColumns struct {
	Desktop GridColumnCount `json:"desktop"`
	Tablet  GridColumnCount `json:"tablet"`
	Mobile  GridColumnCount `json:"mobile"`
}

func (grid *Grid) Validate() error {
	if grid == nil {
		return nil
	}
	if !grid.Mode.Valid() {
		return fmt.Errorf("renderer.Grid: unsupported mode %q", grid.Mode)
	}
	if grid.Columns == nil {
		return nil
	}
	for _, value := range []struct {
		name  string
		count GridColumnCount
	}{
		{name: "desktop", count: grid.Columns.Desktop},
		{name: "tablet", count: grid.Columns.Tablet},
		{name: "mobile", count: grid.Columns.Mobile},
	} {
		if !value.count.Valid() {
			return fmt.Errorf("renderer.Grid: columns.%s must be between 1 and 6", value.name)
		}
	}
	if grid.Columns.Mobile > grid.Columns.Tablet || grid.Columns.Tablet > grid.Columns.Desktop {
		return fmt.Errorf("renderer.Grid: columns must satisfy mobile <= tablet <= desktop")
	}
	return nil
}

type Pagination struct {
	Renderer RendererKey    `json:"renderer,omitempty"`
	Mode     PaginationMode `json:"mode,omitempty"`
}

type ListPage struct {
	ID         string                 `json:"id,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Subtitle   string                 `json:"subtitle,omitempty"`
	ShowHeader *bool                  `json:"show_header,omitempty"`
	Layout     *Layout                `json:"layout,omitempty"`
	Filters    *Filters               `json:"filters,omitempty"`
	Summary    *Summary               `json:"summary,omitempty"`
	Grid       *Grid                  `json:"grid,omitempty"`
	Pagination *Pagination            `json:"pagination,omitempty"`
	GroupBy    *ListGroupBy           `json:"group_by,omitempty"`
	CardSchema *CardSchema            `json:"card_schema,omitempty"`
	Selection  *ListSelection         `json:"selection,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Actions    []Action               `json:"actions,omitempty"`
}

// ListSelection declares server-owned selection for a list of cards. The
// renderer loads selected keys from Source and never treats client state as
// authoritative. ToggleAction references an action from CardSchema.Actions.
type ListSelection struct {
	KeyField      string     `json:"key_field"`
	ToggleAction  string     `json:"toggle_action"`
	ValuesField   string     `json:"values_field"`
	Limit         int        `json:"limit"`
	SelectedLabel string     `json:"selected_label,omitempty"`
	Source        *APIAction `json:"source"`
	Clear         *Action    `json:"clear"`
	Proceed       *Action    `json:"proceed"`
}

// ListGroupBy controls presentation-only grouping of already server-sorted list rows.
// Field references a value returned with each row. The API owns its formatting
// and localization; grouping never changes filtering, ordering or pagination.
type ListGroupBy struct {
	Field          string          `json:"field,omitempty"`
	Type           ListGroupByType `json:"type,omitempty"`
	TodayLabel     string          `json:"today_label,omitempty"`
	YesterdayLabel string          `json:"yesterday_label,omitempty"`
	ThisWeekLabel  string          `json:"this_week_label,omitempty"`
	EarlierLabel   string          `json:"earlier_label,omitempty"`
}

func (group *ListGroupBy) Validate() error {
	if group == nil {
		return nil
	}
	if group.Field == "" {
		return fmt.Errorf("renderer.ListGroupBy: field is required")
	}
	switch group.Type {
	case "", ListGroupByDate:
		return nil
	default:
		return fmt.Errorf("renderer.ListGroupBy: unsupported type %q", group.Type)
	}
}

type Summary struct {
	Title         string              `json:"title,omitempty"`
	TitleFallback string              `json:"title_fallback,omitempty"`
	Presentation  SummaryPresentation `json:"presentation,omitempty"`
	Items         []SummaryItem       `json:"items,omitempty"`
	ShowOnline    *bool               `json:"show_online,omitempty"`
	ShowAction    *bool               `json:"show_action,omitempty"`
	// Resource is resolved by the generator into Load for the current
	// principal. It supplies record data used by summary-bound list controls.
	Resource *Resource     `json:"-"`
	Load     *ResourceLoad `json:"load,omitempty"`
	Trend    *SummaryTrend `json:"trend,omitempty"`
}

type SummaryPresentation string

const (
	SummaryPresentationCompact   SummaryPresentation = "compact"
	SummaryPresentationDashboard SummaryPresentation = "dashboard"
)

// SummaryItem binds one compact summary value to a field loaded by Summary.
// It is presentation metadata only and does not affect a list query.
type SummaryItem struct {
	ID             string `json:"id"`
	Label          string `json:"label,omitempty"`
	LabelKey       string `json:"label_key,omitempty"`
	ValueField     string `json:"value_field"`
	ChangeField    string `json:"change_field,omitempty"`
	DirectionField string `json:"direction_field,omitempty"`
	Icon           string `json:"icon,omitempty"`
	Tone           string `json:"tone,omitempty"`
}

// SummaryTrend binds already prepared series to the same record loaded by
// Summary.Resource. The renderer never calculates, groups or formats points.
type SummaryTrend struct {
	Title           string               `json:"title,omitempty"`
	Subtitle        string               `json:"subtitle,omitempty"`
	PeriodField     string               `json:"period_field,omitempty"`
	AriaLabel       string               `json:"aria_label,omitempty"`
	AriaLabelKey    string               `json:"aria_label_key,omitempty"`
	EmptyLabel      string               `json:"empty_label,omitempty"`
	EmptyLabelKey   string               `json:"empty_label_key,omitempty"`
	LoadingLabel    string               `json:"loading_label,omitempty"`
	LoadingLabelKey string               `json:"loading_label_key,omitempty"`
	Series          []SummaryTrendSeries `json:"series"`
	DateRange       *DateRangeToolbar    `json:"date_range,omitempty"`
}

type SummaryTrendAxis string

const (
	SummaryTrendAxisPrimary   SummaryTrendAxis = "primary"
	SummaryTrendAxisSecondary SummaryTrendAxis = "secondary"
)

// SummaryTrendSeries describes one server-prepared line. Axis allows values
// with different units, such as counts and money, to share one chart.
type SummaryTrendSeries struct {
	ID          string           `json:"id"`
	Label       string           `json:"label,omitempty"`
	LabelKey    string           `json:"label_key,omitempty"`
	PointsField string           `json:"points_field"`
	Tone        string           `json:"tone,omitempty"`
	Axis        SummaryTrendAxis `json:"axis,omitempty"`
	Fill        bool             `json:"fill,omitempty"`
	Dashed      bool             `json:"dashed,omitempty"`
}

// DateRangeToolbar is a presentation contract for a server-side date filter.
// Field is sent as filter[field]=YYYY-MM-DD..YYYY-MM-DD. A preset with Days=0
// clears that filter and therefore represents the complete period.
type DateRangeToolbar struct {
	Field         string            `json:"field"`
	DefaultPreset string            `json:"default_preset,omitempty"`
	Presets       []DateRangePreset `json:"presets,omitempty"`
	Min           string            `json:"min,omitempty"`
	Max           string            `json:"max,omitempty"`
	Placeholder   string            `json:"placeholder,omitempty"`
	ApplyLabel    string            `json:"apply_label,omitempty"`
	CancelLabel   string            `json:"cancel_label,omitempty"`
	StartLabel    string            `json:"start_label,omitempty"`
	EndLabel      string            `json:"end_label,omitempty"`
	EmptyLabel    string            `json:"empty_label,omitempty"`
	DialogLabel   string            `json:"dialog_label,omitempty"`
	PreviousLabel string            `json:"previous_label,omitempty"`
	NextLabel     string            `json:"next_label,omitempty"`
	Months        []string          `json:"months,omitempty"`
	FormatMonths  []string          `json:"format_months,omitempty"`
	Weekdays      []string          `json:"weekdays,omitempty"`
}

type DateRangePreset struct {
	ID       string `json:"id"`
	Label    string `json:"label,omitempty"`
	LabelKey string `json:"label_key,omitempty"`
	Days     int    `json:"days"`
}

func (summary *Summary) Validate() error {
	if summary == nil {
		return nil
	}
	if summary.Resource != nil {
		if err := summary.Resource.Validate("summary resource"); err != nil {
			return err
		}
	}
	switch summary.Presentation {
	case "", SummaryPresentationCompact, SummaryPresentationDashboard:
	default:
		return fmt.Errorf("renderer.Summary: unsupported presentation %q", summary.Presentation)
	}
	ids := make(map[string]struct{}, len(summary.Items))
	for _, item := range summary.Items {
		if item.ID == "" {
			return fmt.Errorf("renderer.Summary: item id is required")
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("renderer.Summary: item %q is duplicated", item.ID)
		}
		ids[item.ID] = struct{}{}
		if item.Label == "" && item.LabelKey == "" {
			return fmt.Errorf("renderer.Summary: item %q label is required", item.ID)
		}
		if item.ValueField == "" {
			return fmt.Errorf("renderer.Summary: item %q value field is required", item.ID)
		}
	}
	if summary.Trend != nil {
		if len(summary.Trend.Series) == 0 {
			return fmt.Errorf("renderer.Summary: trend series are required")
		}
		seriesIDs := make(map[string]struct{}, len(summary.Trend.Series))
		for _, series := range summary.Trend.Series {
			if series.ID == "" || series.PointsField == "" || (series.Label == "" && series.LabelKey == "") {
				return fmt.Errorf("renderer.Summary: trend series id, label and points field are required")
			}
			if _, exists := seriesIDs[series.ID]; exists {
				return fmt.Errorf("renderer.Summary: trend series %q is duplicated", series.ID)
			}
			seriesIDs[series.ID] = struct{}{}
			if series.Axis != "" && series.Axis != SummaryTrendAxisPrimary && series.Axis != SummaryTrendAxisSecondary {
				return fmt.Errorf("renderer.Summary: trend series %q has unsupported axis %q", series.ID, series.Axis)
			}
		}
		if err := summary.Trend.DateRange.Validate("summary trend"); err != nil {
			return err
		}
	}
	return nil
}

func (toolbar *DateRangeToolbar) Validate(scope string) error {
	if toolbar == nil {
		return nil
	}
	if toolbar.Field == "" {
		return fmt.Errorf("renderer.DateRangeToolbar: %s field is required", scope)
	}
	if len(toolbar.Months) != 0 && len(toolbar.Months) != 12 {
		return fmt.Errorf("renderer.DateRangeToolbar: %s months must contain 12 values", scope)
	}
	if len(toolbar.FormatMonths) != 0 && len(toolbar.FormatMonths) != 12 {
		return fmt.Errorf("renderer.DateRangeToolbar: %s format_months must contain 12 values", scope)
	}
	if len(toolbar.Weekdays) != 0 && len(toolbar.Weekdays) != 7 {
		return fmt.Errorf("renderer.DateRangeToolbar: %s weekdays must contain 7 values", scope)
	}
	ids := make(map[string]struct{}, len(toolbar.Presets))
	for _, preset := range toolbar.Presets {
		if preset.ID == "" || (preset.Label == "" && preset.LabelKey == "") || preset.Days < 0 {
			return fmt.Errorf("renderer.DateRangeToolbar: %s preset id, label and non-negative days are required", scope)
		}
		if _, exists := ids[preset.ID]; exists {
			return fmt.Errorf("renderer.DateRangeToolbar: %s preset %q is duplicated", scope, preset.ID)
		}
		ids[preset.ID] = struct{}{}
	}
	if toolbar.DefaultPreset != "" {
		if _, exists := ids[toolbar.DefaultPreset]; !exists {
			return fmt.Errorf("renderer.DateRangeToolbar: %s default preset %q is not declared", scope, toolbar.DefaultPreset)
		}
	}
	for _, value := range []string{toolbar.Min, toolbar.Max} {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("renderer.DateRangeToolbar: %s date %q must use YYYY-MM-DD", scope, value)
		}
	}
	return nil
}

type CardActionLayout string

const (
	CardActionLayoutInline   CardActionLayout = "inline"
	CardActionLayoutEdgeFill CardActionLayout = "edge_fill"
	CardActionLayoutMenu     CardActionLayout = "menu"
)

type CardSchema struct {
	Type             string           `json:"type,omitempty"`
	Variant          CardVariant      `json:"variant,omitempty"`
	Size             SizeToken        `json:"size,omitempty"`
	SurfaceVariant   SurfaceVariant   `json:"surface_variant,omitempty"`
	SurfaceEffect    SurfaceEffect    `json:"surface_effect,omitempty"`
	LeadingAccent    *CardEdgeAccent  `json:"leading_accent,omitempty"`
	BadgeSize        SizeToken        `json:"badge_size,omitempty"`
	ActionSize       SizeToken        `json:"action_size,omitempty"`
	DeleteActionSize SizeToken        `json:"delete_action_size,omitempty"`
	ActionLayout     CardActionLayout `json:"action_layout,omitempty"`
	PrimaryAction    string           `json:"primary_action,omitempty"`
	Icon             *IconBinding     `json:"icon,omitempty"`
	Media            *Media           `json:"media,omitempty"`
	Title            *TextBinding     `json:"title,omitempty"`
	Subtitle         *TextBinding     `json:"subtitle,omitempty"`
	Meta             *TextBinding     `json:"meta,omitempty"`
	SubtitleTone     string           `json:"subtitle_tone,omitempty"`
	Description      *TextBinding     `json:"description,omitempty"`
	Status           *StatusBinding   `json:"status,omitempty"`
	Badges           []Badge          `json:"badges,omitempty"`
	Stats            []Badge          `json:"stats,omitempty"`
	Actions          []Action         `json:"actions,omitempty"`
}

func (schema *CardSchema) Validate() error {
	if schema == nil {
		return nil
	}
	switch schema.ActionLayout {
	case "", CardActionLayoutInline, CardActionLayoutEdgeFill, CardActionLayoutMenu:
	default:
		return fmt.Errorf("renderer.CardSchema: unsupported action layout %q", schema.ActionLayout)
	}
	if schema.LeadingAccent != nil && schema.LeadingAccent.Tone == "" {
		return fmt.Errorf("renderer.CardSchema: leading_accent tone is required")
	}
	for _, binding := range []*TextBinding{schema.Title, schema.Subtitle, schema.Meta, schema.Description} {
		if err := binding.Validate(); err != nil {
			return err
		}
	}
	if schema.Icon != nil && schema.Icon.Field == "" && schema.Icon.IconField == "" {
		return fmt.Errorf("renderer.CardSchema: icon field or icon_field is required")
	}
	return nil
}

// CardEdgeAccent adds an opt-in visual line to the leading edge of a card.
// Tone is an extensible presentation token interpreted by the consuming UI.
type CardEdgeAccent struct {
	Tone ToneToken `json:"tone"`
}

// IconBinding resolves an icon and its visual tone from a row. IconField and
// ToneField let a producer supply catalog-owned presentation values directly.
// Field with IconMap/ToneMap remains available for closed value sets.
type IconBinding struct {
	Field     string            `json:"field,omitempty"`
	IconMap   map[string]string `json:"icon_map,omitempty"`
	ToneMap   map[string]string `json:"tone_map,omitempty"`
	IconField string            `json:"icon_field,omitempty"`
	ToneField string            `json:"tone_field,omitempty"`
	Fallback  string            `json:"fallback,omitempty"`
	Marker    *IconMarker       `json:"marker,omitempty"`
}

// IconMarker is a small non-textual state indicator displayed on a bound icon.
// Its visibility is defined by the producer-owned record condition.
type IconMarker struct {
	VisibleIf *Condition `json:"visible_if,omitempty"`
	Tone      string     `json:"tone,omitempty"`
}

type Media struct {
	Field        string      `json:"field,omitempty"`
	Renderer     RendererKey `json:"renderer,omitempty"`
	Ratio        MediaRatio  `json:"ratio,omitempty"`
	Size         MediaSize   `json:"size,omitempty"`
	Variant      string      `json:"variant,omitempty"`
	GlowField    string      `json:"glow_field,omitempty"`
	GlowFallback string      `json:"glow_fallback,omitempty"`
	GlowEnabled  *bool       `json:"glow_enabled,omitempty"`
	StatusField  string      `json:"status_field,omitempty"`
	Fallback     string      `json:"fallback,omitempty"`
}

type FieldPresentation struct {
	Renderer    RendererKey      `json:"renderer,omitempty"`
	Variant     string           `json:"variant,omitempty"`
	Style       string           `json:"style,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Size        MediaSize        `json:"size,omitempty"`
	Ratio       MediaRatio       `json:"ratio,omitempty"`
	Prefix      string           `json:"prefix,omitempty"`
	Suffix      string           `json:"suffix,omitempty"`
	Hint        string           `json:"hint,omitempty"`
	Description string           `json:"description,omitempty"`
	Rows        uint8            `json:"rows,omitempty"`
	MaxItems    uint16           `json:"max_items,omitempty"`
	InputMode   FieldInputMode   `json:"input_mode,omitempty"`
	VisibleIf   *Condition       `json:"visible_if,omitempty"`
	ToneByValue []FieldValueTone `json:"tone_by_value,omitempty"`
}

// FieldInputMode hints which virtual keyboard a text control should open.
// Validation constraints remain owned by ModuleField checks.
type FieldInputMode string

const (
	FieldInputModeText    FieldInputMode = "text"
	FieldInputModeNumeric FieldInputMode = "numeric"
	FieldInputModeDecimal FieldInputMode = "decimal"
	FieldInputModeEmail   FieldInputMode = "email"
	FieldInputModeTel     FieldInputMode = "tel"
	FieldInputModeURL     FieldInputMode = "url"
	FieldInputModeSearch  FieldInputMode = "search"
)

func (mode FieldInputMode) Valid() bool {
	switch mode {
	case "", FieldInputModeText, FieldInputModeNumeric, FieldInputModeDecimal,
		FieldInputModeEmail, FieldInputModeTel, FieldInputModeURL, FieldInputModeSearch:
		return true
	default:
		return false
	}
}

func (presentation *FieldPresentation) Validate() error {
	if presentation == nil {
		return nil
	}
	if !presentation.InputMode.Valid() {
		return fmt.Errorf("renderer.FieldPresentation: unsupported input mode %q", presentation.InputMode)
	}
	return nil
}

type FieldValueTone struct {
	Value TypedValue `json:"value"`
	Tone  string     `json:"tone"`
}

type FieldMediaConfig struct {
	Item    *MediaGalleryItem    `json:"item,omitempty"`
	Upload  *MediaUploadConfig   `json:"upload,omitempty"`
	Labels  *MediaGalleryLabels  `json:"labels,omitempty"`
	Actions *MediaGalleryActions `json:"actions,omitempty"`
	Cropper *MediaCropperConfig  `json:"cropper,omitempty"`
}

type MediaCropperConfig struct {
	Title        string                     `json:"title,omitempty"`
	Subtitle     string                     `json:"subtitle,omitempty"`
	Hint         string                     `json:"hint,omitempty"`
	ChooseLabel  string                     `json:"choose_label,omitempty"`
	CancelLabel  string                     `json:"cancel_label,omitempty"`
	ConfirmLabel string                     `json:"confirm_label,omitempty"`
	CloseLabel   string                     `json:"close_label,omitempty"`
	Accept       string                     `json:"accept,omitempty"`
	Viewport     MediaCropperViewportConfig `json:"viewport"`
	Output       MediaCropperOutputConfig   `json:"output"`
}

type MediaCropperViewportConfig struct {
	Shape       MediaCropperViewportShape `json:"shape"`
	AspectRatio float64                   `json:"aspect_ratio"`
}

type MediaCropperOutputConfig struct {
	Width    int                        `json:"width"`
	Height   int                        `json:"height"`
	MIMEType MediaCropperOutputMIMEType `json:"mime_type"`
	Quality  float64                    `json:"quality"`
}

func (config *FieldMediaConfig) Validate() error {
	if config == nil {
		return nil
	}
	if config.Item != nil {
		if err := validateMediaGalleryItems("field media", []MediaGalleryItem{*config.Item}); err != nil {
			return err
		}
	}
	return config.Cropper.Validate()
}

func (cropper *MediaCropperConfig) Validate() error {
	if cropper == nil {
		return nil
	}
	switch cropper.Viewport.Shape {
	case MediaCropperViewportCircle, MediaCropperViewportRounded, MediaCropperViewportRectangle:
	default:
		return fmt.Errorf("renderer.MediaCropperConfig: unsupported viewport shape %q", cropper.Viewport.Shape)
	}
	if cropper.Viewport.AspectRatio <= 0 || math.IsNaN(cropper.Viewport.AspectRatio) || math.IsInf(cropper.Viewport.AspectRatio, 0) {
		return fmt.Errorf("renderer.MediaCropperConfig: viewport aspect ratio must be positive")
	}
	for _, label := range []struct {
		name  string
		value string
	}{
		{name: "title", value: cropper.Title},
		{name: "hint", value: cropper.Hint},
		{name: "choose label", value: cropper.ChooseLabel},
		{name: "cancel label", value: cropper.CancelLabel},
		{name: "confirm label", value: cropper.ConfirmLabel},
		{name: "close label", value: cropper.CloseLabel},
	} {
		if strings.TrimSpace(label.value) == "" {
			return fmt.Errorf("renderer.MediaCropperConfig: %s is required", label.name)
		}
	}
	if cropper.Output.Width <= 0 || cropper.Output.Height <= 0 {
		return fmt.Errorf("renderer.MediaCropperConfig: output dimensions must be positive")
	}
	switch cropper.Output.MIMEType {
	case MediaCropperOutputMIMETypeJPEG, MediaCropperOutputMIMETypePNG, MediaCropperOutputMIMETypeWebP:
	default:
		return fmt.Errorf("renderer.MediaCropperConfig: unsupported output mime type %q", cropper.Output.MIMEType)
	}
	if cropper.Output.Quality < 0 || cropper.Output.Quality > 1 || math.IsNaN(cropper.Output.Quality) || math.IsInf(cropper.Output.Quality, 0) {
		return fmt.Errorf("renderer.MediaCropperConfig: output quality must be between 0 and 1")
	}
	return nil
}

type TextBinding struct {
	Field    string     `json:"field,omitempty"`
	Template string     `json:"template,omitempty"`
	Format   TextFormat `json:"format,omitempty"`
}

func (binding *TextBinding) Validate() error {
	if binding == nil {
		return nil
	}
	switch binding.Format {
	case "", TextFormatRelativeTime:
		return nil
	default:
		return fmt.Errorf("renderer.TextBinding: unsupported format %q", binding.Format)
	}
}

type StatusBinding struct {
	ID         string            `json:"id,omitempty"`
	Field      string            `json:"field,omitempty"`
	Type       string            `json:"type,omitempty"`
	Option     string            `json:"option,omitempty"`
	Placement  string            `json:"placement,omitempty"`
	Marker     *bool             `json:"marker,omitempty"`
	OnlineTone string            `json:"online_tone,omitempty"`
	ToneMap    map[string]string `json:"tone_map,omitempty"`
}

type Badge struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Variant   string            `json:"variant,omitempty"`
	Field     string            `json:"field,omitempty"`
	IfField   string            `json:"if_field,omitempty"`
	Value     *TextBinding      `json:"value,omitempty"`
	Option    string            `json:"option,omitempty"`
	Placement string            `json:"placement,omitempty"`
	Label     string            `json:"label,omitempty"`
	LabelKey  string            `json:"label_key,omitempty"`
	LabelMap  map[string]string `json:"label_map,omitempty"`
	Icon      string            `json:"icon,omitempty"`
	Size      SizeToken         `json:"size,omitempty"`
	Tone      string            `json:"tone,omitempty"`
	ToneMap   map[string]string `json:"tone_map,omitempty"`
	Marker    *bool             `json:"marker,omitempty"`
	VisibleIf *Condition        `json:"visible_if,omitempty"`
	Then      *BadgeState       `json:"then,omitempty"`
	Else      *BadgeState       `json:"else,omitempty"`
}

type BadgeState struct {
	ID       string `json:"id,omitempty"`
	Label    string `json:"label,omitempty"`
	LabelKey string `json:"label_key,omitempty"`
	Tone     string `json:"tone,omitempty"`
	Marker   *bool  `json:"marker,omitempty"`
}

type FormPage struct {
	ID       string                 `json:"id,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Subtitle string                 `json:"subtitle,omitempty"`
	Layout   LayoutType             `json:"layout,omitempty"`
	Workflow *FormWorkflow          `json:"workflow,omitempty"`
	Actions  []Action               `json:"actions,omitempty"`
	Sections []FormSection          `json:"sections,omitempty"`
	Fields   []string               `json:"fields,omitempty"`
	Context  map[string]interface{} `json:"context,omitempty"`
}

// FormWorkflow selects the generic step-based form presentation. Steps are
// derived from FormPage.Sections, so labels and field ownership stay declared
// once in the ordinary form contract.
type FormWorkflow struct {
	PreviousLabel string               `json:"previous_label,omitempty"`
	NextLabel     string               `json:"next_label,omitempty"`
	Summary       *FormWorkflowSummary `json:"summary,omitempty"`
}

// FormWorkflowSummary binds a live summary to existing form fields and an
// existing submit action. It adds no transport or module-specific data.
type FormWorkflowSummary struct {
	Eyebrow      string   `json:"eyebrow,omitempty"`
	Title        string   `json:"title,omitempty"`
	Badge        *Badge   `json:"badge,omitempty"`
	Fields       []string `json:"fields,omitempty"`
	SubmitAction string   `json:"submit_action,omitempty"`
	ShowProgress bool     `json:"show_progress,omitempty"`
}

func validateFormWorkflow(page *FormPage) error {
	if page == nil || page.Workflow == nil || page.Workflow.Summary == nil {
		return nil
	}

	declaredFields := make(map[string]struct{}, len(page.Fields))
	for _, field := range page.Fields {
		declaredFields[field] = struct{}{}
	}
	seenFields := make(map[string]struct{}, len(page.Workflow.Summary.Fields))
	for _, field := range page.Workflow.Summary.Fields {
		if _, exists := declaredFields[field]; !exists {
			return fmt.Errorf("renderer.FormWorkflow: summary field %q is not declared by the form", field)
		}
		if _, exists := seenFields[field]; exists {
			return fmt.Errorf("renderer.FormWorkflow: summary field %q is duplicated", field)
		}
		seenFields[field] = struct{}{}
	}

	if page.Workflow.Summary.SubmitAction == "" {
		return nil
	}
	for _, action := range page.Actions {
		if action.ID == page.Workflow.Summary.SubmitAction && action.Behavior == ActionBehaviorSubmit {
			return nil
		}
	}
	return fmt.Errorf("renderer.FormWorkflow: summary submit action %q must reference a form submit action", page.Workflow.Summary.SubmitAction)
}

func validateFormSectionActions(page *FormPage, section FormSection) error {
	ids := append([]string{}, section.Action)
	ids = append(ids, section.Actions...)
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("renderer.Universal: form section %q action %q is duplicated", section.ID, id)
		}
		seen[id] = struct{}{}
		found := false
		for _, action := range page.Actions {
			if action.ID == id {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("renderer.Universal: form section %q action %q is not declared in form page actions", section.ID, id)
		}
	}
	return nil
}

type FormSection struct {
	ID           string                 `json:"id,omitempty"`
	Title        string                 `json:"title,omitempty"`
	StepHint     string                 `json:"step_hint,omitempty"`
	PanelTitle   string                 `json:"panel_title,omitempty"`
	Subtitle     string                 `json:"subtitle,omitempty"`
	LoadingLabel string                 `json:"loading_label,omitempty"`
	Renderer     RendererKey            `json:"renderer,omitempty"`
	Group        string                 `json:"group,omitempty"`
	GroupTitle   string                 `json:"group_title,omitempty"`
	Icon         string                 `json:"icon,omitempty"`
	Action       string                 `json:"action,omitempty"`
	Actions      []string               `json:"actions,omitempty"`
	Mode         string                 `json:"mode,omitempty"`
	Block        *Block                 `json:"block,omitempty"`
	Fields       []string               `json:"fields,omitempty"`
	Columns      FieldMatrixColumnCount `json:"columns,omitempty"`
	Matrix       *FieldMatrix           `json:"matrix,omitempty"`
	ListPage     *ListPage              `json:"list_page,omitempty"`
	Collection   *CollectionConfig      `json:"collection,omitempty"`
	MediaUpload  *MediaUploadConfig     `json:"media_upload,omitempty"`
	MediaItems   []MediaGalleryItem     `json:"media_items,omitempty"`
	MediaLabels  *MediaGalleryLabels    `json:"media_labels,omitempty"`
	MediaActions *MediaGalleryActions   `json:"media_actions,omitempty"`
	Prompts      *PromptList            `json:"prompts,omitempty"`
	DateRange    *DateRangeConfig       `json:"date_range,omitempty"`
	// Resource declares another standard module action rendered inside this
	// section. It stays server-side: Generator resolves it to Load per request.
	Resource *Resource `json:"-"`
	// Load is the generated executable request for Resource. Consumers never
	// construct endpoints or bindings for a resource section.
	Load *ResourceLoad `json:"load,omitempty"`
}

// DateRangeConfig presents two ordinary form fields as one range control. It
// only affects presentation; generated add and update payloads stay flat.
type DateRangeConfig struct {
	StartField    string   `json:"start_field"`
	EndField      string   `json:"end_field"`
	Min           string   `json:"min,omitempty"`
	Max           string   `json:"max,omitempty"`
	DisabledDates []string `json:"disabled_dates,omitempty"`
	Placeholder   string   `json:"placeholder,omitempty"`
	ApplyLabel    string   `json:"apply_label,omitempty"`
	CancelLabel   string   `json:"cancel_label,omitempty"`
	StartLabel    string   `json:"start_label,omitempty"`
	EndLabel      string   `json:"end_label,omitempty"`
	EmptyLabel    string   `json:"empty_label,omitempty"`
	DialogLabel   string   `json:"dialog_label,omitempty"`
	PreviousLabel string   `json:"previous_label,omitempty"`
	NextLabel     string   `json:"next_label,omitempty"`
	Months        []string `json:"months,omitempty"`
	FormatMonths  []string `json:"format_months,omitempty"`
	Weekdays      []string `json:"weekdays,omitempty"`
}

func validateDateRangeSection(page *FormPage, section FormSection) error {
	if section.Renderer != RendererDateRange {
		if section.DateRange != nil {
			return fmt.Errorf("renderer.Universal: form section %q date range requires renderer %q", section.ID, RendererDateRange)
		}
		return nil
	}
	if section.DateRange == nil {
		return fmt.Errorf("renderer.Universal: date range section %q must define date_range", section.ID)
	}
	config := section.DateRange
	if config.StartField == "" || config.EndField == "" || config.StartField == config.EndField {
		return fmt.Errorf("renderer.Universal: date range section %q must define distinct start and end fields", section.ID)
	}
	pageFields := make(map[string]struct{}, len(page.Fields))
	for _, field := range page.Fields {
		pageFields[field] = struct{}{}
	}
	sectionFields := make(map[string]struct{}, len(section.Fields))
	for _, field := range section.Fields {
		sectionFields[field] = struct{}{}
	}
	for _, field := range []string{config.StartField, config.EndField} {
		if _, ok := pageFields[field]; !ok {
			return fmt.Errorf("renderer.Universal: date range section %q field %q is not declared by the form", section.ID, field)
		}
		if _, ok := sectionFields[field]; !ok {
			return fmt.Errorf("renderer.Universal: date range section %q field %q is not declared by the section", section.ID, field)
		}
	}
	if len(config.Months) != 0 && len(config.Months) != 12 {
		return fmt.Errorf("renderer.Universal: date range section %q months must contain 12 values", section.ID)
	}
	if len(config.FormatMonths) != 0 && len(config.FormatMonths) != 12 {
		return fmt.Errorf("renderer.Universal: date range section %q format_months must contain 12 values", section.ID)
	}
	if len(config.Weekdays) != 0 && len(config.Weekdays) != 7 {
		return fmt.Errorf("renderer.Universal: date range section %q weekdays must contain 7 values", section.ID)
	}
	for _, value := range append(append([]string{}, config.Min, config.Max), config.DisabledDates...) {
		if value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return fmt.Errorf("renderer.Universal: date range section %q date %q must use YYYY-MM-DD", section.ID, value)
		}
	}
	return nil
}

type FieldMatrixType string

const (
	FieldMatrixTypeTable FieldMatrixType = "table"
	FieldMatrixTypeList  FieldMatrixType = "list"
)

type FieldMatrixColumnCount uint8

const (
	FieldMatrixColumnsOne   FieldMatrixColumnCount = 1
	FieldMatrixColumnsTwo   FieldMatrixColumnCount = 2
	FieldMatrixColumnsThree FieldMatrixColumnCount = 3
	FieldMatrixColumnsFour  FieldMatrixColumnCount = 4
)

type FieldMatrix struct {
	Type      FieldMatrixType   `json:"type,omitempty"`
	Underline string            `json:"underline,omitempty"`
	List      *FieldMatrixList  `json:"list,omitempty"`
	Table     *FieldMatrixTable `json:"table,omitempty"`
}

type FieldMatrixList struct {
	Fields  []string               `json:"fields,omitempty"`
	Columns FieldMatrixColumnCount `json:"columns,omitempty"`
}

type FieldMatrixTable struct {
	Heads        []string                     `json:"heads,omitempty"`
	Rows         []FieldMatrixRow             `json:"rows,omitempty"`
	Presentation FieldMatrixTablePresentation `json:"presentation,omitempty"`
	Source       *FieldMatrixDataSource       `json:"source,omitempty"`
}

// FieldMatrixTablePresentation selects a reusable visual arrangement for the
// same typed rows and cells. It never changes the data or action contract.
type FieldMatrixTablePresentation string

const (
	FieldMatrixTablePresentationGrid      FieldMatrixTablePresentation = "grid"
	FieldMatrixTablePresentationChips     FieldMatrixTablePresentation = "chips"
	FieldMatrixTablePresentationAccordion FieldMatrixTablePresentation = "accordion"
)

type FieldMatrixRow struct {
	ID          string            `json:"id,omitempty"`
	Label       string            `json:"label,omitempty"`
	Description string            `json:"description,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Tone        string            `json:"tone,omitempty"`
	Cells       []FieldMatrixCell `json:"cells,omitempty"`
}

type FieldMatrixCell struct {
	Field          string `json:"field,omitempty"`
	Label          string `json:"label,omitempty"`
	Text           string `json:"text,omitempty"`
	Icon           string `json:"icon,omitempty"`
	AvailableField string `json:"available_field,omitempty"`
}

// FieldMatrixDataSource connects a table layout to a standard list/update
// pair. The matrix owns only presentation and editable boolean field names;
// the generator resolves executable requests for the referenced actions.
// This keeps matrix consumers free of producer endpoint conventions.
type FieldMatrixDataSource struct {
	IDField  string `json:"id_field,omitempty"`
	KeyField string `json:"key_field,omitempty"`
	// Row maps a record returned by List to a table row. With it, table rows
	// are fully data-driven; Rows is only used for static matrix layouts.
	Row *FieldMatrixDataRow `json:"row,omitempty"`

	// List and Update are producer-only standard action references. Load is
	// the public executable contract built for the current principal.
	List   ActionResource             `json:"-"`
	Update ActionResource             `json:"-"`
	Load   *FieldMatrixDataSourceLoad `json:"load,omitempty"`
}

// FieldMatrixDataRow declares how a list record is presented as one matrix
// row. Text values can be translation keys resolved by the UI's standard
// localization function.
type FieldMatrixDataRow struct {
	LabelField       string            `json:"label_field,omitempty"`
	DescriptionField string            `json:"description_field,omitempty"`
	IconField        string            `json:"icon_field,omitempty"`
	ToneField        string            `json:"tone_field,omitempty"`
	Cells            []FieldMatrixCell `json:"cells,omitempty"`
}

type FieldMatrixDataSourceLoad struct {
	List   ResourceLoad `json:"list"`
	Update ResourceLoad `json:"update"`
}

func validateFormSectionColumns(section FormSection) error {
	switch section.Columns {
	case 0, FieldMatrixColumnsOne, FieldMatrixColumnsTwo, FieldMatrixColumnsThree, FieldMatrixColumnsFour:
		return nil
	default:
		return fmt.Errorf("renderer.Universal: form section %q has unsupported columns", section.ID)
	}
}

func (matrix *FieldMatrix) Validate(sectionID string) error {
	if matrix == nil {
		return nil
	}
	switch matrix.Type {
	case FieldMatrixTypeTable:
		if matrix.Table == nil || matrix.List != nil {
			return fmt.Errorf("renderer.Universal: matrix section %q table type must define only table", sectionID)
		}
		if len(matrix.Table.Heads) == 0 || (len(matrix.Table.Rows) == 0 && (matrix.Table.Source == nil || matrix.Table.Source.Row == nil)) {
			return fmt.Errorf("renderer.Universal: matrix section %q table must define heads and rows or a source row", sectionID)
		}
		switch matrix.Table.Presentation {
		case "", FieldMatrixTablePresentationGrid, FieldMatrixTablePresentationChips, FieldMatrixTablePresentationAccordion:
		default:
			return fmt.Errorf("renderer.Universal: matrix section %q table has unsupported presentation %q", sectionID, matrix.Table.Presentation)
		}
		if matrix.Table.Source != nil {
			source := matrix.Table.Source
			if source.IDField == "" || source.KeyField == "" {
				return fmt.Errorf("renderer.Universal: matrix section %q table source must define id and key fields", sectionID)
			}
			if err := source.List.Validate("field matrix source list"); err != nil {
				return fmt.Errorf("renderer.Universal: matrix section %q: %w", sectionID, err)
			}
			if err := source.Update.Validate("field matrix source update"); err != nil {
				return fmt.Errorf("renderer.Universal: matrix section %q: %w", sectionID, err)
			}
			if source.Row != nil {
				if err := validateFieldMatrixCells(sectionID, -1, len(matrix.Table.Heads), true, source.Row.Cells); err != nil {
					return err
				}
			}
		}
		for rowIndex, row := range matrix.Table.Rows {
			if matrix.Table.Source != nil && row.ID == "" {
				return fmt.Errorf("renderer.Universal: matrix section %q source row %d must define id", sectionID, rowIndex)
			}
			if err := validateFieldMatrixCells(sectionID, rowIndex, len(matrix.Table.Heads), row.Label != "", row.Cells); err != nil {
				return err
			}
		}
	case FieldMatrixTypeList:
		if matrix.List == nil || matrix.Table != nil {
			return fmt.Errorf("renderer.Universal: matrix section %q list type must define only list", sectionID)
		}
		if len(matrix.List.Fields) == 0 {
			return fmt.Errorf("renderer.Universal: matrix section %q list must define fields", sectionID)
		}
		switch matrix.List.Columns {
		case FieldMatrixColumnsOne, FieldMatrixColumnsTwo, FieldMatrixColumnsThree, FieldMatrixColumnsFour:
		default:
			return fmt.Errorf("renderer.Universal: matrix section %q list has unsupported columns", sectionID)
		}
	default:
		return fmt.Errorf("renderer.Universal: matrix section %q has unsupported matrix type %q", sectionID, matrix.Type)
	}
	return nil
}

func validateFieldMatrixCells(sectionID string, rowIndex, heads int, hasLabel bool, cells []FieldMatrixCell) error {
	expectedCells := heads
	if hasLabel {
		expectedCells--
	}
	if len(cells) != expectedCells {
		return fmt.Errorf("renderer.Universal: matrix section %q row %d cells must match heads", sectionID, rowIndex)
	}
	for cellIndex, cell := range cells {
		if (cell.Field == "") == (cell.Text == "") {
			return fmt.Errorf("renderer.Universal: matrix section %q row %d cell %d must define exactly one of field or text", sectionID, rowIndex, cellIndex)
		}
		if cell.AvailableField != "" && cell.Field == "" {
			return fmt.Errorf("renderer.Universal: matrix section %q row %d cell %d availability requires field", sectionID, rowIndex, cellIndex)
		}
	}
	return nil
}

type MediaUploadConfig struct {
	Title        string `json:"title,omitempty"`
	Subtitle     string `json:"subtitle,omitempty"`
	LoadingTitle string `json:"loading_title,omitempty"`
	Accept       string `json:"accept,omitempty"`
	Multiple     bool   `json:"multiple"`
}

type MediaGalleryItem struct {
	ID              string          `json:"id,omitempty"`
	MediaID         int64           `json:"media_id,omitempty"`
	LinkID          int64           `json:"link_id,omitempty"`
	Kind            MediaKind       `json:"kind,omitempty"`
	Src             string          `json:"src,omitempty"`
	Poster          string          `json:"poster,omitempty"`
	Thumbnail       string          `json:"thumbnail,omitempty"`
	Visibility      MediaVisibility `json:"visibility,omitempty"`
	HideFace        bool            `json:"hide_face,omitempty"`
	PrivacyEditable bool            `json:"privacy_editable,omitempty"`
	AccessGranted   *bool           `json:"access_granted,omitempty"`
	Usage           MediaUsage      `json:"usage,omitempty"`
	SortOrder       int             `json:"sort_order"`
	Title           string          `json:"title,omitempty"`
	Description     string          `json:"description,omitempty"`
	// Badges are server-owned annotations for an individual gallery item. They
	// are useful for state that must survive reloads, such as a published media
	// item, without making the browser infer state from a URL or local cache.
	Badges  []Badge  `json:"badges,omitempty"`
	Actions []Action `json:"actions,omitempty"`
}

type MediaGalleryLabels struct {
	Public       string `json:"public,omitempty"`
	Private      string `json:"private,omitempty"`
	Empty        string `json:"empty,omitempty"`
	CoverBadge   string `json:"cover_badge,omitempty"`
	Remove       string `json:"remove,omitempty"`
	Reorder      string `json:"reorder,omitempty"`
	FirstIsCover string `json:"first_is_cover,omitempty"`
	PrivateHint  string `json:"private_hint,omitempty"`
	HideFace     string `json:"hide_face,omitempty"`
	HideFaceHint string `json:"hide_face_hint,omitempty"`
}

type MediaGalleryActions struct {
	Upload   *Action `json:"upload,omitempty"`
	Link     *Action `json:"link,omitempty"`
	Update   *Action `json:"update,omitempty"`
	Reorder  *Action `json:"reorder,omitempty"`
	Recenter *Action `json:"recenter,omitempty"`
	Crop     *Action `json:"crop,omitempty"`
	Remove   *Action `json:"remove,omitempty"`
}

type CollectionConfig struct {
	Module       string             `json:"module,omitempty"`
	Relation     string             `json:"relation,omitempty"`
	Item         *CollectionItem    `json:"item,omitempty"`
	Size         int                `json:"size,omitempty"`
	LoadingLabel string             `json:"loading_label,omitempty"`
	Buckets      []CollectionBucket `json:"buckets,omitempty"`
	EditFields   []string           `json:"edit_fields,omitempty"`
	Modal        *CollectionModal   `json:"modal,omitempty"`
	Actions      []Action           `json:"actions,omitempty"`
}

type CollectionItem struct {
	LabelField       string   `json:"label_field,omitempty"`
	MetaFields       []string `json:"meta_fields,omitempty"`
	DescriptionField string   `json:"description_field,omitempty"`
	MediaField       string   `json:"media_field,omitempty"`
	StatusField      string   `json:"status_field,omitempty"`
}

type CollectionBucket struct {
	ID            string                        `json:"id,omitempty"`
	Title         string                        `json:"title,omitempty"`
	CountLabel    string                        `json:"count_label,omitempty"`
	AddLabel      string                        `json:"add_label,omitempty"`
	ClearLabel    string                        `json:"clear_label,omitempty"`
	ModalTitle    string                        `json:"modal_title,omitempty"`
	ModalSubtitle string                        `json:"modal_subtitle,omitempty"`
	ConfirmLabel  string                        `json:"confirm_label,omitempty"`
	BlockID       string                        `json:"block_id,omitempty"`
	Block         *Block                        `json:"block,omitempty"`
	Predicate     *CollectionPredicate          `json:"predicate,omitempty"`
	Defaults      []CollectionFieldDefaultValue `json:"defaults,omitempty"`
	EditFields    []string                      `json:"edit_fields,omitempty"`
	Actions       []Action                      `json:"actions,omitempty"`
}

type CollectionPredicateOperator string

const (
	CollectionPredicateEquals      CollectionPredicateOperator = "eq"
	CollectionPredicateNotEquals   CollectionPredicateOperator = "ne"
	CollectionPredicateIn          CollectionPredicateOperator = "in"
	CollectionPredicateNotIn       CollectionPredicateOperator = "not_in"
	CollectionPredicateEmpty       CollectionPredicateOperator = "empty"
	CollectionPredicateNotEmpty    CollectionPredicateOperator = "not_empty"
	CollectionPredicateGreaterThan CollectionPredicateOperator = "gt"
	CollectionPredicateLessThan    CollectionPredicateOperator = "lt"
	CollectionPredicateGTE         CollectionPredicateOperator = "gte"
	CollectionPredicateLTE         CollectionPredicateOperator = "lte"
)

type CollectionPredicate struct {
	Field    string                      `json:"field,omitempty"`
	Operator CollectionPredicateOperator `json:"operator,omitempty"`
	Value    *TypedValue                 `json:"value,omitempty"`
	Values   []TypedValue                `json:"values,omitempty"`
}

type CollectionFieldDefaultValue struct {
	Field string     `json:"field,omitempty"`
	Value TypedValue `json:"value"`
}

type TypedValueType string

const (
	TypedValueString TypedValueType = "string"
	TypedValueNumber TypedValueType = "number"
	TypedValueBool   TypedValueType = "bool"
	TypedValueNull   TypedValueType = "null"
)

type TypedValue struct {
	Type   TypedValueType `json:"type"`
	String string         `json:"string,omitempty"`
	Number float64        `json:"number,omitempty"`
	Bool   *bool          `json:"bool,omitempty"`
}

func (v TypedValue) Validate() error {
	switch v.Type {
	case TypedValueString, TypedValueNumber, TypedValueNull:
		return nil
	case TypedValueBool:
		if v.Bool == nil {
			return fmt.Errorf("boolean typed value requires bool")
		}
		return nil
	default:
		return fmt.Errorf("unsupported typed value type %q", v.Type)
	}
}

// MarshalJSON keeps a typed zero value on the wire. A plain omitempty tag on
// Number makes number: 0 indistinguishable from an omitted value to clients.
func (v TypedValue) MarshalJSON() ([]byte, error) {
	type typedValueJSON struct {
		Type   TypedValueType `json:"type"`
		String *string        `json:"string,omitempty"`
		Number *float64       `json:"number,omitempty"`
		Bool   *bool          `json:"bool,omitempty"`
	}

	payload := typedValueJSON{Type: v.Type}
	switch v.Type {
	case TypedValueString:
		payload.String = &v.String
	case TypedValueNumber:
		payload.Number = &v.Number
	case TypedValueBool:
		payload.Bool = v.Bool
	}
	return json.Marshal(payload)
}

type CollectionModal struct {
	Icon                string `json:"icon,omitempty"`
	Search              bool   `json:"search"`
	SearchPlaceholder   string `json:"search_placeholder,omitempty"`
	EmptyLabel          string `json:"empty_label,omitempty"`
	SelectedLabel       string `json:"selected_label,omitempty"`
	TakenLabel          string `json:"taken_label,omitempty"`
	CancelLabel         string `json:"cancel_label,omitempty"`
	ConfirmLoadingLabel string `json:"confirm_loading_label,omitempty"`
}

type Block struct {
	Type           BlockType       `json:"type,omitempty"`
	Variant        BlockVariant    `json:"variant,omitempty"`
	TitleDecor     TitleDecorToken `json:"title_decor,omitempty"`
	TitleBar       ToneToken       `json:"title_bar,omitempty"`
	TitleUnderline ToneToken       `json:"title_underline,omitempty"`
	Inset          InsetToken      `json:"inset,omitempty"`
	MaxWidth       string          `json:"max_width,omitempty"`
	BodyClass      string          `json:"body_class,omitempty"`
	BorderStyle    string          `json:"border_style,omitempty"`
	HoverEnabled   *bool           `json:"hover_enabled,omitempty"`
	Effect         string          `json:"effect,omitempty"`
	Overlays       []BlockOverlay  `json:"overlays,omitempty"`
}

// BlockOverlay places typed badge data over any visual block.
// The renderer resolves badge values against the current record.
type BlockOverlay struct {
	ID       string               `json:"id,omitempty"`
	Position MediaOverlayPosition `json:"position"`
	Badges   []Badge              `json:"badges"`
	Size     SizeToken            `json:"size,omitempty"`
	Wrap     *bool                `json:"wrap,omitempty"`
}

type Stack struct {
	Gap       SpacingToken   `json:"gap,omitempty"`
	Direction DirectionToken `json:"direction,omitempty"`
	Wrap      *bool          `json:"wrap,omitempty"`
	Justify   JustifyToken   `json:"justify,omitempty"`
	Align     AlignToken     `json:"align,omitempty"`
	Inset     InsetToken     `json:"inset,omitempty"`
}

type DisplayComponent struct {
	ID                  string                   `json:"id,omitempty"`
	Type                DisplayComponentType     `json:"type,omitempty"`
	ActionID            string                   `json:"action_id,omitempty"`
	Fields              []string                 `json:"fields,omitempty"`
	MediaItems          []MediaGalleryItem       `json:"media_items,omitempty"`
	Value               interface{}              `json:"value,omitempty"`
	Default             interface{}              `json:"default,omitempty"`
	Visible             *bool                    `json:"visible,omitempty"`
	UpdateAction        ComponentAction          `json:"update_action,omitempty"`
	MainRatio           ComponentRatio           `json:"main_ratio,omitempty"`
	MainRadius          ComponentRadiusToken     `json:"main_radius,omitempty"`
	MainRadiusToken     ComponentRadiusToken     `json:"main_radius_token,omitempty"`
	ThumbRatio          ComponentRatio           `json:"thumb_ratio,omitempty"`
	ThumbsInset         InsetToken               `json:"thumbs_inset,omitempty"`
	ThumbsInsetToken    SpacingToken             `json:"thumbs_inset_token,omitempty"`
	VideoControls       *bool                    `json:"video_controls,omitempty"`
	Size                SizeToken                `json:"size,omitempty"`
	Wrap                *bool                    `json:"wrap,omitempty"`
	Gap                 SpacingToken             `json:"gap,omitempty"`
	Direction           DirectionToken           `json:"direction,omitempty"`
	Justify             JustifyToken             `json:"justify,omitempty"`
	Align               AlignToken               `json:"align,omitempty"`
	Inset               InsetToken               `json:"inset,omitempty"`
	Compact             bool                     `json:"compact,omitempty"`
	Columns             int                      `json:"columns,omitempty"`
	ReadonlyColumns     int                      `json:"readonly_columns,omitempty"`
	DisplayType         ComponentDisplayType     `json:"display_type,omitempty"`
	Items               []DisplayFieldRef        `json:"items,omitempty"`
	CollectionGroups    *DisplayCollectionGroups `json:"collection_groups,omitempty"`
	SeparatorVariant    ToneToken                `json:"separator_variant,omitempty"`
	SeparatorAppearance SeparatorAppearance      `json:"separator_appearance,omitempty"`
	MatrixColumns       []map[string]interface{} `json:"matrix_columns,omitempty"`
	ValueLabel          string                   `json:"value_label,omitempty"`
	ValueFallback       string                   `json:"value_fallback,omitempty"`
	MatrixLabel         string                   `json:"matrix_label,omitempty"`
	MatrixLabelIcon     string                   `json:"matrix_label_icon,omitempty"`
	Block               *Block                   `json:"block,omitempty"`
	Title               string                   `json:"title,omitempty"`
	TitleFallback       string                   `json:"title_fallback,omitempty"`
	Subtitle            string                   `json:"subtitle,omitempty"`
	SubtitleFallback    string                   `json:"subtitle_fallback,omitempty"`
	TitleLevel          int                      `json:"title_level,omitempty"`
	TitleTone           ToneToken                `json:"title_tone,omitempty"`
	BodyClass           string                   `json:"body_class,omitempty"`
}

type DisplayFieldRef struct {
	Field         string `json:"field"`
	Label         string `json:"label,omitempty"`
	LabelFallback string `json:"label_fallback,omitempty"`
}

type DisplayCollectionGroup struct {
	ID            string     `json:"id"`
	Label         string     `json:"label,omitempty"`
	LabelFallback string     `json:"label_fallback,omitempty"`
	Tone          string     `json:"tone,omitempty"`
	ItemCondition *Condition `json:"item_condition,omitempty"`
}

type DisplayCollectionGroups struct {
	SourceField string                   `json:"source_field"`
	Groups      []DisplayCollectionGroup `json:"groups"`
}

type RecordPage struct {
	ID            string            `json:"id,omitempty"`
	Title         string            `json:"title,omitempty"`
	Subtitle      string            `json:"subtitle,omitempty"`
	ShowHeader    *bool             `json:"show_header,omitempty"`
	Badge         string            `json:"badge,omitempty"`
	BadgeTone     string            `json:"badge_tone,omitempty"`
	BadgeTeleport string            `json:"badge_teleport,omitempty"`
	Navigation    *RecordNavigation `json:"navigation,omitempty"`
	Layout        *Layout           `json:"layout,omitempty"`
	Sections      []RecordSection   `json:"sections,omitempty"`
	Theme         *RecordTheme      `json:"theme,omitempty"`
	Actions       []Action          `json:"actions,omitempty"`
}

type RecordNavigation struct {
	Type    string `json:"type,omitempty"`
	Enabled bool   `json:"enabled"`
}

type RecordTheme struct {
	Surfaces   map[string]string      `json:"surfaces,omitempty"`
	Headings   map[string]interface{} `json:"headings,omitempty"`
	Badges     map[string]string      `json:"badges,omitempty"`
	Buttons    map[string]interface{} `json:"buttons,omitempty"`
	Media      map[string]string      `json:"media,omitempty"`
	Components map[string]interface{} `json:"components,omitempty"`
}

type RecordSection struct {
	ID            string                `json:"id,omitempty"`
	Title         string                `json:"title,omitempty"`
	TitleFallback string                `json:"title_fallback,omitempty"`
	TitleLevel    int                   `json:"title_level,omitempty"`
	TitleTone     ToneToken             `json:"title_tone,omitempty"`
	Renderer      RecordSectionRenderer `json:"renderer,omitempty"`
	LayoutSlot    LayoutSlotToken       `json:"layout_slot,omitempty"`
	Order         int                   `json:"order,omitempty"`
	MobileOrder   int                   `json:"mobile_order,omitempty"`
	Block         *Block                `json:"block,omitempty"`
	Stack         *Stack                `json:"stack,omitempty"`
	Components    []DisplayComponent    `json:"components,omitempty"`
}

type ResourceGridPage struct {
	Endpoint string                     `json:"endpoint,omitempty"`
	List     *ResourceGridListConfig    `json:"list,omitempty"`
	Create   *Action                    `json:"create,omitempty"`
	Delete   *Action                    `json:"delete,omitempty"`
	Update   *Action                    `json:"update,omitempty"`
	Card     *CardSchema                `json:"card,omitempty"`
	Status   *ResourceGridStatusConfig  `json:"status,omitempty"`
	Actions  *ResourceGridActionsConfig `json:"actions,omitempty"`
	Text     map[string]string          `json:"text,omitempty"`
	Context  map[string]interface{}     `json:"context,omitempty"`
}

type ResourceGridListConfig struct {
	Size    int                    `json:"size,omitempty"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

type ResourceGridStatusConfig struct {
	VerifyField         string        `json:"verifyField,omitempty"`
	ActiveField         string        `json:"activeField,omitempty"`
	VerifiedValue       string        `json:"verifiedValue,omitempty"`
	PendingValue        string        `json:"pendingValue,omitempty"`
	InactiveValue       string        `json:"inactiveValue,omitempty"`
	InactiveActionValue string        `json:"inactiveActionValue,omitempty"`
	ActiveActionValue   string        `json:"activeActionValue,omitempty"`
	DraftValues         []interface{} `json:"draftValues,omitempty"`
	PendingPayload      interface{}   `json:"pendingPayload,omitempty"`
}

type ResourceGridActionsConfig struct {
	EditRoute interface{} `json:"editRoute,omitempty"`
}

// ActionPresentation describes visual and interaction state shared by every
// action surface. It never carries a request, route or action result.
//
// Action embeds this structure to keep its JSON shape flat. WorkspaceCommand
// uses it as a nested value because its request is resolved separately from a
// standard generator action.
type ActionPresentation struct {
	Icon             string           `json:"icon,omitempty"`
	IconOnly         *bool            `json:"icon_only,omitempty"`
	Variant          ActionVariant    `json:"variant,omitempty"`
	Appearance       ActionAppearance `json:"appearance,omitempty"`
	Placement        ActionPlacement  `json:"placement,omitempty"`
	ActiveAppearance ActionAppearance `json:"active_appearance,omitempty"`
	Active           string           `json:"active,omitempty"`
	Block            *bool            `json:"block,omitempty"`
	VisibleIf        *Condition       `json:"visible_if,omitempty"`
	HiddenIf         *Condition       `json:"hidden_if,omitempty"`
	DisabledIf       *Condition       `json:"disabled_if,omitempty"`
}

func (presentation ActionPresentation) Validate() error {
	if !presentation.Placement.Valid() {
		return fmt.Errorf("unsupported placement %q", presentation.Placement)
	}
	if presentation.VisibleIf != nil && !hasCondition(presentation.VisibleIf) {
		return fmt.Errorf("visible_if is invalid")
	}
	if presentation.HiddenIf != nil && !hasCondition(presentation.HiddenIf) {
		return fmt.Errorf("hidden_if is invalid")
	}
	if presentation.DisabledIf != nil && !hasCondition(presentation.DisabledIf) {
		return fmt.Errorf("disabled_if is invalid")
	}
	return nil
}

type Action struct {
	ActionPresentation
	ID             string         `json:"id,omitempty"`
	Type           ActionType     `json:"type,omitempty"`
	Behavior       ActionBehavior `json:"behavior,omitempty"`
	Label          string         `json:"label,omitempty"`
	LabelKey       string         `json:"label_key,omitempty"`
	AriaLabel      string         `json:"aria_label,omitempty"`
	Title          string         `json:"title,omitempty"`
	SavingLabel    string         `json:"saving_label,omitempty"`
	SavedLabel     string         `json:"saved_label,omitempty"`
	External       *bool          `json:"external,omitempty"`
	Endpoint       string         `json:"endpoint,omitempty"`
	Method         string         `json:"method,omitempty"`
	UniqueEndpoint string         `json:"uniqueEndpoint,omitempty"`
	AfterRoute     interface{}    `json:"afterRoute,omitempty"`
	Route          interface{}    `json:"route,omitempty"`
	API            *APIAction     `json:"api,omitempty"`
	Modal          *ModalAction   `json:"modal,omitempty"`
	Client         *ClientAction  `json:"client,omitempty"`
	Confirm        *Confirm       `json:"confirm,omitempty"`
	AfterSuccess   *ActionResult  `json:"after_success,omitempty"`
	AfterError     *ActionResult  `json:"after_error,omitempty"`
	AriaLabelKey   string         `json:"aria_label_key,omitempty"`
	TitleKey       string         `json:"title_key,omitempty"`
	Test           string         `json:"test,omitempty"`
}

type ActionBehavior string

const (
	ActionBehaviorReset  ActionBehavior = "reset"
	ActionBehaviorSubmit ActionBehavior = "submit"
)

func (action Action) Validate() error {
	if err := action.ActionPresentation.Validate(); err != nil {
		return err
	}
	switch action.Behavior {
	case "", ActionBehaviorReset, ActionBehaviorSubmit:
	default:
		return fmt.Errorf("unsupported behavior %q", action.Behavior)
	}
	if action.AfterSuccess != nil {
		if err := action.AfterSuccess.Validate(); err != nil {
			return fmt.Errorf("after success: %w", err)
		}
	}
	if action.AfterError != nil {
		if action.AfterError.Widget != nil && action.AfterError.Widget.Selection != nil {
			return fmt.Errorf("after error: widget selection is only allowed after success")
		}
		if err := action.AfterError.Validate(); err != nil {
			return fmt.Errorf("after error: %w", err)
		}
	}
	if action.Client != nil {
		if err := action.Client.Validate(); err != nil {
			return fmt.Errorf("client: %w", err)
		}
	}
	return nil
}

type RouteAction struct {
	Path   string                 `json:"path,omitempty"`
	Params map[string]string      `json:"params,omitempty"`
	Query  map[string]interface{} `json:"query,omitempty"`
}

type APIAction struct {
	Method   string                 `json:"method,omitempty"`
	Endpoint string                 `json:"endpoint,omitempty"`
	Params   map[string]string      `json:"params,omitempty"`
	Query    map[string]interface{} `json:"query,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
}

type ModalAction struct {
	Renderer   RendererKey            `json:"renderer,omitempty"`
	Title      string                 `json:"title,omitempty"`
	ShowHeader *bool                  `json:"show_header,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

// ClientAction describes a client capability selected by the API. The
// renderer does not implement the capability itself; an application registers
// the named handler and receives the typed arguments supplied by the producer.
// This keeps browser-only operations, such as Web Push permission, out of a
// server action while preserving the API as the source of its configuration.
type ClientAction struct {
	Name      string                 `json:"name,omitempty"`
	Arguments []ClientActionArgument `json:"arguments,omitempty"`
}

type ClientActionArgument struct {
	Name  string     `json:"name,omitempty"`
	Value TypedValue `json:"value,omitempty"`
}

func (action *ClientAction) Validate() error {
	if action == nil {
		return nil
	}
	if action.Name == "" {
		return fmt.Errorf("name is required")
	}
	seen := make(map[string]struct{}, len(action.Arguments))
	for _, argument := range action.Arguments {
		if argument.Name == "" {
			return fmt.Errorf("argument name is required")
		}
		if _, exists := seen[argument.Name]; exists {
			return fmt.Errorf("argument %q is duplicated", argument.Name)
		}
		seen[argument.Name] = struct{}{}
		if err := argument.Value.Validate(); err != nil {
			return fmt.Errorf("argument %q: %w", argument.Name, err)
		}
	}
	return nil
}

// PromptList declares contextual notices rendered within a form section.
// Prompts use the same Action contract as other renderer controls, so their
// visual content and executable target remain producer-owned.
type PromptList struct {
	Variant string   `json:"variant,omitempty"`
	Items   []Prompt `json:"items,omitempty"`
}

type Prompt struct {
	ID          string     `json:"id,omitempty"`
	Kind        string     `json:"kind,omitempty"`
	Tone        string     `json:"tone,omitempty"`
	Icon        string     `json:"icon,omitempty"`
	Title       string     `json:"title,omitempty"`
	Text        string     `json:"text,omitempty"`
	Action      *Action    `json:"action,omitempty"`
	VisibleIf   *Condition `json:"visible_if,omitempty"`
	Dismissible bool       `json:"dismissible,omitempty"`
}

func (list *PromptList) Validate() error {
	if list == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(list.Items))
	for _, prompt := range list.Items {
		if prompt.ID == "" {
			return fmt.Errorf("prompt id is required")
		}
		if _, exists := seen[prompt.ID]; exists {
			return fmt.Errorf("prompt %q is duplicated", prompt.ID)
		}
		seen[prompt.ID] = struct{}{}
		if prompt.Text == "" && prompt.Title == "" {
			return fmt.Errorf("prompt %q must define title or text", prompt.ID)
		}
		if prompt.VisibleIf != nil && !hasCondition(prompt.VisibleIf) {
			return fmt.Errorf("prompt %q visible_if is invalid", prompt.ID)
		}
		if prompt.Action != nil {
			if err := prompt.Action.Validate(); err != nil {
				return fmt.Errorf("prompt %q action: %w", prompt.ID, err)
			}
		}
	}
	return nil
}

type Confirm struct {
	Title        string `json:"title,omitempty"`
	Message      string `json:"message,omitempty"`
	CancelLabel  string `json:"cancel_label,omitempty"`
	ConfirmLabel string `json:"confirm_label,omitempty"`
}

func (confirm Confirm) Validate() error {
	if confirm.Title == "" {
		return fmt.Errorf("title is required")
	}
	if confirm.Message == "" {
		return fmt.Errorf("message is required")
	}
	if confirm.CancelLabel == "" {
		return fmt.Errorf("cancel_label is required")
	}
	if confirm.ConfirmLabel == "" {
		return fmt.Errorf("confirm_label is required")
	}
	return nil
}

type ActionResult struct {
	Reload string        `json:"reload,omitempty"`
	Toast  string        `json:"toast,omitempty"`
	Route  string        `json:"route,omitempty"`
	Emit   string        `json:"emit,omitempty"`
	Widget *WidgetTarget `json:"widget,omitempty"`
}

func (result ActionResult) Validate() error {
	if result.Widget == nil {
		return nil
	}
	if err := result.Widget.Validate(); err != nil {
		return fmt.Errorf("widget: %w", err)
	}
	return nil
}

type Condition struct {
	Path      string        `json:"path,omitempty"`
	Equals    interface{}   `json:"equals,omitempty"`
	NotEquals interface{}   `json:"not_equals,omitempty"`
	In        []interface{} `json:"in,omitempty"`
	NotIn     []interface{} `json:"not_in,omitempty"`
	Empty     *bool         `json:"empty,omitempty"`
	NotEmpty  *bool         `json:"not_empty,omitempty"`
	Truthy    *bool         `json:"truthy,omitempty"`
	Falsy     *bool         `json:"falsy,omitempty"`
	All       []Condition   `json:"all,omitempty"`
	Any       []Condition   `json:"any,omitempty"`
	Not       interface{}   `json:"not,omitempty"`
}
