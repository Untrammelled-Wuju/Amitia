package behavior

import (
	"encoding/json"
	"time"
)

type EventOrigin string

const (
	OriginInteraction EventOrigin = "interaction"
	OriginChat        EventOrigin = "chat"
	OriginVoice       EventOrigin = "voice"
	OriginProactive   EventOrigin = "proactive"
	OriginAffect      EventOrigin = "affect"
	OriginActivity    EventOrigin = "activity"
	OriginTool        EventOrigin = "tool"
	OriginDesktop     EventOrigin = "desktop"
	OriginPlayback    EventOrigin = "playback"
	OriginRuntime     EventOrigin = "runtime"
	OriginManual      EventOrigin = "manual"
	OriginSystem      EventOrigin = "system"
)

type EventReliability string

const (
	ReliabilityDurable     EventReliability = "durable"
	ReliabilityRecoverable EventReliability = "recoverable"
	ReliabilityEphemeral   EventReliability = "ephemeral"
)

type BehaviorEventEnvelope struct {
	EventID         string          `json:"eventId"`
	EventType       string          `json:"eventType"`
	SchemaVersion   int             `json:"schemaVersion"`
	OccurredAt      time.Time       `json:"occurredAt"`
	ReceivedAt      time.Time       `json:"receivedAt"`
	ExpiresAt       *time.Time      `json:"expiresAt,omitempty"`
	UserID          string          `json:"userId"`
	CharacterID     string          `json:"characterId"`
	ConversationID  string          `json:"conversationId,omitempty"`
	InteractionID   string          `json:"interactionId,omitempty"`
	SessionID       string          `json:"sessionId,omitempty"`
	InstallationID  string          `json:"installationId,omitempty"`
	PetInstanceID   string          `json:"petInstanceId,omitempty"`
	ToolOperationID string          `json:"toolOperationId,omitempty"`
	ReleaseID       string          `json:"releaseId,omitempty"`
	Origin          EventOrigin     `json:"origin"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	CausationID     string          `json:"causationId,omitempty"`
	Sequence        int64           `json:"sequence,omitempty"`
	DedupKey        string          `json:"dedupKey"`
	PriorityHint    int             `json:"priorityHint,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type StableBehaviorState struct {
	ActivityKey        string  `json:"activityKey,omitempty"`
	ActivitySource     string  `json:"activitySource,omitempty"`
	ActivityConfidence float64 `json:"activityConfidence,omitempty"`
	AffectLabel        string  `json:"affectLabel,omitempty"`
	AffectVersion      string  `json:"affectVersion,omitempty"`
	TimePeriod         string  `json:"timePeriod,omitempty"`
	DefaultIdlePref    string  `json:"defaultIdlePref,omitempty"`
}

type TransientBehaviorState struct {
	InteractionID    string `json:"interactionId,omitempty"`
	InteractionPhase string `json:"interactionPhase,omitempty"`
	StatusVersion    int64  `json:"statusVersion,omitempty"`
	ProactiveID      string `json:"proactiveId,omitempty"`
	ProactiveIntent  string `json:"proactiveIntent,omitempty"`
	TemporaryEmotion string `json:"temporaryEmotion,omitempty"`
}

type ToolOperationState struct {
	OperationID    string    `json:"operationId"`
	ToolCategory   string    `json:"toolCategory,omitempty"`
	DisplayClass   string    `json:"displayClass,omitempty"`
	Depth          int       `json:"depth,omitempty"`
	StartedAt      time.Time `json:"startedAt"`
	LastActivityAt time.Time `json:"lastActivityAt"`
	LeaseExpiresAt time.Time `json:"leaseExpiresAt"`
	LongRunning    bool      `json:"longRunning,omitempty"`
}

type VoiceBehaviorState struct {
	SessionID      string    `json:"sessionId,omitempty"`
	State          string    `json:"state,omitempty"`
	TurnID         string    `json:"turnId,omitempty"`
	StateVersion   int64     `json:"stateVersion,omitempty"`
	LeaseExpiresAt time.Time `json:"leaseExpiresAt,omitempty"`
}

type DesktopGestureState struct {
	CurrentGesture  string `json:"currentGesture,omitempty"`
	GestureID       string `json:"gestureId,omitempty"`
	Sequence        int64  `json:"sequence,omitempty"`
	PendingClickWin bool   `json:"pendingClickWin,omitempty"`
}

type ForegroundActionState struct {
	DecisionID      string     `json:"decisionId,omitempty"`
	CommandID       string     `json:"commandId,omitempty"`
	Semantic        string     `json:"semantic,omitempty"`
	ActionKey       string     `json:"actionKey,omitempty"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	MinPlayUntil    *time.Time `json:"minPlayUntil,omitempty"`
	MaxPlayUntil    *time.Time `json:"maxPlayUntil,omitempty"`
	Interruptible   bool       `json:"interruptible,omitempty"`
	InstallationRev int64      `json:"installationRev,omitempty"`
}

type DesiredBehaviorState struct {
	Semantic        string `json:"semantic,omitempty"`
	PreferredAction string `json:"preferredAction,omitempty"`
	SourceLayer     string `json:"sourceLayer,omitempty"`
}

type RecentSemanticRecord struct {
	Semantic  string    `json:"semantic"`
	ActionKey string    `json:"actionKey"`
	At        time.Time `json:"at"`
}

type BehaviorContextSnapshot struct {
	UserID              string                        `json:"userId"`
	CharacterID         string                        `json:"characterId"`
	Revision            int64                         `json:"revision"`
	Stable              StableBehaviorState           `json:"stable"`
	Transient           TransientBehaviorState        `json:"transient"`
	ActiveTools         map[string]ToolOperationState `json:"activeTools"`
	Voice               VoiceBehaviorState            `json:"voice"`
	DesktopGesture      DesktopGestureState           `json:"desktopGesture"`
	Foreground          ForegroundActionState         `json:"foreground"`
	Cooldowns           map[string]time.Time          `json:"cooldowns"`
	RecentSemantics     []RecentSemanticRecord        `json:"recentSemantics"`
	Desired             DesiredBehaviorState          `json:"desired"`
	LastSourceRevisions map[string]int64              `json:"lastSourceRevisions"`
	UpdatedAt           time.Time                     `json:"updatedAt"`
}

type ReduceResult struct {
	ContextChanged    bool     `json:"contextChanged"`
	LayersChanged     []string `json:"layersChanged,omitempty"`
	NeedsDecision     bool     `json:"needsDecision"`
	IsDuplicate       bool     `json:"isDuplicate"`
	IsExpired         bool     `json:"isExpired"`
	IsOutOfOrder      bool     `json:"isOutOfOrder"`
	NeedsSnapshotSync bool     `json:"needsSnapshotSync"`
	Reason            string   `json:"reason,omitempty"`
}

type CandidateAction struct {
	Semantic        string        `json:"semantic"`
	PreferredKeys   []string      `json:"preferredKeys"`
	SourceEventID   string        `json:"sourceEventId"`
	SourceLayer     string        `json:"sourceLayer"`
	Priority        int           `json:"priority"`
	CreatedAt       time.Time     `json:"createdAt"`
	ExpiresAt       *time.Time    `json:"expiresAt,omitempty"`
	MinPlay         time.Duration `json:"minPlay,omitempty"`
	MaxPlay         time.Duration `json:"maxPlay,omitempty"`
	InterruptPolicy string        `json:"interruptPolicy,omitempty"`
	CooldownKey     string        `json:"cooldownKey,omitempty"`
	Cooldown        time.Duration `json:"cooldown,omitempty"`
	MutexGroup      string        `json:"mutexGroup,omitempty"`
	ReturnPolicy    string        `json:"returnPolicy,omitempty"`
	Durable         bool          `json:"durable,omitempty"`
	Confidence      float64       `json:"confidence,omitempty"`
}

type RejectionReason string

const (
	RejectExpired               RejectionReason = "expired"
	RejectMissingAction         RejectionReason = "missing_action"
	RejectCooldownActive        RejectionReason = "cooldown_active"
	RejectLowerPriority         RejectionReason = "lower_priority"
	RejectUninterruptible       RejectionReason = "current_uninterruptible"
	RejectMinPlayNotReached     RejectionReason = "minimum_play_not_reached"
	RejectMutexConflict         RejectionReason = "mutex_conflict"
	RejectRuntimeOfflineEphem   RejectionReason = "runtime_offline_ephemeral"
	RejectStaleContextRevision  RejectionReason = "stale_context_revision"
	RejectDuplicateSemantic     RejectionReason = "duplicate_semantic"
	RejectUserDisabled          RejectionReason = "user_disabled"
	RejectBindingConditionFalse RejectionReason = "binding_condition_false"
	RejectInstallationChanged   RejectionReason = "installation_changed"
	RejectUnsafeManualRequest   RejectionReason = "unsafe_manual_request"
)

type RejectedCandidate struct {
	Candidate CandidateAction   `json:"candidate"`
	Reasons   []RejectionReason `json:"reasons"`
}

type DecisionStatus string

const (
	DecisionStatusSelected         DecisionStatus = "selected"
	DecisionStatusCommandSubmitted DecisionStatus = "command_submitted"
	DecisionStatusPlaying          DecisionStatus = "playing"
	DecisionStatusCompleted        DecisionStatus = "completed"
	DecisionStatusInterrupted      DecisionStatus = "interrupted"
	DecisionStatusFailed           DecisionStatus = "failed"
	DecisionStatusExpired          DecisionStatus = "expired"
	DecisionStatusNoAction         DecisionStatus = "no_action_available"
	DecisionStatusIgnored          DecisionStatus = "ignored"
)

type BehaviorDecision struct {
	DecisionID         string              `json:"decisionId"`
	EventID            string              `json:"eventId"`
	UserID             string              `json:"userId"`
	CharacterID        string              `json:"characterId"`
	InstallationID     string              `json:"installationId,omitempty"`
	ContextRevision    int64               `json:"contextRevision"`
	RulesetVersion     int                 `json:"rulesetVersion"`
	Semantic           string              `json:"semantic,omitempty"`
	ActionKey          string              `json:"actionKey,omitempty"`
	Priority           int                 `json:"priority,omitempty"`
	InterruptPolicy    string              `json:"interruptPolicy,omitempty"`
	MinimumPlayMS      int64               `json:"minimumPlayMs,omitempty"`
	MaximumPlayMS      int64               `json:"maximumPlayMs,omitempty"`
	Status             DecisionStatus      `json:"status"`
	ReasonCode         string              `json:"reasonCode"`
	RejectedCandidates []RejectedCandidate `json:"rejectedCandidates,omitempty"`
	RuntimeCommandID   string              `json:"runtimeCommandId,omitempty"`
	FallbackDepth      int                 `json:"fallbackDepth,omitempty"`
	ReturnPolicy       string              `json:"returnPolicy,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
	StartedAt          *time.Time          `json:"startedAt,omitempty"`
	CompletedAt        *time.Time          `json:"completedAt,omitempty"`
}

type BehaviorDecisionAudit struct {
	BehaviorDecision
	ContextHash string `json:"contextHash,omitempty"`
}

type BehaviorRuntimeCommand struct {
	CommandID            string     `json:"commandId"`
	DecisionID           string     `json:"decisionId"`
	IdempotencyKey       string     `json:"idempotencyKey,omitempty"`
	RuntimeID            string     `json:"runtimeId,omitempty"`
	PetInstanceID        string     `json:"petInstanceId,omitempty"`
	InstallationID       string     `json:"installationId,omitempty"`
	InstallationRevision int64      `json:"installationRevision"`
	ContextRevision      int64      `json:"contextRevision"`
	ActionKey            string     `json:"actionKey"`
	Priority             int        `json:"priority"`
	InterruptPolicy      string     `json:"interruptPolicy,omitempty"`
	MinimumPlayMS        int64      `json:"minimumPlayMs,omitempty"`
	MaximumPlayMS        int64      `json:"maximumPlayMs,omitempty"`
	ReturnPolicy         string     `json:"returnPolicy,omitempty"`
	ExpiresAt            *time.Time `json:"expiresAt,omitempty"`
	ReasonCode           string     `json:"reasonCode,omitempty"`
	Durable              bool       `json:"durable"`
}

type CommandReceipt struct {
	CommandID     string        `json:"commandId"`
	Accepted      bool          `json:"accepted"`
	Status        CommandStatus `json:"status"`
	PendingReason string        `json:"pendingReason,omitempty"`
	Error         string        `json:"error,omitempty"`
	ReceivedAt    time.Time     `json:"receivedAt"`
}

type CommandStatus string

const (
	CmdAccepted CommandStatus = "accepted"
	CmdRejected CommandStatus = "rejected"
	CmdOffline  CommandStatus = "offline"
)

type PlaybackSnapshot struct {
	PetInstanceID     string     `json:"petInstanceId"`
	CurrentActionKey  string     `json:"currentActionKey,omitempty"`
	CurrentDecisionID string     `json:"currentDecisionId,omitempty"`
	IsPlaying         bool       `json:"isPlaying"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	RuntimeOnline     bool       `json:"runtimeOnline"`
	StateRevision     int64      `json:"stateRevision"`
}

type PlaybackPhase string

const (
	PlaybackAccepted    PlaybackPhase = "accepted"
	PlaybackStarted     PlaybackPhase = "started"
	PlaybackCompleted   PlaybackPhase = "completed"
	PlaybackInterrupted PlaybackPhase = "interrupted"
	PlaybackFailed      PlaybackPhase = "failed"
)

type PlaybackFeedback struct {
	CommandID            string        `json:"commandId"`
	DecisionID           string        `json:"decisionId"`
	PetInstanceID        string        `json:"petInstanceId"`
	InstallationRevision int64         `json:"installationRevision"`
	Phase                PlaybackPhase `json:"phase"`
	Sequence             int64         `json:"sequence,omitempty"`
	ActionKey            string        `json:"actionKey,omitempty"`
	ErrorClass           string        `json:"errorClass,omitempty"`
	OccurredAt           time.Time     `json:"occurredAt"`
}

type ActionCapability struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	CategoryKey string `json:"categoryKey"`
	Available   bool   `json:"available"`
	Priority    int    `json:"priority,omitempty"`
}

type ActivePetSnapshot struct {
	InstallationID string                      `json:"installationId"`
	ReleaseID      string                      `json:"releaseId,omitempty"`
	PetInstanceID  string                      `json:"petInstanceId"`
	CharacterID    string                      `json:"characterId"`
	RuntimeOnline  bool                        `json:"runtimeOnline"`
	StateRevision  int64                       `json:"stateRevision"`
	DefaultAction  string                      `json:"defaultAction,omitempty"`
	Actions        map[string]ActionCapability `json:"actions"`
}

type AffectBehaviorSnapshot struct {
	Version    string    `json:"version"`
	Valence    float64   `json:"valence"`
	Arousal    float64   `json:"arousal"`
	Tension    float64   `json:"tension"`
	Stress     float64   `json:"stress"`
	Label      string    `json:"label,omitempty"`
	Confidence float64   `json:"confidence"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ActivityBehaviorSnapshot struct {
	ActivityKey   string     `json:"activityKey"`
	Source        string     `json:"source"`
	Confidence    float64    `json:"confidence"`
	StartedAt     time.Time  `json:"startedAt,omitempty"`
	ExpectedEndAt *time.Time `json:"expectedEndAt,omitempty"`
	Version       string     `json:"version"`
}

type CooldownRecord struct {
	UserID           string    `json:"userId"`
	CharacterID      string    `json:"characterId"`
	CooldownKey      string    `json:"cooldownKey"`
	UntilAt          time.Time `json:"untilAt"`
	SourceDecisionID string    `json:"sourceDecisionId,omitempty"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type InboxStatus string

const (
	InboxPending    InboxStatus = "pending"
	InboxLeased     InboxStatus = "leased"
	InboxProcessed  InboxStatus = "processed"
	InboxIgnored    InboxStatus = "ignored"
	InboxDeadLetter InboxStatus = "dead_letter"
	InboxRetry      InboxStatus = "retry"
)

type InboxRecord struct {
	EventID          string          `json:"eventId"`
	DedupKey         string          `json:"dedupKey"`
	EventType        string          `json:"eventType"`
	SchemaVersion    int             `json:"schemaVersion"`
	UserID           string          `json:"userId"`
	CharacterID      string          `json:"characterId"`
	ConversationID   string          `json:"conversationId,omitempty"`
	InteractionID    string          `json:"interactionId,omitempty"`
	SessionID        string          `json:"sessionId,omitempty"`
	ToolOperationID  string          `json:"toolOperationId,omitempty"`
	InstallationID   string          `json:"installationId,omitempty"`
	PetInstanceID    string          `json:"petInstanceId,omitempty"`
	ReleaseID        string          `json:"releaseId,omitempty"`
	OccurredAt       time.Time       `json:"occurredAt"`
	ReceivedAt       time.Time       `json:"receivedAt"`
	ExpiresAt        *time.Time      `json:"expiresAt,omitempty"`
	Origin           EventOrigin     `json:"origin"`
	CorrelationID    string          `json:"correlationId,omitempty"`
	CausationID      string          `json:"causationId,omitempty"`
	Sequence         int64           `json:"sequence,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	Status           InboxStatus     `json:"status"`
	AttemptCount     int             `json:"attemptCount"`
	LeaseOwner       string          `json:"leaseOwner,omitempty"`
	LeaseExpiresAt   *time.Time      `json:"leaseExpiresAt,omitempty"`
	HeartbeatAt      *time.Time      `json:"heartbeatAt,omitempty"`
	AvailableAt      *time.Time      `json:"availableAt,omitempty"`
	LastErrorCode    string          `json:"lastErrorCode,omitempty"`
	LastErrorMessage string          `json:"lastErrorMessage,omitempty"`
	ProcessedAt      *time.Time      `json:"processedAt,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

type InteractionLifecycleEvent struct {
	InteractionID  string    `json:"interactionId"`
	CharacterID    string    `json:"characterId"`
	UserID         string    `json:"userId"`
	ConversationID string    `json:"conversationId,omitempty"`
	Phase          string    `json:"phase"`
	StatusVersion  int64     `json:"statusVersion"`
	Origin         string    `json:"origin,omitempty"`
	CorrelationID  string    `json:"correlationId,omitempty"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type ChatLifecycleEvent struct {
	InteractionID  string    `json:"interactionId,omitempty"`
	MessageID      string    `json:"messageId,omitempty"`
	CharacterID    string    `json:"characterId"`
	UserID         string    `json:"userId"`
	ConversationID string    `json:"conversationId,omitempty"`
	Phase          string    `json:"phase"`
	StatusVersion  int64     `json:"statusVersion,omitempty"`
	Origin         string    `json:"origin,omitempty"`
	CorrelationID  string    `json:"correlationId,omitempty"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type ToolLifecycleEvent struct {
	InteractionID       string    `json:"interactionId"`
	CharacterID         string    `json:"characterId"`
	UserID              string    `json:"userId"`
	OperationID         string    `json:"operationId"`
	ToolCallID          string    `json:"toolCallId,omitempty"`
	ToolName            string    `json:"toolName,omitempty"`
	ToolCategory        string    `json:"toolCategory,omitempty"`
	DisplayClass        string    `json:"displayClass,omitempty"`
	Phase               string    `json:"phase"`
	Depth               int       `json:"depth,omitempty"`
	ExpectedLongRunning bool      `json:"expectedLongRunning,omitempty"`
	ErrorClass          string    `json:"errorClass,omitempty"`
	OccurredAt          time.Time `json:"occurredAt"`
}

type VoiceLifecycleEvent struct {
	SessionID      string    `json:"sessionId"`
	TurnID         string    `json:"turnId,omitempty"`
	CharacterID    string    `json:"characterId"`
	UserID         string    `json:"userId"`
	ConversationID string    `json:"conversationId,omitempty"`
	Phase          string    `json:"phase"`
	StateVersion   int64     `json:"stateVersion,omitempty"`
	OccurredAt     time.Time `json:"occurredAt"`
}

type DesktopGestureEvent struct {
	PetInstanceID string    `json:"petInstanceId"`
	CharacterID   string    `json:"characterId"`
	UserID        string    `json:"userId"`
	GestureType   string    `json:"gestureType"`
	GestureID     string    `json:"gestureId"`
	Sequence      int64     `json:"sequence"`
	OccurredAt    time.Time `json:"occurredAt"`
}

type RulesetVersion int

const CurrentRulesetVersion RulesetVersion = 1

const MaxRecentSemantics = 32
const MaxCASRetries = 5
const MailboxCapacity = 256
const DefaultDecisionRetentionDays = 30
const DefaultInboxRetentionDays = 7
const MaxFallbackDepth = 6
const MaxBindingASTDepth = 8
