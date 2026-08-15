package capability

import (
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderCatalog interface {
	ListDefinitionsByCapability(capabilityID CapabilityID) []CapabilityProviderDefinition
	ListInstancesByProvider(providerID ProviderID) []CapabilityProviderInstance
	GetDefinitionByID(providerID ProviderID) (CapabilityProviderDefinition, bool)
	GetInstanceByID(instanceID ProviderInstanceID) (CapabilityProviderInstance, bool)
	ListExecutableInstances(capabilityID CapabilityID) []CapabilityProviderInstance
}

type RuntimeCatalog interface {
	Supports(runtimeType RuntimeType) bool
}

type RoutingHostContext interface {
	Platform() runtimeidentity.Platform
	RuntimeID() runtimeidentity.RuntimeID
}

type ProviderCatalogAdapter struct {
	registry *ProviderRegistry
}

func NewProviderCatalogAdapter(registry *ProviderRegistry) *ProviderCatalogAdapter {
	return &ProviderCatalogAdapter{registry: registry}
}

func (a *ProviderCatalogAdapter) ListDefinitionsByCapability(capabilityID CapabilityID) []CapabilityProviderDefinition {
	defs := a.registry.ListByCapability(capabilityID)
	result := make([]CapabilityProviderDefinition, 0, len(defs))
	for _, d := range defs {
		if d == nil {
			continue
		}
		result = append(result, *d)
	}
	return result
}

func (a *ProviderCatalogAdapter) ListInstancesByProvider(providerID ProviderID) []CapabilityProviderInstance {
	insts := a.registry.ListInstancesByProvider(providerID)
	result := make([]CapabilityProviderInstance, 0, len(insts))
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		result = append(result, *inst)
	}
	return result
}

func (a *ProviderCatalogAdapter) GetDefinitionByID(providerID ProviderID) (CapabilityProviderDefinition, bool) {
	def, ok := a.registry.GetByID(providerID)
	if !ok || def == nil {
		return CapabilityProviderDefinition{}, false
	}
	return *def, true
}

func (a *ProviderCatalogAdapter) GetInstanceByID(instanceID ProviderInstanceID) (CapabilityProviderInstance, bool) {
	inst, ok := a.registry.GetInstanceByID(instanceID)
	if !ok || inst == nil {
		return CapabilityProviderInstance{}, false
	}
	return *inst, true
}

func (a *ProviderCatalogAdapter) ListExecutableInstances(capabilityID CapabilityID) []CapabilityProviderInstance {
	insts := a.registry.ListExecutableInstances(capabilityID)
	result := make([]CapabilityProviderInstance, 0, len(insts))
	for _, inst := range insts {
		if inst == nil {
			continue
		}
		result = append(result, *inst)
	}
	return result
}

type RuntimeAdapterCatalogAdapter struct {
	registry *RuntimeAdapterRegistry
}

func NewRuntimeAdapterCatalogAdapter(registry *RuntimeAdapterRegistry) *RuntimeAdapterCatalogAdapter {
	return &RuntimeAdapterCatalogAdapter{registry: registry}
}

func (a *RuntimeAdapterCatalogAdapter) Supports(runtimeType RuntimeType) bool {
	_, ok := a.registry.Resolve(RuntimeBinding{RuntimeType: runtimeType})
	return ok
}
