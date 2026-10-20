package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhase38ReplayIsBoundToOriginalActorAndFreshEvidence(t *testing.T) {
	now := time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-1"))
	outsider, _ := recoveryDevicePrincipalHash(phase37Actor("device-C", "session-1"))
	ctx := context.Background()
	created, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase38-replay-00001", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	approved, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(2*time.Second))
	if err != nil || approved.Request.Status != "approved" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	replay, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(3*time.Second))
	if err != nil || !replay.Idempotent {
		t.Fatalf("valid replay: %+v %v", replay, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, outsider, "approve", now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("other admin replay: %v", err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(6*time.Minute)); !errors.Is(err, ErrRecoveryReviewEvidenceInvalid) && !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("stale evidence replay: %v", err)
	}
	revoked, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, requester, "revoke", now.Add(6*time.Minute))
	if err != nil || revoked.Request.Status != "revoked" {
		t.Fatalf("revoked: %+v %v", revoked, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "revoke", now.Add(6*time.Minute+time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("foreign revoke replay: %v", err)
	}
	repeat, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, requester, "revoke", now.Add(6*time.Minute+time.Second))
	if err != nil || !repeat.Idempotent {
		t.Fatalf("original revoker replay: %+v %v", repeat, err)
	}
	var events int64
	if err := repo.db.Table("knowledge_vector_recovery_review_events").Where("tenant_id = ? AND review_id = ?", "tenant-A", created.Request.ID).Count(&events).Error; err != nil || events != 3 {
		t.Fatalf("replay changed audit events: %d %v", events, err)
	}
}

func TestPhase38PlanRejectsForgedSelfApprovalAndStaleEvidence(t *testing.T) {
	now := time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-1"))
	ctx := context.Background()
	created, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase38-plan-000001", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Table("knowledge_vector_recovery_reviews").Where("id = ?", created.Request.ID).Update("reviewed_by_hash", requester).Error; err != nil {
		t.Fatal(err)
	}
	plan, err := repo.recoveryReviewPlan(ctx, "tenant-A", created.Request.ID, now.Add(3*time.Second))
	if err != nil || plan.EvidenceStatus != "identity_unverified" || plan.ManualReviewReady || plan.Executable {
		t.Fatalf("forged self approval: %+v %v", plan, err)
	}
	if err := repo.db.Table("knowledge_vector_recovery_reviews").Where("id = ?", created.Request.ID).Update("reviewed_by_hash", reviewer).Error; err != nil {
		t.Fatal(err)
	}
	plan, err = repo.recoveryReviewPlan(ctx, "tenant-A", created.Request.ID, now.Add(6*time.Minute))
	if err != nil || plan.EvidenceStatus != "stale" || plan.ManualReviewReady || plan.Executable {
		t.Fatalf("stale approved plan: %+v %v", plan, err)
	}
	if _, err := repo.recoveryReviewPlan(ctx, "tenant-B", created.Request.ID, now.Add(3*time.Second)); err == nil {
		t.Fatal("cross-tenant plan allowed")
	}
}
