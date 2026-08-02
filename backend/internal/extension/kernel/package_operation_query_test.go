package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/sqlite"
)

func newPackageOperationQueryTestDB(t *testing.T) (*Runtime, *Container) {
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

func insertRawOperation(t *testing.T, db *sql.DB, op PackageOperationRecord) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO extension_package_operations (
			operation_id, trace_id, user_id, scope_type, scope_id, extension_id, target_version,
			operation_type, status, current_step, artifact_id, preview_session_id,
			confirmations_json, confirmation_claims_json, error_code, error_detail,
			started_at, updated_at, completed_at, stable_generation, target_generation,
			current_pointer_json, snapshot_requirement_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.OperationID, op.TraceID, op.UserID, op.ScopeType, op.ScopeID, op.ExtensionID, op.TargetVersion,
		op.OperationType, op.Status, op.CurrentStep, op.ArtifactID, op.PreviewSessionID,
		op.ConfirmationsJSON, op.ConfirmationClaimsJSON, op.ErrorCode, op.ErrorDetail,
		op.StartedAt, op.UpdatedAt, op.CompletedAt, op.StableGeneration, op.TargetGeneration,
		op.CurrentPointerJSON, op.SnapshotRequirementHash,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListIncompleteOperationsReadsClaimsAndHash(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	claimsJSON := `{"confirmed":true,"items":[{"id":"perm-1","granted":true}]}`
	hashValue := "sha256:abcdef1234567890"

	op := PackageOperationRecord{
		OperationID:             "op-incomplete-1",
		TraceID:                 "trace-1",
		UserID:                  "user-incomplete",
		ScopeType:               "global",
		ExtensionID:             "ext-1",
		TargetVersion:           "1.0.0",
		OperationType:           "install",
		Status:                  "in_progress",
		CurrentStep:             "commit_installed_tree",
		ArtifactID:              "artifact-1",
		PreviewSessionID:        "preview-1",
		ConfirmationsJSON:       `{"riskAccepted":true}`,
		ConfirmationClaimsJSON:  claimsJSON,
		StartedAt:               "2025-06-01T00:00:00Z",
		UpdatedAt:               "2025-06-01T00:00:00Z",
		SnapshotRequirementHash: hashValue,
	}
	insertRawOperation(t, container.PackageRepository.DB(), op)

	completed := PackageOperationRecord{
		OperationID:             "op-completed-1",
		TraceID:                 "trace-2",
		UserID:                  "user-incomplete",
		ScopeType:               "global",
		ExtensionID:             "ext-2",
		TargetVersion:           "1.0.0",
		OperationType:           "install",
		Status:                  "completed",
		CurrentStep:             "completed",
		ArtifactID:              "artifact-2",
		PreviewSessionID:        "",
		ConfirmationsJSON:       "{}",
		ConfirmationClaimsJSON:  "{}",
		StartedAt:               "2025-06-01T00:00:00Z",
		UpdatedAt:               "2025-06-01T00:00:01Z",
		CompletedAt:             "2025-06-01T00:00:01Z",
		ErrorCode:               "",
		ErrorDetail:             "",
		SnapshotRequirementHash: "should-not-appear",
	}
	insertRawOperation(t, container.PackageRepository.DB(), completed)

	operations, err := container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		t.Fatalf("ListIncompleteOperations failed: %v", err)
	}

	if len(operations) != 1 {
		t.Fatalf("expected 1 incomplete operation, got %d", len(operations))
	}

	got := operations[0]
	if got.OperationID != "op-incomplete-1" {
		t.Fatalf("expected operation_id op-incomplete-1, got %s", got.OperationID)
	}
	if got.ConfirmationClaimsJSON != claimsJSON {
		t.Fatalf("ConfirmationClaimsJSON mismatch: got %q, want %q", got.ConfirmationClaimsJSON, claimsJSON)
	}
	if got.SnapshotRequirementHash != hashValue {
		t.Fatalf("SnapshotRequirementHash mismatch: got %q, want %q", got.SnapshotRequirementHash, hashValue)
	}
	if got.ConfirmationsJSON != `{"riskAccepted":true}` {
		t.Fatalf("ConfirmationsJSON mismatch: got %q", got.ConfirmationsJSON)
	}
	if got.Status != "in_progress" {
		t.Fatalf("expected status in_progress, got %s", got.Status)
	}
}

func TestListIncompleteOperationsMultiRecord(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	requiresRecoveryClaims := `{"recovered":true}`
	installHash := "sha256:install-hash-111"
	updateHash := "sha256:update-hash-222"
	rollbackHash := "sha256:rollback-hash-333"
	uninstallHash := "sha256:uninstall-hash-444"

	ops := []PackageOperationRecord{
		{
			OperationID: "op-running", TraceID: "t-1", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-a", TargetVersion: "1.0.0",
			OperationType: "install", Status: "in_progress", CurrentStep: "commit",
			ArtifactID: "art-a", PreviewSessionID: "ps-a", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: `{"running":true}`, ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:00Z", UpdatedAt: "2025-06-01T00:00:00Z",
			SnapshotRequirementHash: installHash,
		},
		{
			OperationID: "op-recovery", TraceID: "t-2", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-b", TargetVersion: "2.0.0",
			OperationType: "update", Status: "requires_recovery", CurrentStep: "failed-step",
			ArtifactID: "art-b", PreviewSessionID: "ps-b", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: requiresRecoveryClaims, ErrorCode: "PACKAGE_UPDATE_FAILED",
			ErrorDetail: "simulated failure",
			StartedAt:   "2025-06-01T00:00:01Z", UpdatedAt: "2025-06-01T00:00:01Z",
			SnapshotRequirementHash: updateHash,
		},
		{
			OperationID: "op-finalizing", TraceID: "t-3", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-c", TargetVersion: "1.5.0",
			OperationType: "rollback", Status: "finalizing", CurrentStep: "final_gate",
			ArtifactID: "art-c", PreviewSessionID: "", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: "{}", ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:02Z", UpdatedAt: "2025-06-01T00:00:02Z",
			SnapshotRequirementHash: rollbackHash,
		},
		{
			OperationID: "op-install-uninstall", TraceID: "t-4", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-d", TargetVersion: "1.0.0",
			OperationType: "uninstall", Status: "in_progress", CurrentStep: "quarantine_move",
			ArtifactID: "art-d", PreviewSessionID: "", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: "{}", ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:03Z", UpdatedAt: "2025-06-01T00:00:03Z",
			SnapshotRequirementHash: uninstallHash,
		},
		{
			OperationID: "op-completed", TraceID: "t-5", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-e", TargetVersion: "1.0.0",
			OperationType: "install", Status: "completed", CurrentStep: "completed",
			ArtifactID: "art-e", PreviewSessionID: "", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: "{}", ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:00Z", UpdatedAt: "2025-06-01T00:00:01Z",
			CompletedAt:             "2025-06-01T00:00:01Z",
			SnapshotRequirementHash: "should-not-appear-completed",
		},
		{
			OperationID: "op-failed", TraceID: "t-6", UserID: "user-multi",
			ScopeType: "global", ExtensionID: "ext-f", TargetVersion: "1.0.0",
			OperationType: "install", Status: "failed", CurrentStep: "failed",
			ArtifactID: "art-f", PreviewSessionID: "", ConfirmationsJSON: "{}",
			ConfirmationClaimsJSON: "{}", ErrorCode: "INSTALL_FAIL", ErrorDetail: "boom",
			StartedAt: "2025-06-01T00:00:00Z", UpdatedAt: "2025-06-01T00:00:01Z",
			CompletedAt:             "2025-06-01T00:00:01Z",
			SnapshotRequirementHash: "should-not-appear-failed",
		},
	}

	for i := range ops {
		insertRawOperation(t, container.PackageRepository.DB(), ops[i])
	}

	operations, err := container.PackageRepository.ListIncompleteOperations(ctx)
	if err != nil {
		t.Fatalf("ListIncompleteOperations failed: %v", err)
	}

	if len(operations) != 4 {
		t.Fatalf("expected 4 incomplete operations, got %d", len(operations))
	}

	expectedIDs := map[string]bool{
		"op-running":           false,
		"op-recovery":          false,
		"op-finalizing":        false,
		"op-install-uninstall": false,
	}
	for _, op := range operations {
		if _, ok := expectedIDs[op.OperationID]; !ok {
			t.Fatalf("unexpected operation in results: %s", op.OperationID)
		}
		expectedIDs[op.OperationID] = true
	}
	for id, found := range expectedIDs {
		if !found {
			t.Fatalf("missing expected operation: %s", id)
		}
	}

	claimsByID := map[string]string{}
	for _, op := range operations {
		claimsByID[op.OperationID] = op.ConfirmationClaimsJSON
	}
	if claimsByID["op-recovery"] != requiresRecoveryClaims {
		t.Fatalf("op-recovery claims mismatch: %q", claimsByID["op-recovery"])
	}
	if claimsByID["op-running"] != `{"running":true}` {
		t.Fatalf("op-running claims mismatch: %q", claimsByID["op-running"])
	}

	hashByID := map[string]string{}
	for _, op := range operations {
		hashByID[op.OperationID] = op.SnapshotRequirementHash
	}
	if hashByID["op-running"] != installHash {
		t.Fatalf("op-running hash mismatch: %q", hashByID["op-running"])
	}
	if hashByID["op-recovery"] != updateHash {
		t.Fatalf("op-recovery hash mismatch: %q", hashByID["op-recovery"])
	}
	if hashByID["op-finalizing"] != rollbackHash {
		t.Fatalf("op-finalizing hash mismatch: %q", hashByID["op-finalizing"])
	}
	if hashByID["op-install-uninstall"] != uninstallHash {
		t.Fatalf("op-install-uninstall hash mismatch: %q", hashByID["op-install-uninstall"])
	}
}

func TestGetOperationReadsClaimsAndHash(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	claimsJSON := `{"confirmed":true,"items":[{"id":"perm-1","granted":true},{"id":"perm-2","granted":false}]}`
	hashValue := "sha256:getop-hash-abcdef"

	op := PackageOperationRecord{
		OperationID:             "op-getop-1",
		TraceID:                 "trace-getop",
		UserID:                  "user-getop",
		ScopeType:               "global",
		ExtensionID:             "ext-getop",
		TargetVersion:           "2.0.0",
		OperationType:           "update",
		Status:                  "in_progress",
		CurrentStep:             "execute_migrations",
		ArtifactID:              "artifact-getop",
		PreviewSessionID:        "preview-getop",
		ConfirmationsJSON:       `{"riskAccepted":true,"migrationAccepted":true}`,
		ConfirmationClaimsJSON:  claimsJSON,
		ErrorCode:               "",
		ErrorDetail:             "",
		StartedAt:               "2025-06-01T00:00:00Z",
		UpdatedAt:               "2025-06-01T00:00:05Z",
		StableGeneration:        "gen-stable-1",
		TargetGeneration:        "gen-target-1",
		CurrentPointerJSON:      `{"generationId":"gen-target-1"}`,
		SnapshotRequirementHash: hashValue,
	}
	insertRawOperation(t, container.PackageRepository.DB(), op)

	got, _, err := container.PackageRepository.GetOperation(ctx, "user-getop", "op-getop-1")
	if err != nil {
		t.Fatalf("GetOperation failed: %v", err)
	}

	if got.OperationID != "op-getop-1" {
		t.Fatalf("expected operation_id op-getop-1, got %s", got.OperationID)
	}
	if got.UserID != "user-getop" {
		t.Fatalf("expected user-getop got %s", got.UserID)
	}
	if got.OperationType != "update" {
		t.Fatalf("expected update got %s", got.OperationType)
	}
	if got.ConfirmationsJSON != `{"riskAccepted":true,"migrationAccepted":true}` {
		t.Fatalf("ConfirmationsJSON mismatch: %q", got.ConfirmationsJSON)
	}
	if got.ConfirmationClaimsJSON != claimsJSON {
		t.Fatalf("ConfirmationClaimsJSON mismatch: got %q want %q", got.ConfirmationClaimsJSON, claimsJSON)
	}
	if got.SnapshotRequirementHash != hashValue {
		t.Fatalf("SnapshotRequirementHash mismatch: got %q want %q", got.SnapshotRequirementHash, hashValue)
	}
	if got.StableGeneration != "gen-stable-1" {
		t.Fatalf("StableGeneration mismatch: %q", got.StableGeneration)
	}
	if got.TargetGeneration != "gen-target-1" {
		t.Fatalf("TargetGeneration mismatch: %q", got.TargetGeneration)
	}
	if got.CurrentPointerJSON != `{"generationId":"gen-target-1"}` {
		t.Fatalf("CurrentPointerJSON mismatch: %q", got.CurrentPointerJSON)
	}
}

func TestGetCompletedOperationByPreviewReadsClaimsAndHash(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	completedClaims := `{"recovered":true,"completed":"yes"}`
	completedHash := "sha256:completed-hash-999"

	inserted := []PackageOperationRecord{
		{
			OperationID: "op-preview-incomplete", TraceID: "t-1", UserID: "user-preview",
			ScopeType: "global", ExtensionID: "ext-preview", TargetVersion: "1.0.0",
			OperationType: "install", Status: "in_progress", CurrentStep: "commit",
			ArtifactID: "art-1", PreviewSessionID: "preview-session-xyz",
			ConfirmationsJSON: "{}", ConfirmationClaimsJSON: "{}", ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:00Z", UpdatedAt: "2025-06-01T00:00:00Z",
			SnapshotRequirementHash: "incomplete-hash",
		},
		{
			OperationID: "op-preview-completed-old", TraceID: "t-2", UserID: "user-preview",
			ScopeType: "global", ExtensionID: "ext-preview", TargetVersion: "1.0.0",
			OperationType: "install", Status: "completed", CurrentStep: "completed",
			ArtifactID: "art-1", PreviewSessionID: "preview-session-xyz",
			ConfirmationsJSON: "{}", ConfirmationClaimsJSON: "{}", ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:00:00Z", UpdatedAt: "2025-06-01T00:00:01Z",
			CompletedAt:             "2025-06-01T00:00:01Z",
			SnapshotRequirementHash: "old-hash",
		},
		{
			OperationID: "op-preview-completed-new", TraceID: "t-3", UserID: "user-preview",
			ScopeType: "global", ExtensionID: "ext-preview", TargetVersion: "1.0.0",
			OperationType: "install", Status: "completed", CurrentStep: "completed",
			ArtifactID: "art-1", PreviewSessionID: "preview-session-xyz",
			ConfirmationsJSON: "{}", ConfirmationClaimsJSON: completedClaims, ErrorCode: "", ErrorDetail: "",
			StartedAt: "2025-06-01T00:01:00Z", UpdatedAt: "2025-06-01T00:01:01Z",
			CompletedAt:             "2025-06-01T00:01:01Z",
			SnapshotRequirementHash: completedHash,
		},
	}

	for i := range inserted {
		insertRawOperation(t, container.PackageRepository.DB(), inserted[i])
	}

	got, err := container.PackageRepository.GetCompletedOperationByPreview(ctx, "user-preview", "preview-session-xyz")
	if err != nil {
		t.Fatalf("GetCompletedOperationByPreview failed: %v", err)
	}

	if got.OperationID != "op-preview-completed-new" {
		t.Fatalf("expected op-preview-completed-new, got %s", got.OperationID)
	}
	if got.ConfirmationClaimsJSON != completedClaims {
		t.Fatalf("ConfirmationClaimsJSON mismatch: got %q want %q", got.ConfirmationClaimsJSON, completedClaims)
	}
	if got.SnapshotRequirementHash != completedHash {
		t.Fatalf("SnapshotRequirementHash mismatch: got %q want %q", got.SnapshotRequirementHash, completedHash)
	}
}

func TestListOperationsPagination(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		op := PackageOperationRecord{
			OperationID:             "op-page-" + string(rune('a'+i)),
			TraceID:                 "trace-page-" + string(rune('a'+i)),
			UserID:                  "user-page",
			ScopeType:               "global",
			ExtensionID:             "ext-page",
			TargetVersion:           "1.0.0",
			OperationType:           "install",
			Status:                  "in_progress",
			CurrentStep:             "commit",
			ArtifactID:              "artifact-page-" + string(rune('a'+i)),
			ConfirmationsJSON:       "{}",
			ConfirmationClaimsJSON:  `{"page":"` + string(rune('0'+i)) + `"}`,
			StartedAt:               "2025-06-01T00:00:0" + string(rune('0'+i)) + "Z",
			UpdatedAt:               "2025-06-01T00:00:00Z",
			SnapshotRequirementHash: "sha256:page" + string(rune('a'+i)),
		}
		insertRawOperation(t, container.PackageRepository.DB(), op)
	}

	operations, err := container.PackageRepository.ListOperations(ctx, "user-page", 3)
	if err != nil {
		t.Fatalf("ListOperations failed: %v", err)
	}

	if len(operations) != 3 {
		t.Fatalf("expected 3 results with LIMIT 3, got %d", len(operations))
	}

	for _, op := range operations {
		if op.UserID != "user-page" {
			t.Fatalf("unexpected user_id %s", op.UserID)
		}
	}
}

func TestListOperationsReturnsErrorNotSilencedList(t *testing.T) {
	_, container := newPackageOperationQueryTestDB(t)
	ctx := context.Background()

	operations, err := container.PackageRepository.ListOperations(ctx, "user-noops", 50)
	if err != nil {
		t.Fatalf("empty ListOperations should not fail: %v", err)
	}
	if len(operations) != 0 {
		t.Fatalf("expected 0 operations, got %d", len(operations))
	}
}
