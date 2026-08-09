package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ServiceInstanceID string

type ServiceRuntimeState string

const (
	ServiceStateCreated  ServiceRuntimeState = "created"
	ServiceStateStarting ServiceRuntimeState = "starting"
	ServiceStateRunning  ServiceRuntimeState = "running"
	ServiceStateStopping ServiceRuntimeState = "stopping"
	ServiceStateStopped  ServiceRuntimeState = "stopped"
	ServiceStateFailed   ServiceRuntimeState = "failed"
)

var validServiceTransitions = map[ServiceRuntimeState]map[ServiceRuntimeState]struct{}{
	ServiceStateCreated: {
		ServiceStateStarting: {},
	},
	ServiceStateStarting: {
		ServiceStateRunning: {},
		ServiceStateFailed:  {},
	},
	ServiceStateRunning: {
		ServiceStateStopping: {},
		ServiceStateFailed:   {},
	},
	ServiceStateStopping: {
		ServiceStateStopped: {},
		ServiceStateFailed:  {},
	},
}

type ServiceInstance struct {
	ID        ServiceInstanceID
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID

	ServiceID domain.ServiceID

	State ServiceRuntimeState

	Required     bool
	ServiceKind  domain.ServiceKind
	Dependencies []domain.ServiceID

	CreatedAt time.Time
	UpdatedAt time.Time

	StartedAt *time.Time
	StoppedAt *time.Time
	FailedAt  *time.Time

	Metadata map[string]string
}

func BuildServiceInstanceID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) ServiceInstanceID {
	return ServiceInstanceID(fmt.Sprintf("%s/%s", runtimeID, serviceID))
}

func ParseServiceInstanceID(id ServiceInstanceID) (domain.RuntimeInstanceID, domain.ServiceID, error) {
	parts := strings.SplitN(string(id), "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid service instance id format: %s", id)
	}
	return domain.RuntimeInstanceID(parts[0]), domain.ServiceID(parts[1]), nil
}

func IsValidServiceRuntimeState(state ServiceRuntimeState) bool {
	switch state {
	case ServiceStateCreated,
		ServiceStateStarting,
		ServiceStateRunning,
		ServiceStateStopping,
		ServiceStateStopped,
		ServiceStateFailed:
		return true
	}
	return false
}

func CanTransitionServiceState(from ServiceRuntimeState, to ServiceRuntimeState) bool {
	if targets, ok := validServiceTransitions[from]; ok {
		_, exists := targets[to]
		return exists
	}
	return false
}

func IsTerminalServiceState(state ServiceRuntimeState) bool {
	return state == ServiceStateStopped || state == ServiceStateFailed
}

func NewServiceInstance(
	serviceInstanceID ServiceInstanceID,
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	serviceID domain.ServiceID,
	required bool,
	serviceKind domain.ServiceKind,
	dependencies []domain.ServiceID,
	now time.Time,
) (*ServiceInstance, error) {
	if serviceInstanceID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "service instance id must not be empty")
	}
	if runtimeID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "runtime instance id must not be empty")
	}
	if pluginID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "plugin id must not be empty")
	}
	if serviceID == "" {
		return nil, NewTopologyError(ErrInvalidArgument, "service id must not be empty")
	}
	if !domain.IsValidServiceKind(serviceKind) {
		return nil, NewTopologyError(ErrInvalidArgument, "invalid service kind: "+string(serviceKind))
	}

	return &ServiceInstance{
		ID:           serviceInstanceID,
		RuntimeID:    runtimeID,
		PluginID:     pluginID,
		ServiceID:    serviceID,
		State:        ServiceStateCreated,
		Required:     required,
		ServiceKind:  serviceKind,
		Dependencies: copyServiceIDSlice(dependencies),
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     make(map[string]string),
	}, nil
}

func (s *ServiceInstance) Transition(next ServiceRuntimeState, now time.Time) error {
	if !IsValidServiceRuntimeState(s.State) {
		return NewTopologyErrorWithCause(ErrInvalidState, "current service state is invalid",
			NewTopologyError(ErrInvalidState, string(s.State)))
	}
	if !IsValidServiceRuntimeState(next) {
		return NewTopologyErrorWithCause(ErrInvalidState, "target service state is invalid",
			NewTopologyError(ErrInvalidState, string(next)))
	}
	if s.State == next {
		return NewTopologyError(ErrInvalidState, "cannot transition to the same state")
	}
	if IsTerminalServiceState(s.State) {
		return NewTopologyErrorWithCause(ErrInvalidState, "cannot transition from terminal state",
			NewTopologyError(ErrInvalidState, string(s.State)))
	}
	if !CanTransitionServiceState(s.State, next) {
		return NewTopologyErrorWithCause(ErrInvalidState, "service state transition not allowed",
			NewTopologyError(ErrInvalidState, string(s.State)+" -> "+string(next)))
	}

	s.State = next
	s.UpdatedAt = now

	switch next {
	case ServiceStateRunning:
		if s.StartedAt == nil {
			s.StartedAt = &now
		}
	case ServiceStateStopped:
		s.StoppedAt = &now
	case ServiceStateFailed:
		s.FailedAt = &now
	}

	return nil
}

func (s *ServiceInstance) SetMetadata(key, value string, now time.Time) error {
	if key == "" {
		return NewTopologyError(ErrInvalidArgument, "metadata key must not be empty")
	}
	if len(key) > maxMetadataKeyLength {
		return NewTopologyError(ErrInvalidArgument, "metadata key exceeds maximum length")
	}
	if len(value) > maxMetadataValueLength {
		return NewTopologyError(ErrInvalidArgument, "metadata value exceeds maximum length")
	}
	if strings.ContainsAny(key, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f") {
		return NewTopologyError(ErrInvalidArgument, "metadata key contains control characters")
	}
	if s.Metadata == nil {
		s.Metadata = make(map[string]string)
	}
	s.Metadata[key] = value
	s.UpdatedAt = now
	return nil
}

func (s *ServiceInstance) Snapshot() ServiceInstanceSnapshot {
	metadataCopy := make(map[string]string, len(s.Metadata))
	for k, v := range s.Metadata {
		metadataCopy[k] = v
	}

	depsCopy := make([]domain.ServiceID, len(s.Dependencies))
	copy(depsCopy, s.Dependencies)

	var startedAt, stoppedAt, failedAt *time.Time
	if s.StartedAt != nil {
		t := *s.StartedAt
		startedAt = &t
	}
	if s.StoppedAt != nil {
		t := *s.StoppedAt
		stoppedAt = &t
	}
	if s.FailedAt != nil {
		t := *s.FailedAt
		failedAt = &t
	}

	return ServiceInstanceSnapshot{
		ID:           s.ID,
		RuntimeID:    s.RuntimeID,
		PluginID:     s.PluginID,
		ServiceID:    s.ServiceID,
		State:        s.State,
		Required:     s.Required,
		ServiceKind:  s.ServiceKind,
		Dependencies: depsCopy,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
		StartedAt:    startedAt,
		StoppedAt:    stoppedAt,
		FailedAt:     failedAt,
		Metadata:     metadataCopy,
	}
}

func copyServiceIDSlice(src []domain.ServiceID) []domain.ServiceID {
	if src == nil {
		return nil
	}
	dst := make([]domain.ServiceID, len(src))
	copy(dst, src)
	return dst
}
