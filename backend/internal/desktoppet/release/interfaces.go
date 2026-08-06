package release

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type GateStatus string

const (
	GateStatusPassed         GateStatus = "passed"
	GateStatusPassedWithWarn GateStatus = "passed_with_warning"
	GateStatusMissing        GateStatus = "missing"
	GateStatusPending        GateStatus = "pending"
	GateStatusReviewRequired GateStatus = "review_required"
	GateStatusFailed         GateStatus = "failed"
	GateStatusError          GateStatus = "error"
	GateStatusStale          GateStatus = "stale"
)

type QualityGateResult struct {
	GateStatus            GateStatus
	GateID                string
	GateHash              string
	IncludedActionKeys    []string
	RequiredActionKeys    []string
	ExcludedActionKeys    []string
	ActiveRevisionSetHash string
	EvaluationSetHash     string
	ProfileID             string
	RuleSetVersion        string
	RuleSetContentHash    string
	ActionVerdicts        []GateActionVerdict
}

type GateActionVerdict struct {
	ActionKey        string
	ActionName       string
	Required         bool
	Verdict          string
	ExecutionStatus  string
	OverallScore     *float64
	FindingCount     int
	HardGateCount    int
	ActionRevisionID string
	EvaluationID     string
}

func (g GateStatus) IsAllowed() bool {
	return g == GateStatusPassed || g == GateStatusPassedWithWarn
}

func (g GateStatus) ErrorCode() string {
	switch g {
	case GateStatusMissing:
		return "quality_gate_missing"
	case GateStatusPending:
		return "quality_gate_pending"
	case GateStatusReviewRequired:
		return "quality_gate_review_required"
	case GateStatusFailed:
		return "quality_gate_failed"
	case GateStatusError:
		return "quality_gate_error"
	case GateStatusStale:
		return "quality_gate_stale"
	default:
		return "quality_gate_unknown"
	}
}

type ReleaseQualityGateReader interface {
	GetValidGateForRelease(
		ctx context.Context,
		userID string,
		processingTaskID string,
		activeRevisionSetHash string,
	) (*QualityGateResult, error)
}

type RevisionSnapshotData struct {
	ActionKey           string
	ActionRevisionID    string
	ContentHash         string
	ActionConfigHash    string
	FrameSetHash        string
	FrameArtifactIDs    []string
	QualityEvaluationID string
	QualityVerdict      string
	BindingRevision     int64
	ActionConfigJSON    string
	QualityResultHash   string
	FrameCount          int
	Frames              []FrameSnapshotData
}

type FrameSnapshotData struct {
	FrameID      string
	LogicalIndex int
	AssetID      string
	ContentHash  string
	DurationMS   int
	Width        int
	Height       int
	MimeType     string
	StoragePath  string
}

type SnapshotSource interface {
	GetProcessingTaskInfo(processingTaskID string) (*TaskInfo, error)
	ListProcessingActions(processingTaskID string) ([]ActionInfo, error)
	GetActiveRevisionDetail(processingTaskID, actionKey string) (*RevisionDetail, error)
	GetAssetPath(assetID string) (string, error)
	ResolvePreviewArtifactID(processingTaskID, petID string) (string, error)
	GetActiveRevisionSetHash(processingTaskID string) (string, error)
}

type TaskInfo struct {
	ID                string
	GenerationTaskID  string
	ProcessingVersion int
	OutputWidth       int
	OutputHeight      int
	DefaultFPS        int
	CharacterID       string
	PackageName       string
	UserID            string
}

type ActionInfo struct {
	ActionKey           string
	ActionNameSnapshot  string
	Status              string
	Excluded            bool
	LoopType            string
	FPS                 int
	FrameDurationMS     int
	SupportsDefaultIdle bool
	Required            bool
}

type RevisionDetail struct {
	RevisionID     string
	RevisionNumber int
	Status         string
	FrameCount     int
	DurationMS     int
	DefaultFPS     int
	LoopType       string
	ReturnAction   string
	ReturnPolicy   string
	Interruptible  bool
	Priority       int
	CooldownMs     int
	MinimumPlayMs  int
	MaximumPlayMs  int
	MutexGroup     string
	AnchorX        float64
	AnchorY        float64
	QualityVerdict string
	Frames         []FrameSnapshotData
}

type SequenceAllocator interface {
	AllocateSequence(ctx context.Context, petID string) (int, error)
}

type ReleaseRepository interface {
	DB() *gorm.DB
	Transaction(fn func(tx *gorm.DB) error) error

	CreateBuildSnapshot(snapshot *ReleaseBuildSnapshot) error
	GetBuildSnapshot(id string) (*ReleaseBuildSnapshot, error)
	GetBuildSnapshotByInputHash(inputHash string) (*ReleaseBuildSnapshot, error)

	CreateBuildOperation(op *ReleaseBuildOperation) error
	GetBuildOperation(id string) (*ReleaseBuildOperation, error)
	GetBuildOperationByIdempotencyKey(userID, idempotencyKey string) (*ReleaseBuildOperation, error)
	UpdateBuildOperation(op *ReleaseBuildOperation) error
	ListPendingBuildOperations() ([]*ReleaseBuildOperation, error)
	ListStaleBuildOperations(leaseExpiryBefore string) ([]*ReleaseBuildOperation, error)

	CreatePublishJournal(journal *ReleasePublishJournal) error
	GetPublishJournalByOperation(operationID string) (*ReleasePublishJournal, error)
	UpdatePublishJournal(journal *ReleasePublishJournal) error
	ListPendingPublishJournals() ([]*ReleasePublishJournal, error)

	CreateLegacyPackageMapping(mapping *LegacyPackageMapping) error
	GetLegacyPackageMapping(legacyPackageID string) (*LegacyPackageMapping, error)
	UpdateLegacyPackageMapping(mapping *LegacyPackageMapping) error
	ListPendingLegacyMappings() ([]*LegacyPackageMapping, error)

	CreateLegacyMigrationOperation(op *LegacyPackageMigrationOperation) error
	GetLegacyMigrationOperation(id string) (*LegacyPackageMigrationOperation, error)
	UpdateLegacyMigrationOperation(op *LegacyPackageMigrationOperation) error

	GetPetIdentity(petID string) (*PetIdentityData, error)
	GetPetIdentityByCharacter(userID, characterID string) (*PetIdentityData, error)
	CreatePetIdentity(identity *PetIdentityData) error
	CreatePetIdentityTx(tx *gorm.DB, identity *PetIdentityData) error
	UpdatePetIdentity(identity *PetIdentityData) error
	UpdatePetIdentityTx(tx *gorm.DB, identity *PetIdentityData) error

	GetReleaseByContentHash(contentRootHash string) (*ReleaseData, error)
	GetRelease(releaseID string) (*ReleaseData, error)
	CreateRelease(release *ReleaseData) error
	UpdateRelease(release *ReleaseData) error
	ListReleasesByPet(petID string) ([]*ReleaseData, error)
	ListPublishedReleases(userID string) ([]*ReleaseData, error)

	CreateReleaseFiles(files []ReleaseFileData) error
	GetReleaseFiles(releaseID string) ([]ReleaseFileData, error)

	CreateValidationReport(report *ReleaseValidationReport) error
	GetValidationReport(releaseID string) (*ReleaseValidationReport, error)

	CreateEventOutbox(event *ReleaseEventOutbox) error
	ListPendingOutboxEvents(limit int) ([]*ReleaseEventOutbox, error)
	UpdateOutboxEvent(event *ReleaseEventOutbox) error

	CreateBuildRequestInbox(inbox *ReleaseBuildRequestInbox) error
	GetBuildRequestInbox(requestID string) (*ReleaseBuildRequestInbox, error)
	UpdateBuildRequestInbox(inbox *ReleaseBuildRequestInbox) error

	CreateImportSnapshot(snapshot *ImportPackageSnapshot) error
	CreateImportSnapshotTx(tx *gorm.DB, snapshot *ImportPackageSnapshot) error
	GetImportSnapshot(stagingID string) (*ImportPackageSnapshot, error)
	UpdateImportSnapshot(snapshot *ImportPackageSnapshot) error

	AcquireLeaseCAS(op *ReleaseBuildOperation, owner, executionID string) error

	CreateSnapshotTx(tx *gorm.DB, snapshot *ReleaseBuildSnapshot) error
	CreateOperationTx(tx *gorm.DB, op *ReleaseBuildOperation) error
	CreateReleaseTx(tx *gorm.DB, release *ReleaseData) error
	CreateReleaseFilesTx(tx *gorm.DB, files []ReleaseFileData) error
	CreateValidationReportTx(tx *gorm.DB, report *ReleaseValidationReport) error
	CreateOutboxTx(tx *gorm.DB, event *ReleaseEventOutbox) error
	UpdateOperationOwned(tx *gorm.DB, op *ReleaseBuildOperation, expectedState string) error
}

type PetIdentityData struct {
	ID                  string
	OwnerUserID         string
	SourceCharacterID   string
	Name                string
	Slug                string
	BindingPolicy       string
	UpstreamPetID       string
	DefaultActionKey    string
	NextReleaseSequence int
	CreatedAt           string
	UpdatedAt           string
}

type ReleaseStoragePort interface {
	StagingDir(releaseID string) (string, error)
	WorkspaceDir(operationID string) (string, error)
	EnsureWorkspaceDir(operationID string) error
	EnsureStagingDir(releaseID string) error
	RemoveStagingDir(releaseID string) error
	RemoveWorkspaceDir(operationID string) error
	PublishedDir(petID, releaseID string) (string, error)
	PublishedStorageKey(petID, releaseID string) (string, error)
	ArchivePath(petID, releaseID string) (string, error)
	ArchiveStorageKey(petID, releaseID string) (string, error)
	MoveStagingToPublished(petID, releaseID string) error
	MoveWorkspaceToStaging(operationID, releaseID string) error
	RemovePublishedDir(petID, releaseID string) error
	AtomicRenameStagingToPublished(petID, releaseID string) error
}

type ArtifactResolver interface {
	ResolveAbsolutePath(artifactID string) (string, error)
}

type TimeProvider interface {
	Now() time.Time
}

type SystemTime struct{}

func (SystemTime) Now() time.Time { return time.Now() }

var _ = context.Background
