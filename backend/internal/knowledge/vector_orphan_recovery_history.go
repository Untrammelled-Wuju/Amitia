package knowledge

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrRecoveryReviewHistoryInvalid = errors.New("recovery review history inconsistent")

func recoveryReviewHistoryValid(request RecoveryReviewRequest, events []RecoveryReviewEvent) bool {
	if len(events) < 1 || len(events) > 3 || request.ID == "" || request.TenantID == "" || request.ReceiptHash == "" ||
		!recoveryPrincipalIsVerified(request.RequestedByHash) || request.Action != "manual_investigation" || request.CreatedAt.IsZero() ||
		request.UpdatedAt.Before(request.CreatedAt) {
		return false
	}
	ids := make(map[string]struct{}, len(events))
	for i, event := range events {
		if event.ID == "" || event.ReviewID != request.ID || event.TenantID != request.TenantID ||
			event.ReceiptHash != request.ReceiptHash || !recoveryPrincipalIsVerified(event.ActorHash) ||
			event.CreatedAt.IsZero() || event.CreatedAt.Before(request.CreatedAt) || event.CreatedAt.After(request.UpdatedAt) {
			return false
		}
		if _, ok := ids[event.ID]; ok {
			return false
		}
		ids[event.ID] = struct{}{}
		if i > 0 && event.CreatedAt.Before(events[i-1].CreatedAt) {
			return false
		}
	}
	if events[0].Action != "requested" || events[0].ActorHash != request.RequestedByHash ||
		!events[0].CreatedAt.Equal(request.CreatedAt) {
		return false
	}
	switch request.Status {
	case "pending":
		return len(events) == 1 && request.ReviewedByHash == "" && request.UpdatedAt.Equal(request.CreatedAt)
	case "approved":
		return len(events) == 2 && events[1].Action == "approved" &&
			events[1].ActorHash != request.RequestedByHash && events[1].ActorHash == request.ReviewedByHash &&
			events[1].CreatedAt.Equal(request.UpdatedAt)
	case "revoked":
		if len(events) == 2 {
			return events[1].Action == "revoked" && events[1].ActorHash == request.ReviewedByHash &&
				events[1].ActorHash != request.RequestedByHash &&
				events[1].CreatedAt.Equal(request.UpdatedAt)
		}
		return len(events) == 3 && events[1].Action == "approved" &&
			events[1].ActorHash != request.RequestedByHash && events[2].Action == "revoked" &&
			events[2].ActorHash == request.ReviewedByHash && events[2].CreatedAt.Equal(request.UpdatedAt)
	default:
		return false
	}
}

func checkRecoveryReviewHistory(ctx context.Context, tx *gorm.DB, request RecoveryReviewRequest) error {
	var events []RecoveryReviewEvent
	err := tx.WithContext(ctx).Table("knowledge_vector_recovery_review_events").
		Where("tenant_id = ? AND review_id = ?", request.TenantID, request.ID).
		Order("created_at ASC, id ASC").Limit(4).Find(&events).Error
	if err != nil {
		return err
	}
	if !recoveryReviewHistoryValid(request, events) {
		return ErrRecoveryReviewHistoryInvalid
	}
	return nil
}

func recoveryReviewHistoryStatus(ctx context.Context, tx *gorm.DB, request RecoveryReviewRequest) (string, error) {
	err := checkRecoveryReviewHistory(ctx, tx, request)
	if err == nil {
		return "verified", nil
	}
	if errors.Is(err, ErrRecoveryReviewHistoryInvalid) {
		return "inconsistent", nil
	}
	return "unavailable", err
}
