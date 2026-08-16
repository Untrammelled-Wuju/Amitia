package recovery

import (
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type FailureClass string

const (
	FailureProcessCrash             FailureClass = "process_crash"
	FailureRuntimeStartFailure      FailureClass = "runtime_start_failure"
	FailureRuntimeRecoveryExhausted FailureClass = "runtime_recovery_exhausted"
	FailureUpgradeFailure           FailureClass = "upgrade_failure"
	FailurePackageRollbackRequired  FailureClass = "package_rollback_required"
	FailureCheckpointMissing        FailureClass = "checkpoint_missing"
	FailureCheckpointStale          FailureClass = "checkpoint_stale"
	FailureCheckpointCorrupt        FailureClass = "checkpoint_corrupt"
	FailureCheckpointIncompatible   FailureClass = "checkpoint_incompatible"
	FailureHostRecoveryFailure      FailureClass = "host_recovery_failure"
)

type CheckpointClass string

const (
	CheckpointMissing      CheckpointClass = "missing"
	CheckpointCorrupt      CheckpointClass = "corrupt"
	CheckpointStale        CheckpointClass = "stale"
	CheckpointIncompatible CheckpointClass = "incompatible"
	CheckpointCompatible   CheckpointClass = "compatible"
)

type RecoveryLevel int

const (
	RecoveryLevelProcessRestart        RecoveryLevel = 1
	RecoveryLevelRuntimeReconstruction RecoveryLevel = 2
	RecoveryLevelPackageRollback       RecoveryLevel = 3
	RecoveryLevelQuarantine            RecoveryLevel = 4
)

type RecoveryStage string

const (
	RecoveryStageClassifying RecoveryStage = "classifying"
	RecoveryStageQuiescing   RecoveryStage = "quiescing"
	RecoveryStageRollingBack RecoveryStage = "rolling_back"
	RecoveryStageReconciling RecoveryStage = "reconciling"
	RecoveryStageRebuilding  RecoveryStage = "rebuilding"
	RecoveryStageRestarting  RecoveryStage = "restarting"
	RecoveryStageValidating  RecoveryStage = "validating"
	RecoveryStageCompleted   RecoveryStage = "completed"
	RecoveryStageFailed      RecoveryStage = "failed"
)

type RecoveryOperationID string

type RecoveryOperation struct {
	OperationID  RecoveryOperationID
	RuntimeID    domain.RuntimeInstanceID
	ExtensionID  string
	PluginID     domain.PluginID
	FailureClass FailureClass
	Level        RecoveryLevel
	Stage        RecoveryStage
	Attempt      int
	MaxAttempts  int
	StartedAt    time.Time
	CompletedAt  *time.Time
	Result       RecoveryResult
	Checkpoint   *CheckpointInfo
	Error        string
}

type CheckpointInfo struct {
	Class         CheckpointClass
	RuntimeID     domain.RuntimeInstanceID
	PluginID      domain.PluginID
	ExtensionID   string
	Revision      string
	CleanShutdown bool
	CanRebuild    bool
}

type RecoveryResult struct {
	Success         bool
	RequiresRebuild bool
	RequiresRestart bool
	NewLease        bool
	NewConnection   bool
	Quarantined     bool
	Error           string
}

type RuntimeFailureEvent struct {
	RuntimeID      domain.RuntimeInstanceID
	PluginID       domain.PluginID
	ExtensionID    string
	FailureClass   FailureClass
	ProcessCrashed bool
	ExitCode       int
	RestartCount   int
	Timestamp      time.Time
}

type RecoveryRequest struct {
	RuntimeID      domain.RuntimeInstanceID
	ServiceID      string
	FailureClass   FailureClass
	TriggeredBy    string
	IdempotencyKey string
	MaxAttempts    int
	ForceRollback  bool
}

type RecoveryResponse struct {
	OperationID    RecoveryOperationID
	Success        bool
	Stage          RecoveryStage
	Result         RecoveryResult
	Error          error
}

type RecoverableMetadata struct {
	RuntimeID           domain.RuntimeInstanceID
	PluginID            domain.PluginID
	ExtensionID         string
	DesiredRuntimeState domain.RuntimeState
	ServiceIDs          []string
	HasValidCheckpoint  bool
	CanRebuildTopology  bool
}
