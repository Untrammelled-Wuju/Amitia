package runtime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type SupervisorHealthEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	Health  domain.HealthStatus
	Reason  string
	Occurred time.Time
}

type SupervisorRestartEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	Event     RestartEvent
	Scheduled time.Time
	Reason    string
}

type RestartEvent string

const (
	RestartScheduled RestartEvent = "scheduled"
	RestartStarted   RestartEvent = "started"
	RestartSucceeded RestartEvent = "succeeded"
	RestartFailed    RestartEvent = "failed"
	RestartExhausted RestartEvent = "exhausted"
)

type SupervisorQuarantineEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	Quarantined bool
	Reason      string
	Occurred     time.Time
}

type ProcessExitEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	Expected   bool
	ExitCode   int
	OccurredAt time.Time
	Cause      string
}

type SupervisorCrashEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	ExitCode   int
	OccurredAt time.Time
	Cause      string
}

type CleanupEvent struct {
	RuntimeID    domain.RuntimeInstanceID
	ServiceID    domain.ServiceID
	DefinitionID string
	InstanceID   string
	Generation   int64

	Success  bool
	Occurred time.Time
	Cause    string
}

type ServiceProcessIdentity struct {
	RuntimeID  domain.RuntimeInstanceID
	ServiceID  domain.ServiceID
	InstanceID string
	Generation int64
}

func (e SupervisorHealthEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

func (e SupervisorRestartEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

func (e SupervisorQuarantineEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

func (e ProcessExitEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

func (e SupervisorCrashEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

func (e CleanupEvent) Identity() ServiceProcessIdentity {
	return ServiceProcessIdentity{
		RuntimeID:  e.RuntimeID,
		ServiceID:  e.ServiceID,
		InstanceID: e.InstanceID,
		Generation: e.Generation,
	}
}

type SupervisorHealthEventHandler interface {
	HandleSupervisorHealth(ctx context.Context, event SupervisorHealthEvent) error
}

type SupervisorRestartEventHandler interface {
	HandleRestartEvent(ctx context.Context, event SupervisorRestartEvent) error
}

type SupervisorQuarantineEventHandler interface {
	HandleQuarantineEvent(ctx context.Context, event SupervisorQuarantineEvent) error
}

type ProcessExitEventHandler interface {
	HandleProcessExit(ctx context.Context, event ProcessExitEvent) error
}

type CleanupEventHandler interface {
	HandleCleanupEvent(ctx context.Context, event CleanupEvent) error
}
