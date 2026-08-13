package shortcuts

type ShortcutsState string

const (
	ShortcutsStateReady    ShortcutsState = "ready"
	ShortcutsStateBusy     ShortcutsState = "busy"
	ShortcutsStateDisabled ShortcutsState = "disabled"
	ShortcutsStateFrozen   ShortcutsState = "frozen"
)

type ShortcutExposure string

const (
	ShortcutExposureNone      ShortcutExposure = "none"
	ShortcutExposureSiri      ShortcutExposure = "siri"
	ShortcutExposureShortcuts ShortcutExposure = "shortcuts"
	ShortcutExposureSpotlight ShortcutExposure = "spotlight"
	ShortcutExposureAppShortcut ShortcutExposure = "app_shortcut"
	ShortcutExposureAll       ShortcutExposure = "all"
)

type ShortcutExecutionMode string

const (
	ShortcutExecutionModeBackgroundSafe ShortcutExecutionMode = "background_safe"
	ShortcutExecutionModeForegroundDynamic ShortcutExecutionMode = "foreground_dynamic"
	ShortcutExecutionModeForegroundImmediate ShortcutExecutionMode = "foreground_immediate"
	ShortcutExecutionModeForegroundDeferred ShortcutExecutionMode = "foreground_deferred"
)

type ShortcutRiskLevel string

const (
	ShortcutRiskLevelReadOnly   ShortcutRiskLevel = "read_only_low_risk"
	ShortcutRiskLevelMedium     ShortcutRiskLevel = "medium_side_effect"
	ShortcutRiskLevelUIMediated ShortcutRiskLevel = "ui_mediated"
	ShortcutRiskLevelHigh       ShortcutRiskLevel = "high_risk"
)

type ShortcutCanonicalTarget string

const (
	ShortcutCanonicalTargetService    ShortcutCanonicalTarget = "service"
	ShortcutCanonicalTargetNativeCore ShortcutCanonicalTarget = "native_core"
	ShortcutCanonicalTargetTool       ShortcutCanonicalTarget = "tool"
	ShortcutCanonicalTargetInteraction ShortcutCanonicalTarget = "interaction"
)

type ShortcutRuntimeRequirement string

const (
	ShortcutRuntimeRequirementNativeOnly       ShortcutRuntimeRequirement = "native_only"
	ShortcutRuntimeRequirementBackendRead      ShortcutRuntimeRequirement = "backend_read"
	ShortcutRuntimeRequirementBackendInteraction ShortcutRuntimeRequirement = "backend_interaction"
	ShortcutRuntimeRequirementBackendTool      ShortcutRuntimeRequirement = "backend_tool"
	ShortcutRuntimeRequirementForegroundUI     ShortcutRuntimeRequirement = "foreground_ui"
)

type EntityPrivacyLevel string

const (
	EntityPrivacyPublicSystemSafe EntityPrivacyLevel = "public_system_safe"
	EntityPrivacyUserPrivate      EntityPrivacyLevel = "user_private"
	EntityPrivacySensitive        EntityPrivacyLevel = "sensitive"
	EntityPrivacySecret           EntityPrivacyLevel = "secret"
)

type ShortcutsStatus struct {
	Supported           bool             `json:"supported"`
	Enabled             bool             `json:"enabled"`
	State               ShortcutsState   `json:"state"`
	SiriAvailable       bool             `json:"siriAvailable"`
	ShortcutsAvailable  bool             `json:"shortcutsAvailable"`
	SpotlightAvailable  bool             `json:"spotlightAvailable"`
	NativeHostReady     bool             `json:"nativeHostReady"`
	BackendReady        bool             `json:"backendReady"`
	IntentCount         int              `json:"intentCount"`
	AppShortcutCount    int              `json:"appShortcutCount"`
	CatalogCount        int              `json:"catalogCount"`
	SchemaVersion       int              `json:"schemaVersion"`
	Locale              string           `json:"locale"`
}

type ShortcutInvocationScope struct {
	Trigger         string `json:"trigger"`
	RequestID       string `json:"requestId"`
	CorrelationID   string `json:"correlationId"`
	CausationID     string `json:"causationId"`
	UserID          string `json:"userId"`
	CharacterID     string `json:"characterId,omitempty"`
	ConversationID  string `json:"conversationId,omitempty"`
	SessionID       string `json:"sessionId,omitempty"`
	ToolCallID      string `json:"toolCallId,omitempty"`
}

type ShortcutInvocationMetadata struct {
	InvocationID    string `json:"invocationId"`
	IdempotencyKey  string `json:"idempotencyKey"`
	ExecutionMode   ShortcutExecutionMode `json:"executionMode"`
	IsForeground    bool   `json:"isForeground"`
	UserInitiated   bool   `json:"userInitiated"`
	SiriPhrase      string `json:"siriPhrase,omitempty"`
	AutomationID    string `json:"automationId,omitempty"`
}

type ShortcutActionRequest struct {
	ActionID    string                 `json:"actionId"`
	Parameters  map[string]any         `json:"parameters"`
	Scope       ShortcutInvocationScope `json:"scope"`
	Invocation  ShortcutInvocationMetadata `json:"invocation"`
}

type ShortcutActionResult struct {
	Status             string `json:"status"`
	Value              any    `json:"value,omitempty"`
	DisplayText        string `json:"displayText,omitempty"`
	OpensRoute         string `json:"opensRoute,omitempty"`
	RequiresForeground bool   `json:"requiresForeground"`
	ErrorCode          string `json:"errorCode,omitempty"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
	InvocationID       string `json:"invocationId"`
}

type ShortcutActionDescriptor struct {
	ID                  string                 `json:"id"`
	DisplayName         string                 `json:"displayName"`
	Description         string                 `json:"description"`
	Exposure            ShortcutExposure       `json:"exposure"`
	ExecutionMode       ShortcutExecutionMode  `json:"executionMode"`
	RiskLevel           ShortcutRiskLevel      `json:"riskLevel"`
	RequiresConfirmation bool                  `json:"requiresConfirmation"`
	RequiresForeground  bool                   `json:"requiresForeground"`
	RequiresRuntime     ShortcutRuntimeRequirement `json:"requiresRuntime"`
	CanonicalTarget     ShortcutCanonicalTarget `json:"canonicalTarget"`
	CanonicalRef        string                 `json:"canonicalRef,omitempty"`
	ParameterSchema     map[string]any         `json:"parameterSchema,omitempty"`
	Enabled             bool                   `json:"enabled"`
}

type ShortcutActionEntity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Category    string `json:"category"`
	RiskLevel   string `json:"riskLevel"`
	Exposure    string `json:"exposure"`
}

type ShortcutActionCatalog struct {
	Actions    []ShortcutActionEntity `json:"actions"`
	TotalCount int                    `json:"totalCount"`
}

type CharacterEntity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURI   string `json:"avatarUri,omitempty"`
	Privacy     EntityPrivacyLevel `json:"privacy"`
}

type ConversationEntity struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	CharacterID         string `json:"characterId"`
	CharacterDisplayName string `json:"characterDisplayName"`
	LastUsedAt          string `json:"lastUsedAt,omitempty"`
	Privacy             EntityPrivacyLevel `json:"privacy"`
}

type AlarmEntity struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	State       string `json:"state"`
	NextFireAt  string `json:"nextFireAt,omitempty"`
	Privacy     EntityPrivacyLevel `json:"privacy"`
}

type ReminderEntity struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	DueAt       string `json:"dueAt,omitempty"`
	Completed   bool   `json:"completed"`
	Privacy     EntityPrivacyLevel `json:"privacy"`
}

type EntityQueryRequest struct {
	EntityType string   `json:"entityType"`
	IDs        []string `json:"ids,omitempty"`
	Query      string   `json:"query,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

type CharacterQueryRequest struct {
	IDs       []string `json:"ids,omitempty"`
	Query     string   `json:"query,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
	Privacy   EntityPrivacyLevel `json:"privacy"`
}

type ConversationQueryRequest struct {
	IDs          []string `json:"ids,omitempty"`
	CharacterID  string   `json:"characterId,omitempty"`
	Query        string   `json:"query,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	Offset       int      `json:"offset,omitempty"`
	Privacy      EntityPrivacyLevel `json:"privacy"`
	IncludeTitle bool     `json:"includeTitle"`
}

type EntityQueryResult struct {
	Found  []any  `json:"found"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

type DynamicOptionsRequest struct {
	ParameterType string `json:"parameterType"`
	Dependencies  map[string]any `json:"dependencies,omitempty"`
	Query         string `json:"query,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type DynamicOptionsResult struct {
	Options []any  `json:"options"`
	Count   int    `json:"count"`
	Stale   bool   `json:"stale,omitempty"`
	Error   string `json:"error,omitempty"`
}

type ShortcutRuntimeStatus struct {
	Ready             bool   `json:"ready"`
	NativeHostReady   bool   `json:"nativeHostReady"`
	BackendReady      bool   `json:"backendReady"`
	PermissionReady   bool   `json:"permissionReady"`
	InteractionReady  bool   `json:"interactionReady"`
	CanBackground     bool   `json:"canBackground"`
	CanForeground     bool   `json:"canForeground"`
	Error             string `json:"error,omitempty"`
}

type ShortcutRuntimeEnsureRequest struct {
	Requirement ShortcutRuntimeRequirement `json:"requirement"`
	TimeoutMs   int64                      `json:"timeoutMs,omitempty"`
}

type ShortcutContribution struct {
	ActionID            string         `json:"actionId"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Exposure            ShortcutExposure `json:"exposure"`
	Risk                ShortcutRiskLevel `json:"risk"`
	ParameterSchema     map[string]any `json:"parameterSchema,omitempty"`
	RequiredPermissions []string       `json:"requiredPermissions,omitempty"`
}

type ShortcutContributionResult struct {
	Accepted bool   `json:"accepted"`
	ActionID string `json:"actionId,omitempty"`
	Error    string `json:"error,omitempty"`
}

type EntitySnapshot struct {
	Version       int                    `json:"version"`
	GeneratedAt   string                 `json:"generatedAt"`
	Characters    []CharacterEntity      `json:"characters,omitempty"`
	Conversations []ConversationEntity   `json:"conversations,omitempty"`
	Schedule      *SnapshotSchedule      `json:"schedule,omitempty"`
}

type SnapshotSchedule struct {
	NextRefreshAt string `json:"nextRefreshAt,omitempty"`
	TTLSeconds    int    `json:"ttlSeconds"`
}

type IntentDonationRequest struct {
	IntentID   string         `json:"intentId"`
	ActionID   string         `json:"actionId,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Timestamp  string         `json:"timestamp"`
}

type IntentDonationResult struct {
	Success   bool   `json:"success"`
	DonationID string `json:"donationId,omitempty"`
	Error     string `json:"error,omitempty"`
}

type AppShortcutProviderRequest struct {
	IncludeHighRisk bool `json:"includeHighRisk"`
}

type AppShortcutEntry struct {
	ActionID           string `json:"actionId"`
	Title              string `json:"title"`
	Phrase             string `json:"phrase"`
	Exposure           string `json:"exposure"`
	RequiresForeground bool   `json:"requiresForeground"`
}

type ShortcutSettings struct {
	Enabled                   bool `json:"enabled"`
	AskAmitiaEnabled          bool `json:"askAmitiaEnabled"`
	VoiceEnabled              bool `json:"voiceEnabled"`
	AlarmEnabled              bool `json:"alarmEnabled"`
	ReminderEnabled           bool `json:"reminderEnabled"`
	CalendarEnabled           bool `json:"calendarEnabled"`
	ShareEnabled              bool `json:"shareEnabled"`
	ExposeConversationTitles  bool `json:"exposeConversationTitles"`
	SafeToolModeDefault       bool `json:"safeToolModeDefault"`
	BackgroundAutomationSafe  bool `json:"backgroundAutomationSafe"`
}

type AppShortcutUpdateRequest struct {
	RefreshParams bool     `json:"refreshParams"`
	ActionIDs     []string `json:"actionIds,omitempty"`
}

type ParameterSummaryRequest struct {
	IntentID string `json:"intentId"`
	Locale   string `json:"locale,omitempty"`
}

type ParameterSummaryResult struct {
	Summary     string `json:"summary"`
	Parameters  []ParameterDescriptor `json:"parameters"`
}

type ParameterDescriptor struct {
	Name     string `json:"name"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type ConfirmationRequest struct {
	ActionID    string `json:"actionId"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	ObjectName  string `json:"objectName,omitempty"`
	Consequence string `json:"consequence,omitempty"`
}

type ConfirmationResult struct {
	Confirmed bool   `json:"confirmed"`
	Cancelled bool   `json:"cancelled"`
	Error     string `json:"error,omitempty"`
}

type ShortcutErrorRequest struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const BridgeProtocolVersion = 1
