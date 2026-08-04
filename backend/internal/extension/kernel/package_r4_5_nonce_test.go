package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	persistencesqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func newR45NonceTestRepository(t *testing.T) *PackageRepository {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "r45-nonce.db")) + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := persistencesqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return NewPackageRepository(db)
}

func r45NonceBindingFixture(nonce, operationType, extensionID, userID string, issuedAtUnix, expiresAtUnix int64) PackageConfirmationNonceBinding {
	return PackageConfirmationNonceBinding{
		Nonce:         nonce,
		OperationType: operationType,
		ExtensionID:   extensionID,
		UserID:        userID,
		IssuedAt:      confirmationTimestamp(issuedAtUnix),
		ExpiresAt:     confirmationTimestamp(expiresAtUnix),
	}
}

func TestR45NonceConsumeRejectsMissingIssuedAt(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	binding := r45NonceBindingFixture("nonce-missing-issued", "install", "ext-1", "user-1", 0, time.Now().UTC().Add(time.Hour).Unix())
	binding.IssuedAt = ""
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("expected OperationErrTokenStale for missing issuedAt: %v", err)
	}
}

func TestR45NonceConsumeRejectsMalformedIssuedAt(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	binding := r45NonceBindingFixture("nonce-malformed-issued", "install", "ext-1", "user-1", time.Now().UTC().Unix(), time.Now().UTC().Add(time.Hour).Unix())
	binding.IssuedAt = "not-a-timestamp"
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("expected OperationErrTokenStale for malformed issuedAt: %v", err)
	}
}

func TestR45NonceConsumeRejectsMalformedExpiresAt(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	binding := r45NonceBindingFixture("nonce-malformed-expires", "install", "ext-1", "user-1", time.Now().UTC().Unix(), 0)
	binding.ExpiresAt = "bad-time"
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("expected OperationErrTokenStale for malformed expiresAt: %v", err)
	}
}

func TestR45NonceConsumeRejectsInvertedTimeWindow(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-inverted", "install", "ext-1", "user-1", now.Add(-time.Hour).Unix(), now.Add(-2*time.Hour).Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); err == nil {
		t.Fatal("inverted time window should be rejected")
	}
}

func TestR45NonceConsumeRejectsExpiredTimeWindow(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-expired", "install", "ext-1", "user-1", now.Add(-time.Hour).Unix(), now.Add(-30*time.Second).Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); err == nil {
		t.Fatal("expired time window should be rejected")
	}
}

func TestR45NonceConsumeRejectsFutureIssuedAt(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-future", "install", "ext-1", "user-1", now.Add(2*time.Minute).Unix(), now.Add(time.Hour).Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); err == nil {
		t.Fatal("future issuedAt beyond clock skew should be rejected")
	}
}

func TestR45NonceConsumePersistsCompleteBinding(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-persist", "install", "ext-persist", "user-persist", now.Unix(), now.Add(5*time.Minute).Unix())
	op := PackageOperationRecord{OperationID: "op-persist", OperationType: "install", ExtensionID: "ext-persist", UserID: "user-persist",
		IdempotencyKey: "key-persist", RequestHash: "hash-persist", AttemptCount: 1}
	existing, created, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding)
	if err != nil || !created {
		t.Fatalf("first create should succeed: created=%v err=%v", created, err)
	}
	var record struct {
		nonce, operationID, operationType, extensionID, userID, issuedAt, expiresAt, consumedAt string
	}
	row := repo.db.QueryRow(`SELECT nonce, operation_id, operation_type, extension_id, user_id, issued_at, expires_at, consumed_at FROM extension_package_confirmation_nonces WHERE operation_id=?`, existing.OperationID)
	if err := row.Scan(&record.nonce, &record.operationID, &record.operationType, &record.extensionID, &record.userID, &record.issuedAt, &record.expiresAt, &record.consumedAt); err != nil {
		t.Fatalf("query nonce record failed: %v", err)
	}
	if record.nonce != binding.Nonce || record.operationID != existing.OperationID || record.operationType != binding.OperationType {
		t.Fatalf("nonce record identity mismatch: %+v", record)
	}
	if record.extensionID != binding.ExtensionID || record.userID != binding.UserID {
		t.Fatalf("nonce record binding mismatch: %+v", record)
	}
	if record.issuedAt != binding.IssuedAt || record.expiresAt != binding.ExpiresAt {
		t.Fatalf("nonce record temporal mismatch: %+v", record)
	}
	if record.consumedAt == "" {
		t.Fatal("consumed_at should be set")
	}
}

func TestR45NonceReplayAcrossOperationsReturnsTokenStale(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-replay-op", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op1 := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op1, binding); err != nil {
		t.Fatalf("first create should succeed: %v", err)
	}
	op2 := PackageOperationRecord{OperationID: "op-2", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-2", RequestHash: "hash-2", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op2, binding); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("reused nonce across operations should return TOKEN_STALE: %v", err)
	}
}

func TestR45NonceReplayAcrossUsersReturnsTokenStale(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding1 := r45NonceBindingFixture("nonce-user-drift", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op1 := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op1, binding1); err != nil {
		t.Fatal(err)
	}
	binding2 := binding1
	binding2.UserID = "user-2"
	op2 := PackageOperationRecord{OperationID: "op-2", OperationType: "install", ExtensionID: "ext-1", UserID: "user-2",
		IdempotencyKey: "key-2", RequestHash: "hash-2", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op2, binding2); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("nonce user mismatch should return TOKEN_STALE: %v", err)
	}
}

func TestR45NonceReplayAcrossExtensionsReturnsTokenStale(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding1 := r45NonceBindingFixture("nonce-ext-drift", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op1 := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op1, binding1); err != nil {
		t.Fatal(err)
	}
	binding2 := binding1
	binding2.ExtensionID = "ext-2"
	op2 := PackageOperationRecord{OperationID: "op-2", OperationType: "install", ExtensionID: "ext-2", UserID: "user-1",
		IdempotencyKey: "key-2", RequestHash: "hash-2", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op2, binding2); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("nonce extension mismatch should return TOKEN_STALE: %v", err)
	}
}

func TestR45NonceReplayAcrossOperationTypesReturnsTokenStale(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding1 := r45NonceBindingFixture("nonce-type-drift", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op1 := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op1, binding1); err != nil {
		t.Fatal(err)
	}
	binding2 := binding1
	binding2.OperationType = "update"
	op2 := PackageOperationRecord{OperationID: "op-2", OperationType: "update", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-2", RequestHash: "hash-2", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op2, binding2); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("nonce operation type mismatch should return TOKEN_STALE: %v", err)
	}
}

func TestR45SameOperationSameNonceIdempotentRetryAllowed(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding := r45NonceBindingFixture("nonce-idempotent", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	_, created, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding)
	if err != nil || !created {
		t.Fatalf("first create should succeed: created=%v err=%v", created, err)
	}
	opRetry := op
	opRetry.AttemptCount = 2
	existing, created, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), opRetry, binding)
	if err != nil {
		t.Fatalf("same operation + same nonce retry should be allowed: %v", err)
	}
	if created {
		t.Fatal("retry should not create new operation")
	}
	if existing.OperationID != op.OperationID {
		t.Fatalf("retry should return same operation: got %q want %q", existing.OperationID, op.OperationID)
	}
}

func TestR45SameOperationDifferentNonceRejected(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	binding1 := r45NonceBindingFixture("nonce-diff-first", "install", "ext-1", "user-1", now.Unix(), now.Add(5*time.Minute).Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding1); err != nil {
		t.Fatal(err)
	}
	binding2 := binding1
	binding2.Nonce = "nonce-diff-second"
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding2); !IsPackageOperationError(err, OperationErrTokenStale) {
		t.Fatalf("same operation with different nonce should return TOKEN_STALE: %v", err)
	}
}

func TestR45NonceBindingRejectsConsumedBeforeIssued(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	issuedAt := now.Add(5 * time.Minute)
	expiresAt := issuedAt.Add(time.Minute)
	binding := r45NonceBindingFixture("nonce-consumed-before", "install", "ext-1", "user-1", issuedAt.Unix(), expiresAt.Unix())
	op := PackageOperationRecord{OperationID: "op-1", OperationType: "install", ExtensionID: "ext-1", UserID: "user-1",
		IdempotencyKey: "key-1", RequestHash: "hash-1", AttemptCount: 1}
	if _, _, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding); err == nil {
		t.Fatal("issuedAt in future means consumed_at would be before issuedAt")
	}
}

func TestR45ConcurrentNonceReplayAllowsExactlyOneOperation(t *testing.T) {
	repo := newR45NonceTestRepository(t)
	now := time.Now().UTC()
	const workers = 50
	var wg sync.WaitGroup
	results := make(chan string, workers)
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			binding := r45NonceBindingFixture("nonce-concurrent-shared", "install", "ext-concurrent", "user-concurrent", now.Unix(), now.Add(5*time.Minute).Unix())
			op := PackageOperationRecord{
				OperationID: "op-concurrent-" + string(rune('A'+idx)),
				OperationType: "install", ExtensionID: "ext-concurrent", UserID: "user-concurrent",
				IdempotencyKey: "key-concurrent-" + string(rune('a'+idx)), RequestHash: "hash-concurrent-" + string(rune('0'+idx)), AttemptCount: 1,
			}
			_, created, err := repo.CreateOrGetOperationWithConfirmationNonce(context.Background(), op, binding)
			if err != nil {
				errors <- err
				return
			}
			if created {
				results <- op.OperationID
			}
		}(i)
	}
	wg.Wait()
	close(results)
	close(errors)
	var createdCount int
	for range results {
		createdCount++
	}
	if createdCount != 1 {
		t.Fatalf("expected exactly one operation created, got %d", createdCount)
	}
	for err := range errors {
		if !IsPackageOperationError(err, OperationErrTokenStale) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
