package runtime

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RestartAdapter interface {
	HandleRestartEvent(ctx context.Context, event SupervisorRestartEvent) error
	GetRestartSnapshot(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (RestartSnapshot, bool)
	ListRestartSnapshots(runtimeID domain.RuntimeInstanceID) []RestartSnapshot
}

type restartAdapter struct {
	mu      sync.RWMutex
	states  map[domain.RuntimeInstanceID]map[domain.ServiceID]RestartSnapshot
}

func NewRestartAdapter() RestartAdapter {
	return &restartAdapter{
		states: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]RestartSnapshot),
	}
}

func (a *restartAdapter) HandleRestartEvent(ctx context.Context, event SupervisorRestartEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	services, ok := a.states[event.RuntimeID]
	if !ok {
		services = make(map[domain.ServiceID]RestartSnapshot)
		a.states[event.RuntimeID] = services
	}

	now := time.Now()

	existing, exists := services[event.ServiceID]
	if !exists {
		existing = RestartSnapshot{
			RuntimeID:  event.RuntimeID,
			ServiceID:  event.ServiceID,
			Generation: event.Generation,
		}
	}

	switch event.Event {
	case RestartScheduled, RestartStarted:
		existing.Restarting = true
		existing.Exhausted = false
		existing.Reason = truncateReason(event.Reason)
		if event.Event == RestartStarted {
			existing.RestartCount++
			existing.Generation = event.Generation
			t := now
			existing.LastRestartAt = &t
		}
	case RestartSucceeded:
		existing.Restarting = false
		existing.Exhausted = false
		existing.Generation = event.Generation
		existing.Reason = truncateReason(event.Reason)
	case RestartFailed:
		existing.Restarting = false
		existing.Reason = truncateReason(event.Reason)
	case RestartExhausted:
		existing.Restarting = false
		existing.Exhausted = true
		existing.Reason = truncateReason(event.Reason)
	}

	services[event.ServiceID] = existing
	return nil
}

func (a *restartAdapter) GetRestartSnapshot(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (RestartSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return RestartSnapshot{}, false
	}
	snap, ok := services[serviceID]
	if !ok {
		return RestartSnapshot{}, false
	}
	return snap.Clone(), true
}

func (a *restartAdapter) ListRestartSnapshots(runtimeID domain.RuntimeInstanceID) []RestartSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return nil
	}

	result := make([]RestartSnapshot, 0, len(services))
	for _, snap := range services {
		result = append(result, snap.Clone())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ServiceID < result[j].ServiceID
	})
	return result
}

func (a *restartAdapter) RemoveService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if services, ok := a.states[runtimeID]; ok {
		delete(services, serviceID)
		if len(services) == 0 {
			delete(a.states, runtimeID)
		}
	}
}

func (a *restartAdapter) RemoveRuntime(runtimeID domain.RuntimeInstanceID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.states, runtimeID)
}
