package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func phase35ReviewFixture(t *testing.T, now time.Time) (*Repository, string, string) {
	t.Helper()
	repo := phase34Repository(t)
	if err := repo.db.Exec(`CREATE TABLE knowledge_vector_recovery_reviews (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, audit_id TEXT NOT NULL,
 recheck_id TEXT NOT NULL, receipt_hash TEXT NOT NULL, request_key TEXT NOT NULL,
 requested_by_hash TEXT NOT NULL, reviewed_by_hash TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL CHECK(status IN ('pending','approved','revoked')),
 action TEXT NOT NULL CHECK(action = 'manual_investigation'),
 created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, expires_at DATETIME NOT NULL,
 UNIQUE(tenant_id, request_key), UNIQUE(tenant_id, audit_id, recheck_id))`).Error; err != nil {
		t.Fatal(err)
	}
	_, scan, load := phase30PageFixture(10)
	audit, err := repo.auditOrphanPageWithStore(context.Background(), "tenant-A", []string{"collection-A"}, 25, "", now, scan, load)
	if err != nil || audit.SuspectedOrphans != 10 {
		t.Fatalf("audit: %+v %v", audit, err)
	}
	checked, err := repo.recheckAuditWithLease(context.Background(), "tenant-A", audit.AuditID, now, func(context.Context) ([]string, error) { return []string{"collection-A"}, nil }, scan, load)
	if err != nil || checked.Status != "matched" {
		t.Fatalf("recheck: %+v %v", checked, err)
	}
	if err := repo.db.Exec(`CREATE TABLE knowledge_vector_recovery_review_events (
 id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, review_id TEXT NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('requested','approved','revoked')),
 actor_hash TEXT NOT NULL, receipt_hash TEXT NOT NULL, created_at DATETIME NOT NULL
 )`).Error; err != nil {
		t.Fatal(err)
	}
	return repo, audit.AuditID, checked.RecheckID
}

func TestPhase35RecoveryReviewIdempotencyApprovalRevocation(t *testing.T) {
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	first, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00001", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(time.Second))
	if err != nil || first.Request.Status != "pending" || first.Idempotent || first.Executable {
		t.Fatalf("creation: %+v %v", first, err)
	}
	repeated, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00001", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(2*time.Second))
	if err != nil || !repeated.Idempotent || repeated.Request.ID != first.Request.ID {
		t.Fatalf("repeated: %+v %v", repeated, err)
	}
	if _, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00001", "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("foreign actor reused key: %v", err)
	}
	if _, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00002", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(4*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("duplicate evidence: %v", err)
	}
	approved, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", "approve", now.Add(5*time.Second))
	if err != nil || approved.Request.Status != "approved" || approved.Executable {
		t.Fatalf("approval: %+v %v", approved, err)
	}
	revoked, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", "revoke", now.Add(6*time.Second))
	if err != nil || revoked.Request.Status != "revoked" || revoked.Executable {
		t.Fatalf("revocation: %+v %v", revoked, err)
	}
	replay, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", "revoke", now.Add(7*time.Second))
	if err != nil || !replay.Idempotent {
		t.Fatalf("revoke replay: %+v %v", replay, err)
	}
	var events []RecoveryReviewEvent
	if err := repo.db.Table("knowledge_vector_recovery_review_events").Where("tenant_id = ? AND review_id = ?", "tenant-A", first.Request.ID).Order("created_at ASC").Find(&events).Error; err != nil || len(events) != 3 || events[0].Action != "requested" || events[1].Action != "approved" || events[2].Action != "revoked" {
		t.Fatalf("review audit trail lost: %+v %v", events, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", "approve", now.Add(8*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("revoked approval accepted: %v", err)
	}
}

func TestPhase35RecoveryReviewEvidenceExpiryAndTenantIsolation(t *testing.T) {
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	if _, err := repo.createRecoveryReview(ctx, "tenant-B", auditID, recheckID, "request-key-00003", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now); err == nil {
		t.Fatal("foreign tenant proposal accepted")
	}
	if _, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00003", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(6*time.Minute)); !errors.Is(err, ErrRecoveryReviewEvidenceInvalid) {
		t.Fatalf("old evidence accepted: %v", err)
	}
	first, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00004", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-B", first.Request.ID, "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", "approve", now.Add(2*time.Second)); err == nil {
		t.Fatal("foreign approval accepted")
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", "approve", now.Add(16*time.Minute)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("expired approval accepted: %v", err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", "revoke", now.Add(16*time.Minute)); err != nil {
		t.Fatalf("expired request could not be revoked: %v", err)
	}
}

func TestPhase35ReviewRejectsNewerRecheckEvidence(t *testing.T) {
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	first, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "request-key-00005", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`UPDATE knowledge_vector_audit_recheck_results SET observed_at = ? WHERE id = ?`, now.Add(2*time.Second), recheckID).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec(`INSERT INTO knowledge_vector_audit_recheck_results (id,tenant_id,audit_id,page,status,result_hash,receipt_hash,observed_at,created_at) SELECT ?,tenant_id,audit_id,page,'changed',result_hash,receipt_hash,?,? FROM knowledge_vector_audit_recheck_results WHERE id = ?`, "cccccccc-cccc-4ccc-8ccc-cccccccccccc", now.Add(3*time.Second), now.Add(3*time.Second), recheckID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", first.Request.ID, "device-v2:51c9e4bafdc419f567f8ea9c3b15ba7ef171117674271a2636cc1e2d5d5ec840", "approve", now.Add(4*time.Second)); !errors.Is(err, ErrRecoveryReviewEvidenceInvalid) {
		t.Fatalf("older matching recheck accepted: %v", err)
	}
}

func TestPhase35RecoveryReviewEventInsertFailureRollsBack(t *testing.T) {
	now := time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	if err := repo.db.Exec("DROP TABLE knowledge_vector_recovery_review_events").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.createRecoveryReview(context.Background(), "tenant-A", auditID, recheckID, "request-key-00006", "device-v2:65eb2569e45e23e71f43889f25e5f0e6acdd1d550eaf6d03a63aa956cec2e75b", now.Add(time.Second)); err == nil {
		t.Fatal("missing operation log accepted")
	}
	var count int64
	if err := repo.db.Table("knowledge_vector_recovery_reviews").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("partial review persisted: %d %v", count, err)
	}
}
