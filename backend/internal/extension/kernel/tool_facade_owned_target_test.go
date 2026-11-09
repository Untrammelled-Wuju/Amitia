package kernel

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ownedTargetResolver struct {
	request capability.CapabilityResolutionRequest
	result  capability.CapabilityResolution
	err     error
	calls   int
}

func (r *ownedTargetResolver) Resolve(request capability.CapabilityResolutionRequest) (capability.CapabilityResolution, error) {
	r.request = request
	r.calls++
	return r.result, r.err
}

func TestOwnedToolPlacementUsesAuthorizedDevice(t *testing.T) {
	for _, runtime := range []capability.RuntimeType{capability.RuntimeTypeBrowser, capability.RuntimeTypePluginJS, capability.RuntimeTypeWorkspace, capability.RuntimeTypePluginService} {
		t.Run(string(runtime), func(t *testing.T) {
			resolver := &ownedTargetResolver{result: capability.CapabilityResolution{Provider: capability.CapabilityProviderDefinition{ID: "device-provider"}, ProviderInstance: capability.CapabilityProviderInstance{ID: "device-instance"}, ExecutionTarget: capability.InvocationExecutionTarget{Placement: "device", SpaceID: "realm", DeviceID: "phone-a"}}}
			facade := &ToolFacade{capabilityResolver: resolver}
			scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "phone-a"}, Metadata: map[string]any{"ownedDeviceTarget": true}}}
			definition := capability.ToolDefinition{ID: "test.tool", CapabilityID: "test.capability", Runtime: capability.RuntimeBinding{RuntimeType: runtime, Metadata: map[string]any{"modulePlacement": "device"}}}
			resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
			if resolved.target.DeviceID != "phone-a" || resolver.calls != 1 || resolver.request.RequiredDeviceID != "phone-a" || resolver.request.SpaceID != "realm" || resolver.request.AllowCore || resolver.request.RequiredPlacement != capability.ProviderPlacementDevice {
				t.Fatalf("incorrect device routing: resolved=%+v request=%+v", resolved, resolver.request)
			}
			resolver.result.ExecutionTarget.DeviceID = "phone-b"
			if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.resolutionCode != "CAPABILITY_TARGET_MISMATCH" || !resolved.target.IsZero() {
				t.Fatalf("wrong device accepted: %+v", resolved)
			}
			resolver.err = errors.New("device offline")
			if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.missingCapability == "" || !resolved.target.IsZero() {
				t.Fatalf("offline device accepted: %+v", resolved)
			}
		})
	}
}

func TestOwnedGameHostRoutesToActiveGameHostDevice(t *testing.T) {
	resolver := &ownedTargetResolver{result: capability.CapabilityResolution{
		Provider:         capability.CapabilityProviderDefinition{ID: "game-provider"},
		ProviderInstance: capability.CapabilityProviderInstance{ID: "game-instance"},
		ExecutionTarget:  capability.InvocationExecutionTarget{Placement: "device", SpaceID: "realm", DeviceID: "center-device"},
	}}
	facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "center-device"}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
		RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "phone-a"},
		Metadata: map[string]any{
			"ownedDeviceTarget": true,
			"gameHostDeviceId":  "center-device",
		},
	}}
	definition := capability.ToolDefinition{
		ID:           "com.example.game/move",
		CapabilityID: "com.example.game/move",
		ExtensionID:  "com.example.game",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost, Metadata: map[string]any{"modulePlacement": "device"}},
	}
	resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
	if resolved.target.DeviceID != "center-device" || resolver.calls != 1 || resolver.request.RequiredDeviceID != "center-device" || resolver.request.PreferredDeviceID != "center-device" || resolver.request.RequiredPlacement != capability.ProviderPlacementDevice || resolver.request.AllowCore {
		t.Fatalf("game host did not route to active host: resolved=%+v request=%+v", resolved, resolver.request)
	}
	resolver.result.ExecutionTarget.DeviceID = "phone-a"
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.resolutionCode != "CAPABILITY_TARGET_MISMATCH" || !resolved.target.IsZero() {
		t.Fatalf("wrong game host device accepted: %+v", resolved)
	}
}

func TestOwnedCloudToolDoesNotUseDeviceFallback(t *testing.T) {
	resolver := &ownedTargetResolver{result: capability.CapabilityResolution{Provider: capability.CapabilityProviderDefinition{ID: "core-provider"}, ProviderInstance: capability.CapabilityProviderInstance{ID: "core-instance"}, ExecutionTarget: capability.InvocationExecutionTarget{Placement: "core", SpaceID: "realm"}}}
	facade := &ToolFacade{capabilityResolver: resolver}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"ownedDeviceTarget": true}}}
	definition := capability.ToolDefinition{ID: "cloud.tool", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeJavaScript, Metadata: map[string]any{"modulePlacement": "cloud"}}}
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.target.Placement != "core" || resolver.request.AllowDevice || resolver.request.RequiredPlacement != capability.ProviderPlacementCore {
		t.Fatalf("incorrect cloud routing: %+v %+v", resolved, resolver.request)
	}
	resolver.result.ExecutionTarget.Placement = "device"
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.resolutionCode != "CAPABILITY_TARGET_MISMATCH" {
		t.Fatalf("cloud tool used device: %+v", resolved)
	}
}

func TestOwnedDeviceToolRequiresTargetAndLocalGameHostRemainsLocal(t *testing.T) {
	resolver := &ownedTargetResolver{}
	facade := &ToolFacade{capabilityResolver: resolver}
	definition := capability.ToolDefinition{ID: "game.tool", Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeGameHost}}
	scope := InvocationScope{ExecContext: &execution.ExecutionContext{Metadata: map[string]any{"ownedDeviceTarget": true}}}
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.resolutionCode != "CAPABILITY_DEVICE_TARGET_REQUIRED" || resolver.calls != 0 {
		t.Fatalf("missing target accepted: %+v", resolved)
	}
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, InvocationScope{}); !resolved.target.IsZero() || resolved.missingCapability != "" || resolver.calls != 0 {
		t.Fatalf("local game routing changed: %+v", resolved)
	}
}

func TestOwnedLocalWorkspaceUsesCoreProvider(t *testing.T) {
	resolver := &ownedTargetResolver{result: capability.CapabilityResolution{
		Provider:         capability.CapabilityProviderDefinition{ID: "core-workspace-provider"},
		ProviderInstance: capability.CapabilityProviderInstance{ID: "core-workspace-instance"},
		ExecutionTarget:  capability.InvocationExecutionTarget{Placement: "core", SpaceID: "realm"},
	}}
	facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "local-device"}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
		RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "local-device"},
		Metadata:      map[string]any{"ownedDeviceTarget": true},
	}}
	definition := capability.ToolDefinition{
		ID:           "workspace.write",
		CapabilityID: "workspace.write",
		ExtensionID:  "com.amitia.builtin.workspace",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeWorkspace},
	}
	resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
	if resolved.target.Placement != "core" || resolver.calls != 1 || resolver.request.RequiredPlacement != capability.ProviderPlacementCore || resolver.request.AllowDevice || resolver.request.RequiredDeviceID != "" {
		t.Fatalf("local workspace did not use core provider: resolved=%+v request=%+v", resolved, resolver.request)
	}
}

func TestOwnedWorkspaceRoutesToWorkspaceDevice(t *testing.T) {
	resolver := &ownedTargetResolver{result: capability.CapabilityResolution{
		Provider:         capability.CapabilityProviderDefinition{ID: "workspace-provider"},
		ProviderInstance: capability.CapabilityProviderInstance{ID: "workspace-instance"},
		ExecutionTarget:  capability.InvocationExecutionTarget{Placement: "device", SpaceID: "realm", DeviceID: "phone-b"},
	}}
	facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "center-device"}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
		RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "phone-a"},
		Metadata: map[string]any{
			"ownedDeviceTarget": true,
			"workspaceDeviceId": "phone-b",
		},
	}}
	definition := capability.ToolDefinition{
		ID:           "workspace.write",
		CapabilityID: "workspace.write",
		ExtensionID:  "com.example/third-party-workspace",
		ModuleID:     "builtin-workspace",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeWorkspace},
	}
	resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
	if resolved.target.DeviceID != "phone-b" || resolver.calls != 1 || resolver.request.RequiredDeviceID != "phone-b" || resolver.request.PreferredDeviceID != "phone-b" || resolver.request.RequiredPlacement != capability.ProviderPlacementDevice || resolver.request.AllowCore {
		t.Fatalf("workspace did not route to its owner: resolved=%+v request=%+v", resolved, resolver.request)
	}
	resolver.result.ExecutionTarget.DeviceID = "phone-a"
	if resolved := facade.resolveExecutionTarget(context.Background(), definition, scope); resolved.resolutionCode != "CAPABILITY_TARGET_MISMATCH" || !resolved.target.IsZero() {
		t.Fatalf("conversation target accepted for workspace provider: %+v", resolved)
	}
}

func TestOwnedWorkspaceUsesPrimaryBindingDevice(t *testing.T) {
	resolver := &ownedTargetResolver{result: capability.CapabilityResolution{
		Provider:         capability.CapabilityProviderDefinition{ID: "workspace-provider"},
		ProviderInstance: capability.CapabilityProviderInstance{ID: "workspace-instance"},
		ExecutionTarget:  capability.InvocationExecutionTarget{Placement: "device", SpaceID: "realm", DeviceID: "phone-b"},
	}}
	facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "center-device"}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
		RuntimeTarget:      &execution.RuntimeTarget{Placement: "device", DeviceID: "phone-a"},
		PrimaryWorkspaceID: "workspace-b",
		WorkspaceBindings: []execution.WorkspaceBinding{
			{ID: "workspace-a", DeviceID: "phone-a"},
			{ID: "workspace-b", DeviceID: "phone-b"},
		},
		Metadata: map[string]any{"ownedDeviceTarget": true},
	}}
	definition := capability.ToolDefinition{
		ID:           "workspace.write",
		CapabilityID: "workspace.write",
		ExtensionID:  "com.amitia.builtin.workspace",
		Runtime:      capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeWorkspace},
	}
	resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
	if resolved.target.DeviceID != "phone-b" || resolver.calls != 1 || resolver.request.RequiredDeviceID != "phone-b" || resolver.request.RequiredPlacement != capability.ProviderPlacementDevice || resolver.request.AllowCore {
		t.Fatalf("workspace did not use primary binding device: resolved=%+v request=%+v", resolved, resolver.request)
	}
}

func TestOwnedLocalJavaScriptPluginUsesLocalDeviceProvider(t *testing.T) {
	for _, runtime := range []capability.RuntimeType{capability.RuntimeTypeJavaScript, capability.RuntimeTypePluginJS} {
		t.Run(string(runtime), func(t *testing.T) {
			resolver := &ownedTargetResolver{result: capability.CapabilityResolution{
				Provider:         capability.CapabilityProviderDefinition{ID: "local-plugin-provider"},
				ProviderInstance: capability.CapabilityProviderInstance{ID: "local-plugin-instance"},
				ExecutionTarget:  capability.InvocationExecutionTarget{Placement: "device", SpaceID: "realm", DeviceID: "local-device"},
			}}
			facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "local-device"}
			scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
				RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "local-device"},
				Metadata:      map[string]any{"ownedDeviceTarget": true},
			}}
			definition := capability.ToolDefinition{
				ID:           "com.example/plugin/ai",
				CapabilityID: "com.example/plugin/ai",
				ExtensionID:  "com.example/plugin",
				Runtime:      capability.RuntimeBinding{RuntimeType: runtime, Metadata: map[string]any{"modulePlacement": "device"}},
			}
			resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
			if resolved.target.Placement != "device" || resolved.target.DeviceID != "local-device" || resolver.request.RequiredPlacement != capability.ProviderPlacementDevice || resolver.request.RequiredDeviceID != "local-device" || resolver.request.AllowCore {
				t.Fatalf("local %s plugin did not use local device provider: resolved=%+v request=%+v", runtime, resolved, resolver.request)
			}
		})
	}
}

func TestOwnedDirectBuiltinSkipsCapabilityRecovery(t *testing.T) {
	resolver := &ownedTargetResolver{}
	facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "local-device"}
	scope := InvocationScope{SpaceID: "realm", ExecContext: &execution.ExecutionContext{
		RuntimeTarget: &execution.RuntimeTarget{Placement: "device", DeviceID: "local-device"},
		Metadata:      map[string]any{"ownedDeviceTarget": true},
	}}
	definition := capability.ToolDefinition{
		ID:      "builtin.agent_admin.calculate",
		Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeBuiltin, HandlerName: "calculate"},
	}
	resolved := facade.resolveExecutionTarget(context.Background(), definition, scope)
	if !resolved.target.IsZero() || resolved.missingCapability != "" || resolver.calls != 0 {
		t.Fatalf("direct builtin unexpectedly required a provider: %+v calls=%d", resolved, resolver.calls)
	}
}

func TestLocalJavaScriptToolFallsBackToRegisteredRuntime(t *testing.T) {
	for _, runtime := range []capability.RuntimeType{capability.RuntimeTypeJavaScript, capability.RuntimeTypePluginJS} {
		resolver := &ownedTargetResolver{err: capability.ErrNoAvailableProvider}
		facade := &ToolFacade{capabilityResolver: resolver, localDeviceID: "local-device"}
		definition := capability.ToolDefinition{
			ID:           "com.example/plugin/ai",
			CapabilityID: "com.example/plugin/ai",
			ExtensionID:  "com.example/plugin",
			ModuleID:     "plugin-runtime",
			Runtime:      capability.RuntimeBinding{RuntimeType: runtime, HandlerName: "ai"},
		}
		resolved := facade.resolveExecutionTarget(context.Background(), definition, InvocationScope{SpaceID: "realm"})
		if !resolved.target.IsZero() || resolved.missingCapability != "" || resolver.calls != 1 {
			t.Fatalf("local %s tool did not fall back to registered runtime: %+v calls=%d", runtime, resolved, resolver.calls)
		}
	}
}
