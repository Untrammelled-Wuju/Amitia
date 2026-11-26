//go:build rsi_lifecycle

package runtime_supervisor

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func hostSpec(definition string, runtimeType domain.RuntimeType) InstanceSpec {
	return InstanceSpec{
		DefinitionID: DefinitionID(definition),
		ExtensionID:  "com.example/host",
		ModuleID:     "main",
		RuntimeType:  runtimeType,
		Generation:   1,
		Strategy:     StrategySingletonPerModule,
		Limits: ResourceLimits{
			MaxMemoryBytes:     64 * 1024 * 1024,
			MaxConcurrentCalls: 2,
			MaxQueueDepth:      8,
		},
		MaxRestarts:   3,
		RestartWindow: 0,
	}
}

func hostLog(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("m06-s08 "+format, args...)
}

func hostSupervisor(t *testing.T, runtimeTypes ...domain.RuntimeType) (*DefaultSupervisor, map[domain.RuntimeType]*FakeRuntime) {
	t.Helper()
	supervisor := NewDefaultSupervisor()
	runtimes := map[domain.RuntimeType]*FakeRuntime{}
	for _, runtimeType := range runtimeTypes {
		runtime := NewFakeRuntime()
		runtimes[runtimeType] = runtime
		if err := supervisor.RegisterFactory(NewFakeFactory(runtimeType, runtime)); err != nil {
			t.Fatalf("RegisterFactory %s: %v", runtimeType, err)
		}
	}
	return supervisor, runtimes
}

func hostReconcile(t *testing.T, supervisor *DefaultSupervisor, definition string, runtimeType domain.RuntimeType) InstanceSnapshot {
	t.Helper()
	result := supervisor.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: DefinitionID(definition),
		Desired:      DesiredRunning,
		Spec:         hostSpec(definition, runtimeType),
	})
	if result.Error != nil {
		t.Fatalf("Reconcile %s: %v", definition, result.Error)
	}
	instance, err := supervisor.GetInstance(context.Background(), result.InstanceID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	return instance
}

func TestHostIsolationThreeRuntimeTypes(t *testing.T) {
	supervisor, runtimes := hostSupervisor(t, domain.RuntimeTypeBuiltin, domain.RuntimeTypeJavaScript, domain.RuntimeTypeWASM)
	builtin := hostReconcile(t, supervisor, "def-native", domain.RuntimeTypeBuiltin)
	script := hostReconcile(t, supervisor, "def-script", domain.RuntimeTypeJavaScript)
	wasm := hostReconcile(t, supervisor, "def-wasm", domain.RuntimeTypeWASM)
	if builtin.InstanceID == script.InstanceID || script.InstanceID == wasm.InstanceID || builtin.InstanceID == wasm.InstanceID {
		t.Fatal("runtime types share instance ids")
	}
	if builtin.Actual != ActualReady || script.Actual != ActualReady || wasm.Actual != ActualReady {
		t.Fatalf("states=%s/%s/%s", builtin.Actual, script.Actual, wasm.Actual)
	}
	for _, runtime := range runtimes {
		if runtime.StartCount() != 1 {
			t.Fatalf("runtime started %d times", runtime.StartCount())
		}
	}
	hostLog(t, "isolation types=3 instances=3 starts=3")
}

func TestHostIsolationKillOneKeepsOthersHealthy(t *testing.T) {
	supervisor, runtimes := hostSupervisor(t, domain.RuntimeTypeBuiltin, domain.RuntimeTypeJavaScript, domain.RuntimeTypeWASM)
	builtin := hostReconcile(t, supervisor, "def-native", domain.RuntimeTypeBuiltin)
	script := hostReconcile(t, supervisor, "def-script", domain.RuntimeTypeJavaScript)
	wasm := hostReconcile(t, supervisor, "def-wasm", domain.RuntimeTypeWASM)
	if err := supervisor.Stop(context.Background(), wasm.InstanceID, StopReasonCrash); err != nil {
		t.Fatalf("stop wasm: %v", err)
	}
	after, err := supervisor.GetInstance(context.Background(), wasm.InstanceID)
	if err != nil {
		t.Fatalf("get wasm: %v", err)
	}
	if after.Actual != ActualStopped {
		t.Fatalf("wasm state=%s want stopped", after.Actual)
	}
	for _, id := range []string{builtin.InstanceID, script.InstanceID} {
		instance, err := supervisor.GetInstance(context.Background(), id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if instance.Actual != ActualReady || instance.Health != HealthHealthy {
			t.Fatalf("instance %s became %s/%s", id, instance.Actual, instance.Health)
		}
	}
	if runtimes[domain.RuntimeTypeWASM].StopCount() != 1 {
		t.Fatalf("wasm stop count=%d", runtimes[domain.RuntimeTypeWASM].StopCount())
	}
	hostLog(t, "kill_isolated wasm=stopped others=ready healthy=2")
}

func TestHostIsolationColdStartFailureIsolated(t *testing.T) {
	supervisor, runtimes := hostSupervisor(t, domain.RuntimeTypeBuiltin, domain.RuntimeTypeJavaScript)
	builtin := hostReconcile(t, supervisor, "def-native", domain.RuntimeTypeBuiltin)
	wasmFactory := NewFakeFactory(domain.RuntimeTypeWASM, NewFakeRuntime())
	wasmFactory.SetCreateErr(errors.New("wasm cold start failed"))
	if err := supervisor.RegisterFactory(wasmFactory); err != nil {
		t.Fatalf("RegisterFactory wasm: %v", err)
	}
	result := supervisor.Reconcile(context.Background(), ReconcileRequest{
		DefinitionID: "def-wasm",
		Desired:      DesiredRunning,
		Spec:         hostSpec("def-wasm", domain.RuntimeTypeWASM),
	})
	if result.Error == nil {
		t.Fatal("cold start failure was not reported")
	}
	instance, err := supervisor.GetInstance(context.Background(), builtin.InstanceID)
	if err != nil || instance.Actual != ActualReady {
		t.Fatalf("builtin affected by wasm failure: %v %s", err, instance.Actual)
	}
	if runtimes[domain.RuntimeTypeBuiltin].StopCount() != 0 {
		t.Fatal("builtin runtime was stopped by another type's failure")
	}
	hostLog(t, "cold_start_failure isolated=true native=ready")
}

func TestHostIsolationOldFlowUnchanged(t *testing.T) {
	supervisor, runtimes := hostSupervisor(t, domain.RuntimeTypeBuiltin)
	instance := hostReconcile(t, supervisor, "def-legacy", domain.RuntimeTypeBuiltin)
	result := supervisor.Invoke(context.Background(), InvocationRequest{
		InstanceID:   instance.InstanceID,
		InvocationID: "inv-legacy",
		Operation:    "echo",
		Input:        []byte("hello"),
	})
	if result.Error != nil || result.Status != "success" {
		t.Fatalf("legacy invoke status=%s err=%v", result.Status, result.Error)
	}
	if runtimes[domain.RuntimeTypeBuiltin].InvokeCount() != 1 {
		t.Fatalf("legacy invoke count=%d", runtimes[domain.RuntimeTypeBuiltin].InvokeCount())
	}
	hostLog(t, "legacy_flow unchanged=true status=success")
}

func TestHostIsolationRestartAfterKill(t *testing.T) {
	supervisor, _ := hostSupervisor(t, domain.RuntimeTypeJavaScript)
	instance := hostReconcile(t, supervisor, "def-script", domain.RuntimeTypeJavaScript)
	if err := supervisor.Stop(context.Background(), instance.InstanceID, StopReasonCrash); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := supervisor.Restart(context.Background(), instance.InstanceID); err != nil {
		t.Fatalf("restart: %v", err)
	}
	after, err := supervisor.GetInstance(context.Background(), instance.InstanceID)
	if err != nil {
		t.Fatalf("get after restart: %v", err)
	}
	if after.Actual != ActualReady {
		t.Fatalf("state after restart=%s", after.Actual)
	}
	hostLog(t, "restart_after_kill ready=true")
}

func TestHostIsolationCancelledReconcile(t *testing.T) {
	supervisor, _ := hostSupervisor(t, domain.RuntimeTypeBuiltin)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := supervisor.Reconcile(ctx, ReconcileRequest{
		DefinitionID: "def-cancelled",
		Desired:      DesiredRunning,
		Spec:         hostSpec("def-cancelled", domain.RuntimeTypeBuiltin),
	})
	if result.Error == nil {
		t.Fatal("cancelled reconcile created an instance")
	}
	hostLog(t, "cancelled rejected=true")
}

func TestHostIsolationMaliciousPluginCannotKillCore(t *testing.T) {
	supervisor, _ := hostSupervisor(t, domain.RuntimeTypeBuiltin)
	malicious := hostReconcile(t, supervisor, "def-malicious", domain.RuntimeTypeBuiltin)
	if err := supervisor.Stop(context.Background(), malicious.InstanceID, StopReasonQuarantine); err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	healthy := hostReconcile(t, supervisor, "def-core-adjacent", domain.RuntimeTypeBuiltin)
	if healthy.Actual != ActualReady {
		t.Fatalf("core adjacent instance state=%s", healthy.Actual)
	}
	result := supervisor.Invoke(context.Background(), InvocationRequest{
		InstanceID:   healthy.InstanceID,
		InvocationID: "inv-core",
		Operation:    "health",
	})
	if result.Error != nil || result.Status != "success" {
		t.Fatalf("core invoke after quarantine status=%s err=%v", result.Status, result.Error)
	}
	hostLog(t, "malicious quarantined=true core_healthy=true")
}

func TestHostIsolationRegistryIsolation(t *testing.T) {
	first, _ := hostSupervisor(t, domain.RuntimeTypeBuiltin)
	second, _ := hostSupervisor(t, domain.RuntimeTypeBuiltin)
	firstInstance := hostReconcile(t, first, "def-shared", domain.RuntimeTypeBuiltin)
	secondInstance := hostReconcile(t, second, "def-shared", domain.RuntimeTypeBuiltin)
	if firstInstance.InstanceID == secondInstance.InstanceID {
		t.Fatal("two supervisors shared an instance id")
	}
	if err := first.Stop(context.Background(), firstInstance.InstanceID, StopReasonManual); err != nil {
		t.Fatalf("stop first: %v", err)
	}
	instance, err := second.GetInstance(context.Background(), secondInstance.InstanceID)
	if err != nil || instance.Actual != ActualReady {
		t.Fatalf("second supervisor affected: %v %s", err, instance.Actual)
	}
	hostLog(t, "registry_isolation supervisors=2 independent=true")
}
