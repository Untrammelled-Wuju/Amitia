package capability

import (
	"context"
)

type CapabilityService struct {
	registry     *ProviderRegistry
	catalog      ProviderCatalog
	resolver     *Resolver
	toolRegistry *ToolRegistry
}

func NewCapabilityService(registry *ProviderRegistry) *CapabilityService {
	catalog := NewProviderCatalogAdapter(registry)
	return &CapabilityService{
		registry: registry,
		catalog:  catalog,
		resolver: NewResolver(catalog),
	}
}

// SetResolver lets the Kernel use one canonical, fully configured resolver
// (runtime catalog, availability/presence, routing policy) across ToolFacade
// and CapabilityService consumers such as WorkflowExecutionRouter.
func (s *CapabilityService) SetResolver(resolver *Resolver) {
	if s == nil || resolver == nil {
		return
	}
	s.resolver = resolver
}

func (s *CapabilityService) SetToolRegistry(reg *ToolRegistry) {
	s.toolRegistry = reg
}

func (s *CapabilityService) Catalog() ProviderCatalog {
	return s.catalog
}

type CapabilityDescriptor struct {
	ID                      CapabilityID        `json:"id"`
	ProviderCount           int                 `json:"providerCount"`
	ProviderInstanceCount   int                 `json:"providerInstanceCount"`
	ExecutableProviderCount int                 `json:"executableProviderCount"`
	Placements              []ProviderPlacement `json:"placements"`
	ToolIDs                 []string            `json:"toolIds"`
	Metadata                map[string]any      `json:"metadata"`
}

type AvailabilityState string

const (
	AvailabilityNotRegistered        AvailabilityState = "NOT_REGISTERED"
	AvailabilityRegisteredNoProvider AvailabilityState = "REGISTERED_NO_PROVIDER"
	AvailabilityProviderDisabled     AvailabilityState = "REGISTERED_PROVIDER_DISABLED"
	AvailabilityRuntimeUnavailable   AvailabilityState = "REGISTERED_RUNTIME_UNAVAILABLE"
	AvailabilityDeviceOffline        AvailabilityState = "DEVICE_OFFLINE"
	AvailabilityCredentialRequired   AvailabilityState = "CREDENTIAL_REQUIRED"
	AvailabilityPermissionRequired   AvailabilityState = "PERMISSION_REQUIRED"
	AvailabilityAvailable            AvailabilityState = "AVAILABLE"
)

// GetCapability is deprecated. Use GetCapabilityDescriptor for capability queries
// and ToolRegistry/ToolFacade for tool definitions.
func (s *CapabilityService) GetCapability(ctx context.Context, toolID string) (ToolDefinition, bool) {
	if s.toolRegistry == nil {
		return ToolDefinition{}, false
	}
	return s.toolRegistry.Get(ctx, toolID)
}

// ListCapabilities is deprecated. Use ListCapabilityDescriptors instead.
func (s *CapabilityService) ListCapabilities(ctx context.Context) []ToolDefinition {
	if s.toolRegistry == nil {
		return nil
	}
	return s.toolRegistry.List(ctx, ToolFilter{})
}

func (s *CapabilityService) GetCapabilityDescriptor(capID CapabilityID) (CapabilityDescriptor, bool) {
	if s.registry == nil {
		return CapabilityDescriptor{}, false
	}
	defs := s.registry.ListByCapability(capID)
	if len(defs) == 0 {
		return CapabilityDescriptor{}, false
	}
	desc := CapabilityDescriptor{
		ID:                      capID,
		ProviderCount:           len(defs),
		ProviderInstanceCount:   s.registry.CountInstancesByCapability(capID),
		ExecutableProviderCount: s.registry.CountExecutableInstances(capID),
		Placements:              collectPlacements(defs),
		ToolIDs:                 s.toolIDsForCapability(capID),
	}
	return desc, true
}

func (s *CapabilityService) toolIDsForCapability(capID CapabilityID) []string {
	if s.toolRegistry == nil {
		return nil
	}
	return s.toolRegistry.ListToolIDsByCapabilityID(string(capID))
}

func (s *CapabilityService) ListCapabilityDescriptors() []CapabilityDescriptor {
	if s.registry == nil {
		return nil
	}
	allDefs := s.registry.ListAllProviders()
	capSet := make(map[CapabilityID]CapabilityDescriptor)
	for _, def := range allDefs {
		if def == nil {
			continue
		}
		existing, ok := capSet[def.CapabilityID]
		if !ok {
			capSet[def.CapabilityID] = CapabilityDescriptor{
				ID:            def.CapabilityID,
				ProviderCount: 1,
				Placements:    []ProviderPlacement{def.Placement},
			}
			continue
		}
		existing.ProviderCount++
		existing.Placements = appendUniquePlacement(existing.Placements, def.Placement)
		capSet[def.CapabilityID] = existing
	}
	result := make([]CapabilityDescriptor, 0, len(capSet))
	for _, desc := range capSet {
		desc.ProviderInstanceCount = s.registry.CountInstancesByCapability(desc.ID)
		desc.ExecutableProviderCount = s.registry.CountExecutableInstances(desc.ID)
		result = append(result, desc)
	}
	return result
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
	CapabilityID  CapabilityID      `json:"capabilityId"`
	HasDefinition bool              `json:"hasDefinition"`
	HasProvider   bool              `json:"hasProvider"`
	HasInstance   bool              `json:"hasInstance"`
	Executable    bool              `json:"executable"`
	ProviderCount int               `json:"providerCount"`
	InstanceCount int               `json:"instanceCount"`
	State         AvailabilityState `json:"state"`
	Reason        string            `json:"reason,omitempty"`
}

func (s *CapabilityService) DescribeAvailability(toolID CapabilityID) AvailabilityDescription {
	desc := AvailabilityDescription{
		CapabilityID: toolID,
		State:        AvailabilityNotRegistered,
	}
	if s.registry == nil {
		desc.RegistryUnavailable()
		return desc
	}
	defs := s.registry.ListByCapability(toolID)
	if len(defs) == 0 {
		desc.State = AvailabilityNotRegistered
		desc.Reason = "no provider definition registered"
		return desc
	}
	desc.HasDefinition = true
	desc.HasProvider = true
	desc.ProviderCount = len(defs)
	desc.InstanceCount = s.registry.CountInstancesByCapability(toolID)
	desc.Executable = s.registry.CountExecutableInstances(toolID) > 0
	desc.HasInstance = desc.InstanceCount > 0

	if desc.Executable {
		desc.State = AvailabilityAvailable
		return desc
	}
	if !desc.HasInstance {
		desc.State = AvailabilityRuntimeUnavailable
		desc.Reason = "no executable provider instance available"
		return desc
	}
	desc.State = AvailabilityRuntimeUnavailable
	desc.Reason = "provider instances exist but none executable"
	return desc
}

func (d *AvailabilityDescription) RegistryUnavailable() {
	d.State = AvailabilityRuntimeUnavailable
	d.Reason = "registry unavailable"
}

func collectPlacements(defs []*CapabilityProviderDefinition) []ProviderPlacement {
	seen := make(map[ProviderPlacement]bool)
	var result []ProviderPlacement
	for _, def := range defs {
		if def == nil {
			continue
		}
		if !seen[def.Placement] {
			seen[def.Placement] = true
			result = append(result, def.Placement)
		}
	}
	return result
}

func appendUniquePlacement(placements []ProviderPlacement, p ProviderPlacement) []ProviderPlacement {
	for _, existing := range placements {
		if existing == p {
			return placements
		}
	}
	return append(placements, p)
}
