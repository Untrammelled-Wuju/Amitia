package baseline

type ProcessingRevisionID string

type ActionRevisionID string

type QualityEvaluationID string

type PackageReleaseID string

const (
	SourceTypeProcessingBaseline      = "processing_baseline"
	SourceTypeManualEdit              = "manual_edit"
	SourceTypeSingleFrameRegeneration = "single_frame_regeneration"
	SourceTypeFullActionRegeneration  = "full_action_regeneration"
	SourceTypeLegacyImport            = "legacy_import"
)

const (
	OriginSystem    = "system"
	OriginUser      = "user"
	OriginMigration = "migration"
)

const (
	PromotionPolicyFirstRevisionOnly              = "first_revision_only"
	PromotionPolicyReplaceSystemBaselineIfUnchanged = "replace_system_baseline_if_unchanged"
	PromotionPolicyManual                          = "manual"

	PromotionPolicyAlways = "always"
)

const (
	RevisionStatusCommitting       = "committing"
	RevisionStatusReady            = "ready"
	RevisionStatusFailed           = "failed"
	RevisionStatusLegacyUnresolved = "legacy_unresolved"
	RevisionStatusArchived         = "archived"

	RevisionStatusCandidate = "ready"
)

const (
	BridgeStatusReceived             = "received"
	BridgeStatusValidated            = "validated"
	BridgeStatusCommitting           = "committing"
	BridgeStatusRevisionCreated      = "revision_created"
	BridgeStatusBindingCommitted     = "binding_committed"
	BridgeStatusEventsCommitted      = "events_committed"
	BridgeStatusCompleted            = "completed"
	BridgeStatusFailedRetryable      = "failed_retryable"
	BridgeStatusFailedTerminal       = "failed_terminal"

	BridgeStatusProcessingPublished   = "received"
	BridgeStatusActionRevisionCreated = "revision_created"
	BridgeStatusBindingActivated      = "binding_committed"
	BridgeStatusFailed                = "failed_retryable"
)

const (
	InboxStatusReceived        = "received"
	InboxStatusProcessing      = "processing"
	InboxStatusCommitting      = "committing"
	InboxStatusCompleted       = "completed"
	InboxStatusFailedRetryable = "failed_retryable"
	InboxStatusFailedTerminal  = "failed_terminal"
)

const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublishing = "publishing"
	OutboxStatusPublished  = "published"
	OutboxStatusFailed     = "failed"
)

const (
	ContentHashVersionV1 = "v1"
	ContentHashVersionV2 = "v2"
)

const (
	EventActionRevisionCreated     = "desktop_pet.action_revision.created"
	EventActionRevisionActivated   = "desktop_pet.action_revision.activated"
	EventActionRevisionDeactivated = "desktop_pet.action_revision.deactivated"
	EventActionRevisionSuperseded  = "desktop_pet.action_revision.superseded"
)

type CreateBaselineRevisionRequest struct {
	UserID                  string
	CharacterID             string
	ProcessingTaskID        string
	ProcessingActionID      string
	ProcessingRevisionID    string
	ActionKey               string
	ActionConfigJSON        string
	ActionConfigHash        string
	ActionSpecVersion       string
	FrameCount              int
	PlaybackMode            string
	FPS                     int
	AnchorJSON              string
	FrameDurationMS         int
	LoopType                string
	PromotionPolicy         string
	CreatedBy               string
}

type ActionRevision struct {
	ID                         string
	UserID                     string
	CharacterID                string
	ProcessingTaskID           string
	ProcessingActionID         string
	ActionKey                  string
	SourceType                 string
	SourceProcessingRevisionID string
	ParentActionRevisionID     string
	RevisionNumber             int64
	ContentHash                string
	ContentHashVersion         string
	ActionConfigHash           string
	FrameSetHash               string
	Status                     string
	Origin                     string
	FrameCount                 int
	PlaybackMode               string
	FPS                        int
	AnchorJSON                 string
	QualityEvaluationID        string
	QualityVerdict             string
	QualityScore               *float64
	CreatedBy                  string
	CreatedAt                  string
}

type ActionRevisionEvent struct {
	EventID              string
	UserID               string
	CharacterID          string
	ActionKey            string
	ActionRevisionID     string
	PreviousRevisionID   string
	ProcessingRevisionID string
	BindingRevision      int64
	Reason               string
	OccurredAt           string
}

type ActiveRevisionRef struct {
	ActionKey            string
	ActionRevisionID     string
	ContentHash          string
	BindingRevision      int64
	ActionStreamID       string
	QualityEvaluationID  string
	QualityVerdict       string
}

type ActiveRevisionSet struct {
	ProcessingTaskID    string
	UserID              string
	CharacterID         string
	Revisions           []ActiveRevisionRef
	RequiredActionKeys  []string
	OptionalActionKeys  []string
	SetHash             string
	CreatedAt           string
}

type ReturnTarget struct {
	ActionKey string
	Mode      string
}

type AnchorPolicy struct {
	X     float64
	Y     float64
	Space string
}

type ActionConfigSnapshot struct {
	SchemaVersion        int
	ActionKey            string
	DisplayName          string
	SpecVersion          string
	SpecHash             string
	PlaybackMode         string
	FPS                  int
	Interruptible        bool
	InterruptAfterMS     int
	Priority             int
	CooldownMS           int
	MinimumPlayMS        int
	MaximumPlayMS        *int
	MutexGroup           string
	SupportsDefaultIdle  bool
	IsStableStateCandidate bool
	IsTransitionOnly     bool
	ReturnTo             ReturnTarget
	Anchor               AnchorPolicy
	ConfigHash           string
}

type CommitResult struct {
	ActionRevisionID string
	RevisionNumber   int64
	ActionStreamID   string
	BindingRevision  int64
	Bound            bool
}

type ProcessingRevisionValidation struct {
	Valid          bool
	UserID         string
	CharacterID    string
	ProcessingTaskID string
	ProcessingActionID string
	ActionKey      string
	FrameCount     int
	RevisionHash   string
	ContentRootHash string
}
