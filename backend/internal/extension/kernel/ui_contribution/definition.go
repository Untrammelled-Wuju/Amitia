package ui_contribution

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
)

type UIContributionKind string

const (
	UIContributionSchemaPage        UIContributionKind = "schema_page"
	UIContributionWebPage           UIContributionKind = "web_page"
	UIContributionPanel             UIContributionKind = "panel"
	UIContributionCard              UIContributionKind = "card"
	UIContributionBadge             UIContributionKind = "badge"
	UIContributionAction            UIContributionKind = "action"
	UIContributionMenuItem          UIContributionKind = "menu_item"
	UIContributionToolbarItem       UIContributionKind = "toolbar_item"
	UIContributionStatusItem        UIContributionKind = "status_item"
	UIContributionMessageAction     UIContributionKind = "message_action"
	UIContributionMessageRenderer   UIContributionKind = "message_renderer"
	UIContributionComposerAction    UIContributionKind = "composer_action"
	UIContributionSettingsSection   UIContributionKind = "settings_section"
	UIContributionDesktopCommand    UIContributionKind = "desktop_command"
	UIContributionDetailSectionKind UIContributionKind = "detail_section"
	UIContributionChatSidebarKind   UIContributionKind = "chat_sidebar"
)

func (k UIContributionKind) Valid() bool {
	switch k {
	case UIContributionSchemaPage, UIContributionWebPage, UIContributionPanel,
		UIContributionCard, UIContributionBadge, UIContributionAction, UIContributionMenuItem,
		UIContributionToolbarItem, UIContributionStatusItem,
		UIContributionMessageAction, UIContributionMessageRenderer,
		UIContributionComposerAction, UIContributionSettingsSection,
		UIContributionDesktopCommand, UIContributionDetailSectionKind,
		UIContributionChatSidebarKind:
		return true
	}
	return false
}

func (k UIContributionKind) DefaultSandbox() UISandboxType {
	switch k {
	case UIContributionSchemaPage, UIContributionSettingsSection, UIContributionCard, UIContributionBadge:
		return SandboxSchemaRenderer
	case UIContributionWebPage:
		return SandboxWebRestricted
	case UIContributionAction, UIContributionMenuItem, UIContributionToolbarItem,
		UIContributionStatusItem, UIContributionMessageAction,
		UIContributionComposerAction, UIContributionDesktopCommand:
		return SandboxHostNative
	case UIContributionPanel:
		return SandboxSchemaRenderer
	case UIContributionMessageRenderer:
		return SandboxWebRestricted
	case UIContributionDetailSectionKind, UIContributionChatSidebarKind:
		return SandboxSchemaRenderer
	}
	return SandboxHostNative
}

type UISandboxType string

const (
	SandboxHostNative     UISandboxType = "host_native"
	SandboxSchemaRenderer UISandboxType = "schema_renderer"
	SandboxWebRestricted  UISandboxType = "web_restricted"
	SandboxWebIsolated    UISandboxType = "web_isolated"
)

func (s UISandboxType) Valid() bool {
	switch s {
	case SandboxHostNative, SandboxSchemaRenderer, SandboxWebRestricted, SandboxWebIsolated:
		return true
	}
	return false
}

type SlotMultiplicity string

const (
	MultiplicitySingle            SlotMultiplicity = "single"
	MultiplicityMultiple          SlotMultiplicity = "multiple"
	MultiplicityOrderedMultiple   SlotMultiplicity = "ordered_multiple"
	MultiplicityReplaceableSingle SlotMultiplicity = "replaceable_single"
	MultiplicityExclusive         SlotMultiplicity = "exclusive"
)

func (m SlotMultiplicity) Valid() bool {
	switch m {
	case MultiplicitySingle, MultiplicityMultiple, MultiplicityOrderedMultiple,
		MultiplicityReplaceableSingle, MultiplicityExclusive:
		return true
	}
	return false
}

type ContributionID string
type ExtensionID string
type ModuleID string

type UISlotReference struct {
	SlotID          string `json:"slot_id"`
	ContractVersion int    `json:"contract_version"`
}

type LocalizedText struct {
	Default string            `json:"default"`
	I18n    map[string]string `json:"i18n,omitempty"`
}

func (l LocalizedText) Resolve(locale string) string {
	if l.I18n != nil {
		if v, ok := l.I18n[locale]; ok {
			return v
		}
	}
	return l.Default
}

type UIBadgeDefinition struct {
	Text         string `json:"text"`
	Color        string `json:"color,omitempty"`
	Count        int    `json:"count,omitempty"`
	Max          int    `json:"max,omitempty"`
	HideWhenZero bool   `json:"hide_when_zero,omitempty"`
}

type UIDisplayMetadata struct {
	Title       LocalizedText      `json:"title"`
	Description LocalizedText      `json:"description,omitempty"`
	Icon        string             `json:"icon,omitempty"`
	Badge       *UIBadgeDefinition `json:"badge,omitempty"`
	Category    string             `json:"category,omitempty"`
	Keywords    []string           `json:"keywords,omitempty"`
}

type UISandboxPolicy struct {
	Type           UISandboxType `json:"type"`
	CSP            string        `json:"csp,omitempty"`
	AllowedOrigins []string      `json:"allowed_origins,omitempty"`
	EnableScripts  bool          `json:"enable_scripts,omitempty"`
	AllowForms     bool          `json:"allow_forms,omitempty"`
	AllowPopups    bool          `json:"allow_popups,omitempty"`
	MaxMemoryMB    int64         `json:"max_memory_mb,omitempty"`
}

type UIEntryDefinition struct {
	Type        UISandboxType `json:"type"`
	Path        string        `json:"path"`
	SchemaPath  string        `json:"schema_path,omitempty"`
	RuntimeID   string        `json:"runtime_id,omitempty"`
	EntryName   string        `json:"entry_name,omitempty"`
	ContentHash string        `json:"content_hash"`
}

type UICondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type UIVisibilityRule struct {
	RequiredContext []string      `json:"required_context,omitempty"`
	Platforms       []string      `json:"platforms,omitempty"`
	MessageTypes    []string      `json:"message_types,omitempty"`
	Conditions      []UICondition `json:"conditions,omitempty"`
	UserSetting     string        `json:"user_setting,omitempty"`
}

type UIDataContract struct {
	InputSchema     json.RawMessage                   `json:"input_schema,omitempty"`
	OutputSchema    json.RawMessage                   `json:"output_schema,omitempty"`
	RefreshPolicy   string                            `json:"refresh_policy,omitempty"`
	SensitiveFields []string                          `json:"sensitive_fields,omitempty"`
	MaxPayloadBytes int64                             `json:"max_payload_bytes,omitempty"`
	Projection      *UIConversationProjectionContract `json:"projection,omitempty"`
}

// UIConversationProjectionContract declares how arbitrary conversation events
// are folded into a durable visual node. The browser runtime keeps an event
// history per node and exposes the folded state to the contribution renderer.
type UIConversationProjectionContract struct {
	EventTypes  []string `json:"event_types"`
	StartEvents []string `json:"start_events,omitempty"`
	EndEvents   []string `json:"end_events,omitempty"`
	NodeType    string   `json:"node_type,omitempty"`
	KeyPath     string   `json:"key_path,omitempty"`
	TitlePath   string   `json:"title_path,omitempty"`
	MaxEvents   int      `json:"max_events,omitempty"`
}

type UIDataSourceDefinition struct {
	SourceID      string          `json:"source_id"`
	Title         LocalizedText   `json:"title"`
	InputSchema   json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema  json.RawMessage `json:"output_schema,omitempty"`
	RefreshPolicy string          `json:"refresh_policy,omitempty"`
}

type RiskLevel string

const (
	RiskLevelNone     RiskLevel = "none"
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

func (r RiskLevel) Valid() bool {
	switch r {
	case RiskLevelNone, RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical:
		return true
	}
	return false
}

type UIActionTargetType string

const (
	ActionTargetHostCommand      UIActionTargetType = "host_command"
	ActionTargetExtensionRuntime UIActionTargetType = "extension_runtime"
	ActionTargetTool             UIActionTargetType = "tool"
	ActionTargetWorkflow         UIActionTargetType = "workflow"
	ActionTargetNavigation       UIActionTargetType = "navigation"
	ActionTargetDialog           UIActionTargetType = "dialog"
	ActionTargetCopy             UIActionTargetType = "copy"
	ActionTargetOpenResource     UIActionTargetType = "open_resource"
)

type WorkflowActionType string

const (
	WorkflowActionRun    WorkflowActionType = "run"
	WorkflowActionCancel WorkflowActionType = "cancel"
	WorkflowActionStatus WorkflowActionType = "status"
)

func (w WorkflowActionType) Valid() bool {
	switch w {
	case WorkflowActionRun, WorkflowActionCancel, WorkflowActionStatus:
		return true
	}
	return false
}

type UIActionTarget struct {
	Type           UIActionTargetType `json:"type"`
	Command        string             `json:"command,omitempty"`
	ToolID         string             `json:"tool_id,omitempty"`
	WorkflowID     string             `json:"workflow_id,omitempty"`
	WorkflowAction WorkflowActionType `json:"workflow_action,omitempty"`
	RouteID        string             `json:"route_id,omitempty"`
	DialogID       string             `json:"dialog_id,omitempty"`
	Resource       string             `json:"resource,omitempty"`
}

type UIActionDefinition struct {
	ActionID     string          `json:"action_id"`
	Title        LocalizedText   `json:"title"`
	Icon         string          `json:"icon,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	Target       UIActionTarget  `json:"target"`
	RiskLevel    RiskLevel       `json:"risk_level"`
	Confirmation string          `json:"confirmation,omitempty"`
}

type PermissionRequirement struct {
	Name     string `json:"name"`
	Scope    string `json:"scope,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Required bool   `json:"required,omitempty"`
}

type ScopeRule struct {
	RequiredScopes  []string `json:"required_scopes,omitempty"`
	ForbiddenScopes []string `json:"forbidden_scopes,omitempty"`
}

type UIOrderingRule struct {
	Priority int      `json:"priority"`
	Before   []string `json:"before,omitempty"`
	After    []string `json:"after,omitempty"`
	Category string   `json:"category,omitempty"`
	SortKey  string   `json:"sort_key,omitempty"`
}

type UIConflictPolicy struct {
	Strategy string `json:"strategy"`
	Fallback string `json:"fallback,omitempty"`
	Override bool   `json:"override,omitempty"`
}

type UILifecyclePolicy struct {
	Initial      string        `json:"initial"`
	AutoSuspend  bool          `json:"auto_suspend,omitempty"`
	SuspendAfter time.Duration `json:"suspend_after,omitempty"`
	MaxRetries   int           `json:"max_retries,omitempty"`
}

type ContributionIntegrity struct {
	DefinitionHash string `json:"definition_hash"`
	EntryHash      string `json:"entry_hash,omitempty"`
	SchemaHash     string `json:"schema_hash,omitempty"`
	Generation     int64  `json:"generation"`
}

type UIContributionDefinition struct {
	ContributionID  ContributionID           `json:"contribution_id"`
	ExtensionID     ExtensionID              `json:"extension_id"`
	ModuleID        ModuleID                 `json:"module_id"`
	Kind            UIContributionKind       `json:"kind"`
	Slot            UISlotReference          `json:"slot"`
	ContractVersion int                      `json:"contract_version"`
	Display         UIDisplayMetadata        `json:"display"`
	Entry           UIEntryDefinition        `json:"entry"`
	Visibility      UIVisibilityRule         `json:"visibility,omitempty"`
	DataContract    UIDataContract           `json:"data_contract,omitempty"`
	Actions         []UIActionDefinition     `json:"actions,omitempty"`
	DataSources     []UIDataSourceDefinition `json:"data_sources,omitempty"`
	Permissions     []PermissionRequirement  `json:"permissions,omitempty"`
	ScopeRule       ScopeRule                `json:"scope_rule,omitempty"`
	Ordering        UIOrderingRule           `json:"ordering,omitempty"`
	ConflictPolicy  UIConflictPolicy         `json:"conflict_policy,omitempty"`
	Sandbox         UISandboxPolicy          `json:"sandbox"`
	Lifecycle       UILifecyclePolicy        `json:"lifecycle"`
	Integrity       ContributionIntegrity    `json:"integrity"`
}

type UIPerformanceBudget struct {
	FirstPaintMs     int64 `json:"first_paint_ms"`
	BundleSizeKB     int64 `json:"bundle_size_kb"`
	MemoryMB         int64 `json:"memory_mb"`
	EventRatePerSec  int   `json:"event_rate_per_sec"`
	UpdateRateHz     int   `json:"update_rate_hz"`
	RenderMs         int64 `json:"render_ms"`
	MaxConcurrentReq int   `json:"max_concurrent_req"`
	SuspendOnHidden  bool  `json:"suspend_on_hidden"`
}

type UISlotContract struct {
	SlotID            string               `json:"slot_id"`
	Version           int                  `json:"version"`
	SupportedKinds    []UIContributionKind `json:"supported_kinds"`
	InputSchema       json.RawMessage      `json:"input_schema,omitempty"`
	OutputSchema      json.RawMessage      `json:"output_schema,omitempty"`
	AllowedActions    []string             `json:"allowed_actions,omitempty"`
	AllowedSandboxes  []UISandboxType      `json:"allowed_sandboxes"`
	Multiplicity      SlotMultiplicity     `json:"multiplicity"`
	OrderingPolicy    string               `json:"ordering_policy,omitempty"`
	FailurePolicy     string               `json:"failure_policy,omitempty"`
	PerformanceBudget UIPerformanceBudget  `json:"performance_budget"`
}

var (
	ErrContributionIDEmpty     = errors.New("ui_contribution: contribution_id empty")
	ErrExtensionIDEmpty        = errors.New("ui_contribution: extension_id empty")
	ErrModuleIDEmpty           = errors.New("ui_contribution: module_id empty")
	ErrInvalidKind             = errors.New("ui_contribution: invalid kind")
	ErrInvalidSandbox          = errors.New("ui_contribution: invalid sandbox type")
	ErrInvalidMultiplicity     = errors.New("ui_contribution: invalid multiplicity")
	ErrInvalidRiskLevel        = errors.New("ui_contribution: invalid risk level")
	ErrSlotIDEmpty             = errors.New("ui_contribution: slot_id empty")
	ErrEntryPathEmpty          = errors.New("ui_contribution: entry path empty")
	ErrEntryHashEmpty          = errors.New("ui_contribution: entry content_hash empty")
	ErrIntegrityHashEmpty      = errors.New("ui_contribution: integrity definition_hash empty")
	ErrContractVersionZero     = errors.New("ui_contribution: contract_version must be > 0")
	ErrKindNotSupportedBySlot  = errors.New("ui_contribution: kind not supported by slot")
	ErrSandboxNotAllowedBySlot = errors.New("ui_contribution: sandbox not allowed by slot")
	ErrActionNotDeclared       = errors.New("ui_contribution: action not declared in slot")
	ErrPayloadTooLarge         = errors.New("ui_contribution: payload exceeds max")
)

func ValidateDefinition(def *UIContributionDefinition) error {
	if def == nil {
		return errors.New("ui_contribution: nil definition")
	}
	if def.ContributionID == "" {
		return ErrContributionIDEmpty
	}
	if def.ExtensionID == "" {
		return ErrExtensionIDEmpty
	}
	if def.ModuleID == "" {
		return ErrModuleIDEmpty
	}
	if !def.Kind.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidKind, def.Kind)
	}
	if def.Slot.SlotID == "" {
		return ErrSlotIDEmpty
	}
	if def.ContractVersion <= 0 {
		return ErrContractVersionZero
	}
	if def.Entry.Path == "" {
		return ErrEntryPathEmpty
	}
	if def.Entry.ContentHash == "" {
		return ErrEntryHashEmpty
	}
	if !def.Sandbox.Type.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidSandbox, def.Sandbox.Type)
	}
	if def.Integrity.DefinitionHash == "" {
		return ErrIntegrityHashEmpty
	}
	for _, a := range def.Actions {
		if a.ActionID == "" {
			return errors.New("ui_contribution: action_id empty")
		}
		if !a.RiskLevel.Valid() {
			return fmt.Errorf("%w: %s", ErrInvalidRiskLevel, a.RiskLevel)
		}
		if a.Target.Type == "" {
			return errors.New("ui_contribution: action target type empty")
		}
		if a.Target.Type == ActionTargetWorkflow {
			if a.Target.WorkflowID == "" {
				return errors.New("ui_contribution: workflow action requires workflow_id")
			}
			if a.Target.WorkflowAction != "" && !a.Target.WorkflowAction.Valid() {
				return fmt.Errorf("ui_contribution: invalid workflow_action %s", a.Target.WorkflowAction)
			}
		}
	}
	return nil
}

func ValidateAgainstSlot(def *UIContributionDefinition, slot *UISlotContract) error {
	if def == nil || slot == nil {
		return errors.New("ui_contribution: nil def or slot")
	}
	if def.Slot.SlotID != slot.SlotID {
		return fmt.Errorf("ui_contribution: slot mismatch %s != %s", def.Slot.SlotID, slot.SlotID)
	}
	if def.ContractVersion != slot.Version {
		return fmt.Errorf("ui_contribution: contract version mismatch %d != %d", def.ContractVersion, slot.Version)
	}
	kindSupported := false
	for _, k := range slot.SupportedKinds {
		if k == def.Kind {
			kindSupported = true
			break
		}
	}
	if !kindSupported {
		return fmt.Errorf("%w: %s not in slot %s", ErrKindNotSupportedBySlot, def.Kind, slot.SlotID)
	}
	if len(slot.AllowedSandboxes) > 0 {
		sandboxAllowed := false
		for _, s := range slot.AllowedSandboxes {
			if s == def.Sandbox.Type {
				sandboxAllowed = true
				break
			}
		}
		if !sandboxAllowed {
			return fmt.Errorf("%w: %s not in slot %s", ErrSandboxNotAllowedBySlot, def.Sandbox.Type, slot.SlotID)
		}
	}
	if !slot.Multiplicity.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidMultiplicity, slot.Multiplicity)
	}
	for _, a := range def.Actions {
		if len(slot.AllowedActions) > 0 {
			allowed := false
			for _, name := range slot.AllowedActions {
				if name == string(a.Target.Type) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("%w: %s in slot %s", ErrActionNotDeclared, a.Target.Type, slot.SlotID)
			}
		}
	}
	if def.DataContract.MaxPayloadBytes > 0 && slot.PerformanceBudget.MemoryMB > 0 {
		if def.DataContract.MaxPayloadBytes > slot.PerformanceBudget.MemoryMB*1024*1024 {
			return fmt.Errorf("%w: %d > %d", ErrPayloadTooLarge, def.DataContract.MaxPayloadBytes, slot.PerformanceBudget.MemoryMB*1024*1024)
		}
	}
	return nil
}

type UIContext struct {
	ContributionID  string          `json:"contribution_id"`
	ExtensionID     string          `json:"extension_id"`
	ModuleID        string          `json:"module_id"`
	SlotID          string          `json:"slot_id"`
	ContractVersion int             `json:"contract_version"`
	Platform        string          `json:"platform"`
	Theme           ThemeSnapshot   `json:"theme"`
	Locale          string          `json:"locale"`
	Scope           ScopeSummary    `json:"scope"`
	Data            json.RawMessage `json:"data,omitempty"`
}

type ThemeSnapshot struct {
	Mode    string         `json:"mode"`
	Density string         `json:"density"`
	Tokens  map[string]any `json:"tokens,omitempty"`
}

type ScopeSummary struct {
	Scopes    []string `json:"scopes"`
	Character string   `json:"character,omitempty"`
}

type UIBridgeMethod string

const (
	BridgeUIReady             UIBridgeMethod = "ui.ready"
	BridgeUIActionInvoke      UIBridgeMethod = "ui.action.invoke"
	BridgeUIDataRequest       UIBridgeMethod = "ui.data.request"
	BridgeUIDataSubscribe     UIBridgeMethod = "ui.data.subscribe"
	BridgeUIResizeRequest     UIBridgeMethod = "ui.resize.request"
	BridgeUINavigationRequest UIBridgeMethod = "ui.navigation.request"
	BridgeUIDialogRequest     UIBridgeMethod = "ui.dialog.request"
	BridgeUIResourceOpen      UIBridgeMethod = "ui.resource.open"
	BridgeUILog               UIBridgeMethod = "ui.log"
)

func (m UIBridgeMethod) Valid() bool {
	switch m {
	case BridgeUIReady, BridgeUIActionInvoke, BridgeUIDataRequest,
		BridgeUIDataSubscribe, BridgeUIResizeRequest, BridgeUINavigationRequest,
		BridgeUIDialogRequest, BridgeUIResourceOpen, BridgeUILog:
		return true
	}
	return false
}

type UILifecycleState string

const (
	UIStateRegistered UILifecycleState = "registered"
	UIStateLoading    UILifecycleState = "loading"
	UIStateMounted    UILifecycleState = "mounted"
	UIStateVisible    UILifecycleState = "visible"
	UIStateHidden     UILifecycleState = "hidden"
	UIStateSuspended  UILifecycleState = "suspended"
	UIStateFailed     UILifecycleState = "failed"
	UIStateUnmounted  UILifecycleState = "unmounted"
)

func (s UILifecycleState) Valid() bool {
	switch s {
	case UIStateRegistered, UIStateLoading, UIStateMounted,
		UIStateVisible, UIStateHidden, UIStateSuspended,
		UIStateFailed, UIStateUnmounted:
		return true
	}
	return false
}

func (s UILifecycleState) IsTerminal() bool {
	return s == UIStateUnmounted
}

func (s UILifecycleState) IsActive() bool {
	return s == UIStateMounted || s == UIStateVisible
}

type UIErrorCode string

const (
	UIErrContractInvalid    UIErrorCode = "ui_contract_invalid"
	UIErrSlotUnsupported    UIErrorCode = "slot_unsupported"
	UIErrSandboxUnsupported UIErrorCode = "sandbox_unsupported"
	UIErrEntryMissing       UIErrorCode = "entry_missing"
	UIErrBundleIntegrity    UIErrorCode = "bundle_integrity_failed"
	UIErrBridgeAuth         UIErrorCode = "bridge_auth_failed"
	UIErrActionNotDeclared  UIErrorCode = "action_not_declared"
	UIErrPermissionDenied   UIErrorCode = "permission_denied"
	UIErrScopeDenied        UIErrorCode = "scope_denied"
	UIErrPayloadInvalid     UIErrorCode = "payload_invalid"
	UIErrRenderTimeout      UIErrorCode = "render_timeout"
	UIErrRuntimeUnavailable UIErrorCode = "runtime_unavailable"
	UIErrCrashed            UIErrorCode = "ui_crashed"
)

type UIError struct {
	Code    UIErrorCode    `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
}

func (e *UIError) Error() string {
	return fmt.Sprintf("ui_contribution: %s: %s", e.Code, e.Message)
}

func NewUIError(code UIErrorCode, msg string, detail map[string]any) *UIError {
	return &UIError{Code: code, Message: msg, Detail: detail}
}

type SchemaUIDoc = schema_ui.SchemaUIDocument

type WebPageConfig struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type UIContributionDetailSection struct {
	ID         string         `json:"id"`
	Target     string         `json:"target"`
	Slot       string         `json:"slot"`
	Title      LocalizedText  `json:"title"`
	Order      int            `json:"order"`
	Renderer   string         `json:"renderer"`
	SchemaUI   *SchemaUIDoc   `json:"schemaUi,omitempty"`
	WebPage    *WebPageConfig `json:"webPage,omitempty"`
	Permission string         `json:"permission,omitempty"`
}

type UIContributionChatSidebar struct {
	ID          string         `json:"id"`
	Title       LocalizedText  `json:"title"`
	Icon        string         `json:"icon,omitempty"`
	Order       int            `json:"order"`
	DefaultOpen bool           `json:"defaultOpen"`
	Renderer    string         `json:"renderer"`
	SchemaUI    *SchemaUIDoc   `json:"schemaUi,omitempty"`
	WebPage     *WebPageConfig `json:"webPage,omitempty"`
	Permission  string         `json:"permission,omitempty"`
	Width       int            `json:"width,omitempty"`
	Resizable   bool           `json:"resizable,omitempty"`
}

var DefaultSlots = buildDefaultSlotContracts()

// SlotContractFromDefinition converts the public slot registry contract into the
// stricter UIHost contract. extension_slots is the single source of truth for
// slot identity, layout, multiplicity and performance/fallback metadata; UIHost
// only adds sandbox policy required to validate concrete contributions.
func SlotContractFromDefinition(def *extension_slots.SlotDefinition) (*UISlotContract, error) {
	if def == nil || def.SlotID == "" {
		return nil, errors.New("ui_contribution: invalid slot definition")
	}
	kinds := make([]UIContributionKind, 0, len(def.SupportedKinds))
	seenKinds := make(map[UIContributionKind]struct{})
	sandboxes := make([]UISandboxType, 0, 4)
	seenSandboxes := make(map[UISandboxType]struct{})
	for _, rawKind := range def.SupportedKinds {
		kind := UIContributionKind(rawKind)
		if !kind.Valid() {
			return nil, fmt.Errorf("ui_contribution: unsupported slot kind %q for %s", rawKind, def.SlotID)
		}
		if _, ok := seenKinds[kind]; !ok {
			seenKinds[kind] = struct{}{}
			kinds = append(kinds, kind)
		}
		for _, sandbox := range allowedSandboxesForKind(kind) {
			if _, ok := seenSandboxes[sandbox]; ok {
				continue
			}
			seenSandboxes[sandbox] = struct{}{}
			sandboxes = append(sandboxes, sandbox)
		}
	}
	return &UISlotContract{
		SlotID:           string(def.SlotID),
		Version:          def.ContractVersion,
		SupportedKinds:   kinds,
		InputSchema:      append(json.RawMessage(nil), def.ContextSchema...),
		AllowedSandboxes: sandboxes,
		Multiplicity:     SlotMultiplicity(def.Multiplicity),
		OrderingPolicy:   def.OrderingPolicy,
		FailurePolicy:    def.FailurePolicy,
		PerformanceBudget: UIPerformanceBudget{
			FirstPaintMs:     def.PerformanceBudget.FirstPaint.Milliseconds(),
			BundleSizeKB:     def.PerformanceBudget.BundleSize / 1024,
			MemoryMB:         def.PerformanceBudget.MemoryBytes / (1024 * 1024),
			EventRatePerSec:  def.PerformanceBudget.MessageRate,
			UpdateRateHz:     durationToRateHz(def.PerformanceBudget.UpdateFrequency),
			RenderMs:         100,
			MaxConcurrentReq: 4,
			SuspendOnHidden:  def.Layout != extension_slots.LayoutHidden,
		},
	}, nil
}

func buildDefaultSlotContracts() map[string]*UISlotContract {
	out := make(map[string]*UISlotContract)
	for _, def := range extension_slots.DefaultSlots() {
		contract, err := SlotContractFromDefinition(def)
		if err != nil {
			continue
		}
		out[contract.SlotID] = contract
	}
	return out
}

func durationToRateHz(updateFrequency time.Duration) int {
	if updateFrequency <= 0 {
		return 0
	}
	rate := int(time.Second / updateFrequency)
	if rate < 1 {
		return 1
	}
	return rate
}

func allowedSandboxesForKind(kind UIContributionKind) []UISandboxType {
	switch kind {
	case UIContributionSchemaPage, UIContributionSettingsSection, UIContributionPanel,
		UIContributionCard, UIContributionBadge, UIContributionDetailSectionKind,
		UIContributionChatSidebarKind:
		return []UISandboxType{SandboxSchemaRenderer, SandboxWebRestricted, SandboxWebIsolated}
	case UIContributionWebPage, UIContributionMessageRenderer:
		return []UISandboxType{SandboxSchemaRenderer, SandboxWebRestricted, SandboxWebIsolated}
	case UIContributionAction, UIContributionMenuItem, UIContributionToolbarItem,
		UIContributionStatusItem, UIContributionMessageAction, UIContributionComposerAction,
		UIContributionDesktopCommand:
		return []UISandboxType{SandboxHostNative}
	default:
		return []UISandboxType{kind.DefaultSandbox()}
	}
}
