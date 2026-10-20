package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhase36ReviewRejectsSameActorApproval(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	review, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase36-request-0001", "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", "approve", now.Add(2*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("self approval must be refused: %v", err)
	}
	var eventCount int64
	if err := repo.db.Table("knowledge_vector_recovery_review_events").Where("tenant_id = ? AND review_id = ?", "tenant-A", review.Request.ID).Count(&eventCount).Error; err != nil || eventCount != 1 {
		t.Fatalf("self approval modified audit history: %d %v", eventCount, err)
	}
	approved, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2d70999ae1805e4bcef9b4ab3a4b827f578c61740f30076fcdc35c7ae7f586b3", "approve", now.Add(3*time.Second))
	if err != nil || approved.Request.Status != "approved" || approved.Executable {
		t.Fatalf("independent approval: %+v %v", approved, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", "approve", now.Add(3500*time.Millisecond)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("requester approval replay accepted: %v", err)
	}
	repeatedApproval, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2d70999ae1805e4bcef9b4ab3a4b827f578c61740f30076fcdc35c7ae7f586b3", "approve", now.Add(3500*time.Millisecond))
	if err != nil || !repeatedApproval.Idempotent {
		t.Fatalf("original reviewer approval replay denied: %+v %v", repeatedApproval, err)
	}
	revoked, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", "revoke", now.Add(4*time.Second))
	if err != nil || revoked.Request.Status != "revoked" {
		t.Fatalf("revoke: %+v %v", revoked, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2d70999ae1805e4bcef9b4ab3a4b827f578c61740f30076fcdc35c7ae7f586b3", "approve", now.Add(5*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("revoked review reapproved: %v", err)
	}
}

func TestPhase36ReadOnlyRecoveryPlanAndEvidenceExpiry(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	review, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase36-request-0002", "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ready, err := repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(2*time.Second))
	if err != nil || ready.EvidenceStatus != "valid" || !ready.ManualReviewReady || ready.Executable {
		t.Fatalf("plan at valid checkpoint: %+v %v", ready, err)
	}
	if _, err := repo.recoveryReviewPlan(ctx, "tenant-B", review.Request.ID, now.Add(2*time.Second)); err == nil {
		t.Fatal("cross-tenant plan exposed")
	}
	stale, err := repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(6*time.Minute))
	if err != nil || stale.EvidenceStatus != "stale" || stale.ManualReviewReady || stale.Executable {
		t.Fatalf("stale evidence treated as valid: %+v %v", stale, err)
	}
	expired, err := repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(16*time.Minute))
	if err != nil || expired.EvidenceStatus != "expired" || expired.ManualReviewReady {
		t.Fatalf("expired review eligible: %+v %v", expired, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, "device-v2:2591c674ea8032f0f8ce8298598954bd208929f7a61bcdfcef96940b8deb603a", "revoke", now.Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	revoked, err := repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(17*time.Minute))
	if err != nil || revoked.EvidenceStatus != "revoked" || revoked.ManualReviewReady {
		t.Fatalf("revoked plan: %+v %v", revoked, err)
	}
	var count int64
	if err := repo.db.Table("knowledge_vector_recovery_review_events").Where("tenant_id = ? AND review_id = ?", "tenant-A", review.Request.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("read-only plan wrote events: %d %v", count, err)
	}
}
