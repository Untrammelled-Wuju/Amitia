package runtime

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HealthAdapter interface {
	HandleSupervisorHealth(ctx context.Context, event SupervisorHealthEvent) error
	GetServiceHealth(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServiceHealthSnapshot, bool)
	ListServiceHealth(runtimeID domain.RuntimeInstanceID) []ServiceHealthSnapshot
	ReconcileServiceHealth(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, current domain.HealthStatus, reason string, now time.Time)
}

type RuntimeHealthAccessor interface {
	UpdateRuntimeHealth(runtimeID domain.RuntimeInstanceID, health domain.HealthStatus, reason string, now time.Time) error
	GetRuntimeState(runtimeID domain.RuntimeInstanceID) (domain.RuntimeState, error)
}

type HealthAggregatorFunc func(topology RuntimeTopologySnapshot, services []ServiceHealthSnapshot) RuntimeHealthResult

type healthAdapter struct {
	mu         sync.RWMutex
	aggregator HealthAggregator
	topology   RuntimeTopologyStore
	runtime    RuntimeHealthAccessor
	states     map[domain.RuntimeInstanceID]map[domain.ServiceID]ServiceHealthSnapshot
}

func NewHealthAdapter(
	topology RuntimeTopologyStore,
	runtime RuntimeHealthAccessor,
	aggregator HealthAggregator,
) (HealthAdapter, error) {
	if topology == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "topology store must not be nil"}
	}
	if runtime == nil {
		return nil, &TopologyError{Code: ErrInvalidArgument, Message: "runtime accessor must not be nil"}
	}
	if aggregator == nil {
		aggregator = NewHealthAggregator()
	}
	return &healthAdapter{
		topology:   topology,
		runtime:    runtime,
		aggregator: aggregator,
		states:     make(map[domain.RuntimeInstanceID]map[domain.ServiceID]ServiceHealthSnapshot),
	}, nil
}

func (a *healthAdapter) HandleSupervisorHealth(ctx context.Context, event SupervisorHealthEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	services, ok := a.states[event.RuntimeID]
	if !ok {
		services = make(map[domain.ServiceID]ServiceHealthSnapshot)
		a.states[event.RuntimeID] = services
	}

	pluginID := a.resolvePluginID(event.RuntimeID)

	existing, exists := services[event.ServiceID]
	now := event.Occurred
	if now.IsZero() {
		now = time.Now()
	}

	if exists && existing.LastChangedAt.After(now) {
		now = existing.LastChangedAt
	}

	if exists && existing.Health == event.Health {
		existing.LastObservedAt = now
		services[event.ServiceID] = existing
		return nil
	}

	snapshot := ServiceHealthSnapshot{
		PluginID:       pluginID,
		RuntimeID:      event.RuntimeID,
		ServiceID:      event.ServiceID,
		Health:         event.Health,
		LastChangedAt:  now,
		LastObservedAt: now,
		Reason:         truncateReason(event.Reason),
	}
	services[event.ServiceID] = snapshot

	return a.aggregateLocked(event.RuntimeID, now)
}

func (a *healthAdapter) resolvePluginID(runtimeID domain.RuntimeInstanceID) domain.PluginID {
	topologySnapshot, err := a.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return ""
	}
	return topologySnapshot.PluginID
}

func (a *healthAdapter) GetServiceHealth(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (ServiceHealthSnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return ServiceHealthSnapshot{}, false
	}
	snap, ok := services[serviceID]
	if !ok {
		return ServiceHealthSnapshot{}, false
	}
	return snap.Clone(), true
}

func (a *healthAdapter) ListServiceHealth(runtimeID domain.RuntimeInstanceID) []ServiceHealthSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	services, ok := a.states[runtimeID]
	if !ok {
		return nil
	}

	result := make([]ServiceHealthSnapshot, 0, len(services))
	for _, snap := range services {
		result = append(result, snap.Clone())
	}

	sortServiceHealthByID(result)
	return result
}

func (a *healthAdapter) ReconcileServiceHealth(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, current domain.HealthStatus, reason string, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	services, ok := a.states[runtimeID]
	if !ok {
		services = make(map[domain.ServiceID]ServiceHealthSnapshot)
		a.states[runtimeID] = services
	}

	existing, exists := services[serviceID]
	if exists && existing.Health == current {
		existing.LastObservedAt = now
		services[serviceID] = existing
		return
	}

	snapshot := ServiceHealthSnapshot{
		RuntimeID:      runtimeID,
		ServiceID:      serviceID,
		Health:         current,
		LastChangedAt:  now,
		LastObservedAt: now,
		Reason:         truncateReason(reason),
	}
	services[serviceID] = snapshot

	_ = a.aggregateLocked(runtimeID, now)
}

func (a *healthAdapter) aggregateLocked(runtimeID domain.RuntimeInstanceID, now time.Time) error {
	topologySnapshot, err := a.topology.GetTopologySnapshot(runtimeID)
	if err != nil {
		return nil
	}

	services := a.states[runtimeID]
	healths := make([]ServiceHealthSnapshot, 0, len(services))
	for _, snap := range services {
		healths = append(healths, snap)
	}

	result := a.aggregator.AggregateRuntimeHealth(topologySnapshot, healths)

	runtimeState, stateErr := a.runtime.GetRuntimeState(runtimeID)
	if stateErr == nil {
		if domain.IsTerminalRuntimeState(runtimeState) {
			return nil
		}
	}

	return a.runtime.UpdateRuntimeHealth(runtimeID, result.Health, result.Reason, now)
}

func sortServiceHealthByID(services []ServiceHealthSnapshot) {
	sort.Slice(services, func(i, j int) bool {
		return services[i].ServiceID < services[j].ServiceID
	})
}

const maxReasonLength = 256

func truncateReason(reason string) string {
	if len(reason) <= maxReasonLength {
		return reason
	}
	return reason[:maxReasonLength]
}
