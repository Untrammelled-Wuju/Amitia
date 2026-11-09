package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type testStorageProvider struct {
	method    host_api.Method
	operation string
	input     json.RawMessage
}

func (p *testStorageProvider) ExecuteStorage(
	_ context.Context,
	_ host_api.CallRequest,
	method host_api.Method,
	operation string,
	input json.RawMessage,
) (any, error) {
	p.method = method
	p.operation = operation
	p.input = append(json.RawMessage(nil), input...)
	return map[string]any{"ok": true}, nil
}

func TestHostStorageRoutesDispatchByCapability(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	provider := &testStorageProvider{}
	bridge := NewStorageBridge()
	bridge.SetProvider(provider)
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{Storage: bridge}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []host_api.Method{
		host_api.MethodStorageCleanup,
		host_api.MethodStorageBackup,
		host_api.MethodStorageRestore,
	} {
		operation := "preview"
		if method == host_api.MethodStorageBackup {
			operation = "list"
		}
		result := gateway.Call(context.Background(), host_api.CallRequest{
			CallID: "storage-" + string(method),
			RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
				InstanceID:  "runtime-1",
				ExtensionID: domain.ExtensionID("com.amitia/storage"),
				ModuleID:    "storage-runtime",
			},
			Method:  method,
			Version: 1,
			Input:   json.RawMessage(`{"operation":"` + operation + `","input":{"name":"backup"}}`),
		})
		if result.Status != host_api.StatusSuccess {
			t.Fatalf("%s failed: %s %+v", method, result.Status, result.Error)
		}
		if provider.method != method || provider.operation != operation {
			t.Fatalf("unexpected storage dispatch: %s %s", provider.method, provider.operation)
		}
	}
}
