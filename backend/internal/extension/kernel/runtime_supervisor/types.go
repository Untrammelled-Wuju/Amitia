package runtime_supervisor

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type DefinitionID string

type DesiredState string

const (
	DesiredRunning      DesiredState = "running"
	DesiredStopped      DesiredState = "stopped"
	DesiredConnected    DesiredState = "connected"
	DesiredDisconnected DesiredState = "disconnected"
	DesiredPaused       DesiredState = "paused"
)

type ActualState string

const (
	ActualCreated     ActualState = "created"
	ActualStarting    ActualState = "starting"
	ActualReady       ActualState = "ready"
	ActualDegraded    ActualState = "degraded"
	ActualDraining    ActualState = "draining"
	ActualStopping    ActualState = "stopping"
	ActualStopped     ActualState = "stopped"
	ActualCrashed     ActualState = "crashed"
	ActualFailed      ActualState = "failed"
	ActualQuarantined ActualState = "quarantined"
)

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type StopReason string

const (
	StopReasonManual     StopReason = "manual"
	StopReasonDrain      StopReason = "drain"
	StopReasonCrash      StopReason = "crash"
	StopReasonUpdate     StopReason = "update"
	StopReasonRollback   StopReason = "rollback"
	StopReasonDisable    StopReason = "disable"
	StopReasonUninstall  StopReason = "uninstall"
	StopReasonQuarantine StopReason = "quarantine"
	StopReasonDependency StopReason = "dependency_lost"
	StopReasonResource   StopReason = "resource_limit"
	StopReasonCircuit    StopReason = "circuit_open"
)

type InstanceStrategy string

const (
	StrategySingletonPerModule    InstanceStrategy = "singleton_per_module"
	StrategySingletonPerExtension InstanceStrategy = "singleton_per_extension"
	StrategySingletonGlobal       InstanceStrategy = "singleton_global"
	StrategyPerCharacter          InstanceStrategy = "per_character"
	StrategyPerConversation       InstanceStrategy = "per_conversation"
	StrategyPerInvocation         InstanceStrategy = "per_invocation"
	StrategyPool                  InstanceStrategy = "pool"
)

type RestartPolicy string

const (
	RestartNever              RestartPolicy = "never"
	RestartOnCrash            RestartPolicy = "on_crash"
	RestartOnTransientFailure RestartPolicy = "on_transient_failure"
	RestartAlwaysWithLimit    RestartPolicy = "always_with_limit"
	RestartManual             RestartPolicy = "manual"
)

type ResourceLimits struct {
	MaxMemoryBytes     int64
	MaxCPUPercent      float64
	MaxProcesses       int
	MaxConnections     int
	MaxOpenFiles       int
	MaxQueueDepth      int
	MaxConcurrentCalls int
	MaxExecutionTime   time.Duration
}

type RuntimeIdentity struct {
	InstanceID         string
	RuntimeDefinitionID DefinitionID
	ExtensionID        domain.ExtensionID
	ModuleID           domain.ModuleID
	RuntimeType        domain.RuntimeType
	Generation         int64
	SessionNonce       string
}

type InstanceSpec struct {
	DefinitionID    DefinitionID
	ExtensionID     domain.ExtensionID
	ModuleID        domain.ModuleID
	RuntimeType     domain.RuntimeType
	Generation      int64
	Strategy        InstanceStrategy
	Limits          ResourceLimits
	DefinitionHash  string
	DependencySnapID string
	EntryPoint      string
	WorkerCount     int
	Env             map[string]string
	Permissions     []string
	Capabilities    map[string]bool
	Restart         RestartPolicy
	MaxRestarts     int
	RestartWindow   time.Duration
}

type InvocationRequest struct {
	InstanceID    string
	TraceID       string
	InvocationID  string
	ParentID      string
	Deadline      time.Time
	Operation     string
	Input         []byte
	Generation    int64
}

type InvocationResult struct {
	InvocationID string
	Status       string
	Output       []byte
	Error        error
	Duration     time.Duration
}

type HealthReport struct {
	Status    HealthStatus
	Reason    string
	CheckedAt time.Time
	Metrics   map[string]any
}

type ReconcileRequest struct {
	DefinitionID DefinitionID
	Desired      DesiredState
	Spec         InstanceSpec
}

type ReconcileResult struct {
	InstanceID  string
	DefinitionID DefinitionID
	Desired     DesiredState
	Actual      ActualState
	Health      HealthStatus
	Circuit     CircuitState
	Action      string
	Error       error
}

type StateSnapshot struct {
	DefinitionID DefinitionID
	Instances    []InstanceSnapshot
	Generation   int64
	CapturedAt   time.Time
}

type InstanceSnapshot struct {
	InstanceID  string
	Identity    RuntimeIdentity
	Desired     DesiredState
	Actual      ActualState
	Health      HealthStatus
	Circuit     CircuitState
	StartedAt   *time.Time
	StoppedAt   *time.Time
	Restarts    int
	Limits      ResourceLimits
}

type RuntimeHealthSnapshot struct {
	InstanceID  string
	Generation  int64
	Health      HealthStatus
	Circuit     CircuitState
	Actual      ActualState
	Quarantined bool
}

type ManagedRuntime interface {
	Start(ctx context.Context) error
	Invoke(ctx context.Context, request InvocationRequest) InvocationResult
	Health(ctx context.Context) HealthReport
	Stop(ctx context.Context, reason StopReason) error
}

type RuntimeFactory interface {
	Type() domain.RuntimeType
	Validate(spec InstanceSpec) error
	Create(ctx context.Context, spec InstanceSpec) (ManagedRuntime, error)
}

type Supervisor interface {
	RegisterFactory(factory RuntimeFactory) error
	Reconcile(ctx context.Context, request ReconcileRequest) ReconcileResult
	Invoke(ctx context.Context, request InvocationRequest) InvocationResult
	Stop(ctx context.Context, instanceID string, reason StopReason) error
	Drain(ctx context.Context, instanceID string, timeout time.Duration) error
	Restart(ctx context.Context, instanceID string) error
	Snapshot(ctx context.Context, defID DefinitionID) StateSnapshot
	GetInstance(ctx context.Context, instanceID string) (InstanceSnapshot, error)
}

type instanceEntry struct {
	identity    RuntimeIdentity
	runtime     ManagedRuntime
	desired     DesiredState
	actual      ActualState
	health      HealthStatus
	circuit     CircuitState
	startedAt   *time.Time
	stoppedAt   *time.Time
	restarts    int
	limits      ResourceLimits
	definition  DefinitionID
	generation  int64
	lastError   string
	consecFails int
	activeCalls int32
}
