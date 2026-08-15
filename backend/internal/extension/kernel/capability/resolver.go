package capability

import (
	"errors"
	"fmt"
)

type CapabilityResolver interface {
	Resolve(request CapabilityResolutionRequest) (CapabilityResolution, error)
}

type Resolver struct {
	registry *ProviderRegistry
}

func NewResolver(registry *ProviderRegistry) *Resolver {
	return &Resolver{registry: registry}
}

func (r *Resolver) Resolve(request CapabilityResolutionRequest) (CapabilityResolution, error) {
	result := CapabilityResolution{
		CapabilityID: request.CapabilityID,
	}

	if ParseCapabilityID(string(request.CapabilityID)) == "" {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureCapabilityNotRegistered)
		return result, fmt.Errorf("invalid capability id: %s", request.CapabilityID)
	}

	defs := r.registry.ListByCapability(request.CapabilityID)
	if len(defs) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureCapabilityNotRegistered)
		return result, fmt.Errorf("%w: %s", ErrCapabilityNotRegistered, request.CapabilityID)
	}

	filtered := r.applyHardFilter(defs, request)
	if len(filtered) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureNoAvailableProvider)
		return result, fmt.Errorf("%w: %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	instances := r.collectExecutableInstances(filtered, request)
	if len(instances) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureNoAvailableProvider)
		return result, fmt.Errorf("%w: no executable instance for %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	result.CandidateCount = len(filtered)

	ranking := &ResolutionRanking{
		defs:      filtered,
		instances: instances,
		request:   request,
	}

	ranked, err := ranking.Rank()
	if err != nil {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureProviderConflict)
		return result, fmt.Errorf("ranking conflict: %w", err)
	}
	if len(ranked) == 0 {
		result.ReasonCodes = append(result.ReasonCodes, ResolutionFailureNoAvailableProvider)
		return result, fmt.Errorf("%w: no ranked provider for %s", ErrNoAvailableProvider, request.CapabilityID)
	}

	winner := ranked[0]
	result.Provider = *winner.Definition
	result.ProviderInstance = *winner.Instance
	result.ExecutionTarget = buildExecutionTarget(winner.Definition, winner.Instance)

	return result, nil
}

func (r *Resolver) applyHardFilter(defs []*CapabilityProviderDefinition, request CapabilityResolutionRequest) []*CapabilityProviderDefinition {
	var filtered []*CapabilityProviderDefinition
	for _, def := range defs {
		if def == nil {
			continue
		}
		if def.CapabilityID != request.CapabilityID {
			continue
		}
		if request.RequiredPlacement != "" && def.Placement != request.RequiredPlacement {
			continue
		}
		if !request.AllowCore && def.Placement == ProviderPlacementCore && request.RequiredPlacement == "" {
			continue
		}
		if !request.AllowDevice && def.Placement == ProviderPlacementDevice {
			continue
		}
		if request.Platform != "" && !matchPlatform(def.Platforms, request.Platform) {
			continue
		}
		if request.ExtensionID != "" && def.ExtensionID != request.ExtensionID {
			continue
		}
		if request.ModuleID != "" && def.ModuleID != request.ModuleID {
			continue
		}
		filtered = append(filtered, def)
	}
	return filtered
}

func (r *Resolver) collectExecutableInstances(defs []*CapabilityProviderDefinition, request CapabilityResolutionRequest) []*CapabilityProviderInstance {
	seen := make(map[ProviderInstanceID]bool)
	var instances []*CapabilityProviderInstance
	for _, def := range defs {
		if def == nil {
			continue
		}
		providerInsts := r.registry.ListInstancesByProvider(def.ID)
		for _, inst := range providerInsts {
			if inst == nil {
				continue
			}
			if !inst.IsExecutable() {
				continue
			}
			if request.RequiredDeviceID != "" && string(inst.DeviceID) != string(request.RequiredDeviceID) {
				continue
			}
			if seen[inst.ID] {
				continue
			}
			seen[inst.ID] = true
			instances = append(instances, inst)
		}
	}
	return instances
}

func buildExecutionTarget(def *CapabilityProviderDefinition, inst *CapabilityProviderInstance) InvocationExecutionTarget {
	if def == nil || inst == nil {
		return InvocationExecutionTarget{}
	}
	return InvocationExecutionTarget{
		Placement: string(inst.Placement),

		UserID:    inst.UserID,
		DeviceID:  inst.DeviceID,
		RuntimeID: inst.RuntimeID,

		ProviderID:         string(inst.ProviderID),
		ProviderInstanceID: string(inst.ID),

		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
	}
}

var (
	ErrCapabilityNotRegistered       = errors.New("capability: not registered")
	ErrNoAvailableProvider           = errors.New("capability: no available provider")
	ErrCapabilityPlacementUnavailable = errors.New("capability: placement unavailable")
)
