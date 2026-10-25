package capability

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type staticProviderExecutionLookup struct {
	definition CapabilityProviderDefinition
	instance   CapabilityProviderInstance
}

func (l staticProviderExecutionLookup) GetProvider(context.Context, ProviderID) (CapabilityProviderDefinition, bool) {
	return l.definition, true
}

func (l staticProviderExecutionLookup) GetInstance(context.Context, ProviderInstanceID) (CapabilityProviderInstance, bool) {
	return l.instance, true
}

func TestProviderRuntimeExecutionResolverUsesToolRuntimeBinding(t *testing.T) {
	definition := CapabilityProviderDefinition{
		ID:           "workspace-provider",
		CapabilityID: "workspace.write",
		Placement:    ProviderPlacementCore,
		Runtime:      RuntimeBinding{RuntimeType: RuntimeTypeWorkspace, HandlerName: "workspace.manage"},
	}
	instance := CapabilityProviderInstance{
		ID:           "workspace-instance",
		ProviderID:   definition.ID,
		CapabilityID: definition.CapabilityID,
		Placement:    ProviderPlacementCore,
		Health:       HealthReady,
		Availability: ProviderAvailabilityAvailable,
	}
	resolver := NewProviderRuntimeExecutionResolver(staticProviderExecutionLookup{definition: definition, instance: instance})
	route, err := resolver.ResolveRuntimeExecution(context.Background(), ToolDefinition{
		CapabilityID: "workspace.write",
		Runtime: RuntimeBinding{
			RuntimeType: RuntimeTypeBuiltin,
			RuntimeID:   "builtin",
			HandlerName: "apply_file",
			Metadata:    map[string]any{"modulePlacement": "device"},
		},
	}, ToolInvocationContext{
		ExecutionTarget: InvocationExecutionTarget{
			Placement:          "core",
			SpaceID:            runtimeidentity.SpaceID("realm"),
			ProviderID:         string(definition.ID),
			ProviderInstanceID: string(instance.ID),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Binding.RuntimeType != RuntimeTypeBuiltin || route.Binding.RuntimeID != "builtin" || route.Binding.HandlerName != "apply_file" {
		t.Fatalf("tool runtime binding was not honored: %+v", route.Binding)
	}
	if route.Binding.Metadata["modulePlacement"] != "device" {
		t.Fatalf("tool runtime metadata was not merged: %+v", route.Binding.Metadata)
	}
}

func TestProviderRuntimeExecutionResolverExecutesInProcessDeviceRuntimeWithoutRemoteSession(t *testing.T) {
	definition := CapabilityProviderDefinition{
		ID:           "plugin-provider",
		CapabilityID: "com.example/plugin/ai",
		Placement:    ProviderPlacementDevice,
		Runtime:      RuntimeBinding{RuntimeType: RuntimeTypePluginJS, HandlerName: "ai"},
	}
	instance := CapabilityProviderInstance{
		ID:                "plugin-instance",
		ProviderID:        definition.ID,
		CapabilityID:      definition.CapabilityID,
		Placement:         ProviderPlacementDevice,
		SpaceID:           "realm",
		DeviceID:          "local-device",
		RuntimeID:         "kernel-runtime",
		RuntimeInstanceID: "plugin-runtime-instance",
		Health:            HealthReady,
		Availability:      ProviderAvailabilityAvailable,
	}
	resolver := NewProviderRuntimeExecutionResolver(staticProviderExecutionLookup{definition: definition, instance: instance})
	route, err := resolver.ResolveRuntimeExecution(context.Background(), ToolDefinition{
		CapabilityID: definition.CapabilityID,
		Runtime:      RuntimeBinding{RuntimeType: RuntimeTypePluginJS, HandlerName: "ai"},
	}, ToolInvocationContext{
		ExecutionTarget: InvocationExecutionTarget{
			Placement:          "device",
			SpaceID:            "realm",
			DeviceID:           "local-device",
			RuntimeID:          "kernel-runtime",
			ProviderID:         string(definition.ID),
			ProviderInstanceID: string(instance.ID),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.RemoteDevice || route.RuntimeSessionID != "" || route.ProviderRuntimeInstanceID != instance.RuntimeInstanceID {
		t.Fatalf("in-process device runtime was routed as remote: %+v", route)
	}
}
