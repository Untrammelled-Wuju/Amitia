package system

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) WebChatResolveTurnApproval(c *gin.Context) {
	if h.approvalBroker == nil {
		util.ErrorResponse(c, http.StatusNotFound, "审批服务不可用", nil)
		return
	}
	conversationID := strings.TrimSpace(c.Param("id"))
	turnID := strings.TrimSpace(c.Param("turnId"))
	approvalID := strings.TrimSpace(c.Param("approvalId"))
	if conversationID == "" || turnID == "" || approvalID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话、Turn 或审批 ID", nil)
		return
	}
	spaceID := webChatSpaceID(c)
	if _, err := h.requireWebChatConversation(conversationID, spaceID); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	var body struct {
		Approved bool `json:"approved"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "审批决定无效", nil)
		return
	}
	items := h.approvalBroker.List(spaceID, conversationID)
	found := false
	for _, item := range items {
		if item.ID != approvalID {
			continue
		}
		if item.TurnID != "" && item.TurnID != turnID {
			util.ErrorResponse(c, response.InvalidParams, "审批请求不属于当前 Turn", nil)
			return
		}
		found = true
		break
	}
	if !found {
		util.ErrorResponse(c, http.StatusNotFound, "审批请求不存在", nil)
		return
	}
	if err := h.approvalBroker.Resolve(approvalID, body.Approved); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"id": approvalID, "turnId": turnID, "approved": body.Approved})
}
