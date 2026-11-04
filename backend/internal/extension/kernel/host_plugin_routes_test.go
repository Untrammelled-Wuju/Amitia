package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type fakeHostPluginPackageManager struct {
	prepareRequest PrepareHostPluginPackageRequest
	installRequest InstallHostPluginPackageRequest
}

func (f *fakeHostPluginPackageManager) Prepare(_ context.Context, request PrepareHostPluginPackageRequest) (InstallPreview, error) {
	f.prepareRequest = request
	return InstallPreview{
		SessionID: "preview-1", ExtensionID: request.ExtensionID, Version: request.Version,
		Installable: true, SecurityPassed: true, RequiredConfirmations: []string{"confirm.scripts"},
	}, nil
}

func (f *fakeHostPluginPackageManager) Install(_ context.Context, request InstallHostPluginPackageRequest) (KernelInstallResult, error) {
	f.installRequest = request
	return KernelInstallResult{ExtensionID: request.ExtensionID, Version: request.Version, OperationID: "operation-1"}, nil
}

func TestHostPluginPackageRoutesUseGenericManager(t *testing.T) {
	manager := &fakeHostPluginPackageManager{}
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{PluginPackages: manager}); err != nil {
		t.Fatal(err)
	}
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID: "runtime-1", ExtensionID: domain.ExtensionID("com.example/market"), ModuleID: "main",
	}
	previewResult := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "preview", RuntimeIdentity: identity, Method: host_api.MethodPluginPreview, Version: 1,
		Input: json.RawMessage(`{"extensionId":"com.example/tools","version":"1.0.0","packageUri":"https://plugins.example.com/tools.amitiax","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`),
	})
	if previewResult.Status != host_api.StatusSuccess {
		t.Fatalf("preview failed: %s %+v", previewResult.Status, previewResult.Error)
	}
	installResult := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "install", RuntimeIdentity: identity, Method: host_api.MethodPluginInstall, Version: 1,
		Input: json.RawMessage(`{"extensionId":"com.example/tools","version":"1.0.0","previewSessionId":"preview-1","confirmations":{"confirm.scripts":true}}`),
	})
	if installResult.Status != host_api.StatusSuccess {
		t.Fatalf("install failed: %s %+v", installResult.Status, installResult.Error)
	}
	if manager.prepareRequest.ExtensionID != "com.example/tools" || manager.installRequest.PreviewSessionID != "preview-1" {
		t.Fatalf("generic package manager was not used: prepare=%+v install=%+v", manager.prepareRequest, manager.installRequest)
	}
	if !manager.installRequest.Confirmations["confirm.scripts"] {
		t.Fatalf("explicit confirmations were not forwarded: %+v", manager.installRequest)
	}
}
