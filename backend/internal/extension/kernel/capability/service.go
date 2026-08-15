package capability

import (
	"context"
)

type CapabilityService struct {
	registry *ProviderRegistry
	resolver *Resolver
}

func NewCapabilityService(registry *ProviderRegistry) *CapabilityService {
	return &CapabilityService{
		registry: registry,
		resolver: NewResolver(registry),
	}
}

func (s *CapabilityService) GetCapability(ctx context.Context, toolID string) (ToolDefinition, bool) {
	return ToolDefinition{}, false
}

func (s *CapabilityService) ListCapabilities(ctx context.Context) []ToolDefinition {
	return nil
}

func (s *CapabilityService) ListProviders() []*CapabilityProviderDefinition {
	if s.registry == nil {
		return nil
	}
	return s.registry.ListAllProviders()
}

func (s *CapabilityService) ListProviderInstances() []*CapabilityProviderInstance {
	if s.registry == nil {
		return nil
	}
	providers := s.registry.ListAllProviders()
	seen := make(map[ProviderInstanceID]bool)
	var result []*CapabilityProviderInstance
	for _, def := range providers {
		if def == nil {
			continue
		}
		insts := s.registry.ListInstancesByProvider(def.ID)
		for _, inst := range insts {
			if inst == nil || seen[inst.ID] {
				continue
			}
			seen[inst.ID] = true
			result = append(result, inst)
		}
	}
	return result
}

func (s *CapabilityService) Resolve(request CapabilityResolutionRequest) (CapabilityResolution, error) {
	if s.resolver == nil {
		return CapabilityResolution{}, ErrCapabilityNotRegistered
	}
	return s.resolver.Resolve(request)
}

func (s *CapabilityService) HasCapability(toolID CapabilityID) bool {
	if s.registry == nil {
		return false
	}
	defs := s.registry.ListByCapability(toolID)
	return len(defs) > 0
}

func (s *CapabilityService) HasExecutableProvider(toolID CapabilityID) bool {
	if s.registry == nil {
		return false
	}
	return s.registry.CountExecutableInstances(toolID) > 0
}

type AvailabilityDescription struct {
	CapabilityID   CapabilityID `json:"capabilityId"`
	HasDefinition  bool         `json:"hasDefinition"`
	HasProvider    bool         `json:"hasProvider"`
	HasInstance    bool         `json:"hasInstance"`
	Executable     bool         `json:"executable"`
	ProviderCount  int          `json:"providerCount"`
	InstanceCount  int          `json:"instanceCount"`
}

func (s *CapabilityService) DescribeAvailability(toolID CapabilityID) AvailabilityDescription {
	desc := AvailabilityDescription{
		CapabilityID: toolID,
	}
	if s.registry == nil {
		return desc
	}
	defs := s.registry.ListByCapability(toolID)
	desc.HasDefinition = true
	desc.HasProvider = len(defs) > 0
	desc.ProviderCount = len(defs)
	desc.InstanceCount = s.registry.CountInstancesByCapability(toolID)
	desc.Executable = s.registry.CountExecutableInstances(toolID) > 0
	desc.HasInstance = desc.InstanceCount > 0
	return desc
}
