package release

import "time"

type ReleaseBuildSnapshot struct {
	ID                    string                 `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID                string                 `gorm:"column:user_id;type:text" json:"userId"`
	PetID                 string                 `gorm:"column:pet_id;type:text" json:"petId"`
	CharacterID           string                 `gorm:"column:character_id;type:text" json:"characterId"`
	ProcessingTaskID      string                 `gorm:"column:processing_task_id;type:text" json:"processingTaskId"`
	ActiveRevisionSetHash string                 `gorm:"column:active_revision_set_hash;type:text" json:"activeRevisionSetHash"`
	QualityGateID         string                 `gorm:"column:quality_gate_id;type:text" json:"qualityGateId"`
	QualityGateHash       string                 `gorm:"column:quality_gate_hash;type:text" json:"qualityGateHash"`
	DefaultActionKey      string                 `gorm:"column:default_action_key;type:text" json:"defaultActionKey"`
	IncludedActionsJSON   string                 `gorm:"column:included_actions_json;type:text;default:'[]'" json:"includedActionsJson"`
	PackageSchemaVersion  int                    `gorm:"column:package_schema_version;type:integer;default:2" json:"packageSchemaVersion"`
	RuntimeContractVersion string                `gorm:"column:runtime_contract_version;type:text" json:"runtimeContractVersion"`
	BuildConfigHash       string                 `gorm:"column:build_config_hash;type:text" json:"buildConfigHash"`
	CreatedAt             string                 `gorm:"column:created_at;type:text" json:"createdAt"`
}

func (ReleaseBuildSnapshot) TableName() string { return "desktop_pet_release_build_snapshots" }

type ReleaseActionSnapshot struct {
	ActionKey          string `json:"actionKey"`
	ActionRevisionID   string `json:"actionRevisionId"`
	ContentHash        string `json:"contentHash"`
	ActionConfigHash   string `json:"actionConfigHash"`
	FrameSetHash       string `json:"frameSetHash"`
	FrameArtifactIDs   []string `json:"frameArtifactIds"`
	QualityEvaluationID string `json:"qualityEvaluationId"`
	QualityVerdict     string `json:"qualityVerdict"`
}

type ReleaseBuildOperation struct {
	ID                    string `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID                string `gorm:"column:user_id;type:text" json:"userId"`
	PetID                 string `gorm:"column:pet_id;type:text" json:"petId"`
	SnapshotID            string `gorm:"column:snapshot_id;type:text" json:"snapshotId"`
	ReleaseID             string `gorm:"column:release_id;type:text" json:"releaseId"`
	IdempotencyKey        string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	InputHash             string `gorm:"column:input_hash;type:text" json:"inputHash"`
	State                 string `gorm:"column:state;type:text;default:'created'" json:"state"`
	Stage                 string `gorm:"column:stage;type:text;default:''" json:"stage"`
	LeaseOwner            string `gorm:"column:lease_owner;type:text;default:''" json:"leaseOwner"`
	LeaseExpiresAt        string `gorm:"column:lease_expires_at;type:text;default:''" json:"leaseExpiresAt"`
	HeartbeatAt           string `gorm:"column:heartbeat_at;type:text;default:''" json:"heartbeatAt"`
	StagingPathKey        string `gorm:"column:staging_path_key;type:text" json:"stagingPathKey"`
	PublishedPathKey      string `gorm:"column:published_path_key;type:text" json:"publishedPathKey"`
	ErrorCode             string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage          string `gorm:"column:error_message;type:text" json:"errorMessage"`
	RetryCount            int    `gorm:"column:retry_count;type:integer;default:0" json:"retryCount"`
	ResultJSON            string `gorm:"column:result_json;type:text;default:'{}'" json:"resultJson"`
	StartedAt             string `gorm:"column:started_at;type:text" json:"startedAt"`
	UpdatedAt             string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt           string `gorm:"column:completed_at;type:text" json:"completedAt"`
}

func (ReleaseBuildOperation) TableName() string { return "desktop_pet_release_build_operations" }

const (
	BuildOpStateCreated         = "created"
	BuildOpStateSnapshotting    = "snapshotting"
	BuildOpStateBuilding        = "building"
	BuildOpStateValidating      = "validating"
	BuildOpStatePublishing      = "publishing"
	BuildOpStateCompleted       = "completed"
	BuildOpStateFailedRetryable = "failed_retryable"
	BuildOpStateFailedTerminal  = "failed_terminal"
	BuildOpStateCancelled       = "cancelled"
)

const (
	BuildOpStageSnapshotCreated  = "snapshot_created"
	BuildOpStageStagingBuilt     = "staging_built"
	BuildOpStageValidated        = "validated"
	BuildOpStageFilesPublished   = "files_published"
	BuildOpStageDatabaseCommitted = "database_committed"
)

type ReleasePublishJournal struct {
	ID            string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationID   string `gorm:"column:operation_id;type:text" json:"operationId"`
	ReleaseID     string `gorm:"column:release_id;type:text" json:"releaseId"`
	PetID         string `gorm:"column:pet_id;type:text" json:"petId"`
	Stage         string `gorm:"column:stage;type:text;default:'snapshot_created'" json:"stage"`
	ContentRootHash string `gorm:"column:content_root_hash;type:text" json:"contentRootHash"`
	StagingPath   string `gorm:"column:staging_path;type:text" json:"stagingPath"`
	PublishedPath string `gorm:"column:published_path;type:text" json:"publishedPath"`
	ErrorMessage  string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt     string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ReleasePublishJournal) TableName() string { return "desktop_pet_release_publish_journals" }

const (
	JournalStageSnapshotCreated    = "snapshot_created"
	JournalStageStagingBuilt       = "staging_built"
	JournalStageValidated          = "validated"
	JournalStageFilesPublished     = "files_published"
	JournalStageDatabaseCommitted  = "database_committed"
	JournalStageCompleted          = "completed"
	JournalStageFailed             = "failed"
)

type LegacyPackageMapping struct {
	ID                string `gorm:"column:id;primaryKey;type:text" json:"id"`
	LegacyPackageID   string `gorm:"column:legacy_package_id;type:text" json:"legacyPackageId"`
	MigratedPetID     string `gorm:"column:migrated_pet_id;type:text" json:"migratedPetId"`
	MigratedReleaseID string `gorm:"column:migrated_release_id;type:text" json:"migratedReleaseId"`
	MigrationStatus   string `gorm:"column:migration_status;type:text;default:'pending'" json:"migrationStatus"`
	SourceContentHash string `gorm:"column:source_content_hash;type:text" json:"sourceContentHash"`
	ErrorMessage      string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt         string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (LegacyPackageMapping) TableName() string { return "desktop_pet_legacy_package_mappings" }

const (
	LegacyMigrationStatusPending      = "pending"
	LegacyMigrationStatusMigrated     = "migrated"
	LegacyMigrationStatusInvalid      = "invalid"
	LegacyMigrationStatusManualReview = "manual_review"
	LegacyMigrationStatusFailed       = "failed"
)

type LegacyPackageMigrationOperation struct {
	ID              string `gorm:"column:id;primaryKey;type:text" json:"id"`
	LegacyPackageID string `gorm:"column:legacy_package_id;type:text" json:"legacyPackageId"`
	UserID          string `gorm:"column:user_id;type:text" json:"userId"`
	State           string `gorm:"column:state;type:text;default:'pending'" json:"state"`
	StagingPath     string `gorm:"column:staging_path;type:text" json:"stagingPath"`
	ErrorCode       string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage    string `gorm:"column:error_message;type:text" json:"errorMessage"`
	StartedAt       string `gorm:"column:started_at;type:text" json:"startedAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt     string `gorm:"column:completed_at;type:text" json:"completedAt"`
}

func (LegacyPackageMigrationOperation) TableName() string {
	return "desktop_pet_legacy_package_migration_operations"
}

const (
	LegacyMigrationOpStatePending     = "pending"
	LegacyMigrationOpStateValidating  = "validating"
	LegacyMigrationOpStateRebuilding  = "rebuilding"
	LegacyMigrationOpStateCompleted   = "completed"
	LegacyMigrationOpStateFailedRetry = "failed_retryable"
	LegacyMigrationOpStateFailedTerm  = "failed_terminal"
)

type ReleaseLifecycle string

const (
	ReleaseLifecycleBuilding   ReleaseLifecycle = "building"
	ReleaseLifecycleReady      ReleaseLifecycle = "ready"
	ReleaseLifecycleFailed     ReleaseLifecycle = "failed"
	ReleaseLifecycleArchived   ReleaseLifecycle = "archived"
	ReleaseLifecycleRevoked    ReleaseLifecycle = "revoked"
	ReleaseLifecycleCorrupted  ReleaseLifecycle = "corrupted"
)

type ReleaseIntegrityStatus string

const (
	ReleaseIntegrityUnknown  ReleaseIntegrityStatus = "unknown"
	ReleaseIntegrityVerified ReleaseIntegrityStatus = "verified"
	ReleaseIntegrityFailed   ReleaseIntegrityStatus = "failed"
)

type ReleaseCompatibilityStatus string

const (
	ReleaseCompatCompatible   ReleaseCompatibilityStatus = "compatible"
	ReleaseCompatIncompatible ReleaseCompatibilityStatus = "incompatible"
	ReleaseCompatUnknown      ReleaseCompatibilityStatus = "unknown"
)

func IsInstallable(lifecycle string, integrity string, compatibility string) bool {
	return lifecycle == string(ReleaseLifecycleReady) &&
		integrity == string(ReleaseIntegrityVerified) &&
		compatibility == string(ReleaseCompatCompatible)
}

var _ = time.Now
