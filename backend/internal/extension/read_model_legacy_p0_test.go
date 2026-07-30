package extension

import (
	"context"
	"path/filepath"
	"testing"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

func buildLegacyStateProxy(t *testing.T) (*KernelLifecycleProxy, *kernelruntime.Container) {
	t.Helper()
	root := t.TempDir()
	container, err := kernelruntime.NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(context.Background())
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

func TestReadModelMissingAuthoritativeRepositoriesFailsClosed(t *testing.T) {
	container := &kernelruntime.Container{InstallationRepository: &mockInstallationRepo{inst: validTestInstallation()}}
	service := buildTestReadModel(t, container)
	if _, ok, err := service.TryDependencies(context.Background(), "dev.local.test/ext"); err == nil || ok {
		t.Fatalf("missing authoritative repositories must fail closed: ok=%v err=%v", ok, err)
	}
}

func TestLegacyReadRequiresExplicitMigrationState(t *testing.T) {
	ctx := context.Background()
	proxy, container := buildLegacyStateProxy(t)
	if state, err := proxy.LegacyReadState(ctx, "dev.local.test/ext"); err == nil || state != "unknown" {
		t.Fatalf("unregistered migration state must fail closed: state=%s err=%v", state, err)
	}
	if proxy.LegacyReadAllowed(ctx, "dev.local.test/ext") {
		t.Fatal("unregistered migration state must not allow legacy reads")
	}
	_, err := container.Store.DB().ExecContext(ctx, `INSERT INTO extension_package_legacy_migrations
		(extension_id, migration_status, attempt_count, last_error, legacy_path, artifact_id, updated_at)
		VALUES (?, 'requires_manual_migration', 1, '', '', '', 'now')`, "dev.local.test/ext")
	if err != nil {
		t.Fatal(err)
	}
	state, err := proxy.LegacyReadState(ctx, "dev.local.test/ext")
	if err != nil || state != string(LegacyMigrationManualRequired) || proxy.LegacyReadAllowed(ctx, "dev.local.test/ext") {
		t.Fatalf("manual migration state must block production legacy read: state=%s err=%v", state, err)
	}
	if !proxy.LegacyMigrationReadAllowed(WithLegacyMigrationToolContext(ctx), "dev.local.test/ext") {
		t.Fatal("manual migration tool context must allow read-only legacy access")
	}
	if _, err := container.Store.DB().ExecContext(ctx, `UPDATE extension_package_legacy_migrations SET migration_status='unexpected' WHERE extension_id=?`, "dev.local.test/ext"); err != nil {
		t.Fatal(err)
	}
	if state, err := proxy.LegacyReadState(ctx, "dev.local.test/ext"); err == nil || state != "unknown" {
		t.Fatalf("unknown migration value must fail closed: state=%s err=%v", state, err)
	}
}
