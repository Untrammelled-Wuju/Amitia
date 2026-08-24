package runtime

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeTopology struct {
	mu sync.RWMutex

	RuntimeID   domain.RuntimeInstanceID
	PluginID    domain.PluginID
	ExtensionID string

	Services map[domain.ServiceID]*ServiceInstance

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewRuntimeTopology(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, now time.Time) *RuntimeTopology {
	return &RuntimeTopology{
		RuntimeID: runtimeID,
		PluginID:  pluginID,
		Services:  make(map[domain.ServiceID]*ServiceInstance),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (t *RuntimeTopology) AddService(svc *ServiceInstance) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if svc == nil {
		return NewTopologyError(ErrInvalidArgument, "service instance must not be nil")
	}
	if svc.RuntimeID != t.RuntimeID {
		return NewTopologyError(ErrPluginMismatch, "service instance runtime id does not match topology")
	}
	if svc.PluginID != t.PluginID {
		return NewTopologyError(ErrPluginMismatch, "service instance plugin id does not match topology")
	}

	if _, exists := t.Services[svc.ServiceID]; exists {
		return NewTopologyErrorWithCause(ErrDuplicateService, "service already exists in topology",
			NewTopologyError(ErrDuplicateService, string(svc.ServiceID)))
	}

	t.Services[svc.ServiceID] = svc
	t.UpdatedAt = svc.UpdatedAt
	return nil
}

func (t *RuntimeTopology) GetService(serviceID domain.ServiceID) (*ServiceInstance, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	svc, exists := t.Services[serviceID]
	if !exists {
		return nil, NewTopologyErrorWithCause(ErrNotFound, "service not found",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}
	return svc, nil
}

func (t *RuntimeTopology) ListServices() []ServiceInstanceSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ServiceInstanceSnapshot, 0, len(t.Services))
	for _, svc := range t.Services {
		result = append(result, svc.Snapshot())
	}

	sortServicesByID(result)
	return result
}

func (t *RuntimeTopology) ServiceCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Services)
}

func (t *RuntimeTopology) Snapshot() RuntimeTopologySnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	services := make([]ServiceInstanceSnapshot, 0, len(t.Services))
	for _, svc := range t.Services {
		services = append(services, svc.Snapshot())
	}

	sortServicesByID(services)

	return RuntimeTopologySnapshot{
		RuntimeID:   t.RuntimeID,
		PluginID:    t.PluginID,
		ExtensionID: t.ExtensionID,
		Services:    services,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (t *RuntimeTopology) UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	svc, exists := t.Services[serviceID]
	if !exists {
		return NewTopologyErrorWithCause(ErrNotFound, "service not found",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	if err := svc.Transition(next, now); err != nil {
		return err
	}

	t.UpdatedAt = now
	return nil
}

func (t *RuntimeTopology) SetServiceMetadata(serviceID domain.ServiceID, key, value string, now time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	svc, exists := t.Services[serviceID]
	if !exists {
		return NewTopologyErrorWithCause(ErrNotFound, "service not found",
			NewTopologyError(ErrNotFound, string(serviceID)))
	}

	if err := svc.SetMetadata(key, value, now); err != nil {
		return err
	}

	t.UpdatedAt = now
	return nil
}
