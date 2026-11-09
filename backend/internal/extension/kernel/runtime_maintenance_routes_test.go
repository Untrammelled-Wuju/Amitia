package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type testRuntimeMaintenanceProvider struct{}

func (testRuntimeMaintenanceProvider) Overview(context.Context) (any, error) {
	return map[string]any{"running": true, "modules": []any{}}, nil
}

func (testRuntimeMaintenanceProvider) Status(context.Context) (any, error) {
	return map[string]any{"running": true}, nil
}

func (testRuntimeMaintenanceProvider) Config(context.Context) (any, error) {
	return map[string]any{"maxTasks": 4}, nil
}

func (testRuntimeMaintenanceProvider) UpdateConfig(_ context.Context, input json.RawMessage) (any, error) {
	return map[string]any{"updated": true, "input": json.RawMessage(input)}, nil
}

func (testRuntimeMaintenanceProvider) CleanupTemp(context.Context) (any, error) {
	return map[string]any{"cleaned": true}, nil
}

func (testRuntimeMaintenanceProvider) RotateLogs(context.Context) (any, error) {
	return map[string]any{"rotated": true}, nil
}

func (testRuntimeMaintenanceProvider) CheckDBIntegrity(context.Context) (any, error) {
	return map[string]any{"ok": true}, nil
}

func TestHostRuntimeMaintenanceRoutesUseBridge(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	bridge := NewRuntimeMaintenanceBridge()
	bridge.SetProvider(testRuntimeMaintenanceProvider{})
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{RuntimeMaintenance: bridge}); err != nil {
		t.Fatal(err)
	}
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: domain.ExtensionID("com.example/maintenance"),
		ModuleID:    "main",
	}
	for _, method := range []host_api.Method{
		host_api.MethodRuntimeMaintenanceStatus,
		host_api.MethodRuntimeMaintenanceOverview,
		host_api.MethodRuntimeMaintenanceConfig,
		host_api.MethodRuntimeMaintenanceUpdate,
		host_api.MethodRuntimeCleanupTemp,
		host_api.MethodRuntimeRotateLogs,
		host_api.MethodRuntimeCheckDBIntegrity,
	} {
		result := gateway.Call(context.Background(), host_api.CallRequest{
			CallID:          "maintenance-" + string(method),
			RuntimeIdentity: identity,
			Method:          method,
			Version:         1,
			Input:           json.RawMessage(`{"maxTasks":4}`),
		})
		if result.Status != host_api.StatusSuccess {
			t.Fatalf("%s failed: %s %+v", method, result.Status, result.Error)
		}
	}
}
