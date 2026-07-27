package update

import (
	"time"
)

type RollbackStatus string

const (
	RollbackStatusCreated          RollbackStatus = "created"
	RollbackStatusPlanning         RollbackStatus = "planning"
	RollbackStatusStoppingTraffic  RollbackStatus = "stopping_traffic"
	RollbackStatusDrainingNew      RollbackStatus = "draining_new"
	RollbackStatusRestoringData    RollbackStatus = "restoring_data"
	RollbackStatusRestoringGen     RollbackStatus = "restoring_generation"
	RollbackStatusRestoringContrib RollbackStatus = "restoring_contribution"
	RollbackStatusRestoringPerm    RollbackStatus = "restoring_permission"
	RollbackStatusRestoringUI      RollbackStatus = "restoring_ui"
	RollbackStatusRestoringBG      RollbackStatus = "restoring_background"
	RollbackStatusValidating       RollbackStatus = "validating"
	RollbackStatusCompleted        RollbackStatus = "completed"
	RollbackStatusPartial          RollbackStatus = "partial_rollback"
	RollbackStatusFailed           RollbackStatus = "failed"
	RollbackStatusManualIntervention RollbackStatus = "manual_intervention"
)

type RollbackCondition struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail,omitempty"`
}

type ArtifactRollbackPlan struct {
	OldArtifactPath   string `json:"old_artifact_path"`
	OldPackageHash    string `json:"old_package_hash"`
	OldSignatureKeyID string `json:"old_signature_key_id"`
	NewArtifactPath   string `json:"new_artifact_path"`
	CleanupNew        bool   `json:"cleanup_new"`
}

type DefinitionRollbackPlan struct {
	OldDefinitionHash  string `json:"old_definition_hash"`
	OldManifestJSON    string `json:"old_manifest_json"`
	RestoreModules     bool   `json:"restore_modules"`
	RestoreContributions bool `json:"restore_contributions"`
}

type RuntimeRollbackPlan struct {
	OldRuntimeTypeID string   `json:"old_runtime_type_id"`
	OldRuntimeHash   string   `json:"old_runtime_hash"`
	StopNewFirst     bool     `json:"stop_new_first"`
	RestartOld       bool     `json:"restart_old"`
}

type ContributionRollbackPlan struct {
	ContributionsToRestore []string `json:"contributions_to_restore"`
	ContributionsToDisable []string `json:"contributions_to_disable"`
	RestoreAll             bool     `json:"restore_all"`
}

type DataRollbackPlan struct {
	SnapshotID         string `json:"snapshot_id"`
	ReverseMigrationID string `json:"reverse_migration_id,omitempty"`
	RequiresReverse    bool   `json:"requires_reverse"`
	RequiresSnapshot   bool   `json:"requires_snapshot"`
	StopWritesFirst    bool   `json:"stop_writes_first"`
}

type PermissionRollbackPlan struct {
	GrantsToRevoke []string `json:"grants_to_revoke"`
	GrantsToRestore []string `json:"grants_to_restore"`
	RecomputeFromOld bool   `json:"recompute_from_old"`
}

type ScopeRollbackPlan struct {
	BindingsToRevoke  []string `json:"bindings_to_revoke"`
	BindingsToRestore []string `json:"bindings_to_restore"`
	RecomputeFromOld  bool     `json:"recompute_from_old"`
}

type DesktopRollbackPlan struct {
	CloseNewUI         bool     `json:"close_new_ui"`
	RestoreOldSnapshot bool     `json:"restore_old_snapshot"`
	UnregisterNewShortcut bool  `json:"unregister_new_shortcut"`
	RestoreOldShortcut bool     `json:"restore_old_shortcut"`
	ShortcutIDs       []string `json:"shortcut_ids,omitempty"`
}

type UIRollbackPlan struct {
	CloseNewSessions   bool     `json:"close_new_sessions"`
	RevokeNewBridge    bool     `json:"revoke_new_bridge"`
	RestoreOldContrib  bool     `json:"restore_old_contrib"`
	RestoreOldSnapshot bool     `json:"restore_old_snapshot"`
}

type BackgroundRollbackPlan struct {
	TransferSchedule    bool `json:"transfer_schedule"`
	TransferEventSub    bool `json:"transfer_event_sub"`
	TransferHook        bool `json:"transfer_hook"`
	TransferMCP         bool `json:"transfer_mcp"`
	TransferTrustedSvc  bool `json:"transfer_trusted_service"`
	UseOwnershipLease   bool `json:"use_ownership_lease"`
	UseGenerationGate   bool `json:"use_generation_gate"`
}

type SideEffectRollbackPlan struct {
	Assessments     []SideEffectAssessment `json:"assessments"`
	HasNonReversible bool                  `json:"has_non_reversible"`
	RequiresManual   bool                  `json:"requires_manual"`
	PartialRollback  bool                  `json:"partial_rollback"`
}

type SideEffectAssessment struct {
	ContributionID  string `json:"contribution_id"`
	SideEffectClass string `json:"side_effect_class"`
	Reversibility   string `json:"reversibility"`
	CanCompensate   bool   `json:"can_compensate"`
	CompensationAction string `json:"compensation_action,omitempty"`
	Evidence        string `json:"evidence,omitempty"`
}

type RollbackPlan struct {
	RollbackID      string               `json:"rollback_id"`
	OperationID     string               `json:"operation_id"`
	ExtensionID     string               `json:"extension_id"`
	FromGeneration  int64                `json:"from_generation"`
	ToGeneration    int64                `json:"to_generation"`
	Level           RollbackLevel        `json:"level"`
	ArtifactPlan    ArtifactRollbackPlan `json:"artifact_plan"`
	DefinitionPlan  DefinitionRollbackPlan `json:"definition_plan"`
	RuntimePlan     RuntimeRollbackPlan  `json:"runtime_plan"`
	ContributionPlan ContributionRollbackPlan `json:"contribution_plan"`
	DataPlan        DataRollbackPlan     `json:"data_plan"`
	PermissionPlan  PermissionRollbackPlan `json:"permission_plan"`
	ScopePlan       ScopeRollbackPlan    `json:"scope_plan"`
	DesktopPlan     DesktopRollbackPlan  `json:"desktop_plan"`
	UIPlan          UIRollbackPlan       `json:"ui_plan"`
	BackgroundPlan  BackgroundRollbackPlan `json:"background_plan"`
	SideEffectPlan  SideEffectRollbackPlan `json:"side_effect_plan"`
	Preconditions   []RollbackCondition  `json:"preconditions"`
	Postconditions  []RollbackCondition  `json:"postconditions"`
	Automatic       bool                 `json:"automatic"`
	RequiresUserAction bool              `json:"requires_user_action"`
	Status          RollbackStatus       `json:"status"`
	StartedAt       *time.Time           `json:"started_at,omitempty"`
	FinishedAt      *time.Time           `json:"finished_at,omitempty"`
	ErrorCode       string               `json:"error_code,omitempty"`
	ErrorMessage    string               `json:"error_message,omitempty"`
}

type RollbackStepRecord struct {
	StepID      int           `json:"step_id"`
	RollbackID  string        `json:"rollback_id"`
	StepType    string        `json:"step_type"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	FinishedAt  *time.Time    `json:"finished_at,omitempty"`
	ErrorCode   string        `json:"error_code,omitempty"`
	ErrorMessage string       `json:"error_message,omitempty"`
}

type RollbackHealthCheck struct {
	OldRuntimeReady      bool `json:"old_runtime_ready"`
	OldContributionActive bool `json:"old_contribution_active"`
	ToolCallable         bool `json:"tool_callable"`
	UILoadable           bool `json:"ui_loadable"`
	DesktopSnapshotOK    bool `json:"desktop_snapshot_ok"`
	BackgroundUnique     bool `json:"background_unique"`
	StoragePostcondition bool `json:"storage_postcondition"`
	NoNewGenCalls        bool `json:"no_new_gen_calls"`
}

func (h *RollbackHealthCheck) AllPassed() bool {
	return h.OldRuntimeReady && h.OldContributionActive && h.ToolCallable &&
		h.UILoadable && h.DesktopSnapshotOK && h.BackgroundUnique &&
		h.StoragePostcondition && h.NoNewGenCalls
}

type RollbackFeasibility struct {
	Feasible          bool     `json:"feasible"`
	Level             RollbackLevel `json:"level"`
	Blockers          []string `json:"blockers,omitempty"`
	RequiresUserAction bool    `json:"requires_user_action"`
	RequiresDataRestore bool   `json:"requires_data_restore"`
	RequiresReverse    bool    `json:"requires_reverse"`
	HasNonReversibleSE bool    `json:"has_non_reversible_side_effect"`
}

type JournalStepType string

const (
	JournalStepMigrationPlan      JournalStepType = "migration_plan"
	JournalStepSnapshotCreate     JournalStepType = "snapshot_create"
	JournalStepMigrationExecute   JournalStepType = "migration_execute"
	JournalStepMigrationValidate  JournalStepType = "migration_validate"
	JournalStepCanaryStart        JournalStepType = "canary_start"
	JournalStepCanaryStageAdvance JournalStepType = "canary_stage_advance"
	JournalStepCanaryAbort        JournalStepType = "canary_abort"
	JournalStepGenerationSwitch   JournalStepType = "generation_switch"
	JournalStepRollbackPlan       JournalStepType = "rollback_plan"
	JournalStepRollbackExecute    JournalStepType = "rollback_execute"
	JournalStepRollbackValidate   JournalStepType = "rollback_validate"
	JournalStepRollbackCommit     JournalStepType = "rollback_commit"
	JournalStepDataRestore        JournalStepType = "data_restore"
	JournalStepReverseMigration   JournalStepType = "reverse_migration"
	JournalStepPermissionRestore  JournalStepType = "permission_restore"
	JournalStepScopeRestore       JournalStepType = "scope_restore"
	JournalStepUIRestore          JournalStepType = "ui_restore"
	JournalStepBackgroundTransfer JournalStepType = "background_transfer"
	JournalStepResourceCleanup    JournalStepType = "resource_cleanup"
)

type JournalStepStatus string

const (
	JournalStatusStarted   JournalStepStatus = "started"
	JournalStatusCompleted JournalStepStatus = "completed"
	JournalStatusFailed    JournalStepStatus = "failed"
	JournalStatusSkipped   JournalStepStatus = "skipped"
)

type CompensationDefinition struct {
	Action      string `json:"action"`
	Target      string `json:"target"`
	Reversible  bool   `json:"reversible"`
}

type LifecycleJournalEntry struct {
	EntryID      string                 `json:"entry_id"`
	OperationID  string                 `json:"operation_id"`
	StepID       string                 `json:"step_id"`
	StepType     JournalStepType        `json:"step_type"`
	Status       JournalStepStatus      `json:"status"`
	InputHash    string                 `json:"input_hash,omitempty"`
	OutputHash   string                 `json:"output_hash,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	FinishedAt   *time.Time             `json:"finished_at,omitempty"`
	Compensation *CompensationDefinition `json:"compensation,omitempty"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

type UpdateState string

const (
	UpdateStateCreated              UpdateState = "created"
	UpdateStatePreflight            UpdateState = "preflight"
	UpdateStateSnapshotting         UpdateState = "snapshotting"
	UpdateStateMigrating            UpdateState = "migrating"
	UpdateStateValidatingMigration  UpdateState = "validating_migration"
	UpdateStateShadow               UpdateState = "shadow"
	UpdateStateCanary               UpdateState = "canary"
	UpdateStateLimited              UpdateState = "limited"
	UpdateStateExpanded             UpdateState = "expanded"
	UpdateStateFull                 UpdateState = "full"
	UpdateStateCommitting           UpdateState = "committing"
	UpdateStateCompleted            UpdateState = "completed"
	UpdateStateAborting             UpdateState = "aborting"
	UpdateStateRollbackPlanning     UpdateState = "rollback_planning"
	UpdateStateRollingBack          UpdateState = "rolling_back"
	UpdateStateRollbackValidating   UpdateState = "rollback_validating"
	UpdateStateRolledBack           UpdateState = "rolled_back"
	UpdateStatePartialRollback      UpdateState = "partial_rollback"
	UpdateStateFailed               UpdateState = "failed"
	UpdateStateRecoveryRequired     UpdateState = "recovery_required"
	UpdateStateManualIntervention   UpdateState = "manual_intervention"
	UpdateStatePausedShadow         UpdateState = "paused_shadow"
	UpdateStatePausedCanary         UpdateState = "paused_canary"
	UpdateStatePausedLimited        UpdateState = "paused_limited"
	UpdateStatePausedExpanded       UpdateState = "paused_expanded"
)

var validUpdateTransitions = map[UpdateState][]UpdateState{
	UpdateStateCreated:            {UpdateStatePreflight, UpdateStateFailed},
	UpdateStatePreflight:          {UpdateStateSnapshotting, UpdateStateMigrating, UpdateStateFailed, UpdateStateManualIntervention},
	UpdateStateSnapshotting:       {UpdateStateMigrating, UpdateStateFailed},
	UpdateStateMigrating:          {UpdateStateValidatingMigration, UpdateStateFailed, UpdateStateRecoveryRequired},
	UpdateStateValidatingMigration: {UpdateStateShadow, UpdateStateCanary, UpdateStateCommitting, UpdateStateFailed, UpdateStateRecoveryRequired},
	UpdateStateShadow:             {UpdateStateCanary, UpdateStateAborting, UpdateStatePausedShadow},
	UpdateStateCanary:             {UpdateStateLimited, UpdateStateAborting, UpdateStatePausedCanary},
	UpdateStateLimited:            {UpdateStateExpanded, UpdateStateAborting, UpdateStatePausedLimited},
	UpdateStateExpanded:           {UpdateStateFull, UpdateStateAborting, UpdateStatePausedExpanded},
	UpdateStateFull:               {UpdateStateCommitting, UpdateStateAborting},
	UpdateStateCommitting:         {UpdateStateCompleted, UpdateStateFailed},
	UpdateStateCompleted:          {},
	UpdateStateAborting:           {UpdateStateRollbackPlanning, UpdateStateRolledBack, UpdateStatePartialRollback},
	UpdateStateRollbackPlanning:   {UpdateStateRollingBack, UpdateStateManualIntervention},
	UpdateStateRollingBack:        {UpdateStateRollbackValidating, UpdateStateFailed, UpdateStateRecoveryRequired},
	UpdateStateRollbackValidating: {UpdateStateRolledBack, UpdateStatePartialRollback, UpdateStateManualIntervention},
	UpdateStateRolledBack:         {},
	UpdateStatePartialRollback:    {UpdateStateManualIntervention},
	UpdateStateFailed:             {UpdateStateRecoveryRequired, UpdateStateManualIntervention},
	UpdateStateRecoveryRequired:   {UpdateStatePreflight, UpdateStateRollbackPlanning, UpdateStateManualIntervention},
	UpdateStateManualIntervention: {},
	UpdateStatePausedShadow:       {UpdateStateShadow, UpdateStateCanary, UpdateStateAborting},
	UpdateStatePausedCanary:       {UpdateStateCanary, UpdateStateLimited, UpdateStateAborting},
	UpdateStatePausedLimited:      {UpdateStateLimited, UpdateStateExpanded, UpdateStateAborting},
	UpdateStatePausedExpanded:     {UpdateStateExpanded, UpdateStateFull, UpdateStateAborting},
}

func IsValidUpdateTransition(from, to UpdateState) bool {
	allowed, ok := validUpdateTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
