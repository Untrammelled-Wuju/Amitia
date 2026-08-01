package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireArtifactReferenceIdempotent(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("idempotent archive"), time.Now().Add(-time.Hour))

	first, err := repository.AcquireArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "same-owner", time.Time{})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	second, err := repository.AcquireArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "same-owner", time.Time{})
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	if first.ReferenceID != second.ReferenceID {
		t.Fatalf("expected idempotent reference, got %s vs %s", first.ReferenceID, second.ReferenceID)
	}
	if first.ArtifactID != artifact.ArtifactID {
		t.Fatalf("expected artifact ID %s, got %s", artifact.ArtifactID, first.ArtifactID)
	}
	if first.ReferenceType != ArtifactReferenceManualRetention {
		t.Fatalf("expected reference type %s, got %s", ArtifactReferenceManualRetention, first.ReferenceType)
	}
	if first.ReferenceOwnerID != "same-owner" {
		t.Fatalf("expected owner 'same-owner', got %s", first.ReferenceOwnerID)
	}
}

func TestAcquireArtifactReferenceReturnsErrorForDeletedArtifact(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("deleted archive"), time.Now().Add(-time.Hour))
	artifact.DeletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatalf("mark artifact deleted: %v", err)
	}

	_, err := repository.AcquireArtifactReference(context.Background(), artifact.ArtifactID, ArtifactReferenceManualRetention, "owner", time.Time{})
	if err == nil {
		t.Fatal("expected error for deleted artifact")
	}
}

func TestSetArtifactInstalledPathNoChangeIsNoop(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("path archive"), time.Now().Add(-time.Hour))
	artifact.InstalledPath = "/existing/path"
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatalf("put artifact: %v", err)
	}

	guard := PackageWriteGuard{FencingToken: 1}
	err := repository.SetArtifactInstalledPath(context.Background(), artifact.ArtifactID, "/existing/path", guard)
	if err != nil {
		t.Fatalf("expected no error for same path, got: %v", err)
	}

	stored, err := repository.GetArtifact(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.InstalledPath != "/existing/path" {
		t.Fatalf("expected path unchanged, got %s", stored.InstalledPath)
	}
}

func TestSetArtifactInstalledPathChangesWhenDifferent(t *testing.T) {
	repository, store := packageArtifactTestRepository(t)
	artifact := putPackageArtifactForTest(t, repository, store, []byte("path archive"), time.Now().Add(-time.Hour))
	artifact.InstalledPath = "/existing/path"
	if err := repository.PutArtifact(context.Background(), artifact); err != nil {
		t.Fatalf("put artifact: %v", err)
	}

	guard := PackageWriteGuard{FencingToken: 1}
	err := repository.SetArtifactInstalledPath(context.Background(), artifact.ArtifactID, "/new/path", guard)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	stored, err := repository.GetArtifact(context.Background(), artifact.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.InstalledPath != "/new/path" {
		t.Fatalf("expected path to change to /new/path, got %s", stored.InstalledPath)
	}
}

func TestSetArtifactInstalledPathNotFound(t *testing.T) {
	repository, _ := packageArtifactTestRepository(t)
	guard := PackageWriteGuard{FencingToken: 1}
	err := repository.SetArtifactInstalledPath(context.Background(), "nonexistent", "/some/path", guard)
	if err == nil {
		t.Fatal("expected error for nonexistent artifact")
	}
	if !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
		t.Fatalf("expected NotFound error, got: %v", err)
	}
}

func TestRecoverUninstallCompensatedStateVersionStateNoop(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID:       "op-version-noop",
		TraceID:           "trace-version-noop",
		UserID:            "user-1",
		ScopeType:         "global",
		ExtensionID:       installed.ExtensionID,
		TargetVersion:     installed.Version,
		OperationType:     "uninstall",
		Status:            "in_progress",
		CurrentStep:       "move_to_quarantine",
		ArtifactID:        artifact.ArtifactID,
		StartedAt:         now,
		UpdatedAt:         now,
		ConfirmationsJSON: "{}",
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	quarantinePath := filepath.Join(runtime.root, "quarantine", op.OperationID)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(installed.InstallPath, quarantinePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(installed.InstallPath, 0o700); err != nil {
		t.Fatal(err)
	}

	qm := PackageQuarantineMetadata{
		OperationID:              op.OperationID,
		ExtensionID:              op.ExtensionID,
		ArtifactID:               op.ArtifactID,
		OriginalGenerationPath:   installed.InstallPath,
		GenerationQuarantinePath: quarantinePath,
		CurrentQuarantinePath:    installed.InstallPath,
		State:                    "active",
		FencingToken:             1,
	}
	if err := container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{FencingToken: 1}); err != nil {
		t.Fatal(err)
	}

	_, err = runtime.reconcileUninstallPackageGeneration(ctx, op, PackageWriteGuard{FencingToken: 1})
	if err != nil && !errors.Is(err, ErrPackageGenerationNotFound) {
		t.Logf("reconcileUninstallPackageGeneration returned: %v", err)
	}

	err = runtime.reconcileUninstallCompensatedState(ctx, op, PackageWriteGuard{FencingToken: 1})
	if err != nil {
		t.Fatalf("reconcileUninstallCompensatedState: %v", err)
	}

	versionRecord, err := container.PackageRepository.GetPackageVersion(ctx, op.ExtensionID, op.TargetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if versionRecord.VersionState != string(PackageVersionStateCurrent) {
		t.Fatalf("expected version state current, got %s", versionRecord.VersionState)
	}
}

func TestRecoverUninstallCompensatedStateVersionStateRestore(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID:       "op-version-restore",
		TraceID:           "trace-version-restore",
		UserID:            "user-1",
		ScopeType:         "global",
		ExtensionID:       installed.ExtensionID,
		TargetVersion:     installed.Version,
		OperationType:     "uninstall",
		Status:            "in_progress",
		CurrentStep:       "move_to_quarantine",
		ArtifactID:        artifact.ArtifactID,
		StartedAt:         now,
		UpdatedAt:         now,
		ConfirmationsJSON: "{}",
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	versionRecord, err := container.PackageRepository.GetPackageVersion(ctx, op.ExtensionID, op.TargetVersion)
	if err != nil {
		t.Fatal(err)
	}

	db := container.PackageRepository.DB()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE package_versions SET version_state=? WHERE version_id=?`,
		string(PackageVersionStateRetained), versionRecord.VersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE extension_installations SET current_version_id='', current_generation_id='' WHERE extension_id=?`,
		op.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	quarantinePath := filepath.Join(runtime.root, "quarantine", op.OperationID)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(installed.InstallPath, quarantinePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(installed.InstallPath, 0o700); err != nil {
		t.Fatal(err)
	}

	qm := PackageQuarantineMetadata{
		OperationID:              op.OperationID,
		ExtensionID:              op.ExtensionID,
		ArtifactID:               op.ArtifactID,
		OriginalGenerationPath:   installed.InstallPath,
		GenerationQuarantinePath: quarantinePath,
		CurrentQuarantinePath:    installed.InstallPath,
		State:                    "active",
		FencingToken:             1,
	}
	if err := container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{FencingToken: 1}); err != nil {
		t.Fatal(err)
	}

	_, err = runtime.reconcileUninstallPackageGeneration(ctx, op, PackageWriteGuard{FencingToken: 1})
	if err != nil && !errors.Is(err, ErrPackageGenerationNotFound) {
		t.Logf("reconcileUninstallPackageGeneration returned: %v", err)
	}

	err = runtime.reconcileUninstallCompensatedState(ctx, op, PackageWriteGuard{FencingToken: 1})
	if err != nil {
		t.Fatalf("reconcileUninstallCompensatedState: %v", err)
	}

	versionRecord, err = container.PackageRepository.GetPackageVersion(ctx, op.ExtensionID, op.TargetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if versionRecord.VersionState != string(PackageVersionStateCurrent) {
		t.Fatalf("expected version state current after restore, got %s", versionRecord.VersionState)
	}
}
