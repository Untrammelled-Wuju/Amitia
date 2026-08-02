package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
)

func newRecoveryScannerTestDB(t *testing.T) (*Runtime, *Container) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	return runtime, container
}

func insertRecoveryTestOperation(t *testing.T, db *sql.DB, op PackageOperationRecord) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO extension_package_operations (
			operation_id, trace_id, user_id, scope_type, scope_id, extension_id, target_version,
			operation_type, status, current_step, artifact_id, preview_session_id,
			confirmations_json, confirmation_claims_json, error_code, error_detail,
			started_at, updated_at, completed_at, stable_generation, target_generation,
			current_pointer_json, snapshot_requirement_hash, recovery_required
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.OperationID, op.TraceID, op.UserID, op.ScopeType, op.ScopeID, op.ExtensionID, op.TargetVersion,
		op.OperationType, op.Status, op.CurrentStep, op.ArtifactID, op.PreviewSessionID,
		op.ConfirmationsJSON, op.ConfirmationClaimsJSON, op.ErrorCode, op.ErrorDetail,
		op.StartedAt, op.UpdatedAt, op.CompletedAt, op.StableGeneration, op.TargetGeneration,
		op.CurrentPointerJSON, op.SnapshotRequirementHash, op.RecoveryRequired,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecoverPackageOperationsScansIncompleteWithClaimsAndHash(t *testing.T) {
	runtime, container := newRecoveryScannerTestDB(t)
	ctx := context.Background()

	claimsJSON := `{"scanner":"test","confirmed":true,"items":[{"id":"perm-1","granted":true}]}`
	hashValue := "sha256:recovery-scanner-hash-001"

	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID:             "op-recover-scan-1",
		TraceID:                 "trace-scan-1",
		UserID:                  "user-recover-scanner",
		ScopeType:               "global",
		ScopeID:                 "",
		ExtensionID:             "ext-recovery-scan",
		TargetVersion:           "1.0.0",
		OperationType:           "install",
		Status:                  "requires_recovery",
		CurrentStep:             "commit_installed_tree",
		ArtifactID:              "artifact-recovery-scan",
		PreviewSessionID:        "preview-recover-scan",
		ConfirmationsJSON:       `{"riskAccepted":true}`,
		ConfirmationClaimsJSON:  claimsJSON,
		ErrorCode:               "PACKAGE_INSTALL_FAILED",
		ErrorDetail:             "simulated install failure for recovery scan test",
		StartedAt:               now,
		UpdatedAt:               now,
		StableGeneration:        "gen-stable-scan",
		TargetGeneration:        "gen-target-scan",
		CurrentPointerJSON:      `{"generationId":"gen-target-scan"}`,
		SnapshotRequirementHash: hashValue,
		RecoveryRequired:        true,
	}
	insertRecoveryTestOperation(t, container.PackageRepository.DB(), op)

	incompleteOps, err := container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		t.Fatalf("ListIncompleteOperations failed: %v", err)
	}

	if len(incompleteOps) != 1 {
		t.Fatalf("expected 1 incomplete operation, got %d", len(incompleteOps))
	}

	scanned := incompleteOps[0]
	if scanned.OperationID != "op-recover-scan-1" {
		t.Fatalf("expected op-recover-scan-1, got %s", scanned.OperationID)
	}
	if scanned.OperationType != "install" {
		t.Fatalf("expected install, got %s", scanned.OperationType)
	}
	if scanned.ConfirmationClaimsJSON != claimsJSON {
		t.Fatalf("ConfirmationClaimsJSON mismatch: got %q want %q", scanned.ConfirmationClaimsJSON, claimsJSON)
	}
	if scanned.SnapshotRequirementHash != hashValue {
		t.Fatalf("SnapshotRequirementHash mismatch: got %q want %q", scanned.SnapshotRequirementHash, hashValue)
	}
	if scanned.PreviewSessionID != "preview-recover-scan" {
		t.Fatalf("PreviewSessionID mismatch: got %q", scanned.PreviewSessionID)
	}
	if scanned.ErrorCode != "PACKAGE_INSTALL_FAILED" {
		t.Fatalf("ErrorCode mismatch: got %q", scanned.ErrorCode)
	}

	_ = runtime
}

func TestRecoveryScannerDiscardsCompleted(t *testing.T) {
	runtime, container := newRecoveryScannerTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	incompleteOp := PackageOperationRecord{
		OperationID:             "op-scanner-live",
		TraceID:                 "trace-live",
		UserID:                  "user-scanner-filter",
		ScopeType:               "global",
		ScopeID:                 "",
		ExtensionID:             "ext-scanner-live",
		TargetVersion:           "1.0.0",
		OperationType:           "install",
		Status:                  "in_progress",
		CurrentStep:             "commit",
		ArtifactID:              "art-live",
		PreviewSessionID:        "",
		ConfirmationsJSON:       "{}",
		ConfirmationClaimsJSON:  "{}",
		ErrorCode:               "",
		ErrorDetail:             "",
		StartedAt:               now,
		UpdatedAt:               now,
		SnapshotRequirementHash: "sha256:live-hash",
	}
	completedOp := PackageOperationRecord{
		OperationID:             "op-scanner-completed",
		TraceID:                 "trace-completed",
		UserID:                  "user-scanner-filter",
		ScopeType:               "global",
		ScopeID:                 "",
		ExtensionID:             "ext-scanner-completed",
		TargetVersion:           "1.0.0",
		OperationType:           "install",
		Status:                  "completed",
		CurrentStep:             "completed",
		ArtifactID:              "art-completed",
		PreviewSessionID:        "",
		ConfirmationsJSON:       "{}",
		ConfirmationClaimsJSON:  "{}",
		ErrorCode:               "",
		ErrorDetail:             "",
		StartedAt:               now,
		UpdatedAt:               now,
		CompletedAt:             now,
		SnapshotRequirementHash: "sha256:completed-hash-should-not-appear",
	}

	insertRecoveryTestOperation(t, container.PackageRepository.DB(), incompleteOp)
	insertRecoveryTestOperation(t, container.PackageRepository.DB(), completedOp)

	_ = runtime
	incompleteOps, err := container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		t.Fatalf("ListIncompleteOperations failed: %v", err)
	}

	if len(incompleteOps) != 1 {
		t.Fatalf("expected 1 incomplete operation (completed should be filtered), got %d", len(incompleteOps))
	}
	if incompleteOps[0].OperationID != "op-scanner-live" {
		t.Fatalf("expected op-scanner-live, got %s", incompleteOps[0].OperationID)
	}
}

func TestRecoveryScannerReadsAllOperationTypes(t *testing.T) {
	runtime, container := newRecoveryScannerTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	opTypes := []struct {
		opType string
		extID  string
		artID  string
		claims string
		hash   string
	}{
		{"install", "ext-type-install", "art-type-install", `{"install":true}`, "sha256:type-install-hash"},
		{"update", "ext-type-update", "art-type-update", `{"update":true}`, "sha256:type-update-hash"},
		{"rollback", "ext-type-rollback", "art-type-rollback", `{"rollback":true}`, "sha256:type-rollback-hash"},
		{"uninstall", "ext-type-uninstall", "art-type-uninstall", `{"uninstall":true}`, "sha256:type-uninstall-hash"},
	}

	for _, tc := range opTypes {
		op := PackageOperationRecord{
			OperationID:             "op-type-" + tc.opType,
			TraceID:                 "trace-type-" + tc.opType,
			UserID:                  "user-type-test",
			ScopeType:               "global",
			ScopeID:                 "",
			ExtensionID:             tc.extID,
			TargetVersion:           "1.0.0",
			OperationType:           tc.opType,
			Status:                  "requires_recovery",
			CurrentStep:             "failed-step",
			ArtifactID:              tc.artID,
			PreviewSessionID:        "",
			ConfirmationsJSON:       "{}",
			ConfirmationClaimsJSON:  tc.claims,
			ErrorCode:               "TYPE_TEST_FAIL",
			ErrorDetail:             "testing " + tc.opType + " scan",
			StartedAt:               now,
			UpdatedAt:               now,
			SnapshotRequirementHash: tc.hash,
			RecoveryRequired:        true,
		}
		insertRecoveryTestOperation(t, container.PackageRepository.DB(), op)
	}

	_ = runtime
	incompleteOps, err := container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		t.Fatalf("ListIncompleteOperations failed: %v", err)
	}

	if len(incompleteOps) != 4 {
		t.Fatalf("expected 4 incomplete operations (one per type), got %d", len(incompleteOps))
	}

	typeByKey := map[string]PackageOperationRecord{}
	for _, op := range incompleteOps {
		typeByKey[op.OperationType] = op
	}

	for _, tc := range opTypes {
		op, ok := typeByKey[tc.opType]
		if !ok {
			t.Fatalf("missing operation type: %s", tc.opType)
		}
		if op.ConfirmationClaimsJSON != tc.claims {
			t.Fatalf("%s: ConfirmationClaimsJSON mismatch: got %q want %q", tc.opType, op.ConfirmationClaimsJSON, tc.claims)
		}
		if op.SnapshotRequirementHash != tc.hash {
			t.Fatalf("%s: SnapshotRequirementHash mismatch: got %q want %q", tc.opType, op.SnapshotRequirementHash, tc.hash)
		}
		if op.ExtensionID != tc.extID {
			t.Fatalf("%s: ExtensionID mismatch: got %q want %q", tc.opType, op.ExtensionID, tc.extID)
		}
		if op.ArtifactID != tc.artID {
			t.Fatalf("%s: ArtifactID mismatch: got %q want %q", tc.opType, op.ArtifactID, tc.artID)
		}
		if op.ErrorCode != "TYPE_TEST_FAIL" {
			t.Fatalf("%s: ErrorCode should not be corrupted by column shift, got %q", tc.opType, op.ErrorCode)
		}
	}
}
