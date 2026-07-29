package repair_baseline

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

func TestBaseline_E2E_FullChain_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-chain E2E test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")
	if err := os.MkdirAll(extRoot, 0o755); err != nil {
		t.Fatalf("mkdir ext root: %v", err)
	}

	counter := kernel.GlobalLegacyCallCounter()
	initialTotal := counter.Total()

	t.Run("Pack", func(t *testing.T) {
		extensionsDir := testExtensionsDir(t)
		toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
		archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		if _, err := os.Stat(archivePath); err != nil {
			t.Fatalf("packed archive must exist: %v", err)
		}
		t.Logf("Pack: archive built at %s", archivePath)
	})

	t.Run("Sign", func(t *testing.T) {
		installer := amitiax.NewInstaller()
		result := installer.Install(ctx, amitiax.InstallRequest{
			ArchivePath:   filepath.Join(tempDir, "tool-basic.amitiax"),
			TargetDir:     filepath.Join(extRoot, "sign-check"),
			RequireSigned: true,
		})
		if result.Status != amitiax.InstallFailed {
			t.Fatalf("unsigned archive with RequireSigned must fail (sign enforcement), got %s", result.Status)
		}
		t.Logf("Sign: RequireSigned correctly rejects unsigned archive")
	})

	container, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("Container.Recover must succeed: %v", err)
	}

	t.Run("Install", func(t *testing.T) {
		extensionsDir := testExtensionsDir(t)
		toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
		archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
		if _, err := os.Stat(archivePath); err != nil {
			buildArchiveFromExtension(t, toolBasicDir, archivePath)
		}
		targetDir := filepath.Join(extRoot, "tool-basic")
		result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("tool-basic install must succeed: %v", result.Errors)
		}
		if result.Definition.ID == "" {
			t.Fatalf("installed extension must have non-empty ID")
		}
		if err := container.DefinitionRepository.PutExtension(ctx, result.Definition); err != nil {
			t.Fatalf("PutExtension must succeed: %v", err)
		}
		t.Logf("Install: extension %s installed successfully", result.Definition.ID)
	})

	t.Run("GrantPermission", func(t *testing.T) {
		if container.HostAPIGateway == nil {
			t.Fatalf("HostAPIGateway must not be nil")
		}
		gw := container.HostAPIGateway
		testMethod := host_api.Method("test.fullchain.verify")
		err := gw.RegisterRoute(host_api.Route{
			Method:      testMethod,
			Version:     1,
			Permission:  []host_api.PermissionRequirement{{Name: "test.fullchain.perm", Resource: "test"}},
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
			return nil
		}))
		result := gw.Call(ctx, host_api.CallRequest{
			CallID:          "fullchain-1",
			RuntimeIdentity: runtime_supervisor.RuntimeIdentity{InstanceID: "fullchain", ExtensionID: "com.amitia.repair/tool-basic", Generation: 1},
			Method:          testMethod,
			Version:         1,
		})
		if result.Status == host_api.StatusRejected {
			t.Fatalf("call with granted permission must not be rejected")
		}
		t.Logf("GrantPermission: HostAPI call with permission passed, status=%s", result.Status)
	})

	t.Run("BindScope", func(t *testing.T) {
		if container.ScopeManager == nil {
			t.Fatalf("ScopeManager must not be nil")
		}
		binding, err := container.ScopeManager.Bind(ctx, scope.ScopeBindRequest{
			SubjectType: scope.SubjectExtension,
			SubjectID:   "com.amitia.repair/tool-basic",
			Scope: scope.ScopeRef{
				Type:        scope.ScopeExtension,
				ExtensionID: "com.amitia.repair/tool-basic",
			},
			Source: scope.SourceSystem,
		})
		if err != nil {
			t.Fatalf("ScopeManager.Bind must succeed: %v", err)
		}
		if binding.BindingID == "" {
			t.Fatalf("binding must have non-empty ID")
		}
		t.Logf("BindScope: scope binding %s created", binding.BindingID)
	})

	t.Run("Enable", func(t *testing.T) {
		if container.EnablementService == nil {
			t.Fatalf("EnablementService must not be nil")
		}
		subject := enablement.StateSubject{
			Kind: enablement.SubjectExtension,
			ID:   "com.amitia.repair/tool-basic",
		}
		if err := container.EnablementService.Enable(ctx, subject); err != nil {
			t.Fatalf("EnablementService.Enable must succeed: %v", err)
		}
		t.Logf("Enable: extension enabled successfully")
	})

	t.Run("Execute", func(t *testing.T) {
		if container.ToolFacade == nil {
			t.Fatalf("ToolFacade must not be nil")
		}
		tools, err := container.ToolFacade.ModelTools(ctx, kernel.LegacyScope{
			UserID:    "fullchain",
			Channel:   "test",
			SessionID: "fullchain-exec",
		})
		if err != nil {
			t.Fatalf("ToolFacade.ModelTools must not error: %v", err)
		}
		t.Logf("Execute: ModelTools returned %d tools without error", len(tools))
	})

	t.Run("HookEventSchedule", func(t *testing.T) {
		if container.ScheduleService == nil || container.EventService == nil {
			t.Fatalf("ScheduleService and EventService must not be nil")
		}
		if err := container.ScheduleService.Start(ctx); err != nil {
			t.Fatalf("ScheduleService.Start must succeed: %v", err)
		}
		defer container.ScheduleService.Shutdown(ctx)

		if err := container.EventService.Start(ctx); err != nil {
			t.Fatalf("EventService.Start must succeed: %v", err)
		}
		defer container.EventService.Stop()

		if err := container.EventService.RegisterDefaultEventTypes(ctx); err != nil {
			t.Fatalf("RegisterDefaultEventTypes must succeed: %v", err)
		}
		registerTestEventType(t, ctx, container.EventService)

		extID := "com.amitia.repair/tool-basic"
		schedDef := makeScheduleDefinition("fullchain-schedule", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
		schedDef.Trigger.Interval = nil
		if err := container.ScheduleService.InstallDefinition(ctx, schedDef); err != nil {
			t.Fatalf("InstallDefinition must succeed: %v", err)
		}

		subDef := event.EventSubscriptionDefinition{
			ContributionID:    "fullchain-subscription",
			ExtensionID:       extID,
			ModuleID:          "main",
			EventTypeID:       event.EventTypeID("system.test"),
			EventVersionRange: "^1",
			Entry:             "onEvent",
			Enabled:           true,
			Generation:        1,
		}
		if err := container.EventService.RegisterSubscription(ctx, subDef); err != nil {
			t.Fatalf("RegisterSubscription must succeed: %v", err)
		}

		schedules, _ := container.ScheduleService.ListSchedules(ctx, extID)
		if len(schedules) != 1 {
			t.Fatalf("expected 1 schedule, got %d", len(schedules))
		}
		subs := container.EventService.ListSubscriptionsByExtension(ctx, extID)
		if len(subs) != 1 {
			t.Fatalf("expected 1 subscription, got %d", len(subs))
		}
		t.Logf("HookEventSchedule: schedule and event subscription installed")
	})

	t.Run("Update", func(t *testing.T) {
		extensionsDir := testExtensionsDir(t)
		toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
		archivePath := filepath.Join(tempDir, "tool-basic-v2.amitiax")
		buildArchiveFromExtension(t, toolBasicDir, archivePath)
		targetDir := filepath.Join(extRoot, "tool-basic-v2")
		result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
			ArchivePath: archivePath,
			TargetDir:   targetDir,
		})
		if result.Status != amitiax.InstallSucceeded {
			t.Fatalf("update install must succeed: %v", result.Errors)
		}
		t.Logf("Update: new version installed at %s", targetDir)
	})

	t.Run("Restart", func(t *testing.T) {
		container.Close()

		container2, err := kernel.NewContainerBuilder().
			WithDBPath(dbPath).
			WithExtensionRoot(extRoot).
			Build(ctx)
		if err != nil {
			t.Fatalf("ContainerBuilder.Build after restart must succeed: %v", err)
		}
		defer container2.Close()

		if err := container2.Recover(ctx); err != nil {
			t.Fatalf("Container.Recover after restart must succeed: %v", err)
		}

		defs, err := container2.DefinitionRepository.ListExtensions(ctx)
		if err != nil {
			t.Fatalf("ListExtensions after restart must succeed: %v", err)
		}
		if len(defs) == 0 {
			t.Fatalf("extensions must survive restart (Phase 10 restart recovery)")
		}
		t.Logf("Restart: container rebuilt and recovered, %d extensions persisted", len(defs))
	})

	t.Run("Disable", func(t *testing.T) {
		container3, err := kernel.NewContainerBuilder().
			WithDBPath(dbPath).
			WithExtensionRoot(extRoot).
			Build(ctx)
		if err != nil {
			t.Fatalf("ContainerBuilder.Build for disable must succeed: %v", err)
		}
		defer container3.Close()

		subject := enablement.StateSubject{
			Kind: enablement.SubjectExtension,
			ID:   "com.amitia.repair/tool-basic",
		}
		if err := container3.EnablementService.Disable(ctx, subject); err != nil {
			t.Fatalf("EnablementService.Disable must succeed: %v", err)
		}
		t.Logf("Disable: extension disabled successfully")
	})

	t.Run("Uninstall", func(t *testing.T) {
		container4, err := kernel.NewContainerBuilder().
			WithDBPath(dbPath).
			WithExtensionRoot(extRoot).
			Build(ctx)
		if err != nil {
			t.Fatalf("ContainerBuilder.Build for uninstall must succeed: %v", err)
		}
		defer container4.Close()

		if err := container4.ScheduleService.Start(ctx); err != nil {
			t.Fatalf("ScheduleService.Start must succeed: %v", err)
		}
		defer container4.ScheduleService.Shutdown(ctx)

		if err := container4.EventService.Start(ctx); err != nil {
			t.Fatalf("EventService.Start must succeed: %v", err)
		}
		defer container4.EventService.Stop()

		extID := "com.amitia.repair/tool-basic"

		if _, err := container4.EventService.CancelDeliveriesByExtension(ctx, extID, "uninstall"); err != nil {
			t.Fatalf("CancelDeliveriesByExtension must succeed: %v", err)
		}
		if err := container4.EventService.RemoveSubscriptionsByExtension(ctx, extID); err != nil {
			t.Fatalf("RemoveSubscriptionsByExtension must succeed: %v", err)
		}
		if err := container4.ScheduleService.DeleteAllByExtension(ctx, extID); err != nil {
			t.Fatalf("DeleteAllByExtension must succeed: %v", err)
		}

		defs, _ := container4.DefinitionRepository.ListExtensions(ctx)
		for _, def := range defs {
			container4.DefinitionRepository.DeleteExtension(ctx, def.ID, def.Version)
		}

		targetDir := filepath.Join(extRoot, "tool-basic")
		if _, err := os.Stat(targetDir); err == nil {
			removeAllWithRetry(t, targetDir)
		}
		targetDirV2 := filepath.Join(extRoot, "tool-basic-v2")
		if _, err := os.Stat(targetDirV2); err == nil {
			removeAllWithRetry(t, targetDirV2)
		}

		t.Logf("Uninstall: full cleanup completed")
	})

	finalTotal := counter.Total()
	if finalTotal != initialTotal {
		t.Fatalf("LegacyCallCounter must not grow during full-chain E2E: initial=%d final=%d", initialTotal, finalTotal)
	}
	t.Logf("FullChain E2E: LegacyCallCounter stayed at %d throughout entire lifecycle", finalTotal)
}

func TestBaseline_E2E_FullChain_RestartRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping restart recovery test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	container1, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("first build must succeed: %v", err)
	}
	if err := container1.ScheduleService.Start(ctx); err != nil {
		t.Fatalf("ScheduleService.Start must succeed: %v", err)
	}
	if err := container1.EventService.Start(ctx); err != nil {
		t.Fatalf("EventService.Start must succeed: %v", err)
	}
	if err := container1.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		t.Fatalf("RegisterDefaultEventTypes must succeed: %v", err)
	}
	registerTestEventType(t, ctx, container1.EventService)

	extID := "com.amitia.repair/tool-basic"
	schedDef := makeScheduleDefinition("recovery-schedule", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	schedDef.Trigger.Interval = nil
	if err := container1.ScheduleService.InstallDefinition(ctx, schedDef); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	subDef := event.EventSubscriptionDefinition{
		ContributionID:    "recovery-subscription",
		ExtensionID:       extID,
		ModuleID:          "main",
		EventTypeID:       event.EventTypeID("system.test"),
		EventVersionRange: "^1",
		Entry:             "onEvent",
		Enabled:           true,
		Generation:        1,
	}
	if err := container1.EventService.RegisterSubscription(ctx, subDef); err != nil {
		t.Fatalf("RegisterSubscription must succeed: %v", err)
	}

	container1.ScheduleService.Shutdown(ctx)
	container1.EventService.Stop()
	container1.Close()

	container2, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("second build after restart must succeed: %v", err)
	}
	defer container2.Close()

	if err := container2.Recover(ctx); err != nil {
		t.Fatalf("Recover after restart must succeed: %v", err)
	}

	if err := container2.ScheduleService.Start(ctx); err != nil {
		t.Fatalf("ScheduleService.Start after restart must succeed: %v", err)
	}
	defer container2.ScheduleService.Shutdown(ctx)

	if err := container2.EventService.Start(ctx); err != nil {
		t.Fatalf("EventService.Start after restart must succeed: %v", err)
	}
	defer container2.EventService.Stop()

	if err := container2.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		t.Fatalf("RegisterDefaultEventTypes after restart must succeed: %v", err)
	}

	schedules, _ := container2.ScheduleService.ListSchedules(ctx, extID)
	if len(schedules) == 0 {
		t.Fatalf("schedules must survive restart (Phase 10 restart recovery)")
	}
	t.Logf("RestartRecovery: %d schedules persisted after restart", len(schedules))
}

func TestBaseline_E2E_FullChain_LegacyZeroCallThroughout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping legacy zero-call test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(filepath.Join(tempDir, "kernel.db")).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	counter := kernel.GlobalLegacyCallCounter()

	if counter.Total() != 0 {
		t.Fatalf("LegacyCallCounter must be 0 at start, got %d", counter.Total())
	}

	facade := kernel.NewToolFacade(container.ToolRegistry, container.ExecutionKernel, kernel.DefaultToolFacadeConfig())
	scope := kernel.LegacyScope{
		UserID:    "zerocall",
		Channel:   "test",
		SessionID: "zerocall-verify",
	}
	for i := 0; i < 10; i++ {
		_, err := facade.ModelTools(ctx, scope)
		if err != nil {
			t.Fatalf("ModelTools iteration %d must not error: %v", i, err)
		}
	}

	if counter.Total() != 0 {
		t.Fatalf("LegacyCallCounter must stay 0 after 10 ModelTools calls, got %d", counter.Total())
	}

	snap := counter.Snapshot()
	if snap["legacy_fallback_total"] > 0 {
		t.Fatalf("legacy_fallback_total must be 0, got %d", snap["legacy_fallback_total"])
	}
	if snap["legacy_tool_execute_calls"] > 0 {
		t.Fatalf("legacy_tool_execute_calls must be 0, got %d", snap["legacy_tool_execute_calls"])
	}
	if snap["legacy_mcp_execute_calls"] > 0 {
		t.Fatalf("legacy_mcp_execute_calls must be 0, got %d", snap["legacy_mcp_execute_calls"])
	}
	if snap["legacy_package_write_calls"] > 0 {
		t.Fatalf("legacy_package_write_calls must be 0, got %d", snap["legacy_package_write_calls"])
	}
	if snap["duplicate_contribution_registrations"] > 0 {
		t.Fatalf("duplicate_contribution_registrations must be 0, got %d", snap["duplicate_contribution_registrations"])
	}
	if snap["orphan_runtime_instances"] > 0 {
		t.Fatalf("orphan_runtime_instances must be 0, got %d", snap["orphan_runtime_instances"])
	}
	if snap["orphan_ui_sessions"] > 0 {
		t.Fatalf("orphan_ui_sessions must be 0, got %d", snap["orphan_ui_sessions"])
	}
	if snap["failed_cleanup_resources"] > 0 {
		t.Fatalf("failed_cleanup_resources must be 0, got %d", snap["failed_cleanup_resources"])
	}

	t.Logf("LegacyZeroCall: all gate metrics are 0 after 10 ModelTools calls")
	runtime.KeepAlive(facade)
}
