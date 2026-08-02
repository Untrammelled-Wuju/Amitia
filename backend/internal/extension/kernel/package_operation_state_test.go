package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	persistencesqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func newOperationStateTestRepository(t *testing.T) *PackageRepository {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", filepath.ToSlash(filepath.Join(t.TempDir(), "operations.db")))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = db.Close() })
	if err := persistencesqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return NewPackageRepository(db)
}

func operationFixture(id, userID, key, requestHash, extensionID string) PackageOperationRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return PackageOperationRecord{OperationID: id, TraceID: "trace-" + id, UserID: userID,
		ScopeType: "global", ExtensionID: extensionID, TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "install", Status: string(PackageOperationPending), CurrentStep: "created",
		ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now, IdempotencyKey: key,
		RequestHash: requestHash, AttemptCount: 1}
}

func TestCreateOrGetOperationConcurrentIdempotency(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	const workers = 100
	var wait sync.WaitGroup
	results := make(chan PackageOperationRecord, workers)
	errorsSeen := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			op, _, err := repository.CreateOrGetOperation(context.Background(),
				operationFixture(fmt.Sprintf("operation-%03d", i), "user-1", "request-1", "sha256:same", "extension-1"))
			if err != nil {
				errorsSeen <- err
				return
			}
			results <- op
		}(index)
	}
	wait.Wait()
	close(results)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatalf("concurrent create failed: %v", err)
	}
	operationIDs := map[string]bool{}
	for operation := range results {
		operationIDs[operation.OperationID] = true
	}
	if len(operationIDs) != 1 {
		t.Fatalf("expected one authoritative operation, got %v", operationIDs)
	}
	var count int
	if err := repository.db.QueryRow(`SELECT COUNT(*) FROM extension_package_operations WHERE user_id='user-1' AND idempotency_key='request-1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected operation count=%d err=%v", count, err)
	}
}

func TestCreateOrGetOperationRejectsDifferentPayload(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	if _, _, err := repository.CreateOrGetOperation(context.Background(), operationFixture("operation-1", "user-1", "request-1", "sha256:first", "extension-1")); err != nil {
		t.Fatal(err)
	}
	_, _, err := repository.CreateOrGetOperation(context.Background(), operationFixture("operation-2", "user-1", "request-1", "sha256:second", "extension-1"))
	if !IsPackageOperationError(err, OperationErrIdempotencyConflict) {
		t.Fatalf("expected stable idempotency conflict, got %v", err)
	}
}

func TestExtensionLeaseMutualExclusionAndExpiredTakeover(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	for _, operation := range []PackageOperationRecord{
		operationFixture("operation-1", "user-1", "request-1", "sha256:one", "extension-1"),
		operationFixture("operation-2", "user-2", "request-2", "sha256:two", "extension-1"),
	} {
		if _, _, err := repository.CreateOrGetOperation(context.Background(), operation); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.AcquireExtensionLease(context.Background(), "extension-1", "operation-1", "worker-1", 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.AcquireExtensionLease(context.Background(), "extension-1", "operation-2", "worker-2", time.Minute); !IsPackageOperationError(err, OperationErrLeaseConflict) {
		t.Fatalf("expected lease conflict, got %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	lease, err := repository.AcquireExtensionLease(context.Background(), "extension-1", "operation-2", "worker-2", time.Minute)
	if err != nil || lease.OperationID != "operation-2" || lease.CASVersion < 2 {
		t.Fatalf("expired lease was not taken over: %#v err=%v", lease, err)
	}
}

func TestOperationTransitionCASRejectsIllegalAndStaleStatus(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	if _, _, err := repository.CreateOrGetOperation(context.Background(), operationFixture("operation-1", "user-1", "request-1", "sha256:one", "extension-1")); err != nil {
		t.Fatal(err)
	}
	if err := repository.TransitionOperation(context.Background(), "operation-1", []PackageOperationStatus{PackageOperationPending}, PackageOperationCompleted, PackageOperationTransition{}, PackageWriteGuard{}); !IsPackageOperationError(err, OperationErrTransitionConflict) {
		t.Fatalf("expected illegal transition error, got %v", err)
	}
	if err := repository.TransitionOperation(context.Background(), "operation-1", []PackageOperationStatus{PackageOperationPending}, PackageOperationInProgress, PackageOperationTransition{CurrentStep: "prepare"}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	if err := repository.TransitionOperation(context.Background(), "operation-1", []PackageOperationStatus{PackageOperationPending}, PackageOperationFailed, PackageOperationTransition{}, PackageWriteGuard{}); !IsPackageOperationError(err, OperationErrTransitionConflict) {
		t.Fatalf("expected stale transition error, got %v", err)
	}
}

func TestOperationStepSideEffectIdempotencyAndCompensation(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	if _, _, err := repository.CreateOrGetOperation(context.Background(), operationFixture("operation-1", "user-1", "request-1", "sha256:one", "extension-1")); err != nil {
		t.Fatal(err)
	}
	if _, created, err := repository.BeginStep(context.Background(), "operation-1", "write-files", 1, "sha256:input", PackageWriteGuard{}); err != nil || !created {
		t.Fatalf("begin step failed: created=%v err=%v", created, err)
	}
	if _, err := repository.CompleteStep(context.Background(), "operation-1", "write-files", "sha256:input", `{"ok":true}`, "file:sha256:one", PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CompleteStep(context.Background(), "operation-1", "write-files", "sha256:input", `{"ok":true}`, "file:sha256:one", PackageWriteGuard{}); err != nil {
		t.Fatalf("identical side effect retry must be idempotent: %v", err)
	}
	if _, err := repository.CompleteStep(context.Background(), "operation-1", "write-files", "sha256:input", `{"ok":true}`, "file:sha256:two", PackageWriteGuard{}); !IsPackageOperationError(err, OperationErrSideEffectConflict) {
		t.Fatalf("expected side effect conflict, got %v", err)
	}
	if _, err := repository.BeginCompensation(context.Background(), "operation-1", "write-files", "remove-files", PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CompleteCompensation(context.Background(), "operation-1", "write-files", "remove-files", "removed:file:sha256:one", PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
}

func TestOperationCancelAndRetryRequireStateRecoveryAndLease(t *testing.T) {
	repository := newOperationStateTestRepository(t)
	op := operationFixture("operation-1", "user-1", "request-1", "sha256:one", "extension-1")
	if _, _, err := repository.CreateOrGetOperation(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	if err := repository.RequestCancel(context.Background(), op.OperationID); err != nil {
		t.Fatal(err)
	}
	if err := repository.RequestCancel(context.Background(), op.OperationID); !IsPackageOperationError(err, OperationErrCancelNotAllowed) {
		t.Fatalf("expected duplicate cancel rejection, got %v", err)
	}
	if err := repository.TransitionOperation(context.Background(), op.OperationID, []PackageOperationStatus{PackageOperationPending}, PackageOperationFailed,
		PackageOperationTransition{ErrorCode: "FAILED", ErrorDetail: "retryable", RecoveryRequired: true, Completed: true}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.RetryOperation(context.Background(), op.OperationID, "worker-1"); !IsPackageOperationError(err, OperationErrRetryNotAllowed) {
		t.Fatalf("retry without lease must fail, got %v", err)
	}
	if _, err := repository.AcquireExtensionLease(context.Background(), op.ExtensionID, op.OperationID, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	retried, err := repository.RetryOperation(context.Background(), op.OperationID, "worker-1")
	if err != nil || retried.Status != string(PackageOperationPending) || retried.AttemptCount != 2 {
		t.Fatalf("unexpected retry result: %#v err=%v", retried, err)
	}
}

func TestOperationAuthorityMigratesLegacyDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE extension_package_operations (
		operation_id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, user_id TEXT NOT NULL,
		scope_type TEXT NOT NULL, scope_id TEXT NOT NULL DEFAULT '', extension_id TEXT NOT NULL,
		target_version TEXT NOT NULL DEFAULT '', operation_type TEXT NOT NULL, status TEXT NOT NULL,
		current_step TEXT NOT NULL, artifact_id TEXT NOT NULL DEFAULT '', preview_session_id TEXT NOT NULL DEFAULT '',
		confirmations_json TEXT NOT NULL DEFAULT '{}', error_code TEXT NOT NULL DEFAULT '',
		error_detail TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL, updated_at TEXT NOT NULL,
		completed_at TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE extension_package_operation_steps (
		step_id TEXT PRIMARY KEY, operation_id TEXT NOT NULL, step_name TEXT NOT NULL, step_order INTEGER NOT NULL,
		status TEXT NOT NULL, attempt_count INTEGER NOT NULL DEFAULT 0, result_json TEXT NOT NULL DEFAULT '{}',
		error_code TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL DEFAULT '', completed_at TEXT NOT NULL DEFAULT '',
		UNIQUE(operation_id, step_name))`); err != nil {
		t.Fatal(err)
	}
	if err := persistencesqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	for table, columns := range map[string][]string{
		"extension_package_operations":      {"idempotency_key", "request_hash", "recovery_required", "lease_owner", "attempt_count"},
		"extension_package_operation_steps": {"input_hash", "side_effect_evidence", "compensation_status", "cas_version"},
	} {
		for _, column := range columns {
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, table, column).Scan(&count); err != nil || count != 1 {
				t.Fatalf("missing migrated column %s.%s count=%d err=%v", table, column, count, err)
			}
		}
	}
}

func TestValidateQuarantineMetadataFence_TokenScenarios(t *testing.T) {
	op := PackageOperationRecord{OperationID: "op-fence-test", ExtensionID: "ext-fence", ArtifactID: "artifact-fence"}

	testCases := []struct {
		name          string
		metadataToken int64
		recoveryToken int64
		leaseToken    int64
		leaseOpID     string
		leaseErr      error
		wantErr       bool
		wantErrCode   string
	}{
		{
			name:          "cross_process_recovery_metadata_10_recovery_11_live_11_success",
			metadataToken: 10,
			recoveryToken: 11,
			leaseToken:    11,
			leaseOpID:     op.OperationID,
			wantErr:       false,
		},
		{
			name:          "same_process_recovery_metadata_11_recovery_11_live_11_success",
			metadataToken: 11,
			recoveryToken: 11,
			leaseToken:    11,
			leaseOpID:     op.OperationID,
			wantErr:       false,
		},
		{
			name:          "metadata_token_exceeds_recovery_stale",
			metadataToken: 12,
			recoveryToken: 11,
			leaseToken:    11,
			leaseOpID:     op.OperationID,
			wantErr:       true,
			wantErrCode:   OperationErrTokenStale,
		},
		{
			name:          "lease_operation_id_mismatch_rejected",
			metadataToken: 10,
			recoveryToken: 11,
			leaseToken:    11,
			leaseOpID:     "op-other",
			wantErr:       true,
			wantErrCode:   OperationErrLeaseProofMismatch,
		},
		{
			name:          "repository_unavailable_fail_closed",
			metadataToken: 10,
			recoveryToken: 11,
			leaseErr:      NewRepositoryError(RepositoryErrorUnavailable, fmt.Errorf("simulated unavailable")),
			wantErr:       true,
			wantErrCode:   OperationErrProofUnavailable,
		},
		{
			name:          "old_guard_fencing_attempt_rejected",
			metadataToken: 11,
			recoveryToken: 10,
			leaseToken:    11,
			leaseOpID:     op.OperationID,
			wantErr:       true,
			wantErrCode:   OperationErrTokenStale,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			qm := PackageQuarantineMetadata{
				QuarantineID: "qid-fence",
				OperationID:  op.OperationID,
				ExtensionID:  op.ExtensionID,
				TreeHash:     "tree-hash-fence",
				State:        "active",
				FencingToken: tc.metadataToken,
			}
			querier := func(ctx context.Context, extensionID string) (PackageExtensionLease, error) {
				if tc.leaseErr != nil {
					return PackageExtensionLease{}, tc.leaseErr
				}
				return PackageExtensionLease{
					ExtensionID:  extensionID,
					OperationID:  tc.leaseOpID,
					FencingToken: tc.leaseToken,
				}, nil
			}
			err := validateQuarantineMetadataFence(qm, op, tc.recoveryToken, querier)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.wantErrCode != "" && !IsPackageOperationError(err, tc.wantErrCode) {
				t.Fatalf("expected error code %s, got %v", tc.wantErrCode, err)
			}
		})
	}
}
