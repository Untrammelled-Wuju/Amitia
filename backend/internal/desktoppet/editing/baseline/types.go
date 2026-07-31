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
	ContentHashVersionV1 = "v1"
)

const (
	PromotionPolicyFirstRevisionOnly = "first_revision_only"
	PromotionPolicyAlways            = "always"
	PromotionPolicyManual            = "manual"
)

const (
	RevisionStatusCandidate        = "candidate"
	RevisionStatusReady            = "ready"
	RevisionStatusFailed           = "failed"
	RevisionStatusLegacyUnresolved = "legacy_unresolved"
	RevisionStatusArchived         = "archived"
)

const (
	BridgeStatusProcessingPublished  = "processing_published"
	BridgeStatusActionRevisionCreated = "action_revision_created"
	BridgeStatusBindingActivated     = "binding_activated"
	BridgeStatusCompleted            = "completed"
	BridgeStatusFailed               = "failed"
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
	ActionKey        string
	ActionRevisionID string
	ContentHash      string
	BindingRevision  int64
}

type ActiveRevisionSet struct {
	ProcessingTaskID string
	CharacterID      string
	Revisions        []ActiveRevisionRef
	SetHash          string
}
