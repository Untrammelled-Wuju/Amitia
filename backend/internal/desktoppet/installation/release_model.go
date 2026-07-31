package installation

type PetIdentity struct {
	ID                  string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OwnerUserID         string `gorm:"column:owner_user_id;type:text" json:"ownerUserId"`
	SourceCharacterID   string `gorm:"column:source_character_id;type:text" json:"sourceCharacterId"`
	Name                string `gorm:"column:name;type:text" json:"name"`
	Slug                string `gorm:"column:slug;type:text" json:"slug"`
	BindingPolicy       string `gorm:"column:binding_policy;type:text;default:'character_locked'" json:"bindingPolicy"`
	UpstreamPetID       string `gorm:"column:upstream_pet_id;type:text;default:''" json:"upstreamPetId"`
	NextReleaseSequence int    `gorm:"column:next_release_sequence;type:integer;default:0" json:"nextReleaseSequence"`
	DefaultActionKey    string `gorm:"column:default_action_key;type:text;default:''" json:"defaultActionKey"`
	CreatedAt           string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt           string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (PetIdentity) TableName() string { return "desktop_pet_identities" }

const (
	BindingPolicyCharacterLocked = "character_locked"
	BindingPolicyOwnerCharacters = "owner_characters"
	BindingPolicyPublic          = "public"
	BindingPolicyLegacy          = "legacy_inferred"
)

type PackageRelease struct {
	ID                    string `gorm:"column:id;primaryKey;type:text" json:"id"`
	PetID                 string `gorm:"column:pet_id;type:text" json:"petId"`
	OwnerUserID           string `gorm:"column:owner_user_id;type:text" json:"ownerUserId"`
	Version               string `gorm:"column:version;type:text" json:"version"`
	ReleaseSequence       int    `gorm:"column:release_sequence;type:integer;default:0" json:"releaseSequence"`
	SchemaVersion         int    `gorm:"column:schema_version;type:integer;default:2" json:"schemaVersion"`
	Status                string `gorm:"column:status;type:text;default:'draft'" json:"status"`
	Lifecycle             string `gorm:"column:lifecycle;type:text;default:'building'" json:"lifecycle"`
	ContentRootHash       string `gorm:"column:content_root_hash;type:text" json:"contentRootHash"`
	ManifestHash          string `gorm:"column:manifest_hash;type:text" json:"manifestHash"`
	StorageKey            string `gorm:"column:storage_key;type:text" json:"storageKey"`
	ArchiveStorageKey     string `gorm:"column:archive_storage_key;type:text" json:"archiveStorageKey"`
	TotalBytes            int64  `gorm:"column:total_bytes;type:integer;default:0" json:"totalBytes"`
	FileCount             int    `gorm:"column:file_count;type:integer;default:0" json:"fileCount"`
	ActionCount           int    `gorm:"column:action_count;type:integer;default:0" json:"actionCount"`
	DefaultActionKey      string `gorm:"column:default_action_key;type:text" json:"defaultActionKey"`
	MinRuntimeVersion     string `gorm:"column:min_runtime_version;type:text" json:"minRuntimeVersion"`
	SourceType            string `gorm:"column:source_type;type:text;default:'generated'" json:"sourceType"`
	SourceProcessingTask  string `gorm:"column:source_processing_task;type:text" json:"sourceProcessingTask"`
	SourceGenerationTask  string `gorm:"column:source_generation_task;type:text" json:"sourceGenerationTask"`
	QualityGateSnapshotID string `gorm:"column:quality_gate_snapshot_id;type:text" json:"qualityGateSnapshotId"`
	ActiveRevisionSetHash string `gorm:"column:active_revision_set_hash;type:text;default:''" json:"activeRevisionSetHash"`
	QualityGateID         string `gorm:"column:quality_gate_id;type:text;default:''" json:"qualityGateId"`
	QualityGateHash       string `gorm:"column:quality_gate_hash;type:text;default:''" json:"qualityGateHash"`
	BuildSnapshotID       string `gorm:"column:build_snapshot_id;type:text;default:''" json:"buildSnapshotId"`
	IntegrityStatus       string `gorm:"column:integrity_status;type:text;default:'unknown'" json:"integrityStatus"`
	CompatibilityStatus   string `gorm:"column:compatibility_status;type:text;default:'unknown'" json:"compatibilityStatus"`
	ManifestJSON          string `gorm:"column:manifest_json;type:text;default:'{}'" json:"manifestJson"`
	PublishedAt           string `gorm:"column:published_at;type:text" json:"publishedAt"`
	LegacyPackageID       string `gorm:"column:legacy_package_id;type:text;default:''" json:"legacyPackageId"`
	LegacyVersion         int    `gorm:"column:legacy_version;type:integer;default:0" json:"legacyVersion"`
	CreatedAt             string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt             string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (PackageRelease) TableName() string { return "desktop_pet_package_releases" }

const (
	ReleaseStatusDraft       = "draft"
	ReleaseStatusBuilding    = "building"
	ReleaseStatusValidating  = "validating"
	ReleaseStatusPublished   = "published"
	ReleaseStatusRejected    = "rejected"
	ReleaseStatusQuarantined = "quarantined"
	ReleaseStatusSuperseded  = "superseded"
	ReleaseStatusRetired     = "retired"
	ReleaseStatusDeleted     = "deleted"

	SourceTypeGenerated = "generated"
	SourceTypImported   = "imported"
	SourceTypeMigrated  = "migrated"
)

func (r *PackageRelease) IsInstallable() bool {
	return r != nil && (r.Status == ReleaseStatusPublished || r.Status == ReleaseStatusSuperseded)
}

func (r *PackageRelease) IsPublished() bool {
	return r != nil && r.Status == ReleaseStatusPublished
}

type ReleaseFile struct {
	ID        string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ReleaseID string `gorm:"column:release_id;type:text" json:"releaseId"`
	Path      string `gorm:"column:path;type:text" json:"path"`
	SHA256    string `gorm:"column:sha256;type:text" json:"sha256"`
	Bytes     int64  `gorm:"column:bytes;type:integer;default:0" json:"bytes"`
	MediaType string `gorm:"column:media_type;type:text" json:"mediaType"`
	Role      string `gorm:"column:role;type:text" json:"role"`
	ActionKey string `gorm:"column:action_key;type:text" json:"actionKey"`
	FrameID   string `gorm:"column:frame_id;type:text" json:"frameId"`
	CreatedAt string `gorm:"column:created_at;type:text" json:"createdAt"`
}

func (ReleaseFile) TableName() string { return "desktop_pet_release_files" }

type PackageOperation struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationType    string `gorm:"column:operation_type;type:text" json:"operationType"`
	UserID           string `gorm:"column:user_id;type:text" json:"userId"`
	PetID            string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID        string `gorm:"column:release_id;type:text" json:"releaseId"`
	IdempotencyKey   string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	Stage            string `gorm:"column:stage;type:text;default:'prepare'" json:"stage"`
	Status           string `gorm:"column:status;type:text;default:'pending'" json:"status"`
	InputHash        string `gorm:"column:input_hash;type:text" json:"inputHash"`
	SnapshotID       string `gorm:"column:snapshot_id;type:text;default:''" json:"snapshotId"`
	StagingPathKey   string `gorm:"column:staging_path_key;type:text" json:"stagingPathKey"`
	PublishedPathKey string `gorm:"column:published_path_key;type:text" json:"publishedPathKey"`
	LeaseOwner       string `gorm:"column:lease_owner;type:text;default:''" json:"leaseOwner"`
	LeaseExpiresAt   string `gorm:"column:lease_expires_at;type:text;default:''" json:"leaseExpiresAt"`
	HeartbeatAt      string `gorm:"column:heartbeat_at;type:text;default:''" json:"heartbeatAt"`
	ErrorCode        string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage     string `gorm:"column:error_message;type:text" json:"errorMessage"`
	RetryCount       int    `gorm:"column:retry_count;type:integer;default:0" json:"retryCount"`
	ResultJSON       string `gorm:"column:result_json;type:text;default:'{}'" json:"resultJson"`
	StartedAt        string `gorm:"column:started_at;type:text" json:"startedAt"`
	UpdatedAt        string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt      string `gorm:"column:completed_at;type:text" json:"completedAt"`
}

func (PackageOperation) TableName() string { return "desktop_pet_package_operations" }

const (
	PackageOpTypeBuild  = "build"
	PackageOpTypeImport = "import"

	OpStagePrepare      = "prepare"
	OpStageStageFiles   = "stage_files"
	OpStageVerify       = "verify"
	OpStagePublishFiles = "publish_files"
	OpStageCommitDB     = "commit_db"
	OpStageEnqueueRT    = "enqueue_runtime"
	OpStageReconcile    = "reconcile"
	OpStageCompleted     = "completed"

	OpStatusPending   = "pending"
	OpStatusRunning   = "running"
	OpStatusCompleted = "completed"
	OpStatusFailed    = "failed"
	OpStatusRecovery  = "recovery_required"
)

type InstallationOperation struct {
	ID              string `gorm:"column:id;primaryKey;type:text" json:"id"`
	OperationType   string `gorm:"column:operation_type;type:text" json:"operationType"`
	UserID          string `gorm:"column:user_id;type:text" json:"userId"`
	DeviceID        string `gorm:"column:device_id;type:text;default:''" json:"deviceId"`
	InstallationID  string `gorm:"column:installation_id;type:text" json:"installationId"`
	PetID           string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID       string `gorm:"column:release_id;type:text" json:"releaseId"`
	TargetReleaseID string `gorm:"column:target_release_id;type:text" json:"targetReleaseId"`
	IdempotencyKey  string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	RequestHash     string `gorm:"column:request_hash;type:text;default:''" json:"requestHash"`
	Stage           string `gorm:"column:stage;type:text;default:'prepare'" json:"stage"`
	Status          string `gorm:"column:status;type:text;default:'pending'" json:"status"`
	AttemptNumber   int    `gorm:"column:attempt_number;type:integer;default:0" json:"attemptNumber"`
	LeaseOwner      string `gorm:"column:lease_owner;type:text;default:''" json:"leaseOwner"`
	LeaseExpiresAt  string `gorm:"column:lease_expires_at;type:text;default:''" json:"leaseExpiresAt"`
	StagingPathKey  string `gorm:"column:staging_path_key;type:text" json:"stagingPathKey"`
	PublishedPathKey string `gorm:"column:published_path_key;type:text" json:"publishedPathKey"`
	TrashPathKey    string `gorm:"column:trash_path_key;type:text" json:"trashPathKey"`
	ErrorCode       string `gorm:"column:error_code;type:text" json:"errorCode"`
	ErrorMessage    string `gorm:"column:error_message;type:text" json:"errorMessage"`
	RetryCount      int    `gorm:"column:retry_count;type:integer;default:0" json:"retryCount"`
	StartedAt       string `gorm:"column:started_at;type:text" json:"startedAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt     string `gorm:"column:completed_at;type:text" json:"completedAt"`
}

func (InstallationOperation) TableName() string { return "desktop_pet_installation_operations" }

const (
	InstOpTypeInstall      = "install"
	InstOpTypeUpgrade      = "upgrade"
	InstOpTypeDowngrade    = "downgrade"
	InstOpTypeUninstall    = "uninstall"
	InstOpTypeSwitch       = "switch"
	InstOpTypeRepair       = "repair"
	InstOpTypeEnable       = "enable"
	InstOpTypeDisable      = "disable"
	InstOpTypeSettings      = "settings"
	InstOpTypeDefaultAction = "default_action"
	InstOpTypePlayAction    = "play_action"
	InstOpTypeRecenter      = "recenter"
)

type ActiveBinding struct {
	UserID           string `gorm:"column:user_id;primaryKey;type:text" json:"userId"`
	DeviceID         string `gorm:"column:device_id;type:text;default:''" json:"deviceId"`
	InstallationID   string `gorm:"column:installation_id;type:text" json:"installationId"`
	PetID            string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID        string `gorm:"column:release_id;type:text" json:"releaseId"`
	BindingRevision  int    `gorm:"column:binding_revision;type:integer;default:0" json:"bindingRevision"`
	BoundReason      string `gorm:"column:bound_reason;type:text;default:'install'" json:"boundReason"`
	BoundAt          string `gorm:"column:bound_at;type:text" json:"boundAt"`
	DesiredState     string `gorm:"column:desired_state;type:text;default:'disabled'" json:"desiredState"`
	RuntimeSyncState string `gorm:"column:runtime_sync_state;type:text;default:'pending'" json:"runtimeSyncState"`
	DesiredUpdatedAt string `gorm:"column:desired_updated_at;type:text" json:"desiredUpdatedAt"`
	CreatedAt        string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (ActiveBinding) TableName() string { return "desktop_pet_active_bindings" }

type InstallationReleaseHistory struct {
	ID                string `gorm:"column:id;primaryKey;type:text" json:"id"`
	InstallationID    string `gorm:"column:installation_id;type:text" json:"installationId"`
	ReleaseID         string `gorm:"column:release_id;type:text" json:"releaseId"`
	PetID             string `gorm:"column:pet_id;type:text" json:"petId"`
	Version           string `gorm:"column:version;type:text" json:"version"`
	ActivatedAt       string `gorm:"column:activated_at;type:text" json:"activatedAt"`
	DeactivatedAt     string `gorm:"column:deactivated_at;type:text" json:"deactivatedAt"`
	DeactivationReason string `gorm:"column:deactivation_reason;type:text" json:"deactivationReason"`
	IsCurrent         int    `gorm:"column:is_current;type:integer;default:0" json:"isCurrent"`
	CreatedAt         string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt         string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (InstallationReleaseHistory) TableName() string { return "desktop_pet_installation_release_history" }

type PackageValidationReport struct {
	ID           string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ReleaseID    string `gorm:"column:release_id;type:text" json:"releaseId"`
	OperationID  string `gorm:"column:operation_id;type:text" json:"operationId"`
	Source       string `gorm:"column:source;type:text;default:'build'" json:"source"`
	Verdict      string `gorm:"column:verdict;type:text;default:'pending'" json:"verdict"`
	FindingsJSON string `gorm:"column:findings_json;type:text;default:'[]'" json:"findingsJson"`
	FileCount    int    `gorm:"column:file_count;type:integer;default:0" json:"fileCount"`
	ErrorCount   int    `gorm:"column:error_count;type:integer;default:0" json:"errorCount"`
	WarningCount int    `gorm:"column:warning_count;type:integer;default:0" json:"warningCount"`
	CreatedAt    string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (PackageValidationReport) TableName() string { return "desktop_pet_package_validation_reports" }
