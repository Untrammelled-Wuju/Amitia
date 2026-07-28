package repair_baseline

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func makeScheduleDefinition(scheduleID, extensionID string, triggerType schedule.TriggerType, targetType schedule.TargetType) *schedule.ScheduleContributionDefinition {
	now := time.Now().UTC()
	return &schedule.ScheduleContributionDefinition{
		ContributionID: scheduleID,
		ExtensionID:    extensionID,
		ModuleID:       "main",
		ScheduleID:     scheduleID,
		Name:           "Test Schedule",
		Description:    "Phase 10 E2E test schedule",
		Trigger: schedule.ScheduleTriggerDefinition{
			Type: triggerType,
			OneShot: &schedule.OneShotTriggerDefinition{
				RunAt: now.Add(1 * time.Hour),
			},
			Interval: &schedule.IntervalTriggerDefinition{
				Interval: 60 * time.Second,
				AnchorAt: now,
			},
		},
		Target: schedule.ScheduleTargetDefinition{
			Type:          targetType,
			TargetID:      "test-target",
			InputTemplate: json.RawMessage(`{}`),
		},
		EnabledByDefault:  false,
		MisfirePolicy:     schedule.ScheduleMisfirePolicy{Policy: schedule.MisfirePolicyFireOnce, MaxCatchUp: 1},
		OverlapPolicy:     schedule.ScheduleOverlapPolicy{Policy: schedule.OverlapPolicyForbid},
		RetryPolicy:       schedule.ScheduleRetryPolicy{MaxAttempts: 3, InitialBackoff: 100 * time.Millisecond, MaxBackoff: 1 * time.Second, Multiplier: 2, Jitter: 0.2},
		ConcurrencyPolicy: schedule.ScheduleConcurrencyPolicy{MaxConcurrentRuns: 1},
		Version:           "1.0.0",
	}
}

func TestBaseline_E2E_Schedule_InstallOneShot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	def := makeScheduleDefinition("test-oneshot", "com.amitia.repair/schedule-tool", schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed (Phase 10 section 19.5.1): %v", err)
	}

	_, state, err := container.ScheduleService.GetSchedule(ctx, "test-oneshot")
	if err != nil {
		t.Fatalf("GetSchedule must succeed: %v", err)
	}
	if state.Status != schedule.DefinitionStatusCreated {
		t.Fatalf("newly installed schedule must have status 'created', got %s", state.Status)
	}
}

func TestBaseline_E2E_Schedule_EnableDisable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	def := makeScheduleDefinition("test-enable", "com.amitia.repair/schedule-tool", schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	if err := container.ScheduleService.Enable(ctx, "test-enable", 1); err != nil {
		t.Fatalf("Enable must succeed (Phase 10 section 19.5.1): %v", err)
	}
	_, state, _ := container.ScheduleService.GetSchedule(ctx, "test-enable")
	if state.Status != schedule.DefinitionStatusEnabled {
		t.Fatalf("enabled schedule must have status 'enabled', got %s", state.Status)
	}

	if err := container.ScheduleService.Disable(ctx, "test-enable", 1); err != nil {
		t.Fatalf("Disable must succeed (Phase 10 section 19.5.7): %v", err)
	}
	_, state, _ = container.ScheduleService.GetSchedule(ctx, "test-enable")
	if state.Status != schedule.DefinitionStatusDisabled {
		t.Fatalf("disabled schedule must have status 'disabled', got %s", state.Status)
	}
}

func TestBaseline_E2E_Schedule_ManualTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	def := makeScheduleDefinition("test-manual", "com.amitia.repair/schedule-tool", schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}
	if err := container.ScheduleService.Enable(ctx, "test-manual", 1); err != nil {
		t.Fatalf("Enable must succeed: %v", err)
	}

	trigger, err := container.ScheduleService.RunNow(ctx, "test-manual")
	if err != nil {
		t.Fatalf("RunNow must succeed (Phase 10 section 19.5.2): %v", err)
	}
	if trigger == nil {
		t.Fatalf("RunNow must return a trigger record")
	}
	if trigger.ScheduleID != "test-manual" {
		t.Fatalf("trigger schedule ID must match, got %s", trigger.ScheduleID)
	}
}

func TestBaseline_E2E_Schedule_ListByExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	extID := "com.amitia.repair/schedule-tool"
	def := makeScheduleDefinition("test-list", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	list, err := container.ScheduleService.ListSchedules(ctx, extID)
	if err != nil {
		t.Fatalf("ListSchedules must succeed: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("ListSchedules must return at least 1 schedule for extension %s", extID)
	}
}

func TestBaseline_E2E_Schedule_UninstallRemovesSchedule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	def := makeScheduleDefinition("test-uninstall", "com.amitia.repair/schedule-tool", schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	if err := container.ScheduleService.Uninstall(ctx, "test-uninstall"); err != nil {
		t.Fatalf("Uninstall must succeed (Phase 10 section 19.5.7): %v", err)
	}

	_, _, err = container.ScheduleService.GetSchedule(ctx, "test-uninstall")
	if err == nil {
		t.Fatalf("GetSchedule must fail after Uninstall")
	}
}

func TestBaseline_E2E_Schedule_DeleteAllByExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	extID := "com.amitia.repair/schedule-tool"
	for i := 0; i < 3; i++ {
		def := makeScheduleDefinition("test-bulk-"+string(rune('a'+i)), extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
		def.Trigger.Interval = nil
		if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
			t.Fatalf("InstallDefinition %d must succeed: %v", i, err)
		}
	}

	if err := container.ScheduleService.DeleteAllByExtension(ctx, extID); err != nil {
		t.Fatalf("DeleteAllByExtension must succeed: %v", err)
	}

	list, _ := container.ScheduleService.ListSchedules(ctx, extID)
	if len(list) != 0 {
		t.Fatalf("DeleteAllByExtension must remove all schedules, got %d", len(list))
	}
}

func TestBaseline_E2E_Schedule_PauseResume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E schedule test in short mode")
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

	def := makeScheduleDefinition("test-pause", "com.amitia.repair/schedule-tool", schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}
	if err := container.ScheduleService.Enable(ctx, "test-pause", 1); err != nil {
		t.Fatalf("Enable must succeed: %v", err)
	}

	if err := container.ScheduleService.Pause(ctx, "test-pause", 1); err != nil {
		t.Fatalf("Pause must succeed: %v", err)
	}
	_, state, _ := container.ScheduleService.GetSchedule(ctx, "test-pause")
	if !state.Paused {
		t.Fatalf("paused schedule must have Paused=true")
	}

	if err := container.ScheduleService.Resume(ctx, "test-pause", 1); err != nil {
		t.Fatalf("Resume must succeed: %v", err)
	}
	_, state, _ = container.ScheduleService.GetSchedule(ctx, "test-pause")
	if state.Paused {
		t.Fatalf("resumed schedule must have Paused=false")
	}
}
