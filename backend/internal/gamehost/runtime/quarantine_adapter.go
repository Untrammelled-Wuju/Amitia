package runtime

import (
	"context"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type QuarantineAdapter interface {
	HandleQuarantineEvent(ctx context.Context, event SupervisorQuarantineEvent) error
	GetQuarantineSnapshot(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (QuarantineSnapshot, bool)
	ListQuarantineSnapshots(runtimeID domain.RuntimeInstanceID) []QuarantineSnapshot
	IsQuarantined(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) bool
}

type quarantineAdapter struct {
	mu     sync.RWMutex
	states map[domain.RuntimeInstanceID]map[domain.ServiceID]QuarantineSnapshot
}

func NewQuarantineAdapter() QuarantineAdapter {
	return &quarantineAdapter{
		states: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]QuarantineSnapshot),
	}
}

func (a *quarantineAdapter) HandleQuarantineEvent(ctx context.Context, event SupervisorQuarantineEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	services, ok := a.states[event.RuntimeID]
	if !ok {
		services = make(map[domain.ServiceID]QuarantineSnapshot)
		a.states[event.RuntimeID] = services
	}

	if !event.Quarantined {
		delete(services, event.ServiceID)
		return nil
	}

	t := event.Occurred
	snapshot := QuarantineSnapshot{
		RuntimeID:  event.RuntimeID,
		ServiceID:  event.ServiceID,
		Quarantined: true,
		Reason:     truncateReason(event.Reason),
		Since:      &t,
	}
	services[event.ServiceID] = snapshot
	return nil
}

func (a *quarantineAdapter) GetQuarantineSnapshot(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (QuarantineSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return QuarantineSnapshot{}, false
	}
	snap, ok := services[serviceID]
	if !ok {
		return QuarantineSnapshot{}, false
	}
	return snap.Clone(), true
}

func (a *quarantineAdapter) ListQuarantineSnapshots(runtimeID domain.RuntimeInstanceID) []QuarantineSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return nil
	}

	result := make([]QuarantineSnapshot, 0, len(services))
	for _, snap := range services {
		result = append(result, snap.Clone())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ServiceID < result[j].ServiceID
	})
	return result
}

func (a *quarantineAdapter) IsQuarantined(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return false
	}
	snap, ok := services[serviceID]
	return ok && snap.Quarantined
}

func (a *quarantineAdapter) RemoveService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if services, ok := a.states[runtimeID]; ok {
		delete(services, serviceID)
		if len(services) == 0 {
			delete(a.states, runtimeID)
		}
	}
}
