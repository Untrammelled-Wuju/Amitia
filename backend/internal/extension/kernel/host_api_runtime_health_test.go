package kernel

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func makeHealthTestSupervisor(t *testing.T, extID string) (*runtime_supervisor.DefaultSupervisor, string) {
	t.Helper()
	supervisor := runtime_supervisor.NewDefaultSupervisor()
	rt := runtime_supervisor.NewFakeRuntime()
	_ = supervisor.RegisterFactory(runtime_supervisor.NewFakeFactory(domain.RuntimeTypeBuiltin, rt))
	spec := runtime_supervisor.InstanceSpec{
		DefinitionID: "rt-def-health-test",
		ExtensionID:  domain.ExtensionID(extID),
		ModuleID:     "main",
		RuntimeType:  domain.RuntimeTypeBuiltin,
		Generation:   1,
		Strategy:     runtime_supervisor.StrategySingletonPerModule,
		MaxRestarts:  3,
	}
	startResult := supervisor.Reconcile(context.Background(), runtime_supervisor.ReconcileRequest{
		DefinitionID: "rt-def-health-test",
		Desired:      runtime_supervisor.DesiredRunning,
		Spec:         spec,
	})
	if startResult.Error != nil {
		t.Fatalf("start runtime: %v", startResult.Error)
	}
	return supervisor, startResult.InstanceID
}

func makeHealthTestGateway() *host_api.DefaultGateway {
	gw := host_api.NewDefaultGateway()
	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(_ context.Context, _ runtime_supervisor.RuntimeIdentity, _ []host_api.PermissionRequirement) error {
		return nil
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(_ context.Context, _ runtime_supervisor.RuntimeIdentity, _ string, _ host_api.ScopePolicy) error {
		return nil
	}))
	return gw
}

func TestRuntimeHealthRouteReturnsRealHealth(t *testing.T) {
	supervisor, instanceID := makeHealthTestSupervisor(t, "com.example/health-test")
	gw := makeHealthTestGateway()
	if err := setupDefaultHostAPIRoutes(gw, HostAPIRouteDeps{
		RuntimeSupervisor: supervisor,
	}); err != nil {
		t.Fatalf("setupDefaultHostAPIRoutes: %v", err)
	}
	result := gw.Call(context.Background(), host_api.CallRequest{
		CallID: "call-health-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "session-1",
			ExtensionID: "com.example/health-test",
			ModuleID:    "main",
			Generation:  1,
		},
		Method:   host_api.MethodRuntimeHealth,
		Version:  1,
		Deadline: time.Now().Add(5 * time.Second),
	})
	if result.Status != host_api.StatusSuccess {
		t.Fatalf("expected success, got %s: %+v", result.Status, result.Error)
	}
	var out map[string]any
	_ = json.Unmarshal(result.Output, &out)
	if out["extensionId"] != "com.example/health-test" {
		t.Errorf("expected extensionId com.example/health-test, got %v", out["extensionId"])
	}
	instances, ok := out["instances"].([]any)
	if !ok || len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %v", out["instances"])
	}
	inst := instances[0].(map[string]any)
	if inst["instanceId"] != instanceID {
		t.Errorf("expected instanceId %s, got %v", instanceID, inst["instanceId"])
	}
	if inst["health"] != string(runtime_supervisor.HealthHealthy) {
		t.Errorf("expected healthy, got %v", inst["health"])
	}
	if inst["circuit"] != string(runtime_supervisor.CircuitClosed) {
		t.Errorf("expected circuit closed, got %v", inst["circuit"])
	}
	if inst["actual"] != string(runtime_supervisor.ActualReady) {
		t.Errorf("expected ready, got %v", inst["actual"])
	}
	if inst["quarantined"] != false {
		t.Errorf("expected not quarantined, got %v", inst["quarantined"])
	}
	if inst["generation"] != float64(1) {
		t.Errorf("expected generation 1, got %v", inst["generation"])
	}
}

func TestRuntimeHealthRouteCrossExtensionRejected(t *testing.T) {
	supervisor, _ := makeHealthTestSupervisor(t, "com.example/owner")
	gw := makeHealthTestGateway()
	if err := setupDefaultHostAPIRoutes(gw, HostAPIRouteDeps{
		RuntimeSupervisor: supervisor,
	}); err != nil {
		t.Fatalf("setupDefaultHostAPIRoutes: %v", err)
	}
	result := gw.Call(context.Background(), host_api.CallRequest{
		CallID: "call-health-cross",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "session-2",
			ExtensionID: "com.example/attacker",
			ModuleID:    "main",
			Generation:  1,
		},
		Method:   host_api.MethodRuntimeHealth,
		Version:  1,
		Deadline: time.Now().Add(5 * time.Second),
	})
	if result.Status != host_api.StatusFailed {
		t.Fatalf("expected failed for cross-extension, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if result.Error.Code != host_api.ErrorCodeResourceNotFound {
		t.Errorf("expected resource_not_found, got %s", result.Error.Code)
	}
}

func TestRuntimeHealthRouteNoInstances(t *testing.T) {
	supervisor := runtime_supervisor.NewDefaultSupervisor()
	gw := makeHealthTestGateway()
	if err := setupDefaultHostAPIRoutes(gw, HostAPIRouteDeps{
		RuntimeSupervisor: supervisor,
	}); err != nil {
		t.Fatalf("setupDefaultHostAPIRoutes: %v", err)
	}
	result := gw.Call(context.Background(), host_api.CallRequest{
		CallID: "call-health-none",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "session-3",
			ExtensionID: "com.example/no-runtime",
			ModuleID:    "main",
			Generation:  1,
		},
		Method:   host_api.MethodRuntimeHealth,
		Version:  1,
		Deadline: time.Now().Add(5 * time.Second),
	})
	if result.Status != host_api.StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if result.Error.Code != host_api.ErrorCodeResourceNotFound {
		t.Errorf("expected resource_not_found, got %s", result.Error.Code)
	}
}

func TestRuntimeHealthRouteSupervisorNil(t *testing.T) {
	gw := makeHealthTestGateway()
	if err := setupDefaultHostAPIRoutes(gw, HostAPIRouteDeps{}); err != nil {
		t.Fatalf("setupDefaultHostAPIRoutes: %v", err)
	}
	result := gw.Call(context.Background(), host_api.CallRequest{
		CallID: "call-health-nil",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID:  "session-4",
			ExtensionID: "com.example/test",
			ModuleID:    "main",
			Generation:  1,
		},
		Method:   host_api.MethodRuntimeHealth,
		Version:  1,
		Deadline: time.Now().Add(5 * time.Second),
	})
	if result.Status != host_api.StatusFailed {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error == nil {
		t.Fatal("expected error to be non-nil")
	}
	if result.Error.Code != host_api.ErrorCodeHostUnavailable {
		t.Errorf("expected host_unavailable, got %s", result.Error.Code)
	}
}

func TestRuntimeHealthRoutePermissionMappingExists(t *testing.T) {
	perm := host_api.RoutePermissionForMethod(host_api.MethodRuntimeHealth)
	if perm == nil {
		t.Fatal("expected permission mapping for MethodRuntimeHealth")
	}
	if len(perm) != 1 {
		t.Fatalf("expected 1 permission requirement, got %d", len(perm))
	}
	if perm[0].Name != "runtime.health.read" {
		t.Errorf("expected permission runtime.health.read, got %s", perm[0].Name)
	}
	if perm[0].Resource != "runtime" {
		t.Errorf("expected resource runtime, got %s", perm[0].Resource)
	}
}

func TestRuntimeHealthRouteScopePolicyExists(t *testing.T) {
	policy := host_api.RouteScopeForMethod(host_api.MethodRuntimeHealth)
	if !policy.Namespaced {
		t.Error("expected Namespaced=true for MethodRuntimeHealth")
	}
}

func TestRuntimeHealthRouteIsDataRoute(t *testing.T) {
	if !host_api.IsDataRouteMethod(host_api.MethodRuntimeHealth) {
		t.Error("expected MethodRuntimeHealth to be a data route method")
	}
}
