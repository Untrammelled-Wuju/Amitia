package capability

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type inProcessRuntimeCatalog struct{}

func (inProcessRuntimeCatalog) Supports(runtimeType RuntimeType) bool {
	return runtimeType == RuntimeTypePluginJS
}

type offlineRuntimeAvailability struct{}

func (offlineRuntimeAvailability) IsRuntimeOnline(context.Context, runtimeidentity.RuntimeID) (bool, error) {
	return false, nil
}

func (offlineRuntimeAvailability) IsDeviceOffline(context.Context, runtimeidentity.DeviceID) (bool, error) {
	return true, nil
}

func TestResolverDoesNotRequireRemoteSessionForInProcessRuntime(t *testing.T) {
	registry := NewProviderRegistry()
	definition := CapabilityProviderDefinition{
		ID:           "plugin-provider",
		CapabilityID: "com.example/plugin/ai",
		Kind:         ProviderKindExtension,
		Placement:    ProviderPlacementDevice,
		ExtensionID:  "com.example/plugin",
		ModuleID:     "plugin-runtime",
		Runtime: RuntimeBinding{
			RuntimeType: RuntimeTypePluginJS,
			Metadata: map[string]any{
				"extensionId": "com.example/plugin",
				"moduleId":    "plugin-runtime",
			},
		},
	}
	if err := registry.RegisterDefinition(definition); err != nil {
		t.Fatal(err)
	}
	instance := CapabilityProviderInstance{
		ID:           "plugin-instance",
		ProviderID:   definition.ID,
		CapabilityID: definition.CapabilityID,
		Placement:    ProviderPlacementDevice,
		ExtensionID:  definition.ExtensionID,
		ModuleID:     definition.ModuleID,
		SpaceID:      "space",
		DeviceID:     "local-device",
		RuntimeID:    "kernel-runtime",
		Health:       HealthReady,
		Availability: ProviderAvailabilityAvailable,
	}
	if err := registry.RegisterInstance(instance); err != nil {
		t.Fatal(err)
	}
	resolver := NewResolverWithPolicy(NewProviderCatalogAdapter(registry), RoutingPolicy{
		AllowCore:        true,
		AllowDevice:      false,
		RequireHealthy:   true,
		RequireAvailable: true,
	})
	resolver.SetRuntimeCatalog(inProcessRuntimeCatalog{})
	resolver.SetAvailability(offlineRuntimeAvailability{})
	result, err := resolver.Resolve(CapabilityResolutionRequest{
		CapabilityID:      definition.CapabilityID,
		AllowCore:         true,
		AllowDevice:       true,
		RequiredPlacement: ProviderPlacementDevice,
		RequiredDeviceID:  "local-device",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasResult() || result.ProviderInstance.ID != instance.ID {
		t.Fatalf("in-process provider was not resolved: %+v", result)
	}
}
