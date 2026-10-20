package knowledge

import (
	"context"
	"testing"
	"time"
)

func TestPhase40ReadModelsRejectCorruptedEventsAndExpiredRequests(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-2"))
	svc := &service{repo: repo}
	ctx := context.Background()
	created, err := repo.createRecoveryReview(ctx, "tenant-A", auditID, recheckID, "phase40-review-key-00001", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	reviews, err := svc.ListRecoveryReviews(ctx, "tenant-A", auditID, 10)
	if err != nil || len(reviews) != 1 || reviews[0].DisplayStatus != "needs_recheck" || reviews[0].HistoryStatus != "verified" {
		t.Fatalf("pending state: %+v %v", reviews, err)
	}
	history, err := svc.ListRecoveryReviewEvents(ctx, "tenant-A", created.Request.ID)
	if err != nil || history.HistoryStatus != "verified" || len(history.Items) != 1 || history.Truncated || history.Executable {
		t.Fatalf("initial history: %+v %v", history, err)
	}
	if _, err = repo.transitionRecoveryReview(ctx, "tenant-A", created.Request.ID, reviewer, "approve", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	reviews, err = svc.ListRecoveryReviews(ctx, "tenant-A", auditID, 10)
	if err != nil || len(reviews) != 1 || reviews[0].DisplayStatus != "approved_unverified" {
		t.Fatalf("approved state: %+v %v", reviews, err)
	}
	if err = repo.db.Exec("UPDATE knowledge_vector_recovery_review_events SET receipt_hash = 'forged' WHERE tenant_id = ? AND review_id = ? AND action = 'approved'", "tenant-A", created.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	reviews, err = svc.ListRecoveryReviews(ctx, "tenant-A", auditID, 10)
	if err != nil || len(reviews) != 1 || reviews[0].HistoryStatus != "inconsistent" || reviews[0].DisplayStatus != "history_inconsistent" {
		t.Fatalf("forged approval visible as valid: %+v %v", reviews, err)
	}
	history, err = svc.ListRecoveryReviewEvents(ctx, "tenant-A", created.Request.ID)
	if err != nil || history.HistoryStatus != "inconsistent" || len(history.Items) != 2 {
		t.Fatalf("forged history exposed as verified: %+v %v", history, err)
	}
	if _, err = svc.ListRecoveryReviews(ctx, "tenant-B", auditID, 10); err == nil {
		t.Fatal("foreign tenant listed audit")
	}
	if _, err = svc.ListRecoveryReviewEvents(ctx, "tenant-B", created.Request.ID); err == nil {
		t.Fatal("foreign tenant listed review events")
	}
	if err = repo.db.Exec("DELETE FROM knowledge_vector_recovery_review_events WHERE tenant_id = ? AND review_id = ?", "tenant-A", created.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	history, err = svc.ListRecoveryReviewEvents(ctx, "tenant-A", created.Request.ID)
	if err != nil || history.HistoryStatus != "inconsistent" || len(history.Items) != 0 {
		t.Fatalf("empty damaged history treated as verified: %+v %v", history, err)
	}
}

func TestPhase40DisplayStatusFailClosed(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	principal, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	other, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-2"))
	base := RecoveryReviewRequest{Status: "pending", RequestedByHash: principal, ExpiresAt: now.Add(time.Minute)}
	for _, tt := range []struct {
		name, history string
		review        RecoveryReviewRequest
		want          string
	}{
		{"pending_requires_preflight", "verified", base, "needs_recheck"},
		{"unanchored_history", "inconsistent", base, "history_inconsistent"},
		{"history_unavailable", "unavailable", base, "history_inconsistent"},
		{"expired", "verified", RecoveryReviewRequest{Status: "pending", RequestedByHash: principal, ExpiresAt: now}, "expired"},
		{"unverified", "verified", RecoveryReviewRequest{Status: "pending", RequestedByHash: "invalid", ExpiresAt: now.Add(time.Minute)}, "identity_unverified"},
		{"valid_approval_still_rechecks", "verified", RecoveryReviewRequest{Status: "approved", RequestedByHash: principal, ReviewedByHash: other, ExpiresAt: now.Add(time.Minute)}, "approved_unverified"},
		{"same_reviewer", "verified", RecoveryReviewRequest{Status: "approved", RequestedByHash: principal, ReviewedByHash: principal, ExpiresAt: now.Add(time.Minute)}, "identity_unverified"},
		{"revoked", "verified", RecoveryReviewRequest{Status: "revoked", RequestedByHash: principal, ExpiresAt: now.Add(-time.Minute)}, "revoked"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := recoveryReviewDisplayStatus(tt.review, tt.history, now); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPhase40ReadModelRevocationStillWorksWithMissingApprovalHistory(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	repo, auditID, recheckID := phase35ReviewFixture(t, now)
	requester, _ := recoveryDevicePrincipalHash(phase37Actor("device-A", "session-1"))
	reviewer, _ := recoveryDevicePrincipalHash(phase37Actor("device-B", "session-2"))
	created, err := repo.createRecoveryReview(context.Background(), "tenant-A", auditID, recheckID, "phase40-review-key-00002", requester, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.transitionRecoveryReview(context.Background(), "tenant-A", created.Request.ID, reviewer, "approve", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err = repo.db.Exec("DELETE FROM knowledge_vector_recovery_review_events WHERE review_id = ? AND action = 'approved'", created.Request.ID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = repo.transitionRecoveryReview(context.Background(), "tenant-A", created.Request.ID, requester, "revoke", now.Add(3*time.Second)); err != nil {
		t.Fatalf("safety revoke blocked: %v", err)
	}
	svc := &service{repo: repo}
	reviews, err := svc.ListRecoveryReviews(context.Background(), "tenant-A", auditID, 10)
	if err != nil || len(reviews) != 1 || reviews[0].HistoryStatus != "inconsistent" || reviews[0].DisplayStatus != "history_inconsistent" {
		t.Fatalf("invalid revoked events hidden: %+v %v", reviews, err)
	}
}
