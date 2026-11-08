package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type providerRuntimeHealthAdapter struct {
	health capability.HealthStatus
}

func (a providerRuntimeHealthAdapter) Supports(binding capability.RuntimeBinding) bool {
	return binding.RuntimeType == capability.RuntimeTypeGameHost
}

func (a providerRuntimeHealthAdapter) Execute(context.Context, capability.RuntimeBinding, capability.ToolInvocationContext, json.RawMessage) capability.UnifiedToolResult {
	return capability.UnifiedToolResult{}
}

func (a providerRuntimeHealthAdapter) Health(context.Context, capability.RuntimeBinding) capability.HealthStatus {
	return a.health
}

func TestReconcileProviderRuntimeStateRestoresExecutableProviderAfterRuntimeStartup(t *testing.T) {
	registry := capability.NewProviderRegistry()
	provider := capability.CapabilityProviderDefinition{
		ID:           "game-provider",
		CapabilityID: "com.example.game/move",
		Kind:         capability.ProviderKindExtension,
		Placement:    capability.ProviderPlacementDevice,
		ExtensionID:  "com.example.game",
		ModuleID:     "game-runtime",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost},
	}
	if err := registry.RegisterDefinition(provider); err != nil {
		t.Fatal(err)
	}
	instance := capability.CapabilityProviderInstance{
		ID:           "game-instance",
		ProviderID:   provider.ID,
		CapabilityID: provider.CapabilityID,
		Placement:    provider.Placement,
		ExtensionID:  provider.ExtensionID,
		ModuleID:     provider.ModuleID,
		SpaceID:      "space",
		DeviceID:     "device",
		RuntimeID:    "runtime",
		Health:       capability.HealthUnhealthy,
		Availability: capability.ProviderAvailabilityUnknown,
	}
	if err := registry.RegisterInstance(instance); err != nil {
		t.Fatal(err)
	}
	adapters := capability.NewRuntimeAdapterRegistry()
	if err := adapters.RegisterAdapter(capability.RuntimeTypeGameHost, providerRuntimeHealthAdapter{health: capability.HealthReady}); err != nil {
		t.Fatal(err)
	}
	container := &Container{CapabilityProviders: registry, AdapterRegistry: adapters}
	if err := container.ReconcileProviderRuntimeState(context.Background(), capability.RuntimeTypeGameHost); err != nil {
		t.Fatal(err)
	}
	updated, found := registry.GetInstanceByID(instance.ID)
	if !found || !updated.IsExecutable() {
		t.Fatalf("provider instance was not restored to executable: %+v", updated)
	}
}
