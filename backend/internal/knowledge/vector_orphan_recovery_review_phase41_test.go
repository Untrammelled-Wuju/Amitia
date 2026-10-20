package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhase41PlanScopedToCurrentApprovingDevice(t *testing.T) {
	now := time.Date(2026, 10, 16, 21, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	ctx := context.Background()
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-2"))
	id, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase41-plan-request-0001", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		label, principal string
		eligible         bool
	}{
		{"self_approval_denied", requester, false},
		{"independent_device_eligible", reviewer, true},
		{"no_actor_is_not_an_approval", "", false},
		{"unverified_actor", "legacy-actor", false},
	} {
		t.Run(tc.label, func(t *testing.T) {
			plan, err := repo.recoveryReviewPlanForActor(ctx, "tenant-A", id.Request.ID, tc.principal, now.Add(2*time.Second))
			if err != nil {
				t.Fatal(err)
			}
			if plan.CanApprove != tc.eligible || !plan.ManualReviewReady || plan.EvidenceStatus != "valid" || plan.HistoryStatus != "verified" || plan.Executable {
				t.Fatalf("plan unexpectedly eligible or unable to review: %+v", plan)
			}
		})
	}
	plan, err := repo.recoveryReviewPlanForActor(ctx, "tenant-A", id.Request.ID, reviewer, now.Add(6*time.Minute))
	if err != nil || plan.CanApprove || plan.ManualReviewReady || (plan.EvidenceStatus != "stale" && plan.EvidenceStatus != "expired") {
		t.Fatalf("stale plan still actionable: %+v %v", plan, err)
	}
	if _, err := repo.recoveryReviewPlanForActor(ctx, "tenant-B", id.Request.ID, reviewer, now.Add(2*time.Second)); err == nil {
		t.Fatal("cross-tenant plan readable")
	}
	if _, err := repo.transitionRecoveryReview(ctx, "tenant-A", id.Request.ID, requester, "approve", now.Add(3*time.Second)); !errors.Is(err, ErrRecoveryReviewConflict) {
		t.Fatalf("self approval accepted despite preflight: %v", err)
	}
	approved, err := repo.transitionRecoveryReview(ctx, "tenant-A", id.Request.ID, reviewer, "approve", now.Add(3*time.Second))
	if err != nil || approved.Request.Status != "approved" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	plan, err = repo.recoveryReviewPlanForActor(ctx, "tenant-A", id.Request.ID, reviewer, now.Add(4*time.Second))
	if err != nil || plan.CanApprove || plan.Status != "approved" || plan.Executable {
		t.Fatalf("approval cannot be approved again as a new operation: %+v %v", plan, err)
	}
	_, err = repo.transitionRecoveryReview(ctx, "tenant-A", id.Request.ID, requester, "revoke", now.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	plan, err = repo.recoveryReviewPlanForActor(ctx, "tenant-A", id.Request.ID, reviewer, now.Add(6*time.Second))
	if err != nil || plan.CanApprove || plan.EvidenceStatus != "revoked" || plan.ManualReviewReady {
		t.Fatalf("revoked request still actionable: %+v %v", plan, err)
	}
}

func TestPhase41PlanRejectsDamagedHistory(t *testing.T) {
	now := time.Date(2026, 10, 16, 21, 0, 0, 0, time.UTC)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-1"))
	created, err := repo.createRecoveryReview(context.Background(), "tenant-A", auditID, recheckID, "phase41-plan-request-0002", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Exec("UPDATE knowledge_vector_recovery_review_events SET receipt_hash = 'tampered' WHERE tenant_id = ? AND review_id = ?", "tenant-A", created.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	plan, err := repo.recoveryReviewPlanForActor(context.Background(), "tenant-A", created.Request.ID, reviewer, now.Add(2*time.Second))
	if err != nil || plan.CanApprove || plan.ManualReviewReady || plan.EvidenceStatus != "history_inconsistent" || plan.HistoryStatus != "inconsistent" {
		t.Fatalf("invalid history deemed actionable: %+v %v", plan, err)
	}
}
