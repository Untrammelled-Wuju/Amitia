package enablement

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type InstallationState string

const (
	InstallationStateNotInstalled InstallationState = "not_installed"
	InstallationStateInstalling   InstallationState = "installing"
	InstallationStateInstalled    InstallationState = "installed"
	InstallationStateUpdating     InstallationState = "updating"
	InstallationStateRollingBack  InstallationState = "rolling_back"
	InstallationStateUninstalling InstallationState = "uninstalling"
	InstallationStateFailed       InstallationState = "failed"
)

type DefinitionState string

const (
	DefinitionStateValid             DefinitionState = "valid"
	DefinitionStateInvalid           DefinitionState = "invalid"
	DefinitionStateIncompatible      DefinitionState = "incompatible"
	DefinitionStateMigrationRequired DefinitionState = "migration_required"
	DefinitionStateMissingDependency DefinitionState = "missing_dependency"
	DefinitionStateCorrupted         DefinitionState = "corrupted"
)

type EnablementState string

const (
	EnablementEnabled  EnablementState = "enabled"
	EnablementDisabled EnablementState = "disabled"
)

type ScopeState string

const (
	ScopeStateActive   ScopeState = "active"
	ScopeStateInactive ScopeState = "inactive"
	ScopeStateExpired  ScopeState = "expired"
	ScopeStateRevoked  ScopeState = "revoked"
)

type PermissionState string

const (
	PermissionAllowed  PermissionState = "allow"
	PermissionDenied   PermissionState = "deny"
	PermissionExpired  PermissionState = "expired"
	PermissionRevoked  PermissionState = "revoked"
	PermissionApproval PermissionState = "approval_required"
)

type DesiredRuntimeState string

const (
	DesiredRuntimeStarted  DesiredRuntimeState = "started"
	DesiredRuntimeStopped  DesiredRuntimeState = "stopped"
	DesiredRuntimePaused   DesiredRuntimeState = "paused"
	DesiredRuntimeQuarantine DesiredRuntimeState = "quarantine"
)

type ActualRuntimeState string

const (
	ActualRuntimeStarting    ActualRuntimeState = "starting"
	ActualRuntimeRunning     ActualRuntimeState = "running"
	ActualRuntimeReady       ActualRuntimeState = "ready"
	ActualRuntimeDegraded    ActualRuntimeState = "degraded"
	ActualRuntimeStopped     ActualRuntimeState = "stopped"
	ActualRuntimeCrashed     ActualRuntimeState = "crashed"
	ActualRuntimeQuarantined ActualRuntimeState = "quarantined"
)

type HealthState string

const (
	HealthHealthy   HealthState = "healthy"
	HealthDegraded  HealthState = "degraded"
	HealthUnhealthy HealthState = "unhealthy"
	HealthUnknown   HealthState = "unknown"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type AvailabilityState string

const (
	AvailabilityAvailable   AvailabilityState = "available"
	AvailabilityUnavailable AvailabilityState = "unavailable"
	AvailabilityDegraded    AvailabilityState = "degraded"
)

type ExposureState string

const (
	ExposureVisible   ExposureState = "visible"
	ExposureHidden    ExposureState = "hidden"
)

type ExecutionState string

const (
	ExecutionExecutable   ExecutionState = "executable"
	ExecutionInexecutable ExecutionState = "inexecutable"
)

type StateSubjectKind string

const (
	SubjectExtension         StateSubjectKind = "extension"
	SubjectModule            StateSubjectKind = "module"
	SubjectTool              StateSubjectKind = "tool"
	SubjectAgentSkill        StateSubjectKind = "agent_skill"
	SubjectWorkflow          StateSubjectKind = "workflow"
	SubjectMCPServer         StateSubjectKind = "mcp_server"
	SubjectMCPTool           StateSubjectKind = "mcp_tool"
	SubjectUIContribution    StateSubjectKind = "ui_contribution"
	SubjectHook              StateSubjectKind = "hook"
	SubjectEventSubscription StateSubjectKind = "event_subscription"
	SubjectSchedule          StateSubjectKind = "schedule"
	SubjectBackgroundTask    StateSubjectKind = "background_task"
	SubjectProvider          StateSubjectKind = "provider"
)

type StateSubject struct {
	Kind     StateSubjectKind
	ID       string
	ParentID string
	OwnerID  string
}

type StateRuntimeContext struct {
	Scope         string
	CharacterID   string
	ConversationID string
	UserID        string
	Platform      string
	Now           time.Time
	Metadata      map[string]any
}

type StateReason struct {
	Code     string
	Layer    string
	Message  string
	Severity string
}

type EffectiveState struct {
	Subject            StateSubject
	Installed          bool
	DefinitionValid    bool
	Enabled            bool
	ScopeAllowed       bool
	PermissionAllowed  bool
	DesiredReady       bool
	RuntimeReady       bool
	DependencyReady    bool
	Healthy            bool
	CircuitAllows      bool
	Visible            bool
	Executable         bool
	Availability       AvailabilityState
	Exposure           ExposureState
	Execution          ExecutionState
	Reasons            []StateReason
	EvaluatedAt        time.Time
}

var (
	ErrSubjectNotFound    = errors.New("enablement: subject not found")
	ErrAmbiguousState     = errors.New("enablement: ambiguous state")
	ErrInvalidSubject     = errors.New("enablement: invalid subject")
	ErrNoWriteAllowed     = errors.New("enablement: write to deprecated field")
)

type SubjectState struct {
	Subject            StateSubject
	Installation       InstallationState
	Definition         DefinitionState
	Enablement         EnablementState
	DesiredRuntime     DesiredRuntimeState
	ActualRuntime      ActualRuntimeState
	Health             HealthState
	Circuit            CircuitState
	Scope              ScopeState
	Permission         PermissionState
	DependencyReady    bool
	PlatformSupported  bool
	MigrationRequired  bool
	ParentEnabled      *bool
	UpdatedAt          time.Time
}

type StateStore interface {
	Get(ctx context.Context, subject StateSubject) (SubjectState, error)
	SetEnablement(ctx context.Context, subject StateSubject, state EnablementState) error
	SetDesiredRuntime(ctx context.Context, subject StateSubject, state DesiredRuntimeState) error
	List(ctx context.Context, kind StateSubjectKind) ([]SubjectState, error)
}

type EffectiveStateResolver interface {
	Resolve(ctx context.Context, subject StateSubject, runtimeContext StateRuntimeContext) EffectiveState
}

func reason(code, layer, message, severity string) StateReason {
	return StateReason{Code: code, Layer: layer, Message: message, Severity: severity}
}

func (s EffectiveState) HasBlocking() bool {
	for _, r := range s.Reasons {
		if r.Severity == "blocking" || r.Severity == "critical" {
			return true
		}
	}
	return false
}

func (s EffectiveState) PrimaryReason() *StateReason {
	if len(s.Reasons) == 0 {
		return nil
	}
	r := s.Reasons[0]
	return &r
}

func (s EffectiveState) Summary() string {
	if s.Executable {
		return fmt.Sprintf("executable(%s)", s.Subject.Kind)
	}
	if r := s.PrimaryReason(); r != nil {
		return fmt.Sprintf("blocked(%s:%s)", r.Layer, r.Code)
	}
	return "unknown"
}
