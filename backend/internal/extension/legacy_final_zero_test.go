package extension

import (
	"context"
	"path/filepath"
	"testing"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

func newLegacyStateProxy(t *testing.T) (*KernelLifecycleProxy, *kernelruntime.Container) {
	t.Helper()
	root := t.TempDir()
	container, err := kernelruntime.NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := kernelruntime.NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	return NewKernelLifecycleProxy(runtime), container
}

func TestLegacyMigrationCanonicalStatesAndProductionBoundary(t *testing.T) {
	proxy, _ := newLegacyStateProxy(t)
	states := []LegacyMigrationState{LegacyMigrationNotStarted, LegacyMigrationAnalyzing, LegacyMigrationReady, LegacyMigrationMigrating, LegacyMigrationCompleted, LegacyMigrationBlocked, LegacyMigrationManualRequired}
	for index, state := range states {
		extensionID := "com.example/state-" + string(rune('a'+index))
		if err := proxy.recordLegacyMigrationState(context.Background(), legacyMigrationRecord{ExtensionID: extensionID, State: state, Failure: "failure", Source: "legacy-db:" + extensionID, ArtifactID: "artifact-1"}); err != nil {
			t.Fatal(err)
		}
		actual, err := proxy.LegacyReadState(context.Background(), extensionID)
		if err != nil || actual != string(state) {
			t.Fatalf("state mismatch: want=%s got=%s err=%v", state, actual, err)
		}
		if proxy.LegacyReadAllowed(context.Background(), extensionID) {
			t.Fatalf("production legacy read must be blocked for %s", state)
		}
	}
	if proxy.LegacyMigrationReadAllowed(context.Background(), "com.example/state-g") {
		t.Fatal("manual migration read requires explicit tool context")
	}
	if !proxy.LegacyMigrationReadAllowed(WithLegacyMigrationToolContext(context.Background()), "com.example/state-g") {
		t.Fatal("explicit migration tool context must allow manual_required read")
	}
}

func TestLegacyMigrationOldStatesNormalizeAndPersist(t *testing.T) {
	proxy, container := newLegacyStateProxy(t)
	ctx := context.Background()
	for extensionID, oldState := range map[string]string{"com.example/old-complete": "migrated", "com.example/old-manual": "requires_manual_migration"} {
		if _, err := container.Store.DB().ExecContext(ctx, `INSERT INTO extension_package_legacy_migrations (extension_id, migration_status, attempt_count, last_error, legacy_path, artifact_id, updated_at) VALUES (?, ?, 1, '', '', '', 'now')`, extensionID, oldState); err != nil {
			t.Fatal(err)
		}
	}
	state, err := proxy.LegacyReadState(ctx, "com.example/old-complete")
	if err != nil || state != string(LegacyMigrationCompleted) {
		t.Fatalf("old completed state not normalized: state=%s err=%v", state, err)
	}
	state, err = proxy.LegacyReadState(ctx, "com.example/old-manual")
	if err != nil || state != string(LegacyMigrationManualRequired) {
		t.Fatalf("old manual state not normalized: state=%s err=%v", state, err)
	}
	var persisted string
	if err := container.Store.DB().QueryRowContext(ctx, `SELECT migration_status FROM extension_package_legacy_migrations WHERE extension_id = ?`, "com.example/old-complete").Scan(&persisted); err != nil || persisted != string(LegacyMigrationCompleted) {
		t.Fatalf("normalized state not persisted: state=%s err=%v", persisted, err)
	}
}

func TestLegacyMigrationUnknownAndDatabaseErrorsFailClosed(t *testing.T) {
	proxy, container := newLegacyStateProxy(t)
	ctx := context.Background()
	if _, err := container.Store.DB().ExecContext(ctx, `INSERT INTO extension_package_legacy_migrations (extension_id, migration_status, attempt_count, last_error, legacy_path, artifact_id, updated_at) VALUES ('com.example/unknown', 'unexpected', 1, '', '', '', 'now')`); err != nil {
		t.Fatal(err)
	}
	if state, err := proxy.LegacyReadState(ctx, "com.example/unknown"); err == nil || state != "unknown" {
		t.Fatalf("unknown state must fail closed: state=%s err=%v", state, err)
	}
	if proxy.LegacyReadAllowed(ctx, "com.example/unknown") {
		t.Fatal("unknown state enabled production legacy read")
	}
	if err := container.Store.DB().Close(); err != nil {
		t.Fatal(err)
	}
	if state, err := proxy.LegacyReadState(ctx, "com.example/unknown"); err == nil || state != "unknown" {
		t.Fatalf("database error must fail closed: state=%s err=%v", state, err)
	}
}

func TestLegacyMigrationCompletedSurvivesKernelRestartWithoutFallback(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "kernel.db")
	extensionRoot := filepath.Join(root, "extensions")
	build := func() (*KernelLifecycleProxy, *kernelruntime.Container) {
		container, err := kernelruntime.NewContainerBuilder().WithDBPath(dbPath).WithExtensionRoot(extensionRoot).Build(ctx)
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := kernelruntime.NewRuntime(extensionRoot)
		if err != nil {
			container.Close()
			t.Fatal(err)
		}
		runtime.SetContainer(container)
		return NewKernelLifecycleProxy(runtime), container
	}
	first, firstContainer := build()
	if err := first.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: "com.example/restart", State: LegacyMigrationCompleted, Source: "legacy-db:restart", ArtifactID: "artifact-1"}); err != nil {
		firstContainer.Close()
		t.Fatal(err)
	}
	if err := firstContainer.Close(); err != nil {
		t.Fatal(err)
	}
	second, secondContainer := build()
	defer secondContainer.Close()
	state, err := second.LegacyReadState(ctx, "com.example/restart")
	if err != nil || state != string(LegacyMigrationCompleted) {
		t.Fatalf("completed state did not survive restart: state=%s err=%v", state, err)
	}
	if second.LegacyReadAllowed(ctx, "com.example/restart") {
		t.Fatal("completed migration revived production fallback after restart")
	}
}
