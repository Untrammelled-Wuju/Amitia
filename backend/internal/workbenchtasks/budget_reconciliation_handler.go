package workbenchtasks

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
)

type BudgetReconciliationReader interface {
	PendingOwnedModelReconciliationPage(context.Context, mindruntime.OwnedBudgetReconciliationScope, int64, int) ([]mindruntime.ModelUsageRecord, error)
	ReservedOwnedToolReconciliationPage(context.Context, mindruntime.OwnedBudgetReconciliationScope, time.Time, string, int) ([]mindruntime.OwnedToolCallRecord, error)
}

type budgetParentReader interface {
	Get(context.Context, string) (*interaction.InteractionRecord, bool, error)
}

type budgetEvidenceOwnerReader interface {
	CanReviewOwnedBudgetEvidence(context.Context, mindruntime.OwnedBudgetReconciliationScope) bool
}

func (h *Handler) budgetReconciliation(c *gin.Context) {
	if h.budget == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "预算账本暂不可用", "errorCode": "budget_unavailable"})
		return
	}
	kind := c.Query("kind")
	parentID := c.Query("parentInteractionId")
	coordinationID := c.Query("coordinationId")
	if (kind != "model" && kind != "tool") || !validResourceID(parentID) || !validConversation.MatchString(coordinationID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的预算核对范围", "errorCode": "invalid_context"})
		return
	}
	limit := 50
	if c.Query("limit") != "" {
		parsed, err := strconv.Atoi(c.Query("limit"))
		if err != nil || parsed < 1 || parsed > 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "errorCode": "invalid_page"})
			return
		}
		limit = parsed
	}
	var sequence int64
	var createdAt time.Time
	var effectID string
	if kind == "model" {
		if c.Query("afterCreatedAt") != "" || c.Query("afterEffectId") != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "模型核对游标无效", "errorCode": "invalid_page"})
			return
		}
		if raw := c.Query("afterSequence"); raw != "" {
			var err error
			sequence, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || sequence < 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "errorCode": "invalid_page"})
				return
			}
		}
	} else {
		if c.Query("afterSequence") != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "工具核对游标无效", "errorCode": "invalid_page"})
			return
		}
		effectID = c.Query("afterEffectId")
		if raw := c.Query("afterCreatedAt"); raw != "" {
			var err error
			createdAt, err = time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "errorCode": "invalid_page"})
				return
			}
		}
		if createdAt.IsZero() != (effectID == "") || len(effectID) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "工具核对游标无效", "errorCode": "invalid_page"})
			return
		}
	}
	getter, hasGetter := h.records.(budgetParentReader)
	if !hasGetter {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "父任务核对服务不可用", "errorCode": "context_unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	var data any
	completed := false
	h.readConversation(c, func(ctx context.Context, query QueryContext) error {
		authorized, scope, err := authorizeBudgetScope(ctx, getter, query, parentID, coordinationID)
		if err != nil {
			return err
		}
		canReview := false
		if owner, ok := h.budget.(budgetEvidenceOwnerReader); ok {
			canReview = owner.CanReviewOwnedBudgetEvidence(authorized, scope)
		}

		if kind == "model" {
			rows, err := h.budget.PendingOwnedModelReconciliationPage(authorized, scope, sequence, limit)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []mindruntime.ModelUsageRecord{}
			}
			data = gin.H{"protocolVersion": 1, "source": "durable-budget-ledger", "kind": kind, "coordinationId": coordinationID, "parentInteractionId": parentID, "conversationId": query.ConversationID, "queryContext": query, "records": rows, "nextPagePossible": len(rows) == limit, "canReview": canReview}
			if len(rows) == limit {
				data.(gin.H)["nextAfterSequence"] = rows[len(rows)-1].Sequence
			}
		} else {
			rows, err := h.budget.ReservedOwnedToolReconciliationPage(authorized, scope, createdAt, effectID, limit)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []mindruntime.OwnedToolCallRecord{}
			}
			data = gin.H{"protocolVersion": 1, "source": "durable-budget-ledger", "kind": kind, "coordinationId": coordinationID, "parentInteractionId": parentID, "conversationId": query.ConversationID, "queryContext": query, "records": rows, "nextPagePossible": len(rows) == limit, "canReview": canReview}
			if len(rows) == limit {
				data.(gin.H)["nextAfterCreatedAt"] = rows[len(rows)-1].CreatedAt.UTC().Format(time.RFC3339Nano)
				data.(gin.H)["nextAfterEffectId"] = rows[len(rows)-1].EffectID
			}
		}
		if err := coordination.ValidateCurrent(authorized); err != nil {
			return err
		}
		completed = true
		return nil
	})
	if c.Writer.Written() || !completed {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, data)
}
