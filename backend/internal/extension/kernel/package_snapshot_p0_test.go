package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func TestPackageRollbackSnapshotCapturesConfigResourceAndMigration(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(v1.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	installation.Metadata["configuration"] = map[string]any{"theme": "dark", "apiToken": "plaintext-forbidden", "credential": "secret://credential-1"}
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	if err := container.ResourceRepository.PutResource(ctx, domain.ResourceOwnership{ResourceID: "resource-1", OwnerType: "extension", OwnerID: v1.ExtensionID, ResourceType: "index", Reference: "index://one", AcquiredAt: time.Now().UTC(), Metadata: map[string]any{"restore": "required"}}); err != nil {
		t.Fatal(err)
	}
	if err := container.MigrationRepository.SaveMigrationDefinition(ctx, &migration.MigrationDefinition{MigrationID: "migration-1", ExtensionID: v1.ExtensionID, FromVersionRange: "1.0.0", ToVersion: "1.1.0", Direction: migration.DirectionForward, Reversibility: migration.ReversibilitySnapshotReversible, Idempotency: migration.IdempotencyIdempotent, DefinitionHash: "sha256:migration"}); err != nil {
		t.Fatal(err)
	}
	installPackagePipelineVersion(t, runtime, "1.1.0")
	point, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if point.ConfigSnapshotJSON == "" || point.ResourceSnapshotJSON == "" || point.MigrationStateSnapshotJSON == "" || point.UserDataMigrationStateJSON == "" {
		t.Fatal("complete rollback snapshots must be persisted")
	}
	combined := point.ConfigSnapshotJSON + point.SecretRefsJSON + point.ResourceSnapshotJSON
	if strings.Contains(combined, "plaintext-forbidden") {
		t.Fatal("secret plaintext leaked into rollback snapshot")
	}
	if !strings.Contains(point.SecretRefsJSON, "secret://credential-1") {
		t.Fatal("secret reference was not retained")
	}
	if !strings.Contains(point.ResourceSnapshotJSON, "resource-1") || !strings.Contains(point.MigrationStateSnapshotJSON, "migration-1") {
		t.Fatal("resource or migration state missing")
	}
	if err := validatePackageSnapshot(point); err != nil {
		t.Fatal(err)
	}
	point.ResourceSnapshotJSON = strings.Replace(point.ResourceSnapshotJSON, "resource-1", "resource-2", 1)
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("tampered snapshot hash must be rejected")
	}
}

func TestPackageSnapshotMarksIrreversibleMigrationForManualRecovery(t *testing.T) {
	state := packageMigrationStateSnapshot{Mode: "repository", Operations: []packageMigrationOperationSnapshot{{Operation: migration.MigrationOperation{OperationID: "irreversible-1", Status: migration.OperationStatusCompleted, Reversibility: migration.ReversibilityIrreversible}}}}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	forward := PackageRollbackPoint{MigrationStateSnapshotJSON: string(raw)}
	target := PackageRollbackPoint{MigrationStateSnapshotJSON: `{"mode":"none"}`}
	if reason := packageSnapshotManualRecoveryReason(forward, target); !strings.Contains(reason, "irreversible-1") {
		t.Fatalf("expected requires_manual_recovery reason, got %q", reason)
	}
}

func TestPackageForwardRecoveryRestoresRepositoryState(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	definition, err := container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	contributions, err := container.ContributionRepository.ListContributions(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	point, err := runtime.createPackageRollbackPoint(ctx, "operation-forward-test", "forward_recovery", installation, &definition, modules, contributions)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.ModuleRepository.DeleteModules(ctx, installation.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.failPackageRollbackWithForwardRecovery("missing-operation", point, "test_failure", errors.New("rollback target failed"), PackageWriteGuard{}); err == nil {
		t.Fatal("rollback failure must be returned after forward compensation")
	}
	restored, err := container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != len(modules) {
		t.Fatalf("forward recovery restored %d modules, want %d", len(restored), len(modules))
	}
}

func newPackageRestoreEntry(resourceID, reference, contentHash string, size int64, storageRef, restorePath string) packageResourceSnapshotEntry {
	resource := domain.ResourceOwnership{ResourceID: resourceID, OwnerType: "extension", OwnerID: "test-ext", ResourceType: "file", Reference: reference, AcquiredAt: time.Now().UTC()}
	raw, err := json.Marshal(resource)
	if err != nil {
		panic(err)
	}
	return packageResourceSnapshotEntry{
		Resource:                resource,
		ResourceHash:            packageSnapshotDigest(raw),
		RestoreStrategy:         "repository_upsert",
		LogicalPath:             restorePath,
		OriginalPath:            restorePath,
		ContentHash:             contentHash,
		Size:                    size,
		StorageReference:        restorePath,
		ContentStorageReference: storageRef,
	}
}

func storeTestResourceContent(t *testing.T, extRoot string, content []byte) (storageRef, contentHash string, size int64) {
	store := NewResourceContentStore(extRoot)
	ref, hash, sz, err := store.StoreBytes(content)
	if err != nil {
		t.Fatalf("store test content: %v", err)
	}
	return ref, hash, sz
}

func jsonMustMarshal(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func TestPackageRestoreRejectsEmptyContentHash(t *testing.T) {
	runtime, _ := newPackagePipelineRuntime(t)
	entry := newPackageRestoreEntry("empty-hash-res", "file://data/x", "", 0, "", "")
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(context.Background(), "test-ext", point, installation)
	if err == nil {
		t.Fatal("expected failure for empty ContentHash")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeResourceSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeResourceSnapshotInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "content hash missing") {
		t.Fatalf("expected validation failure message, got %v", err)
	}
}

func TestPackageRestoreRejectsEmptyContentStorageReference(t *testing.T) {
	runtime, _ := newPackagePipelineRuntime(t)
	entry := newPackageRestoreEntry("empty-ref-res", "file://data/x", "sha256:aaa", 100, "", "/tmp/target")
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(context.Background(), "test-ext", point, installation)
	if err == nil {
		t.Fatal("expected failure for empty ContentStorageReference")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeResourceSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeResourceSnapshotInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "content storage reference missing") {
		t.Fatalf("expected storage reference failure message, got %v", err)
	}
}

func TestPackageRestoreRejectsMissingContentStorage(t *testing.T) {
	runtime, _ := newPackagePipelineRuntime(t)
	entry := newPackageRestoreEntry("missing-storage-res", "file://data/x", "sha256:2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b", 100, "nonexistent/storage/ref.bin", "/tmp/target")
	entry.Size = 100
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(context.Background(), "test-ext", point, installation)
	if err == nil {
		t.Fatal("expected failure when content storage reference cannot be resolved")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeResourceSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeResourceSnapshotInvalid, got %v", err)
	}
}

func TestPackageRestoreRejectsTamperedContentHash(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	extRoot, _ := filepath.Abs(container.ExtRoot)
	body := []byte("original content for tamper test")
	storageRef, _, _ := storeTestResourceContent(t, extRoot, body)
	entry := newPackageRestoreEntry("tamper-hash-res", "file://data/x", "sha256:deadbeefdeadbeefdeadbeef", int64(len(body)), storageRef, filepath.Join(container.ExtRoot, "tamper", "file.txt"))
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(ctx, "test-ext", point, installation)
	if err == nil {
		t.Fatal("expected failure for hash mismatch between stored content and declared ContentHash")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeResourceSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeResourceSnapshotInvalid, got %v", err)
	}
}

func TestPackageRestoreIsIdempotentWhenTargetHashMatches(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	extRoot, _ := filepath.Abs(container.ExtRoot)
	body := []byte("idempotent restore body")
	storageRef, contentHash, sz := storeTestResourceContent(t, extRoot, body)
	restoreDir := filepath.Join(container.ExtRoot, "idem")
	if err := os.MkdirAll(restoreDir, 0o700); err != nil {
		t.Fatal(err)
	}
	restorePath := filepath.Join(restoreDir, "body.txt")
	if err := os.WriteFile(restorePath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	entry := newPackageRestoreEntry("idem-res", "file://data/x", contentHash, sz, storageRef, restorePath)
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	if err := runtime.restorePackageRepositorySnapshots(ctx, "test-ext", point, installation); err != nil {
		t.Fatalf("idempotent restore must succeed, got %v", err)
	}
	got, err := os.ReadFile(restorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "idempotent restore body") {
		t.Fatalf("target content corrupted after idempotent restore: %q", string(got))
	}
}

func TestPackageRestoreConflictsTargetWithMismatchedHash(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	extRoot, _ := filepath.Abs(container.ExtRoot)
	body := []byte("new content must not replace mismatched target")
	storageRef, contentHash, sz := storeTestResourceContent(t, extRoot, body)
	restoreDir := filepath.Join(container.ExtRoot, "conflict")
	if err := os.MkdirAll(restoreDir, 0o700); err != nil {
		t.Fatal(err)
	}
	restorePath := filepath.Join(restoreDir, "file.txt")
	staleBody := []byte("stale content")
	if err := os.WriteFile(restorePath, staleBody, 0o600); err != nil {
		t.Fatal(err)
	}
	entry := newPackageRestoreEntry("conflict-res", "file://data/x", contentHash, sz, storageRef, restorePath)
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{entry}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(ctx, "test-ext", point, installation)
	if err == nil {
		t.Fatal("restore must fail when target exists with mismatched hash (no-replace semantics)")
	}
	var pkgErr *PackageError
	if errors.As(err, &pkgErr) && pkgErr.Code != PackageErrCodeResourceRestoreTargetChanged {
		t.Fatalf("expected PACKAGE_RESOURCE_RESTORE_TARGET_CHANGED, got %v", err)
	}
	got, readErr := os.ReadFile(restorePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, staleBody) {
		t.Fatalf("target was overwritten despite hash mismatch: %q", string(got))
	}
}

func TestPackageRestoreMidEntryFailure(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	extRoot, _ := filepath.Abs(container.ExtRoot)
	body := []byte("good entry body")
	storageRef, contentHash, sz := storeTestResourceContent(t, extRoot, body)
	restoreDir := filepath.Join(container.ExtRoot, "crash")
	if err := os.MkdirAll(restoreDir, 0o700); err != nil {
		t.Fatal(err)
	}
	goodPath := filepath.Join(restoreDir, "good.txt")
	if err := os.WriteFile(goodPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	good := newPackageRestoreEntry("good-res", "file://good", contentHash, sz, storageRef, goodPath)
	bad := packageResourceSnapshotEntry{
		Resource:                domain.ResourceOwnership{ResourceID: "bad-res", OwnerType: "extension", OwnerID: "test-ext", ResourceType: "file", Reference: "file://bad", AcquiredAt: time.Now().UTC()},
		ResourceHash:            "",
		RestoreStrategy:         "repository_upsert",
		ContentHash:             "",
		ContentStorageReference: "",
	}
	rawBad, _ := json.Marshal(bad.Resource)
	bad.ResourceHash = packageSnapshotDigest(rawBad)
	point := PackageRollbackPoint{ConfigSnapshotJSON: jsonMustMarshal(packageConfigSnapshot{}), ResourceSnapshotJSON: jsonMustMarshal(packageResourceSnapshot{Entries: []packageResourceSnapshotEntry{good, bad}})}
	installation := &domain.ExtensionInstallation{ExtensionID: "test-ext", Metadata: map[string]any{}}
	err := runtime.restorePackageRepositorySnapshots(ctx, "test-ext", point, installation)
	if err == nil {
		t.Fatal("expected failure when second entry violates integrity")
	}
	persisted, err := container.ResourceRepository.ListResources(ctx, "test-ext")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range persisted {
		if r.ResourceID == "good-res" || r.ResourceID == "bad-res" {
			t.Fatalf("resource %s must not be persisted after failed restore", r.ResourceID)
		}
	}
}
