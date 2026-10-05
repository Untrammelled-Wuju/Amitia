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
	for _, runtime := range []capability.RuntimeType{capability.RuntimeTypeGameHost, capability.RuntimeTypeBrowser, capability.RuntimeTypePluginJS, capability.RuntimeTypeWorkspace, capability.RuntimeTypePluginService} {
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
