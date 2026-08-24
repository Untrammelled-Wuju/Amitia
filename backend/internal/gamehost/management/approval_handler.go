package management

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	ghpermission "github.com/u-ai/backend/internal/gamehost/permission"
	middlewaresecurity "github.com/u-ai/backend/internal/middleware/security"
)

type ApprovalHandler struct {
	coordinator *ghpermission.ApprovalCoordinator
}

func NewApprovalHandler(coordinator *ghpermission.ApprovalCoordinator) *ApprovalHandler {
	return &ApprovalHandler{coordinator: coordinator}
}

func (h *ApprovalHandler) ListPending(c *gin.Context) {
	if h == nil || h.coordinator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gamehost approval coordinator unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": h.coordinator.ListPending()})
}

type approvalDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	h.resolve(c, true)
}

func (h *ApprovalHandler) Reject(c *gin.Context) {
	h.resolve(c, false)
}

func (h *ApprovalHandler) resolve(c *gin.Context, approve bool) {
	if h == nil || h.coordinator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "gamehost approval coordinator unavailable"})
		return
	}
	var req approvalDecisionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	id := strings.TrimSpace(c.Param("approvalId"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "approvalId is required"})
		return
	}
	actor := "game_center_user"
	if current := middlewaresecurity.GetActor(c); current != nil {
		if current.UserID != "" {
			actor = "user:" + current.UserID.String()
		} else {
			actor = string(current.ActorType)
		}
	}
	var err error
	if approve {
		err = h.coordinator.Approve(id, actor, req.Reason)
	} else {
		err = h.coordinator.Reject(id, actor, req.Reason)
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
