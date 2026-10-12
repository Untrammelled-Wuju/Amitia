package capability

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestProviderInstanceReconcilerRegistersInProcessWorkspaceInstances(t *testing.T) {
	registry := NewProviderRegistry()
	adapters := NewRuntimeAdapterRegistry()
	adapters.Register(RuntimeTypeWorkspace, NewWorkspaceRuntimeAdapter(nil, func(context.Context) HealthStatus {
		return HealthReady
	}))
	reconciler := NewProviderInstanceReconcilerWithAdapters(nil, registry, runtimeidentity.Identity{
		SpaceID:   "realm",
		DeviceID:  "local-device",
		RuntimeID: "local-runtime",
	}, adapters)

	definition := domain.ExtensionDefinition{
		ID: "com.amitia.builtin.workspace",
		Modules: []domain.ModuleDefinition{
			{
				ID:          "workspace-runtime",
				ExtensionID: "com.amitia.builtin.workspace",
				Type:        domain.ModuleTypeBuiltin,
				Runtime: &domain.RuntimeDefinition{
					Type:       domain.RuntimeTypeBuiltin,
					EntryPoint: "workspace.manage",
				},
				Metadata: map[string]any{
					"providerRuntimeType": string(RuntimeTypeWorkspace),
				},
				Provider: &domain.ProviderMetadata{
					ID:       "workspace",
					Priority: 100,
				},
				ProvidedCapabilities: []domain.ProvidedCapability{{ID: "workspace.write", Version: "1.0.0"}},
				Placement:            domain.ModulePlacementCloud,
			},
			{
				ID:          "workspace-device-runtime",
				ExtensionID: "com.amitia.builtin.workspace",
				Type:        domain.ModuleTypeBuiltin,
				Runtime: &domain.RuntimeDefinition{
					Type:       domain.RuntimeTypeBuiltin,
					EntryPoint: "workspace.manage",
				},
				Metadata: map[string]any{
					"providerRuntimeType": string(RuntimeTypeWorkspace),
				},
				Provider: &domain.ProviderMetadata{
					ID:       "workspace-device",
					Priority: 110,
				},
				ProvidedCapabilities: []domain.ProvidedCapability{{ID: "workspace.write", Version: "1.0.0"}},
				Placement:            domain.ModulePlacementDevice,
			},
		},
	}
	definitions, err := ProviderDefinitionsFromExtension(definition)
	if err != nil {
		t.Fatal(err)
	}
	for _, providerDefinition := range definitions {
		if err := registry.RegisterDefinition(providerDefinition); err != nil {
			t.Fatal(err)
		}
	}

	ids, err := reconciler.ActivateExtension(definition, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected two provider instances, got %d", len(ids))
	}
	instances := registry.ListInstancesByCapability("workspace.write")
	if len(instances) != 2 {
		t.Fatalf("expected two executable workspace instances, got %d", len(instances))
	}
	placements := map[ProviderPlacement]bool{}
	for _, instance := range instances {
		if !instance.IsExecutable() {
			t.Fatalf("instance is not executable: %+v", instance)
		}
		placements[instance.Placement] = true
		if instance.Placement == ProviderPlacementDevice && (instance.SpaceID != "realm" || instance.DeviceID != "local-device" || instance.RuntimeID != "local-runtime") {
			t.Fatalf("device instance identity is incorrect: %+v", instance)
		}
	}
	if !placements[ProviderPlacementCore] || !placements[ProviderPlacementDevice] {
		t.Fatalf("missing expected placements: %+v", placements)
	}
}

func TestProviderDefinitionsHonorRuntimeOverride(t *testing.T) {
	definition := domain.ExtensionDefinition{
		ID: "com.amitia.builtin.workspace",
		Modules: []domain.ModuleDefinition{{
			ID:          "workspace-runtime",
			ExtensionID: "com.amitia.builtin.workspace",
			Type:        domain.ModuleTypeBuiltin,
			Runtime: &domain.RuntimeDefinition{
				Type:       domain.RuntimeTypeBuiltin,
				EntryPoint: "workspace.manage",
			},
			Metadata: map[string]any{
				"providerRuntimeType": string(RuntimeTypeWorkspace),
			},
			ProvidedCapabilities: []domain.ProvidedCapability{{ID: "workspace.write", Version: "1.0.0"}},
			Placement:            domain.ModulePlacementCloud,
		}},
	}
	definitions, err := ProviderDefinitionsFromExtension(definition)
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 1 || definitions[0].Runtime.RuntimeType != RuntimeTypeWorkspace || definitions[0].Runtime.HandlerName != "workspace.manage" {
		t.Fatalf("unexpected provider definition: %+v", definitions)
	}
}
