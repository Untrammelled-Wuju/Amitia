package runtime

import (
	"sort"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type HealthAggregator interface {
	AggregateRuntimeHealth(topology RuntimeTopologySnapshot, services []ServiceHealthSnapshot) RuntimeHealthResult
}

type healthAggregator struct{}

func NewHealthAggregator() HealthAggregator {
	return &healthAggregator{}
}

func (a *healthAggregator) AggregateRuntimeHealth(topology RuntimeTopologySnapshot, services []ServiceHealthSnapshot) RuntimeHealthResult {
	now := time.Now()

	svcHealthMap := make(map[domain.ServiceID]domain.HealthStatus, len(services))
	for _, s := range services {
		svcHealthMap[s.ServiceID] = s.Health
	}

	serviceByID := make(map[domain.ServiceID]ServiceInstanceSnapshot, len(topology.Services))
	for _, svc := range topology.Services {
		serviceByID[svc.ServiceID] = svc
	}

	hasRequired := false
	requiredAllHealthy := true
	anyRequiredUnhealthy := false
	anyRequiredUnknown := false

	for _, svc := range topology.Services {
		if !svc.Required {
			continue
		}
		hasRequired = true
		health, ok := svcHealthMap[svc.ServiceID]
		if !ok || health == domain.HealthUnknown {
			requiredAllHealthy = false
			anyRequiredUnknown = true
			continue
		}
		if health == domain.HealthUnhealthy {
			requiredAllHealthy = false
			anyRequiredUnhealthy = true
			continue
		}
		if health == domain.HealthDegraded {
			requiredAllHealthy = false
		}
	}

	if !hasRequired {
		result := RuntimeHealthResult{
			RuntimeID:     topology.RuntimeID,
			Health:        domain.HealthHealthy,
			AggregatedAt:  now,
			ServiceHealths: copyServiceHealths(services),
		}
		if len(topology.Services) == 0 {
			result.Reason = "zero_service_runtime"
		} else {
			result.Reason = "no_required_services"
		}
		return result
	}

	if anyRequiredUnhealthy {
		return RuntimeHealthResult{
			RuntimeID:      topology.RuntimeID,
			Health:         domain.HealthUnhealthy,
			AggregatedAt:   now,
			ServiceHealths:  copyServiceHealths(services),
			Reason:         "required_service_unhealthy",
		}
	}

	if anyRequiredUnknown {
		return RuntimeHealthResult{
			RuntimeID:      topology.RuntimeID,
			Health:         domain.HealthDegraded,
			AggregatedAt:   now,
			ServiceHealths:  copyServiceHealths(services),
			Reason:         "required_service_unknown_or_degraded",
		}
	}

	if !requiredAllHealthy {
		return RuntimeHealthResult{
			RuntimeID:      topology.RuntimeID,
			Health:         domain.HealthDegraded,
			AggregatedAt:   now,
			ServiceHealths:  copyServiceHealths(services),
			Reason:         "required_service_degraded",
		}
	}

	anyOptionalIssues := false
	for _, svc := range topology.Services {
		if svc.Required {
			continue
		}
		health, ok := svcHealthMap[svc.ServiceID]
		if !ok || health != domain.HealthHealthy {
			anyOptionalIssues = true
			break
		}
	}

	if anyOptionalIssues {
		return RuntimeHealthResult{
			RuntimeID:      topology.RuntimeID,
			Health:         domain.HealthDegraded,
			AggregatedAt:   now,
			ServiceHealths:  copyServiceHealths(services),
			Reason:         "optional_service_impaired",
		}
	}

	return RuntimeHealthResult{
		RuntimeID:      topology.RuntimeID,
		Health:         domain.HealthHealthy,
		AggregatedAt:   now,
		ServiceHealths:  copyServiceHealths(services),
		Reason:         "all_services_healthy",
	}
}

func AggregateRuntimeHealth(topology RuntimeTopologySnapshot, services []ServiceHealthSnapshot) RuntimeHealthResult {
	a := &healthAggregator{}
	return a.AggregateRuntimeHealth(topology, services)
}

func QuarantineImpactOnRuntime(
	graph *DependencyGraph,
	quarantinedService domain.ServiceID,
	requiredSnapshots []ServiceInstanceSnapshot,
) QuarantineImpact {
	hasRequiredDependent := false

	requiredSet := make(map[domain.ServiceID]struct{}, len(requiredSnapshots))
	for _, s := range requiredSnapshots {
		if s.Required {
			requiredSet[s.ServiceID] = struct{}{}
		}
	}

	if graph != nil {
		dependents, err := graph.TransitiveDependents(quarantinedService)
		if err == nil {
			for _, depID := range dependents {
				if _, isRequired := requiredSet[depID]; isRequired {
					hasRequiredDependent = true
					break
				}
			}
		}
	}

	if hasRequiredDependent {
		return QuarantineImpactRuntimeFailed
	}

	quarantinedRequired := false
	for _, s := range requiredSnapshots {
		if s.ServiceID == quarantinedService && s.Required {
			quarantinedRequired = true
			break
		}
	}
	if quarantinedRequired {
		return QuarantineImpactRuntimeFailed
	}

	return QuarantineImpactRuntimeDegraded
}

type QuarantineImpact string

const (
	QuarantineImpactRuntimeFailed  QuarantineImpact = "runtime_failed"
	QuarantineImpactRuntimeDegraded QuarantineImpact = "runtime_degraded"
	QuarantineImpactNone           QuarantineImpact = "none"
)

func copyServiceHealths(services []ServiceHealthSnapshot) []ServiceHealthSnapshot {
	if services == nil {
		return nil
	}
	out := make([]ServiceHealthSnapshot, len(services))
	for i, s := range services {
		out[i] = s.Clone()
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ServiceID < out[j].ServiceID
	})
	return out
}
