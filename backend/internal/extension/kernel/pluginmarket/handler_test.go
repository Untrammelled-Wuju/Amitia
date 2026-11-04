package pluginmarket

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type fakeHost struct {
	calls  []host_api.CallRequest
	output map[host_api.Method]json.RawMessage
}

func (f *fakeHost) Call(_ context.Context, request host_api.CallRequest) host_api.CallResult {
	f.calls = append(f.calls, request)
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: f.output[request.Method]}
}

func TestPluginMarketCatalogUsesHostNetworkAPI(t *testing.T) {
	host := &fakeHost{output: map[host_api.Method]json.RawMessage{
		host_api.MethodNetworkRequest: json.RawMessage(`{"status":200,"body":{"code":200,"msg":"ok","data":{"categories":[{"id":1,"slug":"developer","name":"开发工具"}],"packages":[{"id":42,"extensionId":"com.example/tools","categoryId":1,"displayName":"Tools","summary":"Tools","description":"Tools plugin","publisherId":"com.example","trustLevel":"community","securityScore":80,"status":"published","latestVersion":"1.0.0","category":{"id":1,"slug":"developer","name":"开发工具"},"versions":[{"id":7,"version":"1.0.0","originalName":"tools.amitiax","size":42,"sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","status":"published"}]}]}}}`),
	}}
	handler := New(host, "https://market.example/api/v1/market/catalog")
	result := handler.Execute(context.Background(), capability.ToolInvocationContext{
		InvocationID: "invoke-1", SpaceID: "space-1", ExtensionID: ExtensionID, ModuleID: RuntimeModuleID,
	}, json.RawMessage(`{"action":"catalog"}`))
	if result.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("catalog failed: %+v", result.Error)
	}
	var output struct {
		Listings []listingView `json:"listings"`
	}
	if err := json.Unmarshal(result.Structured, &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Listings) != 1 || output.Listings[0].PackageURI != "https://market.example/api/v1/market/packages/42/download?version=1.0.0" {
		t.Fatalf("unexpected listings: %+v", output.Listings)
	}
	if len(host.calls) != 1 || host.calls[0].Method != host_api.MethodNetworkRequest {
		t.Fatalf("catalog did not use generic network API: %+v", host.calls)
	}
}

func TestPluginMarketPrepareSessionCanCompleteInstall(t *testing.T) {
	host := &fakeHost{output: map[host_api.Method]json.RawMessage{
		host_api.MethodPluginPreview: json.RawMessage(`{"sessionId":"preview-1","extensionId":"com.example/tools","version":"1.0.0","publisher":"com.example","signatureStatus":"valid","trustDecision":"trusted","securityPassed":true,"requiredConfirmations":["confirm.scripts"],"riskFlags":[],"issues":[],"permissions":[]}`),
		host_api.MethodPluginInstall: json.RawMessage(`{"operationId":"operation-1","extensionId":"com.example/tools","version":"1.0.0"}`),
	}}
	handler := New(host, DefaultCatalogURL)
	prepared := handler.Execute(context.Background(), capability.ToolInvocationContext{
		InvocationID: "invoke-prepare", SpaceID: "space-1", ExtensionID: ExtensionID, ModuleID: RuntimeModuleID,
	}, json.RawMessage(`{"action":"prepare","extensionId":"com.example/tools","version":"1.0.0","packageUri":"https://market.example/tools.amitiax","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`))
	if prepared.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("prepare failed: %+v", prepared.Error)
	}
	var preview struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(prepared.Structured, &preview); err != nil {
		t.Fatal(err)
	}
	if preview.SessionID != "preview-1" {
		t.Fatalf("session id = %q, want preview-1", preview.SessionID)
	}
	installed := handler.Execute(context.Background(), capability.ToolInvocationContext{
		InvocationID: "invoke-install", SpaceID: "space-1", ExtensionID: ExtensionID, ModuleID: RuntimeModuleID,
	}, json.RawMessage(`{"action":"install","extensionId":"com.example/tools","version":"1.0.0","sessionId":"preview-1","requiredConfirmations":["confirm.scripts"],"confirmed":true}`))
	if installed.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("install failed: %+v", installed.Error)
	}
}

func TestPluginMarketInstallRequiresConfirmationAndUsesHostAPI(t *testing.T) {
	host := &fakeHost{output: map[host_api.Method]json.RawMessage{
		host_api.MethodPluginInstall: json.RawMessage(`{"operationId":"operation-1","extensionId":"com.example/tools","version":"1.0.0"}`),
	}}
	handler := New(host, DefaultCatalogURL)
	rejected := handler.Execute(context.Background(), capability.ToolInvocationContext{
		InvocationID: "invoke-2", SpaceID: "space-1", ExtensionID: ExtensionID, ModuleID: RuntimeModuleID,
	}, json.RawMessage(`{"action":"install","extensionId":"com.example/tools","version":"1.0.0","sessionId":"preview-1"}`))
	if rejected.Status != capability.ToolResultStatusFailed {
		t.Fatal("install without confirmation must fail")
	}
	accepted := handler.Execute(context.Background(), capability.ToolInvocationContext{
		InvocationID: "invoke-3", SpaceID: "space-1", ExtensionID: ExtensionID, ModuleID: RuntimeModuleID,
	}, json.RawMessage(`{"action":"install","extensionId":"com.example/tools","version":"1.0.0","sessionId":"preview-1","requiredConfirmations":["confirm.scripts"],"confirmed":true}`))
	if accepted.Status != capability.ToolResultStatusSuccess {
		t.Fatalf("confirmed install failed: %+v", accepted.Error)
	}
	if len(host.calls) != 1 || host.calls[0].Method != host_api.MethodPluginInstall {
		t.Fatalf("install did not use generic plugin API: %+v", host.calls)
	}
	if !strings.Contains(string(host.calls[0].Input), `"confirm.scripts":true`) {
		t.Fatalf("required confirmation not forwarded: %s", host.calls[0].Input)
	}
}
