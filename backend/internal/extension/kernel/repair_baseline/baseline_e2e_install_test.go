package repair_baseline

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
)

func TestBaseline_E2E_Install_ToolBasicSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E install test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

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
		t.Fatalf("tool-basic install must succeed (Phase 10 section 19.2), got %s: %v", result.Status, result.Errors)
	}
	if result.Definition.ID == "" {
		t.Fatalf("installed extension must have a non-empty ID")
	}
}

func TestBaseline_E2E_Install_TamperedPackageFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E install test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	toolBasicDir := filepath.Join(extensionsDir, "signature-tampered")
	archivePath := filepath.Join(tempDir, "signature-tampered.amitiax")
	buildArchiveFromExtension(t, toolBasicDir, archivePath)
	targetDir := filepath.Join(extRoot, "signature-tampered")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     targetDir,
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("tampered package with RequireSigned must fail (Phase 10 section 19.2.5), got %s", result.Status)
	}
}

func TestBaseline_E2E_Install_UnknownKeyFailsOrRequiresConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E install test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	unknownKeyDir := filepath.Join(extensionsDir, "signature-unknown-key")
	archivePath := filepath.Join(tempDir, "signature-unknown-key.amitiax")
	buildArchiveFromExtension(t, unknownKeyDir, archivePath)
	targetDir := filepath.Join(extRoot, "signature-unknown-key")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     targetDir,
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("unknown key package with RequireSigned must fail or require confirmation (Phase 10 section 19.2.6), got %s", result.Status)
	}
}

func TestBaseline_E2E_Install_PublisherMismatchFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E install test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	mismatchDir := filepath.Join(extensionsDir, "signature-publisher-mismatch")
	archivePath := filepath.Join(tempDir, "signature-publisher-mismatch.amitiax")
	buildArchiveFromExtension(t, mismatchDir, archivePath)
	targetDir := filepath.Join(extRoot, "signature-publisher-mismatch")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath:   archivePath,
		TargetDir:     targetDir,
		RequireSigned: true,
	})
	if result.Status != amitiax.InstallFailed {
		t.Fatalf("publisher mismatch package with RequireSigned must fail (Phase 10 section 19.2.7), got %s", result.Status)
	}
}

func TestBaseline_E2E_Install_UnsignedPackageWithoutRequireSignedSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E install test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kernel.db")
	extRoot := filepath.Join(tempDir, "extensions")

	container, err := kernel.NewContainerBuilder().
		WithDBPath(dbPath).
		WithExtensionRoot(extRoot).
		Build(ctx)
	if err != nil {
		t.Fatalf("ContainerBuilder.Build must succeed: %v", err)
	}
	defer container.Close()

	extensionsDir := testExtensionsDir(t)
	validDir := filepath.Join(extensionsDir, "signature-valid")
	archivePath := filepath.Join(tempDir, "signature-valid.amitiax")
	buildArchiveFromExtension(t, validDir, archivePath)
	targetDir := filepath.Join(extRoot, "signature-valid")

	result := container.AmitiaxInstaller.Install(ctx, amitiax.InstallRequest{
		ArchivePath: archivePath,
		TargetDir:   targetDir,
	})
	if result.Status != amitiax.InstallSucceeded {
		t.Fatalf("unsigned package without RequireSigned must succeed (Phase 10 section 19.2.4), got %s: %v", result.Status, result.Errors)
	}
}
