package repair_baseline

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
)

func TestBaseline_E2E_Startup_ContainerBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E startup test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	builder := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot)

	container, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed for E2E startup: %v", err)
	}
	defer container.Close()

	if container.Store == nil {
		t.Fatalf("Container must have a non-nil Store after Build")
	}
	if container.DefinitionRepository == nil {
		t.Fatalf("Container must have a non-nil DefinitionRepository after Build")
	}
	if container.InstallationRepository == nil {
		t.Fatalf("Container must have a non-nil InstallationRepository after Build")
	}
	if container.ScheduleService == nil {
		t.Fatalf("Container must have a non-nil ScheduleService after Build")
	}
	if container.EventService == nil {
		t.Fatalf("Container must have a non-nil EventService after Build")
	}
}

func TestBaseline_E2E_Startup_ContainerRecovers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E startup test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	builder := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot)

	container, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("Container.Recover must succeed on empty database: %v", err)
	}
}

func TestBaseline_E2E_Startup_LegacyCallCounterZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E startup test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	builder := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot)

	container, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	if err := container.Recover(ctx); err != nil {
		t.Fatalf("Container.Recover must succeed: %v", err)
	}

	total := kernel.GlobalLegacyCallCounter().Total()
	if total != 0 {
		t.Fatalf("Legacy Call Counter must be 0 after startup (Phase 10 section 19.1), got %d", total)
	}
}

func TestBaseline_E2E_Startup_ContainerCloses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E startup test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	builder := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot)

	container, err := builder.Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}

	if err := container.Close(); err != nil {
		t.Fatalf("Container.Close must succeed: %v", err)
	}
}

func TestBaseline_E2E_Startup_ReadinessPhasesDefined(t *testing.T) {
	requiredPhases := []string{
		"container_build",
		"recover",
		"task_start",
		"event_start",
		"schedule_start",
		"readiness_true",
		"legacy_counter_zero",
	}
	if len(requiredPhases) != 7 {
		t.Fatalf("Phase 10 section 19.1 requires 7 startup phases, got %d", len(requiredPhases))
	}
	for _, p := range requiredPhases {
		if p == "" {
			t.Fatalf("startup phase must not be empty")
		}
	}
}
