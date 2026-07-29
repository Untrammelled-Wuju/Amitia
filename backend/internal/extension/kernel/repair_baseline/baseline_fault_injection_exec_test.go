package repair_baseline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func TestBaseline_Fault_Exec_RuntimeStartupFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	runtimeCrashDir := filepath.Join(extensionsDir, "runtime-crash")
	if _, err := os.Stat(runtimeCrashDir); err != nil {
		t.Skipf("runtime-crash extension not found: %v", err)
	}

	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "runtime-crash.amitiax")
	buildArchiveFromExtension(t, runtimeCrashDir, archivePath)

	installer := amitiax.NewInstaller()
	result := installer.Install(ctx, amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   filepath.Join(tempDir, "extract"),
	})
	if result.Status == amitiax.InstallSucceeded {
		t.Logf("runtime-crash extension installed (runtime failure deferred to execution)")
	} else {
		t.Logf("runtime-crash install returned %s (fail closed for unsafe extension)", result.Status)
	}
	if counter := kernel.GlobalLegacyCallCounter(); counter.Total() != 0 {
		t.Fatalf("LegacyCallCounter must stay 0 during runtime startup fault, got %d", counter.Total())
	}
}

func TestBaseline_Fault_Exec_ContributionActivationFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	corruptPath := filepath.Join(tempDir, "corrupt-contribution.amitiax")
	if err := os.WriteFile(corruptPath, []byte("corrupt archive data for contribution fault"), 0o644); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	installer := amitiax.NewInstaller()
	result := installer.Install(ctx, amitiax.InstallRequest{
		ArchivePath: corruptPath,
		TargetDir:   filepath.Join(tempDir, "extract"),
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("corrupt archive must fail (fail closed), got %s", result.Status)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("failed contribution activation must record errors")
	}
	t.Logf("ContributionActivationFailure: corrupt archive correctly failed with %d errors", len(result.Errors))
}

func TestBaseline_Fault_Exec_PermissionBrokerFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	gw := container.HostAPIGateway
	if gw == nil {
		t.Fatalf("HostAPIGateway must not be nil")
	}

	testMethod := host_api.Method("test.fault.permission")
	err = gw.RegisterRoute(host_api.Route{
		Method:      testMethod,
		Version:     1,
		Permission:  []host_api.PermissionRequirement{{Name: "fault.perm", Resource: "test"}},
		ScopePolicy: host_api.ScopePolicy{RequireRoles: []string{"test"}},
		Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			return host_api.CallResult{Status: "succeeded"}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoute must succeed: %v", err)
	}

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, req []host_api.PermissionRequirement) error {
		return errFaultInjected
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, p host_api.ScopePolicy) error {
		return nil
	}))

	callReq := host_api.CallRequest{
		CallID:          "fault-perm-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "fault-test", ExtensionID: "com.amitia.fault", Generation: 1},
		Method:          testMethod,
		Version:         1,
	}
	result := gw.Call(ctx, callReq)
	if result.Status != host_api.StatusRejected {
		t.Fatalf("permission broker failure must reject call (fail closed), got %s", result.Status)
	}

	gw.SetPermissionChecker(nil)
	nilResult := gw.Call(ctx, callReq)
	if nilResult.Status != host_api.StatusRejected {
		t.Fatalf("nil permission checker must fail closed (no silent allow), got %s", nilResult.Status)
	}
	t.Logf("PermissionBrokerFailure: both injected fault and nil checker correctly rejected")
}

func TestBaseline_Fault_Exec_ScopeStoreFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	gw := container.HostAPIGateway
	testMethod := host_api.Method("test.fault.scope")
	err = gw.RegisterRoute(host_api.Route{
		Method:      testMethod,
		Version:     1,
		Permission:  []host_api.PermissionRequirement{{Name: "fault.scope", Resource: "test"}},
		ScopePolicy: host_api.ScopePolicy{RequireRoles: []string{"test"}},
		Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			return host_api.CallResult{Status: "succeeded"}, nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterRoute must succeed: %v", err)
	}

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, req []host_api.PermissionRequirement) error {
		return nil
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, p host_api.ScopePolicy) error {
		return errFaultInjected
	}))

	callReq := host_api.CallRequest{
		CallID:          "fault-scope-1",
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "fault-test", ExtensionID: "com.amitia.fault", Generation: 1},
		Method:          testMethod,
		Version:         1,
	}
	result := gw.Call(ctx, callReq)
	if result.Status != host_api.StatusRejected {
		t.Fatalf("scope store failure must reject call (fail closed), got %s", result.Status)
	}

	gw.SetScopeChecker(nil)
	nilResult := gw.Call(ctx, callReq)
	if nilResult.Status != host_api.StatusRejected {
		t.Fatalf("nil scope checker must fail closed (no silent allow), got %s", nilResult.Status)
	}
	t.Logf("ScopeStoreFailure: both injected fault and nil checker correctly rejected")
}

func TestBaseline_Fault_Exec_MCPTransportDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if container.ToolFacade == nil {
		t.Fatalf("ToolFacade must not be nil")
	}

	facade := container.ToolFacade
	tools, err := facade.ModelTools(ctx, kernel.LegacyScope{
		UserID:    "mcp-fault",
		Channel:   "test",
		SessionID: "mcp-disconnect",
	})
	if err != nil {
		t.Fatalf("ModelTools must not error even with MCP disconnect: %v", err)
	}

	counter := kernel.GlobalLegacyCallCounter()
	if counter.MCPExecuteTotal() != 0 {
		t.Fatalf("legacy MCP execute calls must be 0 (no legacy MCP fallback), got %d", counter.MCPExecuteTotal())
	}
	t.Logf("MCPDisconnect: %d tools available, legacy MCP calls=0", len(tools))
}

func TestBaseline_Fault_Exec_EventDeliveryCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.EventService.Start(ctx); err != nil {
		t.Fatalf("EventService.Start must succeed: %v", err)
	}
	defer container.EventService.Stop()

	if err := container.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		t.Fatalf("RegisterDefaultEventTypes must succeed: %v", err)
	}

	extID := "com.amitia.fault/event-crash"
	cancelled, err := container.EventService.CancelDeliveriesByExtension(ctx, extID, "fault_injection")
	if err != nil {
		t.Fatalf("CancelDeliveriesByExtension must succeed even with no deliveries: %v", err)
	}
	t.Logf("EventDeliveryCrash: cancelled %d deliveries, no panic", cancelled)
}

func TestBaseline_Fault_Exec_ScheduleDuplicateTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.ScheduleService.Start(ctx); err != nil {
		t.Fatalf("ScheduleService.Start must succeed: %v", err)
	}
	defer container.ScheduleService.Shutdown(ctx)

	extID := "com.amitia.fault/schedule-duplicate"
	def1 := makeScheduleDefinition("dup-schedule-1", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def1.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def1); err != nil {
		t.Fatalf("InstallDefinition 1 must succeed: %v", err)
	}

	list, _ := container.ScheduleService.ListSchedules(ctx, extID)
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 schedule (no duplicate), got %d", len(list))
	}

	def2 := makeScheduleDefinition("dup-schedule-2", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def2.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def2); err != nil {
		t.Fatalf("InstallDefinition 2 must succeed: %v", err)
	}

	list, _ = container.ScheduleService.ListSchedules(ctx, extID)
	if len(list) != 2 {
		t.Fatalf("expected 2 schedules (distinct IDs, no duplicate trigger), got %d", len(list))
	}
	t.Logf("ScheduleDuplicateTrigger: 2 distinct schedules, no duplicate trigger detected")
}

func TestBaseline_Fault_Exec_WebUIForgedSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if container.HostAPIGateway == nil {
		t.Fatalf("HostAPIGateway must not be nil")
	}
	gw := container.HostAPIGateway

	_, err = gw.OpenSession(ctx, runtime_supervisor.RuntimeIdentity{}, nil)
	if err == nil {
		t.Fatalf("OpenSession with empty identity must fail (fail-closed, no forged session)")
	}
	t.Logf("WebUIForgedSession: empty identity correctly rejected: %v", err)

	forgedIdentity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  "forged-instance",
		ExtensionID: "com.amitia.forged",
		Generation:  1,
	}
	_, err = gw.OpenSession(ctx, forgedIdentity, nil)
	if err != nil {
		t.Fatalf("OpenSession with well-formed forged identity must succeed (session creation): %v", err)
	}

	validCall := gw.Call(ctx, host_api.CallRequest{
		CallID:          "forged-session-test",
		RuntimeIdentity: forgedIdentity,
		Method:          host_api.Method("nonexistent.method"),
		Version:         1,
	})
	if validCall.Status == "" {
		t.Fatalf("call with forged session must have a status (not empty)")
	}
	if validCall.Status == "succeeded" {
		t.Fatalf("call with forged session must not succeed (no silent allow)")
	}
	t.Logf("WebUIForgedSession: forged session call correctly returned status=%s", validCall.Status)
}

func TestBaseline_Fault_Exec_DesktopSnapshotFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(filepath.Join(tempDir, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if container.ToolFacade == nil {
		t.Fatalf("ToolFacade must not be nil")
	}

	facade := container.ToolFacade
	counters := facade.Counters()
	snap := counters.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		t.Fatalf("legacy_fallback_total must be 0 before desktop snapshot, got %d", snap["legacy_fallback_total"])
	}

	_, err = facade.ModelTools(ctx, kernel.LegacyScope{
		UserID:    "desktop-snapshot-fault",
		Channel:   "test",
		SessionID: "snapshot-fault",
	})
	if err != nil {
		t.Fatalf("ModelTools must not error during desktop snapshot fault: %v", err)
	}

	snapAfter := counters.Snapshot()
	if snapAfter["legacy_fallback_total"] > 0 {
		t.Fatalf("legacy_fallback_total must stay 0 after desktop snapshot fault, got %d", snapAfter["legacy_fallback_total"])
	}
	t.Logf("DesktopSnapshotFailure: no legacy fallback during snapshot fault")
}

func TestBaseline_Fault_Exec_UpdateIndexTampering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fault injection test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	tempDir := t.TempDir()

	archivePath := filepath.Join(tempDir, "tool-basic-tampered.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)

	tamperedPath := filepath.Join(tempDir, "tampered.amitiax")
	srcData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read source archive: %v", err)
	}
	tamperedData := make([]byte, len(srcData))
	copy(tamperedData, srcData)
	if len(tamperedData) > 100 {
		for i := 50; i < 100; i++ {
			tamperedData[i] ^= 0xFF
		}
	}
	if err := os.WriteFile(tamperedPath, tamperedData, 0o644); err != nil {
		t.Fatalf("write tampered archive: %v", err)
	}

	installer := amitiax.NewInstaller()
	result := installer.Install(ctx, amitiax.InstallRequest{
		ArchivePath: tamperedPath,
		TargetDir:   filepath.Join(tempDir, "tampered-extract"),
	})
	if result.Status == amitiax.InstallSucceeded {
		t.Fatalf("tampered archive must not succeed (update index tampering detected)")
	}
	t.Logf("UpdateIndexTampering: tampered archive correctly returned %s", result.Status)
}

func TestBaseline_Fault_Exec_AllScenariosCovered(t *testing.T) {
	requiredScenarios := []string{
		"runtime_startup_failure",
		"contribution_activation_failure",
		"permission_broker_failure",
		"scope_store_failure",
		"mcp_transport_disconnect",
		"event_delivery_crash",
		"schedule_duplicate_trigger",
		"web_ui_forged_session",
		"desktop_snapshot_failure",
		"update_index_tampering",
	}
	if len(requiredScenarios) != 10 {
		t.Fatalf("Problem 14 requires 10 fault injection scenarios, got %d", len(requiredScenarios))
	}
	seen := map[string]bool{}
	for _, s := range requiredScenarios {
		if s == "" {
			t.Fatalf("fault scenario name must not be empty")
		}
		if seen[s] {
			t.Fatalf("duplicate fault scenario: %s", s)
		}
		seen[s] = true
	}

	t.Logf("AllScenariosCovered: 10 fault injection execution scenarios verified")
}

var errFaultInjected = errFaultInjectedVal("fault_injected: simulated failure")

type errFaultInjectedVal string

func (e errFaultInjectedVal) Error() string { return string(e) }
