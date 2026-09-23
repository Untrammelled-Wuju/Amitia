package capability

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestExtensionProviderReconcilerPreservesBuiltinProviders(t *testing.T) {
	registry := NewProviderRegistry()
	builtinProvider := CapabilityProviderDefinition{
		ID:           "com.amitia.builtin.search.provider",
		CapabilityID: "search.web",
		Kind:         ProviderKindBuiltin,
		Placement:    ProviderPlacementCore,
		ExtensionID:  "com.amitia.builtin.search",
		ModuleID:     "search-runtime",
		Runtime: RuntimeBinding{
			RuntimeType: RuntimeTypeSearch,
			RuntimeID:   "search-runtime",
			HandlerName: "web.run",
		},
	}
	if err := registry.RegisterDefinition(builtinProvider); err != nil {
		t.Fatal(err)
	}
	if err := registry.RegisterInstance(CapabilityProviderInstance{
		ID:           "builtin-search-instance",
		ProviderID:   builtinProvider.ID,
		CapabilityID: builtinProvider.CapabilityID,
		Placement:    builtinProvider.Placement,
		Health:       HealthReady,
		Availability: ProviderAvailabilityAvailable,
	}); err != nil {
		t.Fatal(err)
	}

	definition := domain.ExtensionDefinition{
		ID:      "com.amitia.builtin.search",
		Version: domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		Modules: []domain.ModuleDefinition{
			{
				ID:          "search-runtime",
				ExtensionID: "com.amitia.builtin.search",
				Type:        domain.ModuleTypeBuiltin,
				Runtime: &domain.RuntimeDefinition{
					Type:       domain.RuntimeTypeBuiltin,
					EntryPoint: "web.run",
				},
				ProvidedCapabilities: []domain.ProvidedCapability{{ID: "search.web", Version: "1.0.0"}},
			},
		},
	}

	reconciler := NewExtensionProviderReconciler(nil, registry)
	if err := reconciler.ReconcileDefinitions(definition); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.GetByID(builtinProvider.ID); !ok {
		t.Fatal("builtin provider was removed by extension reconciliation")
	}
	if instances := registry.ListInstancesByProvider(builtinProvider.ID); len(instances) == 0 {
		t.Fatal("builtin provider instance was removed by extension reconciliation")
	}
	if err := reconciler.ReconcileDefinitions(definition); err != nil {
		t.Fatal(err)
	}
	if !registry.HasByID(builtinProvider.ID) {
		t.Fatal("builtin provider was removed on repeated reconciliation")
	}
}
