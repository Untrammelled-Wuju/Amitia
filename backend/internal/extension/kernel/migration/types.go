package migration

import (
	"encoding/json"
	"time"
)

type MigrationType string

const (
	MigrationTypeSchema        MigrationType = "schema"
	MigrationTypeData          MigrationType = "data"
	MigrationTypeConfiguration MigrationType = "configuration"
	MigrationTypeResourceIndex MigrationType = "resource_index"
	MigrationTypeCacheRebuild  MigrationType = "cache_rebuild"
	MigrationTypeStorageLayout MigrationType = "storage_layout"
)

type MigrationDirection string

const (
	DirectionForward MigrationDirection = "forward"
	DirectionReverse MigrationDirection = "reverse"
	DirectionRepair  MigrationDirection = "repair"
)

type Reversibility string

const (
	ReversibilityFullyReversible       Reversibility = "fully_reversible"
	ReversibilitySnapshotReversible    Reversibility = "snapshot_reversible"
	ReversibilityReverseScriptRequired Reversibility = "reverse_script_required"
	ReversibilityIrreversible          Reversibility = "irreversible"
)

type Idempotency string

const (
	IdempotencyIdempotent           Idempotency = "idempotent"
	IdempotencyCheckpointIdempotent Idempotency = "checkpoint_idempotent"
	IdempotencyNonIdempotent        Idempotency = "non_idempotent"
)

type DataDomain struct {
	Domain    string `json:"domain"`
	Storage   string `json:"storage"`
	Namespace string `json:"namespace"`
}

type MigrationCondition struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Expected json.RawMessage `json:"expected"`
	Actual   json.RawMessage `json:"actual,omitempty"`
}

type PermissionRequirement struct {
	PermissionID string `json:"permission_id"`
	Scope        string `json:"scope"`
}

type ScopeRule struct {
	BindingType string   `json:"binding_type"`
	ModuleIDs   []string `json:"module_ids"`
	Partitions  []string `json:"partitions"`
}

type TaskResourceLimits struct {
	MaxMemoryMB     int64 `json:"max_memory_mb"`
	MaxCPUPercent   int   `json:"max_cpu_percent"`
	MaxDiskMB       int64 `json:"max_disk_mb"`
	MaxDurationSecs int   `json:"max_duration_secs"`
}

type MigrationDefinition struct {
	MigrationID string `json:"migration_id"`
	ExtensionID string `json:"extension_id"`
	ModuleID    string `json:"module_id"`

	FromVersionRange string `json:"from_version_range"`
	ToVersion        string `json:"to_version"`

	Entry       string             `json:"entry"`
	RuntimeType string             `json:"runtime_type"`
	Direction   MigrationDirection `json:"direction"`

	DataDomains      []DataDomain    `json:"data_domains"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	CheckpointSchema json.RawMessage `json:"checkpoint_schema"`

	Idempotency   Idempotency   `json:"idempotency"`
	Reversibility Reversibility `json:"reversibility"`

	ForwardMigrationID *string `json:"forward_migration_id,omitempty"`
	ReverseMigrationID *string `json:"reverse_migration_id,omitempty"`

	Precondition  []MigrationCondition `json:"precondition"`
	Postcondition []MigrationCondition `json:"postcondition"`

	PermissionRequirements []PermissionRequirement `json:"permission_requirements"`
	ScopeRule              ScopeRule               `json:"scope_rule"`
	ResourceLimits         TaskResourceLimits      `json:"resource_limits"`

	DefinitionHash string `json:"definition_hash"`
}

type MigrationNodeID string

type MigrationNode struct {
	NodeID      MigrationNodeID   `json:"node_id"`
	MigrationID string            `json:"migration_id"`
	Stage       string            `json:"stage"`
	DependsOn   []MigrationNodeID `json:"depends_on"`
}

type MigrationEdge struct {
	From MigrationNodeID `json:"from"`
	To   MigrationNodeID `json:"to"`
	Type string          `json:"type"`
}

type MigrationGraph struct {
	ExtensionID string          `json:"extension_id"`
	Nodes       []MigrationNode `json:"nodes"`
	Edges       []MigrationEdge `json:"edges"`
}

type MigrationPathStep struct {
	StepID      int                `json:"step_id"`
	NodeID      MigrationNodeID    `json:"node_id"`
	MigrationID string             `json:"migration_id"`
	FromVersion string             `json:"from_version"`
	ToVersion   string             `json:"to_version"`
	Direction   MigrationDirection `json:"direction"`
}

type MigrationPath struct {
	Steps       []MigrationPathStep `json:"steps"`
	FromVersion string              `json:"from_version"`
	ToVersion   string              `json:"to_version"`
	IsDirect    bool                `json:"is_direct"`
}

type MigrationOperationStatus string

const (
	OperationStatusCreated            MigrationOperationStatus = "created"
	OperationStatusSnapshotting       MigrationOperationStatus = "snapshotting"
	OperationStatusMigrating          MigrationOperationStatus = "migrating"
	OperationStatusValidating         MigrationOperationStatus = "validating"
	OperationStatusCompleted          MigrationOperationStatus = "completed"
	OperationStatusFailed             MigrationOperationStatus = "failed"
	OperationStatusCancelled          MigrationOperationStatus = "cancelled"
	OperationStatusRecoveryRequired   MigrationOperationStatus = "recovery_required"
	OperationStatusManualIntervention MigrationOperationStatus = "manual_intervention"
)

type MigrationOperation struct {
	OperationID         string                   `json:"operation_id"`
	ExtensionID         string                   `json:"extension_id"`
	FromVersion         string                   `json:"from_version"`
	ToVersion           string                   `json:"to_version"`
	FromDefinitionHash  string                   `json:"from_definition_hash"`
	ToDefinitionHash    string                   `json:"to_definition_hash"`
	MigrationPath       MigrationPath            `json:"migration_path"`
	SnapshotID          string                   `json:"snapshot_id"`
	Status              MigrationOperationStatus `json:"status"`
	CurrentStep         int                      `json:"current_step"`
	CheckpointID        string                   `json:"checkpoint_id"`
	TaskRunID           string                   `json:"task_run_id"`
	StartedAt           time.Time                `json:"started_at"`
	FinishedAt          *time.Time               `json:"finished_at,omitempty"`
	ErrorCode           string                   `json:"error_code,omitempty"`
	ErrorMessage        string                   `json:"error_message,omitempty"`
	Reversibility       Reversibility            `json:"reversibility"`
	RequiresUserConfirm bool                     `json:"requires_user_confirm"`
	UserConfirmed       bool                     `json:"user_confirmed"`
}

type MigrationStepRecord struct {
	StepID       int        `json:"step_id"`
	OperationID  string     `json:"operation_id"`
	MigrationID  string     `json:"migration_id"`
	Status       string     `json:"status"`
	InputHash    string     `json:"input_hash"`
	OutputHash   string     `json:"output_hash"`
	CheckpointID string     `json:"checkpoint_id,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

type MigrationCheckpoint struct {
	CheckpointID   string          `json:"checkpoint_id"`
	OperationID    string          `json:"operation_id"`
	StepID         int             `json:"step_id"`
	MigrationID    string          `json:"migration_id"`
	Stage          string          `json:"stage"`
	Cursor         json.RawMessage `json:"cursor"`
	BatchNumber    int             `json:"batch_number"`
	ProcessedCount int             `json:"processed_count"`
	InputHash      string          `json:"input_hash"`
	DefinitionHash string          `json:"definition_hash"`
	SnapshotID     string          `json:"snapshot_id"`
	CreatedAt      time.Time       `json:"created_at"`
}

type MigrationPlanInput struct {
	ExtensionID         string
	FromVersion         string
	ToVersion           string
	FromDefinitionHash  string
	ToDefinitionHash    string
	AvailableMigrations []MigrationDefinition
	Platform            string
	HostVersion         string
}

type MigrationPlanOutput struct {
	Path                MigrationPath `json:"path"`
	SnapshotScope       []string      `json:"snapshot_scope"`
	EstimatedRisk       string        `json:"estimated_risk"`
	EstimatedSpaceBytes int64         `json:"estimated_space_bytes"`
	CanAutoRollback     bool          `json:"can_auto_rollback"`
	RequiresUserConfirm bool          `json:"requires_user_confirm"`
	HasIrreversible     bool          `json:"has_irreversible"`
	Reversibility       Reversibility `json:"reversibility"`
}

type ValidationResult struct {
	Passed   bool     `json:"passed"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type ValidationCheck struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Passed bool            `json:"passed"`
	Detail json.RawMessage `json:"detail,omitempty"`
}

type SnapshotEntryType string

const (
	SnapshotEntrySQLite    SnapshotEntryType = "sqlite"
	SnapshotEntryFile      SnapshotEntryType = "file"
	SnapshotEntryDirectory SnapshotEntryType = "directory"
)

type SnapshotEntry struct {
	EntryID    string            `json:"entry_id"`
	SnapshotID string            `json:"snapshot_id"`
	Type       SnapshotEntryType `json:"type"`
	SourcePath string            `json:"source_path"`
	SnapPath   string            `json:"snap_path"`
	SizeBytes  int64             `json:"size_bytes"`
	Hash       string            `json:"hash"`
	PageCount  int               `json:"page_count,omitempty"`
	WALHandled bool              `json:"wal_handled,omitempty"`
}

type SnapshotManifest struct {
	SnapshotID      string          `json:"snapshot_id"`
	ExtensionID     string          `json:"extension_id"`
	OperationID     string          `json:"operation_id"`
	Generation      int64           `json:"generation"`
	Entries         []SnapshotEntry `json:"entries"`
	TotalBytes      int64           `json:"total_bytes"`
	ManifestHash    string          `json:"manifest_hash"`
	CreatedAt       time.Time       `json:"created_at"`
	RetentionPolicy string          `json:"retention_policy"`
}

type SpaceEstimate struct {
	CurrentDataBytes   int64 `json:"current_data_bytes"`
	TargetStagingBytes int64 `json:"target_staging_bytes"`
	SnapshotBytes      int64 `json:"snapshot_bytes"`
	TemporaryBytes     int64 `json:"temporary_bytes"`
	SafetyMarginBytes  int64 `json:"safety_margin_bytes"`
	TotalRequired      int64 `json:"total_required"`
	AvailableBytes     int64 `json:"available_bytes"`
	Sufficient         bool  `json:"sufficient"`
}

type ForbiddenMigrationDomain string

const (
	ForbiddenHostDatabase    ForbiddenMigrationDomain = "host_database"
	ForbiddenHostUserAccount ForbiddenMigrationDomain = "host_user_account"
	ForbiddenPermissionStore ForbiddenMigrationDomain = "permission_store"
	ForbiddenSecretStore     ForbiddenMigrationDomain = "secret_store"
	ForbiddenSystemConfig    ForbiddenMigrationDomain = "system_configuration"
	ForbiddenElectronConfig  ForbiddenMigrationDomain = "electron_configuration"
)

var ForbiddenDomains = map[ForbiddenMigrationDomain]bool{
	ForbiddenHostDatabase:    true,
	ForbiddenHostUserAccount: true,
	ForbiddenPermissionStore: true,
	ForbiddenSecretStore:     true,
	ForbiddenSystemConfig:    true,
	ForbiddenElectronConfig:  true,
}

type WriteStrategy string

const (
	WriteStrategyOldOnly              WriteStrategy = "old_only"
	WriteStrategyNewOnly              WriteStrategy = "new_only"
	WriteStrategyDualWriteValidated   WriteStrategy = "dual_write_validated"
	WriteStrategyStagedWrite          WriteStrategy = "staged_write"
	WriteStrategyReadCompatibleShared WriteStrategy = "read_compatible_shared"
)

type SideEffectClass string

const (
	SideEffectNone          SideEffectClass = "none"
	SideEffectReadOnly      SideEffectClass = "read_only"
	SideEffectReversible    SideEffectClass = "reversible"
	SideEffectIdempotent    SideEffectClass = "idempotent"
	SideEffectNonIdempotent SideEffectClass = "non_idempotent"
	SideEffectExternal      SideEffectClass = "external"
)

type SideEffectReversibility string

const (
	SEReversible    SideEffectReversibility = "reversible"
	SECompensatable SideEffectReversibility = "compensatable"
	SEIdempotent    SideEffectReversibility = "idempotent"
	SENonReversible SideEffectReversibility = "non_reversible"
	SEUnknown       SideEffectReversibility = "unknown"
)
