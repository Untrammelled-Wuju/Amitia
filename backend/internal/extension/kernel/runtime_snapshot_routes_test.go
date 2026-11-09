package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func TestHostRuntimeSnapshotUsesSharedObservabilityContract(t *testing.T) {
	gateway := host_api.NewDefaultGateway()
	gateway.SetPermissionChecker(host_api.PermissionCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, []host_api.PermissionRequirement) error {
		return nil
	}))
	gateway.SetScopeChecker(host_api.ScopeCheckerFunc(func(context.Context, runtime_supervisor.RuntimeIdentity, string, host_api.ScopePolicy) error {
		return nil
	}))
	bridge := NewRuntimeObservabilityBridge(nil)
	if err := setupDefaultHostAPIRoutes(gateway, HostAPIRouteDeps{RuntimeObservability: bridge}); err != nil {
		t.Fatal(err)
	}
	result := gateway.Call(context.Background(), host_api.CallRequest{
		CallID: "runtime-snapshot-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "runtime-1",
			ExtensionID: domain.ExtensionID("com.example/runtime-insights"),
			ModuleID:    "main",
		},
		Method:  host_api.MethodRuntimeSnapshot,
		Version: 1,
		Input:   json.RawMessage(`{}`),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("runtime snapshot failed: %s %+v", result.Status, result.Error)
	}
	var output struct {
		Meta struct {
			GeneratedAt string `json:"generatedAt"`
		} `json:"meta"`
		Summary map[string]any `json:"summary"`
	}
	if err := json.Unmarshal(result.Output, &output); err != nil {
		t.Fatal(err)
	}
	if output.Meta.GeneratedAt == "" || output.Summary == nil {
		t.Fatalf("unexpected runtime snapshot: %+v", output)
	}
}
