// Package domain provides GameHost domain types.
//
// IMPORTANT: This package contains domain-specific projections for the GameHost runtime.
// It does NOT define authoritative kernel types.
//
// Key boundaries:
//
//   - RuntimeInstance is the GameHost plugin domain runtime lifecycle state.
//     It is NOT the Device Runtime protocol session (owned by deviceruntime.Service).
//     Process ownership remains in the trusted runtime/process supervisor.
//
//   - RuntimeInstanceID is the GameHost domain instance identifier.
//     It is NOT the same as runtimeidentity.RuntimeID (shared device/runtime identity).
package domain

import (
	"strings"
	"time"
)

// RuntimeInstanceID identifies a GameHost domain runtime instance.
// This is a GameHost-specific identifier and is NOT the same as
// runtimeidentity.RuntimeID from the shared kernel.
type RuntimeInstanceID string

// RuntimeState represents a state in the GameHost plugin runtime lifecycle.
type RuntimeState string

const (
	RuntimeStateCreated    RuntimeState = "created"
	RuntimeStateStarting   RuntimeState = "starting"
	RuntimeStateRunning    RuntimeState = "running"
	RuntimeStateDegraded   RuntimeState = "degraded"
	RuntimeStateSuspended  RuntimeState = "suspended"
	RuntimeStateRestarting RuntimeState = "restarting"
	RuntimeStateStopping   RuntimeState = "stopping"
	RuntimeStateStopped    RuntimeState = "stopped"
	RuntimeStateFailed     RuntimeState = "failed"
)

// RuntimeInstance represents the GameHost plugin runtime lifecycle state.
//
// This is a domain-specific projection for GameHost plugin management. It is NOT:
//   - The Device Runtime protocol session (owned by deviceruntime.Service)
//   - The Runtime_orchestrator ProviderInstance (owned by runtimeorchestrator)
//
// Process start/stop operations must go through the trusted Process Supervisor.
// This model tracks state only; it does not directly own physical processes.
type RuntimeInstance struct {
	ID       RuntimeInstanceID
	PluginID PluginID

	State       RuntimeState
	StateReason string

	Health HealthState

	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	StoppedAt   *time.Time
	SuspendedAt *time.Time
	FailedAt    *time.Time

	Metadata map[string]string
}

var validRuntimeTransitions = map[RuntimeState]map[RuntimeState]struct{}{
	RuntimeStateCreated: {
		RuntimeStateStarting: {},
	},
	RuntimeStateStarting: {
		RuntimeStateRunning:  {},
		RuntimeStateDegraded: {},
		RuntimeStateStopping: {},
		RuntimeStateFailed:   {},
	},
	RuntimeStateRunning: {
		RuntimeStateDegraded:   {},
		RuntimeStateSuspended:  {},
		RuntimeStateRestarting: {},
		RuntimeStateStopping:   {},
		RuntimeStateFailed:     {},
	},
	RuntimeStateDegraded: {
		RuntimeStateRunning:   {},
		RuntimeStateSuspended: {},
		RuntimeStateStopping:  {},
		RuntimeStateFailed:    {},
	},
	RuntimeStateSuspended: {
		RuntimeStateRunning:  {},
		RuntimeStateDegraded: {},
		RuntimeStateStopping: {},
		RuntimeStateFailed:   {},
	},
	RuntimeStateRestarting: {
		RuntimeStateStopping: {},
		RuntimeStateFailed:   {},
	},
	RuntimeStateStopping: {
		RuntimeStateStopped: {},
		RuntimeStateFailed:  {},
	},
	RuntimeStateStopped: {
		RuntimeStateStarting: {},
	},
}

var activeRuntimeStates = map[RuntimeState]struct{}{
	RuntimeStateStarting:   {},
	RuntimeStateRunning:    {},
	RuntimeStateDegraded:   {},
	RuntimeStateSuspended:  {},
	RuntimeStateRestarting: {},
	RuntimeStateStopping:   {},
}

func CanTransitionRuntimeState(from RuntimeState, to RuntimeState) bool {
	if targets, ok := validRuntimeTransitions[from]; ok {
		_, exists := targets[to]
		return exists
	}
	return false
}

func AllRuntimeStates() []RuntimeState {
	return []RuntimeState{
		RuntimeStateCreated,
		RuntimeStateStarting,
		RuntimeStateRunning,
		RuntimeStateDegraded,
		RuntimeStateSuspended,
		RuntimeStateRestarting,
		RuntimeStateStopping,
		RuntimeStateStopped,
		RuntimeStateFailed,
	}
}

func IsTerminalRuntimeState(state RuntimeState) bool {
	return state == RuntimeStateFailed
}

func IsActiveRuntimeState(state RuntimeState) bool {
	_, ok := activeRuntimeStates[state]
	return ok
}

func IsValidRuntimeState(state RuntimeState) bool {
	for _, s := range AllRuntimeStates() {
		if s == state {
			return true
		}
	}
	return false
}

func NewRuntimeInstance(id RuntimeInstanceID, pluginID PluginID, now time.Time) (*RuntimeInstance, error) {
	if id == "" {
		return nil, NewHostError(ErrInvalidArgument, "runtime instance id must not be empty")
	}
	if pluginID == "" {
		return nil, NewHostError(ErrInvalidArgument, "plugin id must not be empty")
	}

	return &RuntimeInstance{
		ID:       id,
		PluginID: pluginID,

		State:       RuntimeStateCreated,
		StateReason: "",

		Health: HealthState{
			Status:    HealthUnknown,
			Message:   "",
			UpdatedAt: now,
		},

		CreatedAt: now,
		UpdatedAt: now,

		Metadata: make(map[string]string),
	}, nil
}

func (r *RuntimeInstance) Transition(next RuntimeState, reason string, now time.Time) error {
	if !IsValidRuntimeState(r.State) {
		return NewHostErrorWithCause(ErrInvalidState, "current state is invalid", NewHostError(ErrInvalidState, string(r.State)))
	}
	if !IsValidRuntimeState(next) {
		return NewHostErrorWithCause(ErrInvalidState, "target state is invalid", NewHostError(ErrInvalidState, string(next)))
	}
	if r.State == next {
		return NewHostError(ErrInvalidState, "cannot transition to the same state")
	}
	if IsTerminalRuntimeState(r.State) {
		return NewHostErrorWithCause(ErrInvalidState, "cannot transition from terminal state", NewHostError(ErrInvalidState, string(r.State)))
	}
	if !CanTransitionRuntimeState(r.State, next) {
		return NewHostErrorWithCause(ErrInvalidState, "runtime state transition not allowed",
			NewHostError(ErrInvalidState, string(r.State)+" -> "+string(next)))
	}

	r.State = next
	r.StateReason = reason
	r.UpdatedAt = now

	switch next {
	case RuntimeStateRunning:
		if r.StartedAt == nil {
			r.StartedAt = &now
		}
	case RuntimeStateStopped:
		r.StoppedAt = &now
	case RuntimeStateSuspended:
		r.SuspendedAt = &now
	case RuntimeStateFailed:
		r.FailedAt = &now
	}

	return nil
}

func (r *RuntimeInstance) UpdateHealth(health HealthState, now time.Time) error {
	if !IsValidRuntimeState(r.State) {
		return NewHostErrorWithCause(ErrInvalidState, "current state is invalid", NewHostError(ErrInvalidState, string(r.State)))
	}

	r.Health = health
	r.Health.UpdatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *RuntimeInstance) SetMetadata(key string, value string, now time.Time) error {
	if key == "" {
		return NewHostError(ErrInvalidArgument, "metadata key must not be empty")
	}
	if len(key) > maxMetadataKeyLength {
		return NewHostError(ErrInvalidArgument, "metadata key exceeds maximum length")
	}
	if len(value) > maxMetadataValueLength {
		return NewHostError(ErrInvalidArgument, "metadata value exceeds maximum length")
	}
	if strings.ContainsAny(key, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewHostError(ErrInvalidArgument, "metadata key contains control characters")
	}
	if r.Metadata == nil {
		r.Metadata = make(map[string]string)
	}
	r.Metadata[key] = value
	r.UpdatedAt = now
	return nil
}
