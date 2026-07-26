package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type StartupPhase string

const (
	StartupPhaseCore             StartupPhase = "core"
	StartupPhaseStorage          StartupPhase = "storage"
	StartupPhaseMigration        StartupPhase = "migration"
	StartupPhaseSecurityRecovery StartupPhase = "security_recovery"
	StartupPhaseKernel           StartupPhase = "kernel"
	StartupPhaseDefinitions      StartupPhase = "definitions"
	StartupPhaseRegistries       StartupPhase = "registries"
	StartupPhaseReconciliation   StartupPhase = "reconciliation"
	StartupPhaseRuntimes         StartupPhase = "runtimes"
	StartupPhaseContributions    StartupPhase = "contributions"
	StartupPhaseSchedulers       StartupPhase = "schedulers"
	StartupPhaseReady            StartupPhase = "ready"
)

type ShutdownPhase string

const (
	ShutdownPhaseRequested    ShutdownPhase = "requested"
	ShutdownPhaseStopNewWork  ShutdownPhase = "stop_new_work"
	ShutdownPhaseDrain        ShutdownPhase = "drain"
	ShutdownPhasePauseSched   ShutdownPhase = "pause_schedules"
	ShutdownPhaseStopRuntimes ShutdownPhase = "stop_runtimes"
	ShutdownPhaseCloseConn    ShutdownPhase = "close_connections"
	ShutdownPhaseReleaseRes   ShutdownPhase = "release_resources"
	ShutdownPhaseFlush        ShutdownPhase = "flush"
	ShutdownPhasePersistState ShutdownPhase = "persist_recovery_state"
	ShutdownPhaseCloseStorage ShutdownPhase = "close_storage"
	ShutdownPhaseExit         ShutdownPhase = "exit"
)

type ShutdownReason string

const (
	ShutdownReasonNormal      ShutdownReason = "normal"
	ShutdownReasonRestart     ShutdownReason = "restart"
	ShutdownReasonUpdate      ShutdownReason = "update"
	ShutdownReasonForced      ShutdownReason = "forced"
	ShutdownReasonCrash       ShutdownReason = "crash"
	ShutdownReasonUserLogout  ShutdownReason = "user_logout"
	ShutdownReasonSystemPower ShutdownReason = "system_power"
)

type StartupFailureMode string

const (
	FailureModeFailFast        StartupFailureMode = "fail_fast"
	FailureModeDegrade         StartupFailureMode = "degrade"
	FailureModeSkip            StartupFailureMode = "skip"
	FailureModeRetry           StartupFailureMode = "retry"
	FailureModeQuarantine      StartupFailureMode = "quarantine"
	FailureModeManualRecovery  StartupFailureMode = "manual_recovery"
)

type ComponentHealth struct {
	ComponentID string
	State       ComponentState
	Healthy     bool
	Message     string
	CheckedAt   time.Time
	Metadata    map[string]any
}

type ComponentState string

const (
	ComponentStatePending      ComponentState = "pending"
	ComponentStateStarting     ComponentState = "starting"
	ComponentStateStarted      ComponentState = "started"
	ComponentStateReady        ComponentState = "ready"
	ComponentStateDegraded     ComponentState = "degraded"
	ComponentStateFailed       ComponentState = "failed"
	ComponentStateSkipped      ComponentState = "skipped"
	ComponentStateRolledBack   ComponentState = "rolled_back"
	ComponentStateStopping     ComponentState = "stopping"
	ComponentStateStopped      ComponentState = "stopped"
	ComponentStateQuarantined  ComponentState = "quarantined"
)

type RetryPolicy struct {
	MaxAttempts int
	InitialDelay time.Duration
	MaxDelay time.Duration
	BackoffFactor float64
	Jitter bool
}

type BootstrapComponent struct {
	ID           string
	Phase        StartupPhase
	Dependencies []string
	Required     bool
	Timeout      time.Duration
	ReadyTimeout time.Duration
	StopTimeout  time.Duration
	RetryPolicy  RetryPolicy
	FailureMode  StartupFailureMode
	Metadata     map[string]any
}

type BootstrapPlan struct {
	Components []BootstrapComponent
	Phases     []StartupPhase
	PlanHash   string
	CreatedAt  time.Time
	StartupID  string
}

type RecoveryContext struct {
	StartupID         string
	CleanShutdown     bool
	LastShutdownID    string
	InterruptedComponents []string
	PendingRecovery   []string
	ScannedAt         time.Time
}

var (
	ErrComponentNotFound      = errors.New("lifecycle: component not found")
	ErrCircularDependency     = errors.New("lifecycle: circular dependency detected")
	ErrMissingDependency      = errors.New("lifecycle: missing dependency")
	ErrDuplicateComponent     = errors.New("lifecycle: duplicate component id")
	ErrIllegalCrossPhase      = errors.New("lifecycle: illegal cross-phase dependency")
	ErrStartupInProgress      = errors.New("lifecycle: startup already in progress")
	ErrShutdownInProgress     = errors.New("lifecycle: shutdown already in progress")
	ErrNotReady               = errors.New("lifecycle: not ready")
	ErrCoreComponentFailed    = errors.New("lifecycle: core component failed")
	ErrTimeout                = errors.New("lifecycle: operation timed out")
)

type LifecycleError struct {
	ComponentID string
	Phase       string
	Code        string
	Cause       error
	Metadata    map[string]any
}

func (e *LifecycleError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("lifecycle[%s/%s] %s: %v", e.ComponentID, e.Phase, e.Code, e.Cause)
	}
	return fmt.Sprintf("lifecycle[%s/%s] %s", e.ComponentID, e.Phase, e.Code)
}

func (e *LifecycleError) Unwrap() error { return e.Cause }

func wrapLifecycleError(componentID, phase, code string, cause error) *LifecycleError {
	return &LifecycleError{ComponentID: componentID, Phase: phase, Code: code, Cause: cause}
}

func phaseOrder(p StartupPhase) int {
	switch p {
	case StartupPhaseCore: return 0
	case StartupPhaseStorage: return 1
	case StartupPhaseMigration: return 2
	case StartupPhaseSecurityRecovery: return 3
	case StartupPhaseKernel: return 4
	case StartupPhaseDefinitions: return 5
	case StartupPhaseRegistries: return 6
	case StartupPhaseReconciliation: return 7
	case StartupPhaseRuntimes: return 8
	case StartupPhaseContributions: return 9
	case StartupPhaseSchedulers: return 10
	case StartupPhaseReady: return 11
	default: return 100
	}
}

func shutdownPhaseOrder(p ShutdownPhase) int {
	switch p {
	case ShutdownPhaseRequested: return 0
	case ShutdownPhaseStopNewWork: return 1
	case ShutdownPhaseDrain: return 2
	case ShutdownPhasePauseSched: return 3
	case ShutdownPhaseStopRuntimes:
		return 4
	case ShutdownPhaseCloseConn: return 5
	case ShutdownPhaseReleaseRes: return 6
	case ShutdownPhaseFlush: return 7
	case ShutdownPhasePersistState: return 8
	case ShutdownPhaseCloseStorage: return 9
	case ShutdownPhaseExit: return 10
	default: return 100
	}
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("lifecycle: context is nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
