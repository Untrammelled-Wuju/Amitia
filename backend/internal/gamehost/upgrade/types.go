package upgrade

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type UpgradeOperationState string

const (
	UpgradeStatePreparing   UpgradeOperationState = "preparing"
	UpgradeStateQuiescing   UpgradeOperationState = "quiescing"
	UpgradeStateUpdating    UpgradeOperationState = "updating"
	UpgradeStateMigrating   UpgradeOperationState = "migrating"
	UpgradeStateReconciling UpgradeOperationState = "reconciling"
	UpgradeStateResuming    UpgradeOperationState = "resuming"
	UpgradeStateCompleted   UpgradeOperationState = "completed"
	UpgradeStateFailed      UpgradeOperationState = "failed"
)

type UpgradeOperationID string

type RuntimeUpgradeSnapshot struct {
	RuntimeID            domain.RuntimeInstanceID
	PluginID             domain.PluginID
	RuntimeState         domain.RuntimeState
	WasRunning           bool
	WasSuspended         bool
	PreUpgradeGeneration int64
}

type UpgradeRequest struct {
	ExtensionID       string
	TargetVersion     string
	SessionID         string
	UserID            string
	ScopeType         string
	ScopeID           string
	ConfirmationToken string
	IdempotencyKey    string
}

type UpgradeResult struct {
	OperationID      UpgradeOperationID
	ExtensionID      string
	Success          bool
	Stage            UpgradeOperationState
	QuiescedRuntimes []domain.RuntimeInstanceID
	AffectedPlugins  []domain.PluginID
	ResumedRuntimes  []domain.RuntimeInstanceID
	FailedRuntimes   []domain.RuntimeInstanceID
	Error            error
}
