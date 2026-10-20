package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhase39HistoryRejectsTamperButAllowsSafetyRevocation(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-2"))
	ctx := context.Background()
	created, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase39-review-key-00001", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := repo.recoveryReviewPlan(ctx, "tenant-A", created.Request.ID, now.Add(2*time.Second))
	if err != nil || plan.HistoryStatus != "verified" || !plan.ManualReviewReady {
		t.Fatalf("initial plan: %+v %v", plan, err)
	}
	if err := repo.db.Exec("UPDATE knowledge_vector_recovery_review_events SET receipt_hash = ? WHERE review_id = ?", "forged", created.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	plan, err = repo.recoveryReviewPlan(ctx, "tenant-A", created.Request.ID, now.Add(2*time.Second))
	if err != nil || plan.HistoryStatus != "inconsistent" || plan.EvidenceStatus != "history_inconsistent" || plan.ManualReviewReady || plan.Executable {
		t.Fatalf("forged history allowed: %+v %v", plan, err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewHistoryInvalid) {
		t.Fatalf("forged history approved: %v", err)
	}
	if _, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase39-review-key-00001", requester, now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewHistoryInvalid) {
		t.Fatalf("forged history replay: %v", err)
	}
	revoked, err := repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, requester, "revoke", now.Add(4*time.Second))
	if err != nil || revoked.Request.Status != "revoked" || revoked.Executable {
		t.Fatalf("safety revoke: %+v %v", revoked, err)
	}
	plan, err = repo.recoveryReviewPlan(ctx, "tenant-A", created.Request.ID, now.Add(5*time.Second))
	if err != nil || plan.EvidenceStatus != "revoked" || plan.HistoryStatus != "inconsistent" || plan.ManualReviewReady {
		t.Fatalf("corrupt revoked plan: %+v %v", plan, err)
	}
}

func TestPhase39EventHistoryStateTransitionsAndTamper(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-1"))
	ctx := context.Background()
	review, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase39-review-key-00002", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, reviewer, "approve", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	plan, err := repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(3*time.Second))
	if err != nil || plan.HistoryStatus != "verified" || !plan.ManualReviewReady {
		t.Fatalf("approved history: %+v %v", plan, err)
	}
	if err := repo.db.Exec("DELETE FROM knowledge_vector_recovery_review_events WHERE review_id = ? AND action = 'approved'", review.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, reviewer, "approve", now.Add(4*time.Second)); !errors.Is(err, ErrRecoveryReviewHistoryInvalid) {
		t.Fatalf("missing approval event replayed: %v", err)
	}
	plan, err = repo.recoveryReviewPlan(ctx, "tenant-A", review.Request.ID, now.Add(4*time.Second))
	if err != nil || plan.HistoryStatus != "inconsistent" || plan.ManualReviewReady {
		t.Fatalf("approved without evidence: %+v %v", plan, err)
	}
	if _, err := repo.recoveryReviewPlan(ctx, "tenant-B", review.Request.ID, now.Add(4*time.Second)); err == nil {
		t.Fatal("cross-tenant audit exposed")
	}
	revoked, err := repo.transitionRecoveryReview(ctx, "tenant-A", review.Request.ID, requester, "revoke", now.Add(5*time.Second))
	if err != nil || revoked.Request.Status != "revoked" {
		t.Fatalf("safety revoke denied: %+v %v", revoked, err)
	}
}

func TestPhase39HistoryRejectsDuplicateAndFutureEvents(t *testing.T) {
	now := time.Date(2026, 10, 16, 20, 0, 0, 0, time.UTC)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	review := RecoveryReviewRequest{ID: "review", TenantID: "tenant-A", ReceiptHash: "hash", RequestedByHash: requester, Status: "pending", Action: "manual_investigation", CreatedAt: now, UpdatedAt: now}
	initial := RecoveryReviewEvent{ID: "event", TenantID: "tenant-A", ReviewID: "review", Action: "requested", ActorHash: requester, ReceiptHash: "hash", CreatedAt: now}
	if !recoveryReviewHistoryValid(review, []RecoveryReviewEvent{initial}) {
		t.Fatal("valid pending rejected")
	}
	for _, events := range [][]RecoveryReviewEvent{
		nil,
		{initial, initial},
		{{ID: "event", TenantID: "tenant-B", ReviewID: "review", Action: "requested", ActorHash: requester, ReceiptHash: "hash", CreatedAt: now}},
		{{ID: "event", TenantID: "tenant-A", ReviewID: "review", Action: "requested", ActorHash: requester, ReceiptHash: "hash", CreatedAt: now.Add(time.Second)}},
	} {
		if recoveryReviewHistoryValid(review, events) {
			t.Fatalf("invalid trail accepted: %+v", events)
		}
	}
}
