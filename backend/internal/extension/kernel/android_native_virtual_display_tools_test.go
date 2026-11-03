package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type readyAndroidNativeProvider struct{}

func (readyAndroidNativeProvider) Execute(context.Context, capability.AndroidBridgeRequest) capability.AndroidBridgeResponse {
	return capability.AndroidBridgeResponse{Status: "success"}
}

func (readyAndroidNativeProvider) Health(context.Context) capability.HealthStatus {
	return capability.HealthReady
}

func TestAndroidNativeVirtualDisplayToolsAreModelReady(t *testing.T) {
	definitions := collectAndroidNativeToolDefinitions()
	byID := make(map[string]capability.ToolDefinition, len(definitions))
	for _, definition := range definitions {
		byID[definition.ID] = definition
	}

	expected := map[string]string{
		"android.virtual_display.status":  "android_virtual_display_status",
		"android.virtual_display.create":  "android_virtual_display_create",
		"android.virtual_display.launch":  "android_virtual_display_launch",
		"android.virtual_display.capture": "android_virtual_display_capture",
		"android.virtual_display.tap":     "android_virtual_display_tap",
		"android.virtual_display.swipe":   "android_virtual_display_swipe",
		"android.virtual_display.key":     "android_virtual_display_key",
		"android.virtual_display.text":    "android_virtual_display_text",
		"android.virtual_display.release": "android_virtual_display_release",
	}
	cache := capability.NewJSONSchemaCache()
	for id, modelName := range expected {
		definition, ok := byID[id]
		if !ok {
			t.Fatalf("missing virtual display tool: %s", id)
		}
		if definition.ModelName != modelName {
			t.Fatalf("virtual display tool model name mismatch: %s: %s", id, definition.ModelName)
		}
		if !definition.Enabled {
			t.Fatalf("virtual display tool is disabled: %s", id)
		}
		if !definition.ModelExposure.ExposedByDefault {
			t.Fatalf("virtual display tool is not exposed by default: %s", id)
		}
		if err := cache.ValidateToolDefinition(definition); err != nil {
			t.Fatalf("virtual display tool schema invalid: %s: %v", id, err)
		}
	}
}

func TestAndroidNativeToolsRegisterExecutableProviders(t *testing.T) {
	toolRegistry := capability.NewToolRegistry()
	providerRegistry := capability.NewProviderRegistry()
	if err := registerAndroidNativeToolsIfPresent(
		context.Background(),
		toolRegistry,
		providerRegistry,
		readyAndroidNativeProvider{},
	); err != nil {
		t.Fatal(err)
	}

	for _, capabilityID := range []capability.CapabilityID{
		"android.display.status",
		"android.virtual_display.status",
	} {
		definitions := providerRegistry.ListByCapability(capabilityID)
		if len(definitions) != 1 {
			t.Fatalf("capability %s provider definitions = %d", capabilityID, len(definitions))
		}
		instances := providerRegistry.ListExecutableInstances(capabilityID)
		if len(instances) != 1 {
			t.Fatalf("capability %s executable instances = %d", capabilityID, len(instances))
		}
		if instances[0].Placement != capability.ProviderPlacementCore {
			t.Fatalf("capability %s placement = %s", capabilityID, instances[0].Placement)
		}
	}

	tool, ok := toolRegistry.Get(context.Background(), "android.display.status")
	if !ok {
		t.Fatal("android.display.status tool not registered")
	}
	if tool.CapabilityID != capability.CapabilityID("android.display.status") {
		t.Fatalf("tool capability ID = %q", tool.CapabilityID)
	}
	if tool.ProviderID == "" {
		t.Fatal("tool provider ID is empty")
	}
	if tool.RoutingMode != capability.RoutingModeProviderRequired {
		t.Fatalf("tool routing mode = %q", tool.RoutingMode)
	}
	if tool.Runtime.RuntimeType != capability.RuntimeTypeAndroid_Native {
		t.Fatalf("tool runtime type = %q", tool.Runtime.RuntimeType)
	}

	resolver := capability.NewResolver(capability.NewProviderCatalogAdapter(providerRegistry))
	resolution, err := resolver.Resolve(capability.CapabilityResolutionRequest{
		CapabilityID:       capability.CapabilityID("android.display.status"),
		RequiredPlacement:  capability.ProviderPlacementCore,
		PreferredPlacement: capability.ProviderPlacementCore,
		AllowCore:          true,
		AllowDevice:        false,
	})
	if err != nil || !resolution.HasResult() {
		t.Fatalf("android display provider did not resolve: %v %#v", err, resolution)
	}
}
