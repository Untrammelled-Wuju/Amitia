package workbenchtasks

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
)

type BudgetEvidenceReviewer interface {
	AttachOwnedBudgetEvidence(context.Context, mindruntime.OwnedBudgetEvidenceReview) error
}

type budgetEvidenceReviewRequest struct {
	Kind        string `json:"kind" binding:"required"`
	RecordID    string `json:"recordId" binding:"required"`
	EvidenceRef string `json:"evidenceRef" binding:"required"`
}

func (h *Handler) budgetReconciliationReview(c *gin.Context) {
	actor, exists := auth.FromContext(c.Request.Context())
	if !exists || actor == nil || (actor.PrincipalType != auth.PrincipalLocalUI && actor.PrincipalType != auth.PrincipalTrustedDevice) {
		c.JSON(http.StatusForbidden, gin.H{"error": "人工预算证据登记需要当前资源所有者身份", "errorCode": "access_denied"})
		return
	}
	reviewer, ok := h.budget.(BudgetEvidenceReviewer)
	getter, hasGetter := h.records.(budgetParentReader)
	if !ok || !hasGetter {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "预算证据登记服务不可用", "errorCode": "budget_unavailable"})
		return
	}
	parentID, coordinationID := c.Query("parentInteractionId"), c.Query("coordinationId")
	if !validResourceID(parentID) || !validConversation.MatchString(coordinationID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的预算核对范围", "errorCode": "invalid_context"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	var payload budgetEvidenceReviewRequest
	if err := c.ShouldBindJSON(&payload); err != nil || (payload.Kind != "model" && payload.Kind != "tool") || !(validResourceID(payload.RecordID) && len(payload.RecordID) <= 255) || !validResourceID(payload.EvidenceRef) || len(payload.EvidenceRef) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "errorCode": "invalid_evidence"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	completed := false
	var contextForResponse QueryContext
	h.readConversation(c, func(ctx context.Context, query QueryContext) error {
		authorized, scope, err := authorizeBudgetScope(ctx, getter, query, parentID, coordinationID)
		if err != nil {
			return err
		}
		err = reviewer.AttachOwnedBudgetEvidence(authorized, mindruntime.OwnedBudgetEvidenceReview{
			Scope: scope, Kind: payload.Kind, RecordID: payload.RecordID, EvidenceRef: payload.EvidenceRef,
		})
		if errors.Is(err, mindruntime.ErrOwnedBudgetReviewForbidden) {
			return coordination.ErrCapabilityGrant
		}
		if errors.Is(err, mindruntime.ErrOwnedBudgetReviewConflict) {
			return coordination.ErrRequestConflict
		}
		if err != nil {
			return err
		}
		if err := coordination.ValidateCurrent(authorized); err != nil {
			return err
		}
		completed = true
		contextForResponse = query
		return nil
	})
	if c.Writer.Written() || !completed {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"recorded": true, "effect": "evidence-only", "budgetReleased": false, "executionConfirmed": false, "queryContext": contextForResponse, "conversationId": contextForResponse.ConversationID, "coordinationId": coordinationID, "parentInteractionId": parentID, "kind": payload.Kind, "recordId": payload.RecordID, "evidenceRef": payload.EvidenceRef})
}
