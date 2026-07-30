package contracts

type GenerationMode string

const (
	GenerationModeSpriteSheet GenerationMode = "sprite_sheet"
	GenerationModeKeyframe    GenerationMode = "keyframe"
	GenerationModeSingleFrame GenerationMode = "single_frame"
	GenerationModeLegacyFrame GenerationMode = "legacy_frame"
)

func IsValidGenerationMode(m string) bool {
	switch GenerationMode(m) {
	case GenerationModeSpriteSheet, GenerationModeKeyframe, GenerationModeSingleFrame, GenerationModeLegacyFrame:
		return true
	default:
		return false
	}
}

type ProcessingSourceManifest struct {
	SchemaVersion       int
	GenerationTaskID    string
	GenerationActionID  string
	GenerationAttemptID string
	ActionKey           string
	GenerationMode      GenerationMode
	PrimaryArtifactID   string
	PrimaryFilePath     string
	ContentHash         string
	Layout              *SpriteSheetLayout
	Frames              []SourceFrameRef
	ReferenceAssetID    string
	ActionSpecHash      string
	ProviderReceiptID   string
}

type SpriteSheetLayout struct {
	Rows             int
	Columns          int
	CellWidth        int
	CellHeight       int
	ExpectedWidth    int
	ExpectedHeight   int
	EmptyCellIndexes []int
}

type SourceFrameRef struct {
	FrameIndex      int
	ArtifactID      string
	RelativePath    string
	ContentHash     string
	CellIndex       *int
	CropMinX        int
	CropMinY        int
	CropMaxX        int
	CropMaxY        int
	ExpectedWidth   int
	ExpectedHeight  int
}

type RevisionRef struct {
	RevisionID   string
	RevisionHash string
	SourceKind   string
	SourceID     string
	ActionKey    string
}

type QualityGateResult struct {
	RevisionID       string
	EvaluationID     string
	Status           string
	Verdict          string
	Passed           bool
	BlockingFindings []string
	RuleVersion      string
	EvaluatedHash    string
}

type RuntimeOperationResult struct {
	OperationID      string
	DesiredRevision  int64
	DeliveryStatus   string
	RuntimeOnline    bool
	EffectiveStatus  string
	ErrorCode        string
}

const (
	DeliveryStatusQueued        = "queued"
	DeliveryStatusSent          = "sent"
	DeliveryStatusAcked         = "acked"
	DeliveryStatusApplied       = "applied"
	DeliveryStatusFailed        = "failed"
	DeliveryStatusPendingSync   = "pending_sync"
	DeliveryStatusPendingRuntime = "pending_runtime"
	DeliveryStatusExpired       = "expired"
)

const (
	EffectiveStatusActive      = "active"
	EffectiveStatusPending     = "pending_runtime"
	EffectiveStatusFailed      = "failed"
	EffectiveStatusOfflineWait = "offline_wait"
)

type QualityVerdict string

const (
	VerdictAccepted          QualityVerdict = "accepted"
	VerdictAcceptedWithWarn  QualityVerdict = "accepted_with_warning"
	VerdictNeedsReview       QualityVerdict = "needs_review"
	VerdictRejected          QualityVerdict = "rejected"
	VerdictEvaluationFailed  QualityVerdict = "evaluation_failed"
)

func MapVerdictToReleaseStatus(v QualityVerdict) string {
	switch v {
	case VerdictAccepted:
		return "pass"
	case VerdictAcceptedWithWarn:
		return "warning"
	case VerdictNeedsReview, VerdictRejected, VerdictEvaluationFailed:
		return "fail"
	default:
		return "fail"
	}
}
