package capability

type ResolutionFailure string

const (
	ResolutionFailureNone                ResolutionFailure = ""
	ResolutionFailureCapabilityNotRegistered ResolutionFailure = "CAPABILITY_NOT_REGISTERED"
	ResolutionFailureNoAvailableProvider ResolutionFailure = "CAPABILITY_NO_AVAILABLE_PROVIDER"
	ResolutionFailurePlacementUnavailable ResolutionFailure = "CAPABILITY_PLACEMENT_UNAVAILABLE"
	ResolutionFailureDeviceUnavailable   ResolutionFailure = "CAPABILITY_DEVICE_UNAVAILABLE"
	ResolutionFailureRuntimeUnavailable  ResolutionFailure = "CAPABILITY_RUNTIME_UNAVAILABLE"
	ResolutionFailureProviderConflict    ResolutionFailure = "CAPABILITY_PROVIDER_CONFLICT"
)

type ResolutionRanking struct {
	defs      []CapabilityProviderDefinition
	instances []CapabilityProviderInstance
	request   CapabilityResolutionRequest
}

func (r *ResolutionRanking) Rank() ([]RankedProvider, error) {
	var scored []RankedProvider
	for i := range r.defs {
		def := r.defs[i]
		inst := r.matchInstance(def)
		if inst == nil {
			continue
		}
		score := scoreProvider(&def, inst, r.request)
		scored = append(scored, RankedProvider{
			Definition: &def,
			Instance:   inst,
			Score:      score,
		})
	}
	if len(scored) == 0 {
		return nil, nil
	}
	sortRankedProviders(scored)
	return scored, nil
}

type RankedProvider struct {
	Definition *CapabilityProviderDefinition
	Instance   *CapabilityProviderInstance
	Score      int
}

func scoreProvider(def *CapabilityProviderDefinition, inst *CapabilityProviderInstance, req CapabilityResolutionRequest) int {
	score := 0

	if req.RequiredPlacement != "" && string(def.Placement) == string(req.RequiredPlacement) {
		score += 1000
	}
	if req.PreferredPlacement != "" && string(def.Placement) == string(req.PreferredPlacement) {
		score += 500
	}
	if req.PreferredDeviceID != "" && string(inst.DeviceID) == string(req.PreferredDeviceID) {
		score += 400
	}
	if req.RequiredDeviceID != "" && string(inst.DeviceID) == string(req.RequiredDeviceID) {
		score += 800
	}
	if req.PreferredRuntimeID != "" && string(inst.RuntimeID) == string(req.PreferredRuntimeID) {
		score += 300
	}

	score += def.Priority * 10

	if req.AllowCore && def.Placement == ProviderPlacementCore {
		score += 50
	}

	if inst.Health == HealthReady {
		score += 100
	} else if inst.Health == HealthDegraded {
		score += 25
	}

	return score
}

func (r *ResolutionRanking) matchInstance(def CapabilityProviderDefinition) *CapabilityProviderInstance {
	for i := range r.instances {
		inst := r.instances[i]
		if inst.ProviderID != def.ID {
			continue
		}
		if !inst.IsExecutable() {
			continue
		}
		if r.request.Platform != "" && !matchPlatform(def.Platforms, r.request.Platform) {
			continue
		}
		return &inst
	}
	return nil
}

func sortRankedProviders(providers []RankedProvider) {
	n := len(providers)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if compareRanked(providers[j], providers[i]) {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}
}

func compareRanked(a, b RankedProvider) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Definition.ID != b.Definition.ID {
		return a.Definition.ID < b.Definition.ID
	}
	return a.Instance.ID < b.Instance.ID
}
