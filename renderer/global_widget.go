package renderer

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GlobalWidget describes a shell-level UI surface. It does not create a route
// and can either load the action that registered it or compose a workspace of
// referenced module actions.
type GlobalWidget struct {
	Surface   WidgetSurface    `json:"surface"`
	Workspace *WorkspaceWidget `json:"workspace,omitempty"`
}

func (widget GlobalWidget) Identity() Identity {
	return UniversalIdentity()
}

func (widget GlobalWidget) Clone() GlobalWidget {
	cloned := widget
	cloned.Surface = cloneWidgetSurface(widget.Surface)
	cloned.Workspace = cloneWorkspaceWidget(widget.Workspace)
	return cloned
}

// LocalizeGlobalWidget resolves the user-visible command labels while keeping
// the widget request contract detached from its producer value.
func LocalizeGlobalWidget(widget GlobalWidget, resolve TextResolver) GlobalWidget {
	localized := widget.Clone()
	if resolve == nil {
		return localized
	}
	localized.Surface.CloseLabel = resolve(localized.Surface.CloseLabel, "")
	localized.Surface.BackLabel = resolve(localized.Surface.BackLabel, "")
	localized.Surface.MoreLabel = resolve(localized.Surface.MoreLabel, "")
	if localized.Surface.Trigger != nil {
		localized.Surface.Trigger.Label = resolve(localized.Surface.Trigger.Label, "")
		if localized.Surface.Trigger.Badge != nil && localized.Surface.Trigger.Badge.LabelKey != "" {
			localized.Surface.Trigger.Badge.Label = resolve("", localized.Surface.Trigger.Badge.LabelKey)
		}
	}
	if localized.Workspace == nil {
		return localized
	}
	for index := range localized.Workspace.Commands {
		localized.Workspace.Commands[index].Label = resolve(localized.Workspace.Commands[index].Label, "")
		if confirm := localized.Workspace.Commands[index].Confirm; confirm != nil {
			localizer := textLocalizer{resolve: resolve}
			localizer.localizeTextFields(&confirm.Title, &confirm.Message, &confirm.CancelLabel, &confirm.ConfirmLabel)
		}
	}
	localizer := textLocalizer{resolve: resolve}
	for index := range localized.Workspace.ComposerActions {
		localizer.localizeRendererAction(&localized.Workspace.ComposerActions[index])
	}
	for index := range localized.Workspace.FooterActions {
		localizer.localizeRendererAction(&localized.Workspace.FooterActions[index])
	}
	return localized
}

func (widget GlobalWidget) Validate() error {
	if err := widget.Surface.Validate(); err != nil {
		return fmt.Errorf("renderer.GlobalWidget: surface: %w", err)
	}
	if widget.Surface.Kind == WidgetSurfaceInline && widget.Workspace != nil {
		return fmt.Errorf("renderer.GlobalWidget: inline surface does not support workspace")
	}
	if widget.Workspace != nil {
		if err := widget.Workspace.Validate(); err != nil {
			return fmt.Errorf("renderer.GlobalWidget: workspace: %w", err)
		}
	}
	return nil
}

type WidgetSurfaceKind string

const (
	WidgetSurfaceDrawer WidgetSurfaceKind = "drawer"
	WidgetSurfacePopup  WidgetSurfaceKind = "popup"
	WidgetSurfaceInline WidgetSurfaceKind = "inline"
)

type WidgetPlacement string

const (
	WidgetPlacementShellStart WidgetPlacement = "shell_start"
	WidgetPlacementShellEnd   WidgetPlacement = "shell_end"
	WidgetPlacementCenter     WidgetPlacement = "center"
)

type WidgetLoadPolicy string

const (
	WidgetLoadOnOpen WidgetLoadPolicy = "on_open"
	WidgetLoadEager  WidgetLoadPolicy = "eager"
)

type WidgetSurface struct {
	Kind       WidgetSurfaceKind `json:"kind"`
	Placement  WidgetPlacement   `json:"placement"`
	LoadPolicy WidgetLoadPolicy  `json:"load_policy"`
	CloseLabel string            `json:"close_label,omitempty"`
	BackLabel  string            `json:"back_label,omitempty"`
	MoreLabel  string            `json:"more_label,omitempty"`
	// Size is a renderer token used by consumers to select the appropriate
	// compact or expansive surface treatment without producer CSS.
	Size SizeToken `json:"size,omitempty"`
	// Trigger declares an optional shell control that opens this widget. Its
	// Badge binds to the widget summary record, so the consumer never has to
	// infer a domain-specific counter.
	Trigger *WidgetTrigger `json:"trigger,omitempty"`
}

// WidgetTrigger is a compact, typed shell control for a global widget. Label
// is localized by the generator; Badge reuses the shared renderer badge
// contract and reads the workspace summary record.
type WidgetTrigger struct {
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Badge *Badge `json:"badge,omitempty"`
}

func (trigger WidgetTrigger) Validate() error {
	if strings.TrimSpace(trigger.Label) == "" {
		return fmt.Errorf("label is required")
	}
	if strings.TrimSpace(trigger.Icon) == "" {
		return fmt.Errorf("icon is required")
	}
	if trigger.Badge != nil && strings.TrimSpace(trigger.Badge.Field) == "" && trigger.Badge.Value == nil {
		return fmt.Errorf("badge field or value is required")
	}
	return nil
}

func (surface WidgetSurface) Validate() error {
	switch surface.Kind {
	case WidgetSurfaceDrawer, WidgetSurfacePopup, WidgetSurfaceInline:
	default:
		return fmt.Errorf("unsupported kind %q", surface.Kind)
	}
	switch surface.Placement {
	case WidgetPlacementShellStart, WidgetPlacementShellEnd, WidgetPlacementCenter:
	default:
		return fmt.Errorf("unsupported placement %q", surface.Placement)
	}
	if surface.Kind == WidgetSurfaceDrawer && surface.Placement == WidgetPlacementCenter {
		return fmt.Errorf("drawer does not support placement %q", surface.Placement)
	}
	if surface.Kind == WidgetSurfaceInline && surface.Placement != WidgetPlacementShellStart {
		return fmt.Errorf("inline requires placement %q", WidgetPlacementShellStart)
	}
	switch surface.LoadPolicy {
	case WidgetLoadOnOpen, WidgetLoadEager:
	default:
		return fmt.Errorf("unsupported load policy %q", surface.LoadPolicy)
	}
	if surface.Kind == WidgetSurfaceInline && surface.LoadPolicy != WidgetLoadEager {
		return fmt.Errorf("inline requires load policy %q", WidgetLoadEager)
	}
	if surface.Kind == WidgetSurfaceInline && surface.Trigger != nil {
		return fmt.Errorf("inline does not support trigger")
	}
	if surface.Size != "" {
		switch surface.Size {
		case SizeXS, SizeSM, SizeMD, SizeLG, SizeXL:
		default:
			return fmt.Errorf("unsupported size %q", surface.Size)
		}
	}
	if surface.Trigger != nil {
		if err := surface.Trigger.Validate(); err != nil {
			return fmt.Errorf("trigger: %w", err)
		}
	}
	return nil
}

// WorkspaceWidget composes server resources into a generic master-detail
// shell surface. Resources remain normal module actions.
type WorkspaceWidget struct {
	Selection       WorkspaceSelection `json:"selection"`
	Mode            WorkspaceMode      `json:"mode,omitempty"`
	Summary         *Resource          `json:"summary,omitempty"`
	Master          Resource           `json:"master"`
	Detail          Resource           `json:"detail"`
	ComposerActions []Action           `json:"composer_actions,omitempty"`
	Commands        []WorkspaceCommand `json:"commands,omitempty"`
	// FooterActions are regular typed actions rendered below the master list.
	// They give compact popup workspaces a server-declared route or modal
	// target without requiring a client-side special case.
	FooterActions []Action                `json:"footer_actions,omitempty"`
	Subscriptions []WorkspaceSubscription `json:"subscriptions,omitempty"`
}

func (workspace WorkspaceWidget) Validate() error {
	if err := workspace.Mode.Validate(); err != nil {
		return fmt.Errorf("mode: %w", err)
	}
	if workspace.Selection.Field == "" {
		return fmt.Errorf("selection field is required")
	}
	if err := workspace.Master.Validate("master"); err != nil {
		return err
	}
	if workspace.Summary != nil {
		if err := workspace.Summary.Validate("summary"); err != nil {
			return err
		}
	}
	if err := workspace.Detail.Validate("detail"); err != nil {
		return err
	}
	if !workspace.Detail.hasSelectionBinding(workspace.Selection.Field) {
		return fmt.Errorf("detail must bind selection field %q", workspace.Selection.Field)
	}
	seenComposerActions := make(map[string]struct{}, len(workspace.ComposerActions))
	for index := range workspace.ComposerActions {
		action := workspace.ComposerActions[index]
		if action.ID == "" {
			return fmt.Errorf("composer action %d: id is required", index)
		}
		if err := action.Validate(); err != nil {
			return fmt.Errorf("composer action %d: %w", index, err)
		}
		if _, exists := seenComposerActions[action.ID]; exists {
			return fmt.Errorf("composer action %q is duplicated", action.ID)
		}
		seenComposerActions[action.ID] = struct{}{}
	}
	seenCommands := make(map[string]struct{}, len(workspace.Commands))
	for index, command := range workspace.Commands {
		if err := command.Validate(); err != nil {
			return fmt.Errorf("command %d: %w", index, err)
		}
		if _, exists := seenCommands[command.ID]; exists {
			return fmt.Errorf("command %q is duplicated", command.ID)
		}
		seenCommands[command.ID] = struct{}{}
	}
	seenFooterActions := make(map[string]struct{}, len(workspace.FooterActions))
	for index := range workspace.FooterActions {
		action := workspace.FooterActions[index]
		if action.ID == "" {
			return fmt.Errorf("footer action %d: id is required", index)
		}
		if action.Type == "" {
			return fmt.Errorf("footer action %q: type is required", action.ID)
		}
		if err := action.Validate(); err != nil {
			return fmt.Errorf("footer action %q: %w", action.ID, err)
		}
		if _, exists := seenFooterActions[action.ID]; exists {
			return fmt.Errorf("footer action %q is duplicated", action.ID)
		}
		seenFooterActions[action.ID] = struct{}{}
	}
	seenSubscriptions := make(map[string]struct{}, len(workspace.Subscriptions))
	for index, subscription := range workspace.Subscriptions {
		if err := subscription.Validate(); err != nil {
			return fmt.Errorf("subscription %d: %w", index, err)
		}
		correlationField := ""
		if subscription.Correlation != nil {
			correlationField = subscription.Correlation.EventField
		}
		condition, err := json.Marshal(subscription.EventCondition)
		if err != nil {
			return fmt.Errorf("subscription %d event condition: %w", index, err)
		}
		key := subscription.Module + "\x00" + strings.Join(subscription.Actions, "\x00") + "\x00" + correlationField + "\x00" + string(condition)
		if _, exists := seenSubscriptions[key]; exists {
			return fmt.Errorf("subscription %d is duplicated", index)
		}
		seenSubscriptions[key] = struct{}{}
	}
	return nil
}

// WorkspaceMode controls only the visible master/detail composition. The
// master resource remains the source of the selected record in both modes.
type WorkspaceMode string

const (
	WorkspaceModeMasterDetail WorkspaceMode = "master_detail"
	WorkspaceModeDetailOnly   WorkspaceMode = "detail_only"
)

func (mode WorkspaceMode) Validate() error {
	switch mode {
	case "", WorkspaceModeMasterDetail, WorkspaceModeDetailOnly:
		return nil
	default:
		return fmt.Errorf("unsupported value %q", mode)
	}
}

type WorkspaceSelection struct {
	// Field identifies the current master row. Bindings may read this field or
	// another declared scalar field from that same selected row.
	Field string `json:"field"`
}

// ActionResource is a reference to an existing standard module action.
// Generator owns its request and response contracts.
type ActionResource struct {
	Module string `json:"module"`
	Action string `json:"action"`
}

func (resource ActionResource) Validate(name string) error {
	if resource.Module == "" {
		return fmt.Errorf("%s module is required", name)
	}
	if resource.Action == "" {
		return fmt.Errorf("%s action is required", name)
	}
	return nil
}

// Resource adds request bindings to an ActionResource. URL, method,
// paging, sorting and response presentation remain owned by the action.
type Resource struct {
	ActionResource
	Bindings []RequestBinding `json:"bindings,omitempty"`
}

func cloneResource(value *Resource) *Resource {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Bindings = cloneRequestBindings(value.Bindings)
	return &cloned
}

// WorkspaceCommand invokes a standard write action from the current workspace
// selection. The generator resolves its request contract and validates every
// typed binding; a producer never supplies a URL or an untyped request body.
type WorkspaceCommand struct {
	ID string `json:"id"`
	// Label is a producer translation key and is localized in /api/config.
	Label string `json:"label"`
	// Trigger controls when the workspace invokes a command. An omitted trigger
	// keeps the command user initiated; the command is rendered as either an
	// action or an input form according to Input.
	Trigger WorkspaceCommandTrigger `json:"trigger,omitempty"`
	// Presentation shares the visual contract of normal renderer actions. It
	// deliberately excludes request and routing fields: those are generated
	// from Resource.
	Presentation *ActionPresentation `json:"presentation,omitempty"`
	// Input permits an add command to bind values entered in the workspace.
	// Field definitions remain owned by the target module defrec response.
	Input *WorkspaceCommandInput `json:"input,omitempty"`
	// Confirm describes an optional confirmation step before the generated
	// request is executed. Its text values are producer translation keys.
	Confirm *Confirm `json:"confirm,omitempty"`
	// AfterSuccess applies the typed result of this standard command to the
	// shell, for example by opening another registered global widget. The
	// command request remains generated from Resource.
	AfterSuccess *ActionResult `json:"after_success,omitempty"`
	// RequireSelection controls whether the command needs a selected master
	// row. It defaults to true. A false value is valid only for commands whose
	// bindings do not read the selection runtime scope.
	RequireSelection *bool `json:"require_selection,omitempty"`
	// Multi says the command may be applied to several master rows at once.
	// The renderer offers a selection of rows and runs the command for each of
	// them; a row the command is not visible for is not offered.
	Multi bool `json:"multi,omitempty"`
	Resource
	Refresh []WorkspaceRefreshTarget `json:"refresh"`
}

func (command WorkspaceCommand) Validate() error {
	if command.ID == "" {
		return fmt.Errorf("id is required")
	}
	if command.Label == "" {
		return fmt.Errorf("label is required")
	}
	if err := command.Trigger.Validate(); err != nil {
		return fmt.Errorf("trigger: %w", err)
	}
	if command.Trigger != "" && command.Input != nil {
		return fmt.Errorf("triggered command must not declare input")
	}
	// Several rows cannot answer one form, and a command the workspace starts
	// by itself is never applied to a selection of rows.
	if command.Multi && (command.Input != nil || command.Trigger != "") {
		return fmt.Errorf("multi command must not declare input or trigger")
	}
	if command.Multi && command.RequireSelection != nil && !*command.RequireSelection {
		return fmt.Errorf("multi command requires selection")
	}
	if command.Trigger == WorkspaceCommandTriggerSelectionOpen && command.RequireSelection != nil && !*command.RequireSelection {
		return fmt.Errorf("selection_open trigger requires selection")
	}
	if err := command.Input.Validate(command.Bindings); err != nil {
		return fmt.Errorf("input: %w", err)
	}
	if command.Confirm != nil {
		if err := command.Confirm.Validate(); err != nil {
			return fmt.Errorf("confirm: %w", err)
		}
	}
	if err := command.Resource.Validate("resource"); err != nil {
		return err
	}
	if command.AfterSuccess != nil {
		if err := command.AfterSuccess.Validate(); err != nil {
			return fmt.Errorf("after success: %w", err)
		}
	}
	if command.RequireSelection != nil && !*command.RequireSelection {
		for _, binding := range command.Bindings {
			if binding.Source.Runtime != nil && binding.Source.Runtime.Scope == RuntimeValueSourceSelection {
				return fmt.Errorf("does not require selection but binding reads selection")
			}
		}
	}
	if len(command.Refresh) == 0 {
		return fmt.Errorf("refresh targets are required")
	}
	return ValidateWorkspaceRefreshTargets(command.Refresh)
}

// WorkspaceCommandTrigger identifies a lifecycle event that can invoke a
// command without a separate user action. The trigger remains independent of
// the business domain and operates only on the current workspace selection.
type WorkspaceCommandTrigger string

const (
	// WorkspaceCommandTriggerSelectionOpen runs once after the workspace has
	// loaded the detail resource for a newly opened selection.
	WorkspaceCommandTriggerSelectionOpen WorkspaceCommandTrigger = "selection_open"
)

func (trigger WorkspaceCommandTrigger) Validate() error {
	switch trigger {
	case "", WorkspaceCommandTriggerSelectionOpen:
		return nil
	default:
		return fmt.Errorf("unsupported value %q", trigger)
	}
}

// WorkspaceCommandInput declares values that the workspace may collect for a
// standard add command. The module defrec endpoint remains the single source
// of field metadata; Fields is only its allowlist.
type WorkspaceCommandInput struct {
	Fields []string `json:"fields"`
}

func (input *WorkspaceCommandInput) Validate(bindings []RequestBinding) error {
	inputBindings := make([]RequestBinding, 0)
	for _, binding := range bindings {
		if binding.Source.Runtime != nil && binding.Source.Runtime.Scope == RuntimeValueSourceInput {
			inputBindings = append(inputBindings, binding)
		}
	}
	if input == nil {
		if len(inputBindings) > 0 {
			return fmt.Errorf("runtime input source requires input declaration")
		}
		return nil
	}
	if len(input.Fields) == 0 {
		return fmt.Errorf("fields are required")
	}
	allowed := make(map[string]struct{}, len(input.Fields))
	for index, field := range input.Fields {
		if field == "" {
			return fmt.Errorf("field %d is required", index)
		}
		if _, exists := allowed[field]; exists {
			return fmt.Errorf("field %q is duplicated", field)
		}
		allowed[field] = struct{}{}
	}
	bound := make(map[string]struct{}, len(inputBindings))
	for _, binding := range inputBindings {
		if binding.Target != RequestBindingBody {
			return fmt.Errorf("runtime input source only supports body bindings")
		}
		field := binding.Source.Runtime.Field
		if _, exists := allowed[field]; !exists {
			return fmt.Errorf("runtime input field %q is not declared", field)
		}
		if binding.Field != field {
			return fmt.Errorf("runtime input field %q must bind body field %q", field, field)
		}
		bound[field] = struct{}{}
	}
	for _, field := range input.Fields {
		if _, exists := bound[field]; !exists {
			return fmt.Errorf("field %q has no input binding", field)
		}
	}
	return nil
}

func (resource Resource) Validate(name string) error {
	if err := resource.ActionResource.Validate(name); err != nil {
		return err
	}
	return ValidateRequestBindings(resource.Bindings)
}

func (resource Resource) hasSelectionBinding(field string) bool {
	for _, binding := range resource.Bindings {
		if binding.Source.Runtime != nil &&
			binding.Source.Runtime.Scope == RuntimeValueSourceSelection &&
			binding.Source.Runtime.Field == field {
			return true
		}
	}
	return false
}

type RequestBindingTarget string

const (
	RequestBindingPathByKey RequestBindingTarget = "path_by_key"
	RequestBindingPathValue RequestBindingTarget = "path_value"
	RequestBindingFilter    RequestBindingTarget = "filter"
	RequestBindingBody      RequestBindingTarget = "body"
)

// RequestBinding applies a typed literal or a known runtime value to a
// generated request. The target determines the request parameter name:
// path_by_key and path_value map to view, update or delete placeholders;
// filter derives filter[field] from Field; body derives a JSON body field.
type RequestBinding struct {
	Target RequestBindingTarget `json:"target"`
	Field  string               `json:"field,omitempty"`
	Source ValueSource          `json:"source"`
}

type RuntimeValueSource string

const (
	RuntimeValueSourceCurrentUser RuntimeValueSource = "current_user"
	RuntimeValueSourceSelection   RuntimeValueSource = "selection"
	RuntimeValueSourceInput       RuntimeValueSource = "input"
)

type RuntimeValue struct {
	Scope RuntimeValueSource `json:"scope"`
	Field string             `json:"field"`
}

// ValueSource is a closed union. Literal values are serialized with
// their type; runtime values can only come from a generator-defined scope.
type ValueSource struct {
	Literal *TypedValue   `json:"literal,omitempty"`
	Runtime *RuntimeValue `json:"runtime,omitempty"`
}

func (source ValueSource) Validate() error {
	variants := 0
	if source.Literal != nil {
		variants++
		if err := source.Literal.Validate(); err != nil {
			return fmt.Errorf("literal: %w", err)
		}
	}
	if source.Runtime != nil {
		variants++
		if source.Runtime.Field == "" {
			return fmt.Errorf("runtime field is required")
		}
		switch source.Runtime.Scope {
		case RuntimeValueSourceCurrentUser, RuntimeValueSourceSelection, RuntimeValueSourceInput:
		default:
			return fmt.Errorf("runtime scope %q is unsupported", source.Runtime.Scope)
		}
	}
	if variants != 1 {
		return fmt.Errorf("must contain exactly one of literal or runtime")
	}
	return nil
}

func ValidateRequestBindings(bindings []RequestBinding) error {
	seen := make(map[string]struct{}, len(bindings))
	for index, binding := range bindings {
		switch binding.Target {
		case RequestBindingPathByKey, RequestBindingPathValue:
			if binding.Field != "" {
				return fmt.Errorf("binding %d %s must not define field", index, binding.Target)
			}
		case RequestBindingFilter:
			if binding.Field == "" {
				return fmt.Errorf("binding %d filter field is required", index)
			}
		case RequestBindingBody:
			if binding.Field == "" {
				return fmt.Errorf("binding %d body field is required", index)
			}
		default:
			return fmt.Errorf("binding %d has unsupported target %q", index, binding.Target)
		}
		if err := binding.Source.Validate(); err != nil {
			return fmt.Errorf("binding %d source: %w", index, err)
		}
		key := string(binding.Target) + "\x00" + binding.Field
		if _, exists := seen[key]; exists {
			return fmt.Errorf("binding %d duplicates %s %q", index, binding.Target, binding.Field)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type WorkspaceRefreshTarget string

const (
	WorkspaceRefreshSummary WorkspaceRefreshTarget = "summary"
	WorkspaceRefreshMaster  WorkspaceRefreshTarget = "master"
	WorkspaceRefreshDetail  WorkspaceRefreshTarget = "detail"
)

func ValidateWorkspaceRefreshTargets(targets []WorkspaceRefreshTarget) error {
	seen := make(map[WorkspaceRefreshTarget]struct{}, len(targets))
	for index, target := range targets {
		switch target {
		case WorkspaceRefreshSummary, WorkspaceRefreshMaster, WorkspaceRefreshDetail:
		default:
			return fmt.Errorf("refresh target %d is unsupported: %q", index, target)
		}
		if _, exists := seen[target]; exists {
			return fmt.Errorf("refresh target %q is duplicated", target)
		}
		seen[target] = struct{}{}
	}
	return nil
}

type WorkspaceSubscription struct {
	Module  string   `json:"module"`
	Actions []string `json:"actions"`
	// EventCondition is evaluated against { event } on the consumer. It lets
	// one standard action expose several typed realtime effects without the
	// client branching on a module-specific payload convention.
	EventCondition *Condition `json:"event_condition,omitempty"`
	// Correlation narrows refreshes to the selected master row. When absent,
	// every authorized matching realtime event refreshes the declared targets.
	Correlation *WorkspaceCorrelationBinding `json:"correlation,omitempty"`
	Refresh     []WorkspaceRefreshTarget     `json:"refresh"`
	Toast       *WorkspaceSubscriptionToast  `json:"toast,omitempty"`
}

// WorkspaceSubscriptionToast asks a generic workspace surface to show a
// short-lived client toast after it refreshes its master resource. Bindings are
// resolved against the first current master row; a missing row suppresses the
// effect. The producer supplies only field bindings, never client text.
type WorkspaceSubscriptionToast struct {
	Title   *TextBinding `json:"title,omitempty"`
	Message *TextBinding `json:"message,omitempty"`
	Tone    string       `json:"tone,omitempty"`
}

func (toast WorkspaceSubscriptionToast) Validate() error {
	if toast.Title == nil && toast.Message == nil {
		return fmt.Errorf("title or message is required")
	}
	if toast.Title != nil && toast.Title.Field == "" && toast.Title.Template == "" {
		return fmt.Errorf("title binding is required")
	}
	if toast.Message != nil && toast.Message.Field == "" && toast.Message.Template == "" {
		return fmt.Errorf("message binding is required")
	}
	return nil
}

// WorkspaceCorrelationBinding identifies a declared realtime event field. The
// workspace selection is the implicit target and is checked by the generator.
type WorkspaceCorrelationBinding struct {
	EventField string `json:"event_field"`
}

func (subscription WorkspaceSubscription) Validate() error {
	if subscription.Module == "" {
		return fmt.Errorf("module is required")
	}
	if len(subscription.Actions) == 0 {
		return fmt.Errorf("actions are required")
	}
	seenActions := make(map[string]struct{}, len(subscription.Actions))
	for _, action := range subscription.Actions {
		if action == "" {
			return fmt.Errorf("action is required")
		}
		if _, exists := seenActions[action]; exists {
			return fmt.Errorf("action %q is duplicated", action)
		}
		seenActions[action] = struct{}{}
	}
	if subscription.Correlation != nil && subscription.Correlation.EventField == "" {
		return fmt.Errorf("correlation event field is required when correlation is set")
	}
	if subscription.EventCondition != nil && !hasCondition(subscription.EventCondition) {
		return fmt.Errorf("event condition is invalid")
	}
	if len(subscription.Refresh) == 0 && subscription.Toast == nil {
		return fmt.Errorf("refresh targets are required")
	}
	if subscription.Toast != nil {
		if err := subscription.Toast.Validate(); err != nil {
			return fmt.Errorf("toast: %w", err)
		}
	}
	return ValidateWorkspaceRefreshTargets(subscription.Refresh)
}

type WidgetTargetState string

const (
	WidgetTargetOpen  WidgetTargetState = "open"
	WidgetTargetClose WidgetTargetState = "close"
)

// WidgetTarget is the typed result of a standard action that controls a
// registered global widget.
type WidgetTarget struct {
	ID        string                        `json:"id"`
	State     WidgetTargetState             `json:"state"`
	Selection *WidgetSelectionResultBinding `json:"selection,omitempty"`
	Refresh   []WorkspaceRefreshTarget      `json:"refresh,omitempty"`
}

// ActionResultField identifies a scalar field declared by a standard action
// result contract.
type ActionResultField string

const (
	ActionResultFieldValue      ActionResultField = "value"
	ActionResultFieldPrimaryKey ActionResultField = "primary_key"
	ActionResultFieldDelete     ActionResultField = "delete"
)

// ActionResultSource identifies a typed scalar field from a standard action
// response. Generator resolves Resource and verifies Field against the action
// result contract.
type ActionResultSource struct {
	Resource ActionResource    `json:"resource"`
	Field    ActionResultField `json:"field"`
}

func (source ActionResultSource) Validate() error {
	if err := source.Resource.Validate("source resource"); err != nil {
		return err
	}
	if source.Field == "" {
		return fmt.Errorf("source field is required")
	}
	return nil
}

// WidgetSelectionResultBinding reads a typed source from the successful
// standard action response. The widget selection field is declared only by
// the target workspace, so source and target cannot be confused.
type WidgetSelectionResultBinding struct {
	Source ActionResultSource `json:"source"`
}

func (target WidgetTarget) Validate() error {
	if target.ID == "" {
		return fmt.Errorf("widget id is required")
	}
	switch target.State {
	case WidgetTargetOpen, WidgetTargetClose:
	default:
		return fmt.Errorf("unsupported widget state %q", target.State)
	}
	if target.Selection != nil {
		if err := target.Selection.Source.Validate(); err != nil {
			return fmt.Errorf("selection: %w", err)
		}
	}
	if target.State == WidgetTargetClose && target.Selection != nil {
		return fmt.Errorf("closed widget cannot set selection")
	}
	return ValidateWorkspaceRefreshTargets(target.Refresh)
}

// Actions returns every action declared by the current renderer tree. The
// returned values are detached copies and retain their native result fields.
func (render Universal) Actions() []Action {
	var result []Action
	appendAction := func(action Action) {
		result = append(result, cloneActionValue(action))
	}
	appendActions := func(actions []Action) {
		for _, action := range actions {
			appendAction(action)
		}
	}
	appendList := func(page *ListPage) {
		if page == nil {
			return
		}
		appendActions(page.Actions)
		if page.CardSchema != nil {
			appendActions(page.CardSchema.Actions)
		}
	}
	appendList(render.List)
	if render.Form != nil {
		appendActions(render.Form.Actions)
		for _, section := range render.Form.Sections {
			appendList(section.ListPage)
			if section.Collection != nil {
				appendActions(section.Collection.Actions)
				for _, bucket := range section.Collection.Buckets {
					appendActions(bucket.Actions)
				}
			}
			if section.MediaActions != nil {
				for _, action := range []*Action{
					section.MediaActions.Upload,
					section.MediaActions.Link,
					section.MediaActions.Reorder,
					section.MediaActions.Recenter,
					section.MediaActions.Crop,
					section.MediaActions.Remove,
				} {
					if action != nil {
						appendAction(*action)
					}
				}
			}
		}
	}
	if render.Record != nil {
		appendActions(render.Record.Actions)
	}
	if render.ResourceGrid != nil {
		for _, action := range []*Action{render.ResourceGrid.Create, render.ResourceGrid.Delete, render.ResourceGrid.Update} {
			if action != nil {
				appendAction(*action)
			}
		}
		if render.ResourceGrid.Card != nil {
			appendActions(render.ResourceGrid.Card.Actions)
		}
	}
	return result
}

// ResourceLoad is the resolved request for a standard action resource. The
// response itself provides the existing ListPage, RecordPage or FormPage
// presentation metadata.
type ResourceLoad struct {
	Request  APIAction        `json:"request"`
	Bindings []RequestBinding `json:"bindings,omitempty"`
}

// WorkspaceCommandLoad is the generated request contract for one workspace
// command. The UI executes it by applying the typed bindings to Request.
type WorkspaceCommandLoad struct {
	ID           string                     `json:"id"`
	Request      APIAction                  `json:"request"`
	Bindings     []RequestBinding           `json:"bindings,omitempty"`
	Input        *WorkspaceCommandInputLoad `json:"input,omitempty"`
	AfterSuccess *ActionResult              `json:"after_success,omitempty"`
}

// WorkspaceCommandInputLoad contains the generated definition request for a
// command input. The UI loads it and keeps only command.Input.Fields.
type WorkspaceCommandInputLoad struct {
	Definition ResourceLoad `json:"definition"`
}

type WidgetLoad struct {
	Resource *ResourceLoad          `json:"resource,omitempty"`
	Summary  *ResourceLoad          `json:"summary,omitempty"`
	Master   *ResourceLoad          `json:"master,omitempty"`
	Detail   *ResourceLoad          `json:"detail,omitempty"`
	Commands []WorkspaceCommandLoad `json:"commands,omitempty"`
}

func cloneWorkspaceWidget(value *WorkspaceWidget) *WorkspaceWidget {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Summary != nil {
		summary := *value.Summary
		summary.Bindings = cloneRequestBindings(value.Summary.Bindings)
		cloned.Summary = &summary
	}
	cloned.Master.Bindings = cloneRequestBindings(value.Master.Bindings)
	cloned.Detail.Bindings = cloneRequestBindings(value.Detail.Bindings)
	cloned.ComposerActions = cloneActions(value.ComposerActions)
	cloned.Commands = cloneWorkspaceCommands(value.Commands)
	cloned.FooterActions = cloneActions(value.FooterActions)
	cloned.Subscriptions = cloneWorkspaceSubscriptions(value.Subscriptions)
	return &cloned
}

func cloneWidgetSurface(value WidgetSurface) WidgetSurface {
	cloned := value
	if value.Trigger == nil {
		return cloned
	}
	trigger := *value.Trigger
	if value.Trigger.Badge != nil {
		badge := *value.Trigger.Badge
		badge.Value = cloneTextBinding(value.Trigger.Badge.Value)
		badge.Marker = clonePtr(value.Trigger.Badge.Marker)
		badge.ToneMap = cloneMap(value.Trigger.Badge.ToneMap)
		badge.Then = cloneBadgeState(value.Trigger.Badge.Then)
		badge.Else = cloneBadgeState(value.Trigger.Badge.Else)
		trigger.Badge = &badge
	}
	cloned.Trigger = &trigger
	return cloned
}

func cloneWorkspaceCommands(values []WorkspaceCommand) []WorkspaceCommand {
	if values == nil {
		return nil
	}
	cloned := make([]WorkspaceCommand, len(values))
	for index, value := range values {
		cloned[index] = value
		if value.Presentation != nil {
			presentation := cloneActionPresentation(*value.Presentation)
			cloned[index].Presentation = &presentation
		}
		if value.Input != nil {
			input := *value.Input
			input.Fields = cloneSlice(value.Input.Fields)
			cloned[index].Input = &input
		}
		if value.Confirm != nil {
			confirm := *value.Confirm
			cloned[index].Confirm = &confirm
		}
		cloned[index].Bindings = cloneRequestBindings(value.Bindings)
		cloned[index].Refresh = cloneSlice(value.Refresh)
		cloned[index].AfterSuccess = cloneActionResult(value.AfterSuccess)
		if value.RequireSelection != nil {
			requireSelection := *value.RequireSelection
			cloned[index].RequireSelection = &requireSelection
		}
	}
	return cloned
}

func cloneRequestBindings(values []RequestBinding) []RequestBinding {
	if values == nil {
		return nil
	}
	cloned := make([]RequestBinding, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].Source = cloneValueSource(value.Source)
	}
	return cloned
}

func cloneValueSource(value ValueSource) ValueSource {
	cloned := value
	if value.Literal != nil {
		literal := *value.Literal
		if value.Literal.Bool != nil {
			boolValue := *value.Literal.Bool
			literal.Bool = &boolValue
		}
		cloned.Literal = &literal
	}
	if value.Runtime != nil {
		runtime := *value.Runtime
		cloned.Runtime = &runtime
	}
	return cloned
}

func cloneWorkspaceSubscriptions(values []WorkspaceSubscription) []WorkspaceSubscription {
	if values == nil {
		return nil
	}
	cloned := make([]WorkspaceSubscription, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].Actions = cloneSlice(value.Actions)
		cloned[index].Refresh = cloneSlice(value.Refresh)
		if value.Correlation != nil {
			correlation := *value.Correlation
			cloned[index].Correlation = &correlation
		}
		cloned[index].EventCondition = cloneCondition(value.EventCondition)
		cloned[index].Toast = cloneWorkspaceSubscriptionToast(value.Toast)
	}
	return cloned
}

func cloneWorkspaceSubscriptionToast(value *WorkspaceSubscriptionToast) *WorkspaceSubscriptionToast {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Title = cloneTextBinding(value.Title)
	cloned.Message = cloneTextBinding(value.Message)
	return &cloned
}

func cloneWidgetTarget(value *WidgetTarget) *WidgetTarget {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Selection != nil {
		selection := *value.Selection
		cloned.Selection = &selection
	}
	cloned.Refresh = cloneSlice(value.Refresh)
	return &cloned
}

func cloneResourceLoad(value *ResourceLoad) *ResourceLoad {
	if value == nil {
		return nil
	}
	cloned := *value
	cloned.Request = *cloneAPIAction(&value.Request)
	cloned.Bindings = cloneRequestBindings(value.Bindings)
	return &cloned
}

func cloneWorkspaceCommandLoads(values []WorkspaceCommandLoad) []WorkspaceCommandLoad {
	if values == nil {
		return nil
	}
	cloned := make([]WorkspaceCommandLoad, len(values))
	for index, value := range values {
		cloned[index] = value
		cloned[index].Request = *cloneAPIAction(&value.Request)
		cloned[index].Bindings = cloneRequestBindings(value.Bindings)
		if value.Input != nil {
			input := *value.Input
			input.Definition = *cloneResourceLoad(&value.Input.Definition)
			cloned[index].Input = &input
		}
		cloned[index].AfterSuccess = cloneActionResult(value.AfterSuccess)
	}
	return cloned
}

func (value WidgetLoad) Clone() WidgetLoad {
	return WidgetLoad{
		Resource: cloneResourceLoad(value.Resource),
		Summary:  cloneResourceLoad(value.Summary),
		Master:   cloneResourceLoad(value.Master),
		Detail:   cloneResourceLoad(value.Detail),
		Commands: cloneWorkspaceCommandLoads(value.Commands),
	}
}
