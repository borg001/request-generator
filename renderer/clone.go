package renderer

func (r Universal) Clone() Universal {
	return Universal{
		List:         cloneListPage(r.List),
		Form:         cloneFormPage(r.Form),
		Record:       cloneRecordPage(r.Record),
		ResourceGrid: cloneResourceGridPage(r.ResourceGrid),
	}
}

func cloneListPage(v *ListPage) *ListPage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.ShowHeader = clonePtr(v.ShowHeader)
	cp.Layout = cloneLayout(v.Layout)
	cp.Filters = cloneFilters(v.Filters)
	cp.Summary = cloneSummary(v.Summary)
	cp.GroupBy = cloneListGroupBy(v.GroupBy)
	cp.Grid = cloneGrid(v.Grid)
	cp.Pagination = clonePagination(v.Pagination)
	cp.CardSchema = cloneCardSchema(v.CardSchema)
	cp.Selection = cloneListSelection(v.Selection)
	cp.Context = cloneMap(v.Context)
	cp.Actions = cloneActions(v.Actions)
	return &cp
}

func cloneListSelection(v *ListSelection) *ListSelection {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Source = cloneAPIAction(v.Source)
	cp.Clear = cloneAction(v.Clear)
	cp.Proceed = cloneAction(v.Proceed)
	return &cp
}

func cloneFormPage(v *FormPage) *FormPage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Workflow = cloneFormWorkflow(v.Workflow)
	cp.Actions = cloneActions(v.Actions)
	cp.Sections = cloneFormSections(v.Sections)
	cp.Fields = cloneSlice(v.Fields)
	cp.Context = cloneMap(v.Context)
	return &cp
}

func cloneFormWorkflow(v *FormWorkflow) *FormWorkflow {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Summary != nil {
		summary := *v.Summary
		summary.Fields = cloneSlice(v.Summary.Fields)
		if v.Summary.Badge != nil {
			badges := cloneBadges([]Badge{*v.Summary.Badge})
			summary.Badge = &badges[0]
		}
		cp.Summary = &summary
	}
	return &cp
}

func cloneRecordPage(v *RecordPage) *RecordPage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.ShowHeader = clonePtr(v.ShowHeader)
	cp.Navigation = cloneRecordNavigation(v.Navigation)
	cp.Layout = cloneLayout(v.Layout)
	cp.Sections = cloneRecordSections(v.Sections)
	cp.Theme = cloneRecordTheme(v.Theme)
	cp.Actions = cloneActions(v.Actions)
	return &cp
}

func cloneRecordNavigation(v *RecordNavigation) *RecordNavigation {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneRecordTheme(v *RecordTheme) *RecordTheme {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Surfaces = cloneMap(v.Surfaces)
	cp.Headings = cloneMap(v.Headings)
	cp.Badges = cloneMap(v.Badges)
	cp.Buttons = cloneMap(v.Buttons)
	cp.Media = cloneMap(v.Media)
	cp.Components = cloneMap(v.Components)
	return &cp
}

func cloneResourceGridPage(v *ResourceGridPage) *ResourceGridPage {
	if v == nil {
		return nil
	}
	cp := *v
	cp.List = cloneResourceGridListConfig(v.List)
	cp.Create = cloneAction(v.Create)
	cp.Delete = cloneAction(v.Delete)
	cp.Update = cloneAction(v.Update)
	cp.Card = cloneCardSchema(v.Card)
	cp.Status = cloneResourceGridStatusConfig(v.Status)
	cp.Actions = cloneResourceGridActionsConfig(v.Actions)
	cp.Text = cloneMap(v.Text)
	cp.Context = cloneMap(v.Context)
	return &cp
}

func cloneResourceGridListConfig(v *ResourceGridListConfig) *ResourceGridListConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Filters = cloneMap(v.Filters)
	return &cp
}

func cloneResourceGridStatusConfig(v *ResourceGridStatusConfig) *ResourceGridStatusConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.DraftValues = cloneSlice(v.DraftValues)
	cp.PendingPayload = cloneInterface(v.PendingPayload)
	return &cp
}

func cloneResourceGridActionsConfig(v *ResourceGridActionsConfig) *ResourceGridActionsConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.EditRoute = cloneRouteValue(v.EditRoute)
	return &cp
}

func cloneLayout(v *Layout) *Layout {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Slots = cloneSlice(v.Slots)
	cp.MobileSlots = cloneSlice(v.MobileSlots)
	return &cp
}

func cloneFilters(v *Filters) *Filters {
	if v == nil {
		return nil
	}
	cp := *v
	cp.SecondaryEnabled = clonePtr(v.SecondaryEnabled)
	cp.Levels = cloneSlice(v.Levels)
	cp.Primary = cloneSlice(v.Primary)
	cp.Secondary = cloneSlice(v.Secondary)
	cp.More = cloneSlice(v.More)
	cp.Nested = cloneSlice(v.Nested)
	cp.Groups = cloneFilterGroups(v.Groups)
	cp.PillRows = cloneMapRows(v.PillRows)
	cp.SecondaryPillRows = cloneMapRows(v.SecondaryPillRows)
	cp.Reset = cloneFilterReset(v.Reset)
	cp.Text = clonePtr(v.Text)
	cp.RangePresets = cloneFilterRangePresets(v.RangePresets)
	cp.DateRange = cloneDateRangeToolbar(v.DateRange)
	return &cp
}

func cloneFilterGroups(values []FilterGroup) []FilterGroup {
	if values == nil {
		return nil
	}
	out := make([]FilterGroup, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Fields = cloneSlice(value.Fields)
		out[i].Sections = cloneFilterGroupSections(value.Sections)
		out[i].Items = cloneFilterGroupItems(value.Items)
	}
	return out
}

func cloneFilterGroupItems(values []FilterGroupItem) []FilterGroupItem {
	if values == nil {
		return nil
	}
	out := make([]FilterGroupItem, len(values))
	for i, value := range values {
		out[i] = value
		if value.Group != nil {
			cloned := cloneFilterGroups([]FilterGroup{*value.Group})
			out[i].Group = &cloned[0]
		}
	}
	return out
}

func cloneFilterGroupSections(values []FilterGroupSection) []FilterGroupSection {
	if values == nil {
		return nil
	}
	out := make([]FilterGroupSection, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Fields = cloneSlice(value.Fields)
	}
	return out
}

func cloneFilterRangePresets(values []FilterRangePresets) []FilterRangePresets {
	if values == nil {
		return nil
	}
	out := make([]FilterRangePresets, len(values))
	for i, value := range values {
		out[i] = FilterRangePresets{Field: value.Field, Presets: cloneSlice(value.Presets)}
	}
	return out
}

func cloneMapRows(values [][]FilterPill) [][]FilterPill {
	if values == nil {
		return nil
	}
	out := make([][]FilterPill, len(values))
	for i, row := range values {
		if row == nil {
			continue
		}
		out[i] = make([]FilterPill, len(row))
		for j, item := range row {
			out[i][j] = item
		}
	}
	return out
}

func cloneFilterReset(v *FilterReset) *FilterReset {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Preserve = cloneSlice(v.Preserve)
	return &cp
}

func cloneGrid(v *Grid) *Grid {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Columns != nil {
		columns := *v.Columns
		cp.Columns = &columns
	}
	return &cp
}

func clonePagination(v *Pagination) *Pagination {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneSummary(v *Summary) *Summary {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Items = cloneSlice(v.Items)
	cp.ShowOnline = clonePtr(v.ShowOnline)
	cp.ShowAction = clonePtr(v.ShowAction)
	cp.Resource = cloneResource(v.Resource)
	cp.Load = cloneResourceLoad(v.Load)
	if v.Trend != nil {
		cp.Trend = clonePtr(v.Trend)
		cp.Trend.Series = cloneSlice(v.Trend.Series)
		cp.Trend.DateRange = cloneDateRangeToolbar(v.Trend.DateRange)
	}
	return &cp
}

func cloneDateRangeToolbar(v *DateRangeToolbar) *DateRangeToolbar {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Presets = cloneSlice(v.Presets)
	cp.Months = cloneSlice(v.Months)
	cp.FormatMonths = cloneSlice(v.FormatMonths)
	cp.Weekdays = cloneSlice(v.Weekdays)
	return &cp
}

func cloneListGroupBy(v *ListGroupBy) *ListGroupBy {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneCardSchema(v *CardSchema) *CardSchema {
	if v == nil {
		return nil
	}
	cp := *v
	cp.LeadingAccent = cloneCardEdgeAccent(v.LeadingAccent)
	cp.Media = cloneMedia(v.Media)
	cp.Icon = cloneIconBinding(v.Icon)
	cp.Title = cloneTextBinding(v.Title)
	cp.Subtitle = cloneTextBinding(v.Subtitle)
	cp.Meta = cloneTextBinding(v.Meta)
	cp.Description = cloneTextBinding(v.Description)
	cp.Status = cloneStatusBinding(v.Status)
	cp.Badges = cloneBadges(v.Badges)
	cp.Stats = cloneBadges(v.Stats)
	cp.Actions = cloneActions(v.Actions)
	return &cp
}

func cloneCardEdgeAccent(v *CardEdgeAccent) *CardEdgeAccent {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneIconBinding(v *IconBinding) *IconBinding {
	if v == nil {
		return nil
	}
	cp := *v
	cp.IconMap = cloneMap(v.IconMap)
	cp.ToneMap = cloneMap(v.ToneMap)
	cp.Marker = cloneIconMarker(v.Marker)
	return &cp
}

func cloneIconMarker(v *IconMarker) *IconMarker {
	if v == nil {
		return nil
	}
	cp := *v
	cp.VisibleIf = cloneCondition(v.VisibleIf)
	return &cp
}

func cloneMedia(v *Media) *Media {
	if v == nil {
		return nil
	}
	cp := *v
	cp.GlowEnabled = clonePtr(v.GlowEnabled)
	return &cp
}

func cloneTextBinding(v *TextBinding) *TextBinding {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneBadges(values []Badge) []Badge {
	if values == nil {
		return nil
	}
	out := make([]Badge, len(values))
	for i, v := range values {
		out[i] = v
		out[i].Value = cloneTextBinding(v.Value)
		out[i].Marker = clonePtr(v.Marker)
		out[i].ToneMap = cloneMap(v.ToneMap)
		out[i].LabelMap = cloneMap(v.LabelMap)
		out[i].VisibleIf = cloneCondition(v.VisibleIf)
		out[i].Then = cloneBadgeState(v.Then)
		out[i].Else = cloneBadgeState(v.Else)
	}
	return out
}

func cloneBadgeState(v *BadgeState) *BadgeState {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Marker = clonePtr(v.Marker)
	return &cp
}

func cloneStatusBinding(v *StatusBinding) *StatusBinding {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Marker = clonePtr(v.Marker)
	cp.ToneMap = cloneMap(v.ToneMap)
	return &cp
}

func cloneFormSections(values []FormSection) []FormSection {
	if values == nil {
		return nil
	}
	out := make([]FormSection, len(values))
	for i, v := range values {
		out[i] = v
		out[i].Block = cloneBlock(v.Block)
		out[i].Actions = cloneSlice(v.Actions)
		out[i].Fields = cloneSlice(v.Fields)
		out[i].Matrix = cloneFieldMatrix(v.Matrix)
		out[i].ListPage = cloneListPage(v.ListPage)
		out[i].Collection = cloneCollectionConfig(v.Collection)
		out[i].MediaUpload = clonePtr(v.MediaUpload)
		out[i].MediaItems = cloneMediaGalleryItems(v.MediaItems)
		out[i].MediaLabels = clonePtr(v.MediaLabels)
		out[i].MediaActions = cloneMediaGalleryActions(v.MediaActions)
		out[i].Prompts = clonePromptList(v.Prompts)
		out[i].DateRange = cloneDateRangeConfig(v.DateRange)
		out[i].Resource = cloneResource(v.Resource)
		out[i].Load = cloneResourceLoad(v.Load)
	}
	return out
}

func cloneDateRangeConfig(v *DateRangeConfig) *DateRangeConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.DisabledDates = cloneSlice(v.DisabledDates)
	cp.Months = cloneSlice(v.Months)
	cp.FormatMonths = cloneSlice(v.FormatMonths)
	cp.Weekdays = cloneSlice(v.Weekdays)
	return &cp
}

func cloneFieldMatrix(v *FieldMatrix) *FieldMatrix {
	if v == nil {
		return nil
	}
	cp := *v
	if v.List != nil {
		cp.List = &FieldMatrixList{Fields: cloneSlice(v.List.Fields), Columns: v.List.Columns}
	}
	if v.Table != nil {
		cp.Table = &FieldMatrixTable{Heads: cloneSlice(v.Table.Heads), Rows: make([]FieldMatrixRow, len(v.Table.Rows)), Presentation: v.Table.Presentation, Source: cloneFieldMatrixDataSource(v.Table.Source)}
		for i, row := range v.Table.Rows {
			cp.Table.Rows[i] = FieldMatrixRow{ID: row.ID, Label: row.Label, Description: row.Description, Icon: row.Icon, Tone: row.Tone, Cells: cloneSlice(row.Cells)}
		}
	}
	return &cp
}

func cloneFieldMatrixDataSource(v *FieldMatrixDataSource) *FieldMatrixDataSource {
	if v == nil {
		return nil
	}
	cp := *v
	cp.List = ActionResource{Module: v.List.Module, Action: v.List.Action}
	cp.Update = ActionResource{Module: v.Update.Module, Action: v.Update.Action}
	if v.Load != nil {
		cp.Load = &FieldMatrixDataSourceLoad{List: *cloneResourceLoad(&v.Load.List), Update: *cloneResourceLoad(&v.Load.Update)}
	}
	if v.Row != nil {
		cp.Row = &FieldMatrixDataRow{
			LabelField:       v.Row.LabelField,
			DescriptionField: v.Row.DescriptionField,
			IconField:        v.Row.IconField,
			ToneField:        v.Row.ToneField,
			Cells:            cloneSlice(v.Row.Cells),
		}
	}
	return &cp
}

func CloneFieldPresentation(v *FieldPresentation) *FieldPresentation {
	if v == nil {
		return nil
	}
	cp := *v
	cp.VisibleIf = cloneCondition(v.VisibleIf)
	cp.ToneByValue = cloneSlice(v.ToneByValue)
	return &cp
}

func CloneFieldMediaConfig(v *FieldMediaConfig) *FieldMediaConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Item = cloneMediaGalleryItem(v.Item)
	cp.Upload = clonePtr(v.Upload)
	cp.Labels = clonePtr(v.Labels)
	cp.Actions = cloneMediaGalleryActions(v.Actions)
	cp.Cropper = clonePtr(v.Cropper)
	return &cp
}

func cloneMediaGalleryActions(v *MediaGalleryActions) *MediaGalleryActions {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Upload = cloneAction(v.Upload)
	cp.Link = cloneAction(v.Link)
	cp.Update = cloneAction(v.Update)
	cp.Reorder = cloneAction(v.Reorder)
	cp.Recenter = cloneAction(v.Recenter)
	cp.Crop = cloneAction(v.Crop)
	cp.Remove = cloneAction(v.Remove)
	return &cp
}

func cloneMediaGalleryItems(values []MediaGalleryItem) []MediaGalleryItem {
	if values == nil {
		return nil
	}
	out := make([]MediaGalleryItem, len(values))
	for i := range values {
		out[i] = values[i]
		out[i].AccessGranted = clonePtr(values[i].AccessGranted)
		out[i].Actions = cloneActions(values[i].Actions)
	}
	return out
}

func cloneMediaGalleryItem(v *MediaGalleryItem) *MediaGalleryItem {
	if v == nil {
		return nil
	}
	cp := *v
	cp.AccessGranted = clonePtr(v.AccessGranted)
	cp.Actions = cloneActions(v.Actions)
	return &cp
}

func cloneCollectionConfig(v *CollectionConfig) *CollectionConfig {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Item = cloneCollectionItem(v.Item)
	cp.Buckets = cloneCollectionBuckets(v.Buckets)
	cp.EditFields = cloneSlice(v.EditFields)
	cp.Modal = cloneCollectionModal(v.Modal)
	cp.Actions = cloneActions(v.Actions)
	return &cp
}

func cloneCollectionItem(v *CollectionItem) *CollectionItem {
	if v == nil {
		return nil
	}
	cp := *v
	cp.MetaFields = cloneSlice(v.MetaFields)
	return &cp
}

func cloneCollectionBuckets(values []CollectionBucket) []CollectionBucket {
	if values == nil {
		return nil
	}
	out := make([]CollectionBucket, len(values))
	for i, v := range values {
		out[i] = v
		out[i].Predicate = cloneCollectionPredicate(v.Predicate)
		out[i].Defaults = cloneSlice(v.Defaults)
		out[i].EditFields = cloneSlice(v.EditFields)
		out[i].Actions = cloneActions(v.Actions)
	}
	return out
}

func cloneCollectionPredicate(v *CollectionPredicate) *CollectionPredicate {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Value = clonePtr(v.Value)
	cp.Values = cloneSlice(v.Values)
	return &cp
}

func cloneCollectionModal(v *CollectionModal) *CollectionModal {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneRecordSections(values []RecordSection) []RecordSection {
	if values == nil {
		return nil
	}
	out := make([]RecordSection, len(values))
	for i, v := range values {
		out[i] = v
		out[i].Block = cloneBlock(v.Block)
		out[i].Stack = cloneStack(v.Stack)
		out[i].Components = cloneDisplayComponents(v.Components)
	}
	return out
}

func cloneBlock(v *Block) *Block {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Overlays = cloneBlockOverlays(v.Overlays)
	return &cp
}

func cloneStack(v *Stack) *Stack {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Wrap != nil {
		wrap := *v.Wrap
		cp.Wrap = &wrap
	}
	return &cp
}

func cloneDisplayComponents(values []DisplayComponent) []DisplayComponent {
	if values == nil {
		return nil
	}
	out := make([]DisplayComponent, len(values))
	for i, v := range values {
		out[i] = v
		out[i].Fields = cloneSlice(v.Fields)
		out[i].Items = cloneSlice(v.Items)
		out[i].CollectionGroups = cloneDisplayCollectionGroups(v.CollectionGroups)
		out[i].Block = cloneBlock(v.Block)
		if v.Visible != nil {
			visible := *v.Visible
			out[i].Visible = &visible
		}
		if v.Wrap != nil {
			wrap := *v.Wrap
			out[i].Wrap = &wrap
		}
		if v.VideoControls != nil {
			videoControls := *v.VideoControls
			out[i].VideoControls = &videoControls
		}
		if v.MatrixColumns != nil {
			out[i].MatrixColumns = make([]map[string]interface{}, len(v.MatrixColumns))
			for j, column := range v.MatrixColumns {
				out[i].MatrixColumns[j] = cloneMap(column)
			}
		}
	}
	return out
}

func cloneBlockOverlays(values []BlockOverlay) []BlockOverlay {
	if values == nil {
		return nil
	}
	out := make([]BlockOverlay, len(values))
	for index, overlay := range values {
		out[index] = overlay
		out[index].Badges = cloneBadges(overlay.Badges)
		out[index].Wrap = clonePtr(overlay.Wrap)
	}
	return out
}

func cloneDisplayCollectionGroups(value *DisplayCollectionGroups) *DisplayCollectionGroups {
	if value == nil {
		return nil
	}
	cp := *value
	cp.Groups = make([]DisplayCollectionGroup, len(value.Groups))
	for index, group := range value.Groups {
		cp.Groups[index] = group
		cp.Groups[index].ItemCondition = cloneCondition(group.ItemCondition)
	}
	return &cp
}

func cloneActions(values []Action) []Action {
	if values == nil {
		return nil
	}
	out := make([]Action, len(values))
	for i := range values {
		out[i] = cloneActionValue(values[i])
	}
	return out
}

func cloneAction(v *Action) *Action {
	if v == nil {
		return nil
	}
	cp := cloneActionValue(*v)
	return &cp
}

func cloneActionValue(v Action) Action {
	v.ActionPresentation = cloneActionPresentation(v.ActionPresentation)
	v.AfterRoute = cloneRouteValue(v.AfterRoute)
	v.Route = cloneRouteValue(v.Route)
	v.API = cloneAPIAction(v.API)
	v.Modal = cloneModalAction(v.Modal)
	v.Client = cloneClientAction(v.Client)
	v.Confirm = cloneConfirm(v.Confirm)
	v.AfterSuccess = cloneActionResult(v.AfterSuccess)
	v.AfterError = cloneActionResult(v.AfterError)
	return v
}

func cloneClientAction(v *ClientAction) *ClientAction {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Arguments = cloneSlice(v.Arguments)
	for index := range cp.Arguments {
		cp.Arguments[index].Value.Bool = clonePtr(v.Arguments[index].Value.Bool)
	}
	return &cp
}

func clonePromptList(v *PromptList) *PromptList {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Items = make([]Prompt, len(v.Items))
	for index := range v.Items {
		cp.Items[index] = v.Items[index]
		cp.Items[index].Action = cloneAction(v.Items[index].Action)
		cp.Items[index].VisibleIf = cloneCondition(v.Items[index].VisibleIf)
	}
	return &cp
}

func cloneActionPresentation(value ActionPresentation) ActionPresentation {
	cloned := value
	cloned.IconOnly = clonePtr(value.IconOnly)
	cloned.Block = clonePtr(value.Block)
	cloned.VisibleIf = cloneCondition(value.VisibleIf)
	cloned.HiddenIf = cloneCondition(value.HiddenIf)
	cloned.DisabledIf = cloneCondition(value.DisabledIf)
	return cloned
}

func cloneRouteAction(v *RouteAction) *RouteAction {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Params = cloneMap(v.Params)
	cp.Query = cloneMap(v.Query)
	return &cp
}

func cloneRouteValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return typed
	case *RouteAction:
		return cloneRouteAction(typed)
	case RouteAction:
		cloned := cloneRouteAction(&typed)
		if cloned == nil {
			return nil
		}
		return *cloned
	case map[string]interface{}:
		return cloneMap(typed)
	default:
		return typed
	}
}

func cloneAPIAction(v *APIAction) *APIAction {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Params = cloneMap(v.Params)
	cp.Query = cloneMap(v.Query)
	cp.Payload = cloneMap(v.Payload)
	return &cp
}

func cloneModalAction(v *ModalAction) *ModalAction {
	if v == nil {
		return nil
	}
	cp := *v
	cp.ShowHeader = clonePtr(v.ShowHeader)
	cp.Data = cloneMap(v.Data)
	return &cp
}

func cloneConfirm(v *Confirm) *Confirm {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneActionResult(v *ActionResult) *ActionResult {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Widget = cloneWidgetTarget(v.Widget)
	return &cp
}

func cloneCondition(v *Condition) *Condition {
	if v == nil {
		return nil
	}
	cp := *v
	cp.In = cloneSlice(v.In)
	cp.NotIn = cloneSlice(v.NotIn)
	cp.Empty = clonePtr(v.Empty)
	cp.NotEmpty = clonePtr(v.NotEmpty)
	cp.Truthy = clonePtr(v.Truthy)
	cp.Falsy = clonePtr(v.Falsy)
	cp.All = cloneConditions(v.All)
	cp.Any = cloneConditions(v.Any)
	cp.Not = cloneConditionValue(v.Not)
	return &cp
}

func cloneConditionValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case *Condition:
		return cloneCondition(typed)
	case Condition:
		cloned := cloneCondition(&typed)
		if cloned == nil {
			return nil
		}
		return *cloned
	case map[string]interface{}:
		return cloneMap(typed)
	default:
		return typed
	}
}

func cloneConditions(values []Condition) []Condition {
	if values == nil {
		return nil
	}
	out := make([]Condition, len(values))
	for i := range values {
		out[i] = *cloneCondition(&values[i])
	}
	return out
}

func clonePtr[T any](v *T) *T {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	out := make([]T, len(values))
	for i, v := range values {
		out[i] = cloneValue(v)
	}
	return out
}

func cloneMap[K comparable, V any](values map[K]V) map[K]V {
	if values == nil {
		return nil
	}
	out := make(map[K]V, len(values))
	for k, v := range values {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue[T any](value T) T {
	cloned := cloneInterface(any(value))
	typed, ok := cloned.(T)
	if !ok {
		return value
	}
	return typed
}

func cloneInterface(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return cloneMap(v)
	case []interface{}:
		return cloneSlice(v)
	case map[string]string:
		return cloneMap(v)
	case []string:
		return cloneSlice(v)
	case map[string][]string:
		return cloneMapStringSlice(v)
	case map[string]bool:
		return cloneMap(v)
	case []bool:
		return cloneSlice(v)
	case map[string]int:
		return cloneMap(v)
	case []int:
		return cloneSlice(v)
	case map[string]int64:
		return cloneMap(v)
	case []int64:
		return cloneSlice(v)
	case map[string]float64:
		return cloneMap(v)
	case []float64:
		return cloneSlice(v)
	default:
		return value
	}
}

func cloneMapStringSlice(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	out := make(map[string][]string, len(values))
	for k, v := range values {
		out[k] = cloneSlice(v)
	}
	return out
}
