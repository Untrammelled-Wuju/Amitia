package knowledge

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const recoveryReviewEvidenceLifetime = 5 * time.Minute
const recoveryReviewApprovalLifetime = 15 * time.Minute

var recoveryReviewKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,96}$`)
var ErrRecoveryReviewConflict = errors.New("recovery review conflict")
var ErrRecoveryReviewEvidenceInvalid = errors.New("recovery review evidence unavailable")

type RecoveryReviewRequest struct {
	ID              string    `json:"id" gorm:"column:id;primaryKey"`
	TenantID        string    `json:"-" gorm:"column:tenant_id"`
	AuditID         string    `json:"auditId" gorm:"column:audit_id"`
	RecheckID       string    `json:"recheckId" gorm:"column:recheck_id"`
	ReceiptHash     string    `json:"receiptHash" gorm:"column:receipt_hash"`
	RequestKey      string    `json:"-" gorm:"column:request_key"`
	RequestedByHash string    `json:"-" gorm:"column:requested_by_hash"`
	ReviewedByHash  string    `json:"-" gorm:"column:reviewed_by_hash"`
	Status          string    `json:"status" gorm:"column:status"`
	HistoryStatus   string    `json:"historyStatus" gorm:"-"`
	DisplayStatus   string    `json:"displayStatus" gorm:"-"`
	Action          string    `json:"action" gorm:"column:action"`
	CreatedAt       time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt       time.Time `json:"updatedAt" gorm:"column:updated_at"`
	ExpiresAt       time.Time `json:"expiresAt" gorm:"column:expires_at"`
}

type RecoveryReviewEvent struct {
	ID          string    `json:"id" gorm:"column:id;primaryKey"`
	TenantID    string    `json:"-" gorm:"column:tenant_id"`
	ReviewID    string    `json:"reviewId" gorm:"column:review_id"`
	Action      string    `json:"action" gorm:"column:action"`
	ActorHash   string    `json:"-" gorm:"column:actor_hash"`
	ReceiptHash string    `json:"receiptHash" gorm:"column:receipt_hash"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
}

func writeRecoveryReviewEvent(tx *gorm.DB, tenantID, reviewID, action, actorHash, receiptHash string, now time.Time) error {
	event := RecoveryReviewEvent{ID: uuid.NewString(), TenantID: tenantID, ReviewID: reviewID,
		Action: action, ActorHash: actorHash, ReceiptHash: receiptHash, CreatedAt: now}
	return tx.Table("knowledge_vector_recovery_review_events").Create(&event).Error
}

type RecoveryReviewPlan struct {
	ReviewID          string    `json:"reviewId"`
	AuditID           string    `json:"auditId"`
	Status            string    `json:"status"`
	EvidenceStatus    string    `json:"evidenceStatus"`
	ReceiptHash       string    `json:"receiptHash"`
	Action            string    `json:"action"`
	ManualReviewReady bool      `json:"manualReviewReady"`
	CanApprove        bool      `json:"canApprove"`
	HistoryStatus     string    `json:"historyStatus"`
	Executable        bool      `json:"executable"`
	CheckedAt         time.Time `json:"checkedAt"`
}

type RecoveryReviewResponse struct {
	Request    RecoveryReviewRequest `json:"request"`
	Idempotent bool                  `json:"idempotent"`
	Executable bool                  `json:"executable"`
}

type RecoveryReviewEventHistory struct {
	ReviewID      string                `json:"reviewId"`
	HistoryStatus string                `json:"historyStatus"`
	Items         []RecoveryReviewEvent `json:"items"`
	Truncated     bool                  `json:"truncated"`
	Executable    bool                  `json:"executable"`
}

func recoveryReviewDisplayStatus(review RecoveryReviewRequest, historyStatus string, now time.Time) string {
	if historyStatus != "verified" {
		return "history_inconsistent"
	}
	if review.Status == "revoked" {
		return "revoked"
	}
	if !recoveryPrincipalIsVerified(review.RequestedByHash) ||
		(review.Status == "approved" && (!recoveryPrincipalIsVerified(review.ReviewedByHash) || review.RequestedByHash == review.ReviewedByHash)) {
		return "identity_unverified"
	}
	if !now.Before(review.ExpiresAt) {
		return "expired"
	}
	if review.Status == "pending" {
		return "needs_recheck"
	}
	if review.Status == "approved" {
		return "approved_unverified"
	}
	return "history_inconsistent"
}

func recoveryReviewApprovalEligible(review RecoveryReviewRequest, actorHash, historyStatus, evidenceStatus string, now time.Time) bool {
	return review.Status == "pending" && historyStatus == "verified" && evidenceStatus == "valid" &&
		recoveryPrincipalIsVerified(review.RequestedByHash) && recoveryPrincipalIsVerified(actorHash) &&
		actorHash != review.RequestedByHash && now.Before(review.ExpiresAt)
}

func validateRecoveryReviewEvidence(ctx context.Context, tx *gorm.DB, tenantID, auditID, recheckID string, now time.Time) (string, error) {
	var session orphanAuditSessionRow
	if err := tx.WithContext(ctx).Table("knowledge_vector_audit_sessions").Where("tenant_id = ? AND id = ?", tenantID, auditID).Take(&session).Error; err != nil {
		return "", err
	}
	if session.Page < 1 || now.After(session.ExpiresAt) {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	_, anchored, err := auditReceiptAnchoredPrefix(ctx, tx, tenantID, auditID, session.Page)
	if err != nil {
		return "", err
	}
	if !anchored {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	receipt, err := getAuditReceipt(ctx, tx, tenantID, auditID, session.Page)
	if err != nil {
		return "", err
	}
	if !receipt.valid() || receipt.ResultJSON != session.LastResultJSON || receipt.RequestHash != hashAuditBytes([]byte(session.LastCursor)) {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	var observation OrphanVectorAuditPageResult
	if observation, err = auditReceiptResult(receipt); err != nil {
		return "", err
	}
	if observation.SuspectedOrphans == 0 || observation.InFlight > 0 {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	var checked OrphanAuditRecheckRecord
	if err := tx.WithContext(ctx).Table("knowledge_vector_audit_recheck_results").Where("tenant_id = ? AND audit_id = ? AND id = ?", tenantID, auditID, recheckID).Take(&checked).Error; err != nil {
		return "", err
	}
	var latest OrphanAuditRecheckRecord
	if err := tx.WithContext(ctx).Table("knowledge_vector_audit_recheck_results").Where("tenant_id = ? AND audit_id = ?", tenantID, auditID).Order("observed_at DESC, created_at DESC, id DESC").Take(&latest).Error; err != nil {
		return "", err
	}
	if latest.ID != recheckID {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	if checked.Page != session.Page || checked.Status != "matched" || checked.ReceiptHash != receipt.ReceiptHash || checked.ResultHash == "" || checked.ObservedAt.After(now.Add(30*time.Second)) || now.Sub(checked.ObservedAt) > recoveryReviewEvidenceLifetime {
		return "", ErrRecoveryReviewEvidenceInvalid
	}
	return receipt.ReceiptHash, nil
}

func (r *Repository) createRecoveryReview(ctx context.Context, tenantID, auditID, recheckID, key, actorHash string, now time.Time) (RecoveryReviewResponse, error) {
	var out RecoveryReviewResponse
	if strings.TrimSpace(tenantID) == "" || !recoveryPrincipalIsVerified(actorHash) || !recoveryReviewKeyPattern.MatchString(key) {
		return out, fmt.Errorf("invalid recovery review request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return out, fmt.Errorf("invalid audit identifier")
	}
	if _, err := uuid.Parse(recheckID); err != nil {
		return out, fmt.Errorf("invalid recheck identifier")
	}
	now = now.UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing RecoveryReviewRequest
		err := tx.Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND request_key = ?", tenantID, key).Take(&existing).Error
		if err == nil {
			if existing.AuditID != auditID || existing.RecheckID != recheckID || existing.RequestedByHash != actorHash {
				return ErrRecoveryReviewConflict
			}
			if err := checkRecoveryReviewHistory(ctx, tx, existing); err != nil {
				return err
			}
			out = RecoveryReviewResponse{Request: existing, Idempotent: true, Executable: false}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		receiptHash, err := validateRecoveryReviewEvidence(ctx, tx, tenantID, auditID, recheckID, now)
		if err != nil {
			return err
		}
		request := RecoveryReviewRequest{
			ID: uuid.NewString(), TenantID: tenantID, AuditID: auditID, RecheckID: recheckID,
			ReceiptHash: receiptHash, RequestKey: key, RequestedByHash: actorHash,
			Status: "pending", Action: "manual_investigation", CreatedAt: now,
			UpdatedAt: now, ExpiresAt: now.Add(recoveryReviewApprovalLifetime),
		}
		if err := tx.Table("knowledge_vector_recovery_reviews").Create(&request).Error; err != nil {
			return ErrRecoveryReviewConflict
		}
		if err := writeRecoveryReviewEvent(tx, tenantID, request.ID, "requested", actorHash, receiptHash, now); err != nil {
			return err
		}
		out = RecoveryReviewResponse{Request: request, Executable: false}
		return nil
	})
	if errors.Is(err, ErrRecoveryReviewConflict) {
		var existing RecoveryReviewRequest
		if readErr := r.db.WithContext(ctx).Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND request_key = ?", tenantID, key).Take(&existing).Error; readErr == nil &&
			existing.AuditID == auditID && existing.RecheckID == recheckID && existing.RequestedByHash == actorHash {
			if historyErr := checkRecoveryReviewHistory(ctx, r.db, existing); historyErr != nil {
				return out, historyErr
			}
			return RecoveryReviewResponse{Request: existing, Idempotent: true, Executable: false}, nil
		}
	}
	return out, err
}

func (r *Repository) transitionRecoveryReview(ctx context.Context, tenantID, id, actorHash, transition string, now time.Time) (RecoveryReviewResponse, error) {
	var out RecoveryReviewResponse
	if strings.TrimSpace(tenantID) == "" || !recoveryPrincipalIsVerified(actorHash) || (transition != "approve" && transition != "revoke") {
		return out, fmt.Errorf("invalid review transition")
	}
	if _, err := uuid.Parse(id); err != nil {
		return out, fmt.Errorf("invalid review identifier")
	}
	now = now.UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request RecoveryReviewRequest
		if err := tx.Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND id = ?", tenantID, id).Take(&request).Error; err != nil {
			return err
		}
		if transition == "approve" {
			if !recoveryPrincipalIsVerified(request.RequestedByHash) || request.RequestedByHash == actorHash || !now.Before(request.ExpiresAt) {
				return ErrRecoveryReviewConflict
			}
			if err := checkRecoveryReviewHistory(ctx, tx, request); err != nil {
				return err
			}
		}
		target := "approved"
		if transition == "revoke" {
			target = "revoked"
		}
		if request.Status == target {
			if !recoveryPrincipalIsVerified(request.ReviewedByHash) || request.ReviewedByHash != actorHash {
				return ErrRecoveryReviewConflict
			}
			if transition == "approve" {
				if !recoveryPrincipalIsVerified(request.RequestedByHash) || request.RequestedByHash == actorHash || !now.Before(request.ExpiresAt) {
					return ErrRecoveryReviewConflict
				}
				receiptHash, err := validateRecoveryReviewEvidence(ctx, tx, tenantID, request.AuditID, request.RecheckID, now)
				if err != nil {
					return err
				}
				if receiptHash != request.ReceiptHash {
					return ErrRecoveryReviewEvidenceInvalid
				}
			}
			out = RecoveryReviewResponse{Request: request, Idempotent: true, Executable: false}
			return nil
		}
		if request.Status == "revoked" || (transition == "approve" && (!recoveryPrincipalIsVerified(request.RequestedByHash) || request.Status != "pending" || !now.Before(request.ExpiresAt) || request.RequestedByHash == actorHash)) {
			return ErrRecoveryReviewConflict
		}
		if transition == "approve" {
			receiptHash, err := validateRecoveryReviewEvidence(ctx, tx, tenantID, request.AuditID, request.RecheckID, now)
			if err != nil {
				return err
			}
			if receiptHash != request.ReceiptHash {
				return ErrRecoveryReviewEvidenceInvalid
			}
		}
		allowed := []string{"pending", "approved"}
		if transition == "approve" {
			allowed = []string{"pending"}
		}
		updateQuery := tx.Table("knowledge_vector_recovery_reviews").Where("id = ? AND tenant_id = ? AND status IN ?", id, tenantID, allowed)
		if transition == "approve" {
			updateQuery = updateQuery.Where("requested_by_hash <> ? AND requested_by_hash LIKE ? AND expires_at > ?", actorHash, recoveryPrincipalV2Prefix+"%", now)
		}
		update := updateQuery.Updates(map[string]any{"status": target, "reviewed_by_hash": actorHash, "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrRecoveryReviewConflict
		}
		if err := writeRecoveryReviewEvent(tx, tenantID, request.ID, target, actorHash, request.ReceiptHash, now); err != nil {
			return err
		}
		request.Status = target
		request.ReviewedByHash = actorHash
		request.UpdatedAt = now
		out = RecoveryReviewResponse{Request: request, Executable: false}
		return nil
	})
	return out, err
}

func (s *service) CreateRecoveryReview(ctx context.Context, tenantID, auditID, recheckID, key, actorHash string) (RecoveryReviewResponse, error) {
	return s.repo.createRecoveryReview(ctx, tenantID, auditID, recheckID, key, actorHash, time.Now().UTC())
}

func (s *service) TransitionRecoveryReview(ctx context.Context, tenantID, id, actorHash, transition string) (RecoveryReviewResponse, error) {
	return s.repo.transitionRecoveryReview(ctx, tenantID, id, actorHash, transition, time.Now().UTC())
}

func (s *service) ListRecoveryReviews(ctx context.Context, tenantID, auditID string, limit int) ([]RecoveryReviewRequest, error) {
	items := []RecoveryReviewRequest{}
	if strings.TrimSpace(tenantID) == "" || limit < 1 || limit > 30 {
		return items, fmt.Errorf("invalid review history request")
	}
	if _, err := uuid.Parse(auditID); err != nil {
		return items, fmt.Errorf("invalid audit identifier")
	}
	now := time.Now().UTC()
	err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session orphanAuditSessionRow
		if err := tx.Table("knowledge_vector_audit_sessions").Where("tenant_id = ? AND id = ?", tenantID, auditID).Take(&session).Error; err != nil {
			return err
		}
		if err := tx.Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND audit_id = ?", tenantID, auditID).
			Order("created_at DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
			return err
		}
		for i := range items {
			status, err := recoveryReviewHistoryStatus(ctx, tx, items[i])
			if err != nil {
				return err
			}
			items[i].HistoryStatus = status
			items[i].DisplayStatus = recoveryReviewDisplayStatus(items[i], status, now)
		}
		return nil
	})
	return items, err
}

func (s *service) ListRecoveryReviewEvents(ctx context.Context, tenantID, id string) (RecoveryReviewEventHistory, error) {
	result := RecoveryReviewEventHistory{ReviewID: id, HistoryStatus: "unavailable", Items: []RecoveryReviewEvent{}, Executable: false}
	if strings.TrimSpace(tenantID) == "" {
		return result, fmt.Errorf("tenant required")
	}
	if _, err := uuid.Parse(id); err != nil {
		return result, fmt.Errorf("invalid review identifier")
	}
	err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var review RecoveryReviewRequest
		if err := tx.Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND id = ?", tenantID, id).Take(&review).Error; err != nil {
			return err
		}
		var events []RecoveryReviewEvent
		if err := tx.Table("knowledge_vector_recovery_review_events").
			Where("tenant_id = ? AND review_id = ?", tenantID, id).
			Order("created_at ASC, id ASC").Limit(26).Find(&events).Error; err != nil {
			return err
		}
		result.Truncated = len(events) > 25
		if result.Truncated {
			events = events[:25]
		}
		result.Items = events
		result.HistoryStatus = "inconsistent"
		if !result.Truncated && recoveryReviewHistoryValid(review, events) {
			result.HistoryStatus = "verified"
		}
		return nil
	})
	return result, err
}

func (r *Repository) recoveryReviewPlan(ctx context.Context, tenantID, id string, now time.Time) (RecoveryReviewPlan, error) {
	return r.recoveryReviewPlanForActor(ctx, tenantID, id, "", now)
}

func (r *Repository) recoveryReviewPlanForActor(ctx context.Context, tenantID, id, actorHash string, now time.Time) (RecoveryReviewPlan, error) {
	plan := RecoveryReviewPlan{ReviewID: id, Action: "manual_investigation", EvidenceStatus: "unavailable", HistoryStatus: "unavailable", Executable: false, CheckedAt: now.UTC()}
	if ctx == nil || strings.TrimSpace(tenantID) == "" {
		return plan, fmt.Errorf("invalid recovery review plan request")
	}
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	if _, err := uuid.Parse(id); err != nil {
		return plan, fmt.Errorf("invalid review identifier")
	}
	now = now.UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request RecoveryReviewRequest
		if err := tx.Table("knowledge_vector_recovery_reviews").Where("tenant_id = ? AND id = ?", tenantID, id).Take(&request).Error; err != nil {
			return err
		}
		plan.AuditID = request.AuditID
		plan.ReceiptHash = request.ReceiptHash
		plan.Status = request.Status
		historyStatus, err := recoveryReviewHistoryStatus(ctx, tx, request)
		plan.HistoryStatus = historyStatus
		if err != nil {
			return err
		}
		if request.Status == "revoked" {
			plan.EvidenceStatus = "revoked"
			return nil
		}
		if !recoveryPrincipalIsVerified(request.RequestedByHash) || (request.Status == "approved" && (!recoveryPrincipalIsVerified(request.ReviewedByHash) || request.RequestedByHash == request.ReviewedByHash)) {
			plan.EvidenceStatus = "identity_unverified"
			return nil
		}
		if historyStatus != "verified" {
			plan.EvidenceStatus = "history_inconsistent"
			return nil
		}
		if !now.Before(request.ExpiresAt) {
			plan.EvidenceStatus = "expired"
			return nil
		}
		receiptHash, err := validateRecoveryReviewEvidence(ctx, tx, tenantID, request.AuditID, request.RecheckID, now)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, ErrRecoveryReviewEvidenceInvalid) || errors.Is(err, gorm.ErrRecordNotFound) {
				plan.EvidenceStatus = "stale"
				return nil
			}
			return err
		}
		if receiptHash != request.ReceiptHash {
			plan.EvidenceStatus = "stale"
			return nil
		}
		plan.EvidenceStatus = "valid"
		plan.ManualReviewReady = request.Status == "pending" || request.Status == "approved"
		plan.CanApprove = recoveryReviewApprovalEligible(request, actorHash, plan.HistoryStatus, plan.EvidenceStatus, now)
		return nil
	})
	return plan, err
}

func (s *service) GetRecoveryReviewPlan(ctx context.Context, tenantID, id, actorHash string) (RecoveryReviewPlan, error) {
	return s.repo.recoveryReviewPlanForActor(ctx, tenantID, id, actorHash, time.Now().UTC())
}
