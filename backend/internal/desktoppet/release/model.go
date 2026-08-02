package release

import "time"

type ReleaseBuildSnapshot struct {
	ID                     string `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID                 string `gorm:"column:user_id;type:text" json:"userId"`
	PetID                  string `gorm:"column:pet_id;type:text" json:"petId"`
	CharacterID            string `gorm:"column:character_id;type:text" json:"characterId"`
	ProcessingTaskID       string `gorm:"column:processing_task_id;type:text" json:"processingTaskId"`
	ActiveRevisionSetHash  string `gorm:"column:active_revision_set_hash;type:text" json:"activeRevisionSetHash"`
	ActiveRevisionSetJSON  string `gorm:"column:active_revision_set_json;type:text" json:"activeRevisionSetJson"`
	QualityGateID          string `gorm:"column:quality_gate_id;type:text" json:"qualityGateId"`
	QualityGateHash        string `gorm:"column:quality_gate_hash;type:text" json:"qualityGateHash"`
	QualityGateJSON        string `gorm:"column:quality_gate_json;type:text" json:"qualityGateJson"`
	DefaultActionKey       string `gorm:"column:default_action_key;type:text" json:"defaultActionKey"`
	IncludedActionsJSON    string `gorm:"column:included_actions_json;type:text;default:'[]'" json:"includedActionsJson"`
	RequiredActionsJSON    string `gorm:"column:required_actions_json;type:text;default:'[]'" json:"requiredActionsJson"`
	ExcludedActionsJSON    string `gorm:"column:excluded_actions_json;type:text;default:'[]'" json:"excludedActionsJson"`
	ActionSnapshotsJSON    string `gorm:"column:action_snapshots_json;type:text;default:'[]'" json:"actionSnapshotsJson"`
	PreviewSnapshotJSON    string `gorm:"column:preview_snapshot_json;type:text" json:"previewSnapshotJson"`
	PackageSchemaVersion   int    `gorm:"column:package_schema_version;type:integer;default:2" json:"packageSchemaVersion"`
	PackageContractHash    string `gorm:"column:package_contract_hash;type:text" json:"packageContractHash"`
	RuntimeContractVersion string `gorm:"column:runtime_contract_version;type:text" json:"runtimeContractVersion"`
	BuildConfigHash        string `gorm:"column:build_config_hash;type:text" json:"buildConfigHash"`
	InputHash              string `gorm:"column:input_hash;type:text" json:"inputHash"`
	SnapshotHash           string `gorm:"column:snapshot_hash;type:text" json:"snapshotHash"`
	CreatedAt              string `gorm:"column:created_at;type:text" json:"createdAt"`
}

func (ReleaseBuildSnapshot) TableName() string { return "desktop_pet_release_build_snapshots" }

type ReleaseActionSnapshot struct {
	ActionKey           string   `json:"actionKey"`
	ActionRevisionID    string   `json:"actionRevisionId"`
	ContentHash         string   `json:"contentHash"`
	ActionConfigHash    string   `json:"actionConfigHash"`
	FrameSetHash        string   `json:"frameSetHash"`
	FrameArtifactIDs    []string `json:"frameArtifactIds"`
	QualityEvaluationID string   `json:"qualityEvaluationId"`
	QualityVerdict      string   `json:"qualityVerdict"`
	BindingRevision     int64    `json:"bindingRevision"`
	ActionConfigJSON    string   `json:"actionConfigJson"`
	QualityResultHash   string   `json:"qualityResultHash"`
}

type ReleaseBuildOperation struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	UserID           string `gorm:"column:user_id;type:text" json:"userId"`
	PetID            string `gorm:"column:pet_id;type:text" json:"petId"`
	SnapshotID       string `gorm:"column:snapshot_id;type:text" json:"snapshotId"`
	ReleaseID        string `gorm:"column:release_id;type:text" json:"releaseId"`
	IdempotencyKey   string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	ExecutionID      string `gorm:"column:execution_id;type:text;default:''" json:"executionId"`
	InputHash        string `gorm:"column:input_hash;type:text" json:"inputHash"`
	State            string `gorm:"column:state;type:text;default:'created'" json:"state"`
	Stage            string `gorm:"column:stage;type:text;default:''" json:"stage"`
	LeaseOwner       string `gorm:"column:lease_owner;type:text;default:''" json:"leaseOwner"`
	LeaseExpiresAt   string `gorm:"column:lease_expires_at;type:text;default:''" json:"leaseExpiresAt"`
	HeartbeatAt      string `gorm:"column:heartbeat_at;type:text;default:''" json:"heartbeatAt"`
	StagingPathKey   string `gorm:"column:staging_path_key;type:text" json:"stagingPathKey"`
	PublishedPathKey string `gorm:"column:published_path_key;type:text" json:"publishedPathKey"`
	ErrorCode        string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage     string `gorm:"column:error_message;type:text" json:"errorMessage"`
	RetryCount       int    `gorm:"column:retry_count;type:integer;default:0" json:"retryCount"`
	ResultJSON       string `gorm:"column:result_json;type:text;default:'{}'" json:"resultJson"`
	StartedAt        string `gorm:"column:started_at;type:text" json:"startedAt"`
	UpdatedAt        string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt      string `gorm:"column:completed_at;type:text" json:"completedAt"`
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
	BuildOpStageSnapshotCreated   = "snapshot_created"
	BuildOpStageStagingBuilt      = "staging_built"
	BuildOpStageValidated         = "validated"
	BuildOpStageFilesPublished    = "files_published"
	BuildOpStageDatabaseCommitted = "database_committed"
)

type ReleasePublishJournal struct {
	ID              string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationID     string `gorm:"column:operation_id;type:text" json:"operationId"`
	ReleaseID       string `gorm:"column:release_id;type:text" json:"releaseId"`
	PetID           string `gorm:"column:pet_id;type:text" json:"petId"`
	Stage           string `gorm:"column:stage;type:text;default:'snapshot_created'" json:"stage"`
	ContentRootHash string `gorm:"column:content_root_hash;type:text" json:"contentRootHash"`
	StagingPath     string `gorm:"column:staging_path;type:text" json:"stagingPath"`
	PublishedPath   string `gorm:"column:published_path;type:text" json:"publishedPath"`
	ErrorMessage    string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt       string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ReleasePublishJournal) TableName() string { return "desktop_pet_release_publish_journals" }

const (
	JournalStageSnapshotCreated   = "snapshot_created"
	JournalStageStagingBuilt      = "staging_built"
	JournalStageValidated         = "validated"
	JournalStageFilesPublished    = "files_published"
	JournalStageDatabaseCommitted = "database_committed"
	JournalStageCompleted         = "completed"
	JournalStageFailed            = "failed"
)

type LegacyPackageMapping struct {
	ID                   string `gorm:"column:id;primaryKey;type:text" json:"id"`
	LegacyPackageID      string `gorm:"column:legacy_package_id;type:text" json:"legacyPackageId"`
	MigratedPetID        string `gorm:"column:migrated_pet_id;type:text" json:"migratedPetId"`
	MigratedReleaseID    string `gorm:"column:migrated_release_id;type:text" json:"migratedReleaseId"`
	MigrationStatus      string `gorm:"column:migration_status;type:text;default:'pending'" json:"migrationStatus"`
	SourceContentHash    string `gorm:"column:source_content_hash;type:text" json:"sourceContentHash"`
	OwnerUserId          string `gorm:"column:owner_user_id;type:text" json:"ownerUserId"`
	SourceManifestHash   string `gorm:"column:source_manifest_hash;type:text" json:"sourceManifestHash"`
	MigrationOperationId string `gorm:"column:migration_operation_id;type:text" json:"migrationOperationId"`
	ErrorMessage         string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt            string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at;type:text" json:"updatedAt"`
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
	ReleaseLifecycleBuilding  ReleaseLifecycle = "building"
	ReleaseLifecycleReady     ReleaseLifecycle = "ready"
	ReleaseLifecycleFailed    ReleaseLifecycle = "failed"
	ReleaseLifecycleArchived  ReleaseLifecycle = "archived"
	ReleaseLifecycleRevoked   ReleaseLifecycle = "revoked"
	ReleaseLifecycleCorrupted ReleaseLifecycle = "corrupted"
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

type ReleaseValidationReport struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ReleaseID        string `gorm:"column:release_id;type:text" json:"releaseId"`
	OperationID      string `gorm:"column:operation_id;type:text" json:"operationId"`
	SnapshotID       string `gorm:"column:snapshot_id;type:text" json:"snapshotId"`
	Source           string `gorm:"column:source;type:text;default:'build'" json:"source"`
	ValidatorVersion string `gorm:"column:validator_version;type:text;default:'2.0'" json:"validatorVersion"`
	Verdict          string `gorm:"column:verdict;type:text;default:'pending'" json:"verdict"`
	FindingsJSON     string `gorm:"column:findings_json;type:text;default:'[]'" json:"findingsJson"`
	FileCount        int    `gorm:"column:file_count;type:integer;default:0" json:"fileCount"`
	ErrorCount       int    `gorm:"column:error_count;type:integer;default:0" json:"errorCount"`
	WarningCount     int    `gorm:"column:warning_count;type:integer;default:0" json:"warningCount"`
	ManifestHash     string `gorm:"column:manifest_hash;type:text" json:"manifestHash"`
	ContentRootHash  string `gorm:"column:content_root_hash;type:text" json:"contentRootHash"`
	ArchiveHash      string `gorm:"column:archive_hash;type:text" json:"archiveHash"`
	CreatedAt        string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ReleaseValidationReport) TableName() string { return "desktop_pet_release_validation_reports" }

type ReleaseEventOutbox struct {
	ID                string `gorm:"column:id;primaryKey;type:text" json:"id"`
	EventID           string `gorm:"column:event_id;type:text" json:"eventId"`
	EventType         string `gorm:"column:event_type;type:text" json:"eventType"`
	AggregateType     string `gorm:"column:aggregate_type;type:text;default:'release'" json:"aggregateType"`
	AggregateID       string `gorm:"column:aggregate_id;type:text" json:"aggregateId"`
	AggregateSequence int64  `gorm:"column:aggregate_sequence;type:integer;default:0" json:"aggregateSequence"`
	PayloadJSON       string `gorm:"column:payload_json;type:text" json:"payloadJson"`
	PayloadHash       string `gorm:"column:payload_hash;type:text" json:"payloadHash"`
	Status            string `gorm:"column:status;type:text;default:'pending'" json:"status"`
	AttemptCount      int    `gorm:"column:attempt_count;type:integer;default:0" json:"attemptCount"`
	AvailableAt       string `gorm:"column:available_at;type:text" json:"availableAt"`
	LastError         string `gorm:"column:last_error;type:text" json:"lastError"`
	CreatedAt         string `gorm:"column:created_at;type:text" json:"createdAt"`
	PublishedAt       string `gorm:"column:published_at;type:text" json:"publishedAt"`
}

func (ReleaseEventOutbox) TableName() string { return "desktop_pet_release_event_outbox" }

type ReleaseBuildRequestInbox struct {
	ID             string `gorm:"column:id;primaryKey;type:text" json:"id"`
	RequestID      string `gorm:"column:request_id;type:text" json:"requestId"`
	UserID         string `gorm:"column:user_id;type:text" json:"userId"`
	IdempotencyKey string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	InputHash      string `gorm:"column:input_hash;type:text" json:"inputHash"`
	PayloadJSON    string `gorm:"column:payload_json;type:text" json:"payloadJson"`
	PayloadHash    string `gorm:"column:payload_hash;type:text" json:"payloadHash"`
	Status         string `gorm:"column:status;type:text;default:'pending'" json:"status"`
	OperationID    string `gorm:"column:operation_id;type:text" json:"operationId"`
	CreatedAt      string `gorm:"column:created_at;type:text" json:"createdAt"`
	ProcessedAt    string `gorm:"column:processed_at;type:text" json:"processedAt"`
	LastError      string `gorm:"column:last_error;type:text" json:"lastError"`
}

func (ReleaseBuildRequestInbox) TableName() string { return "desktop_pet_release_build_request_inbox" }

type ImportPackageSnapshot struct {
	ID                    string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ImportStagingID       string `gorm:"column:import_staging_id;type:text" json:"importStagingId"`
	SourcePackageHash     string `gorm:"column:source_package_hash;type:text" json:"sourcePackageHash"`
	SourceManifestHash    string `gorm:"column:source_manifest_hash;type:text" json:"sourceManifestHash"`
	SourceSchemaVersion   int    `gorm:"column:source_schema_version;type:integer;default:0" json:"sourceSchemaVersion"`
	NormalizationWarnings string `gorm:"column:normalization_warnings;type:text" json:"normalizationWarnings"`
	SelectedActionsJSON   string `gorm:"column:selected_actions_json;type:text;default:'[]'" json:"selectedActionsJson"`
	BindingDecision       string `gorm:"column:binding_decision;type:text" json:"bindingDecision"`
	LicenseDecision       string `gorm:"column:license_decision;type:text" json:"licenseDecision"`
	RuntimeCompatibility  string `gorm:"column:runtime_compatibility;type:text" json:"runtimeCompatibility"`
	UserID                string `gorm:"column:user_id;type:text" json:"userId"`
	PetID                 string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID             string `gorm:"column:release_id;type:text" json:"releaseId"`
	CreatedAt             string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ImportPackageSnapshot) TableName() string { return "desktop_pet_import_package_snapshots" }

type ReleaseFrameSnapshot struct {
	FrameID         string
	LogicalIndex    int
	AssetID         string
	ContentHash     string
	DurationMS      int
	MIME            string
	Width           int
	Height          int
	ByteSize        int64
	StorageKey      string
	MaskAssetID     string
	MaskContentHash string
	TransformHash   string
	MeasurementHash string
}

type ReleasePreviewSnapshot struct {
	ArtifactID  string
	StorageKey  string
	ContentHash string
	MIME        string
	Width       int
	Height      int
	ByteSize    int64
}

type ReleaseData struct {
	ID                    string
	PetID                 string
	OwnerUserID           string
	Version               string
	ReleaseSequence       int
	SchemaVersion         int
	Lifecycle             string
	ContentRootHash       string
	ManifestHash          string
	StorageKey            string
	ArchiveStorageKey     string
	TotalBytes            int64
	FileCount             int
	ActionCount           int
	DefaultActionKey      string
	MinRuntimeVersion     string
	SourceType            string
	SourceProcessingTask  string
	SourceGenerationTask  string
	ActiveRevisionSetHash string
	QualityGateID         string
	QualityGateHash       string
	EvaluationSetHash     string
	BuildSnapshotID       string
	IntegrityStatus       string
	CompatibilityStatus   string
	ManifestJSON          string
	PublishedAt           string
	ArchiveHash           string
	ArchiveBytes          int64
	LifecycleRevision     int
	IntegrityRevision     int
	ArchivedAt            string
	RevokedAt             string
	RevocationReason      string
	LegacyPackageID       string
	LegacyVersion         int
	CreatedAt             string
	UpdatedAt             string
}

type ReleaseFileData struct {
	ID        string
	ReleaseID string
	Path      string
	SHA256    string
	Bytes     int64
	MediaType string
	Role      string
	ActionKey string
	FrameID   string
	CreatedAt string
}

var _ = time.Now
