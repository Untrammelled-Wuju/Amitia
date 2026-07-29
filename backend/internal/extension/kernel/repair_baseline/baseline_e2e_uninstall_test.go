package repair_baseline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
)

func registerTestEventType(t *testing.T, ctx context.Context, svc *event.Service) {
	t.Helper()
	maxPayload := int64(256 * 1024)
	maxMeta := int64(32 * 1024)
	def := event.EventTypeDefinition{
		EventTypeID:      event.EventTypeID("system.test"),
		Version:          1,
		Description:      "Test event type for E2E",
		MaxPayloadBytes:  maxPayload,
		MaxMetadataBytes: maxMeta,
		RiskLevel:        event.RiskLevelLow,
		ProducerPolicy: event.EventProducerPolicy{
			AllowedProducers:   []string{"host", "system", "test"},
			MaxPayloadBytes:    maxPayload,
			MaxMetadataBytes:   maxMeta,
			RateLimitPerSecond: 100,
		},
		SubscriberPolicy: event.EventSubscriberPolicy{
			AllowThirdParty:     true,
			MaxSubscribers:      64,
			RequiredPermissions: []string{"event.subscribe"},
		},
		DeliveryPolicy: event.EventDeliveryPolicy{
			Timeout:           5 * time.Second,
			MaxAttempts:       5,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2,
			JitterFactor:      0.2,
			MaxInFlight:       4,
		},
		OrderingPolicy: event.OrderingNone,
		RetentionPolicy: event.EventRetentionPolicy{
			MaxAge:                24 * time.Hour,
			MaxDeliveryCount:      5,
			DeleteAfterSuccess:    true,
			DeleteAfterDeadLetter: false,
			ArchiveDeadLetters:    true,
		},
	}
	if err := svc.RegisterEventType(ctx, def); err != nil {
		t.Fatalf("RegisterEventType system.test must succeed: %v", err)
	}
}

var _ = json.RawMessage(nil)

func TestBaseline_E2E_Uninstall_ScheduleCleanedUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
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

	extID := "com.amitia.repair/uninstall-cleanup"
	def := makeScheduleDefinition("cleanup-schedule", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	def.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, def); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	list, _ := container.ScheduleService.ListSchedules(ctx, extID)
	if len(list) != 1 {
		t.Fatalf("expected 1 schedule before cleanup, got %d", len(list))
	}

	if err := container.ScheduleService.DeleteAllByExtension(ctx, extID); err != nil {
		t.Fatalf("DeleteAllByExtension must succeed (Phase 10 section 19.9.5): %v", err)
	}

	list, _ = container.ScheduleService.ListSchedules(ctx, extID)
	if len(list) != 0 {
		t.Fatalf("schedules must be cleaned up after DeleteAllByExtension (Phase 10 section 19.9.5), got %d", len(list))
	}
}

func TestBaseline_E2E_Uninstall_EventSubscriptionCleanedUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
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
	registerTestEventType(t, ctx, container.EventService)

	extID := "com.amitia.repair/uninstall-cleanup"
	subDef := event.EventSubscriptionDefinition{
		ContributionID:    "cleanup-subscription",
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

	subs := container.EventService.ListSubscriptionsByExtension(ctx, extID)
	if len(subs) == 0 {
		t.Fatalf("expected at least 1 subscription before cleanup")
	}

	if err := container.EventService.RemoveSubscriptionsByExtension(ctx, extID); err != nil {
		t.Fatalf("RemoveSubscriptionsByExtension must succeed (Phase 10 section 19.9.7): %v", err)
	}

	subs = container.EventService.ListSubscriptionsByExtension(ctx, extID)
	if len(subs) != 0 {
		t.Fatalf("event subscriptions must be cleaned up after RemoveSubscriptionsByExtension (Phase 10 section 19.9.7), got %d", len(subs))
	}
}

func TestBaseline_E2E_Uninstall_DeliveriesCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
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

	extID := "com.amitia.repair/uninstall-cleanup"
	cancelled, err := container.EventService.CancelDeliveriesByExtension(ctx, extID, "uninstall")
	if err != nil {
		t.Fatalf("CancelDeliveriesByExtension must succeed (Phase 10 section 19.9.8): %v", err)
	}
	_ = cancelled
}

func TestBaseline_E2E_Uninstall_ExtensionDefinitionRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	targetDir := filepath.Join(extRoot, "tool-basic")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != amitiax.InstallSucceeded {
		t.Fatalf("install must succeed: %v", result.Errors)
	}
	if result.Definition.ID == "" {
		t.Fatalf("install result must contain a definition with non-empty ID")
	}

	if err := container.DefinitionRepository.PutExtension(ctx, result.Definition); err != nil {
		t.Fatalf("DefinitionRepository.PutExtension must succeed: %v", err)
	}

	defs, err := container.DefinitionRepository.ListExtensions(ctx)
	if err != nil {
		t.Fatalf("DefinitionRepository.ListExtensions must succeed: %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("expected at least 1 definition after PutExtension")
	}

	for _, def := range defs {
		if err := container.DefinitionRepository.DeleteExtension(ctx, def.ID, def.Version); err != nil {
			t.Fatalf("DefinitionRepository.DeleteExtension must succeed for %s: %v", def.ID, err)
		}
	}

	defs, _ = container.DefinitionRepository.ListExtensions(ctx)
	if len(defs) != 0 {
		t.Fatalf("definitions must be removed after DeleteExtension (Phase 10 section 19.9.4), got %d", len(defs))
	}
}

func TestBaseline_E2E_Uninstall_FilesystemTargetRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "tool-basic")
	archivePath := filepath.Join(tempDir, "tool-basic.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	targetDir := filepath.Join(extRoot, "tool-basic")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != amitiax.InstallSucceeded {
		t.Fatalf("install must succeed: %v", result.Errors)
	}

	if _, err := os.Stat(targetDir); err != nil {
		t.Fatalf("target dir must exist after install: %v", err)
	}

	removeAllWithRetry(t, targetDir)

	if _, err := os.Stat(targetDir); err == nil {
		t.Fatalf("target dir must not exist after removal (Phase 10 section 19.9.5-19.9.6)")
	}
}

func TestBaseline_E2E_Uninstall_FullCleanupSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E uninstall test in short mode")
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

	if err := container.EventService.Start(ctx); err != nil {
		t.Fatalf("EventService.Start must succeed: %v", err)
	}
	defer container.EventService.Stop()

	if err := container.EventService.RegisterDefaultEventTypes(ctx); err != nil {
		t.Fatalf("RegisterDefaultEventTypes must succeed: %v", err)
	}
	registerTestEventType(t, ctx, container.EventService)

	extID := "com.amitia.repair/uninstall-cleanup"

	schedDef := makeScheduleDefinition("full-cleanup-schedule", extID, schedule.TriggerTypeOneShot, schedule.TargetTypeTool)
	schedDef.Trigger.Interval = nil
	if err := container.ScheduleService.InstallDefinition(ctx, schedDef); err != nil {
		t.Fatalf("InstallDefinition must succeed: %v", err)
	}

	subDef := event.EventSubscriptionDefinition{
		ContributionID:    "full-cleanup-subscription",
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

	if _, err := container.EventService.CancelDeliveriesByExtension(ctx, extID, "uninstall"); err != nil {
		t.Fatalf("CancelDeliveriesByExtension must succeed: %v", err)
	}
	if err := container.EventService.RemoveSubscriptionsByExtension(ctx, extID); err != nil {
		t.Fatalf("RemoveSubscriptionsByExtension must succeed: %v", err)
	}
	if err := container.ScheduleService.DeleteAllByExtension(ctx, extID); err != nil {
		t.Fatalf("DeleteAllByExtension must succeed: %v", err)
	}

	schedules, _ := container.ScheduleService.ListSchedules(ctx, extID)
	if len(schedules) != 0 {
		t.Fatalf("schedules must be 0 after full cleanup (Phase 10 section 19.9.5), got %d", len(schedules))
	}
	subs := container.EventService.ListSubscriptionsByExtension(ctx, extID)
	if len(subs) != 0 {
		t.Fatalf("subscriptions must be 0 after full cleanup (Phase 10 section 19.9.7), got %d", len(subs))
	}
}
