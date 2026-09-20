package system

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func (h *Handler) WebChatListApprovals(c *gin.Context) {
	if h.approvalBroker == nil {
		util.SuccessResponse(c, []any{})
		return
	}
	spaceID := webChatSpaceID(c)
	conversationID := strings.TrimSpace(c.Query("conversationId"))
	if conversationID != "" {
		if _, err := h.requireWebChatConversation(conversationID, spaceID); err != nil {
			util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
			return
		}
	}
	util.SuccessResponse(c, h.approvalBroker.List(spaceID, conversationID))
}

func (h *Handler) WebChatResolveApproval(c *gin.Context) {
	if h.approvalBroker == nil {
		util.ErrorResponse(c, http.StatusNotFound, "审批服务不可用", nil)
		return
	}
	var body struct {
		Approved bool `json:"approved"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "审批决定无效", nil)
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	spaceID := webChatSpaceID(c)
	items := h.approvalBroker.List(spaceID, "")
	conversationID := ""
	for _, item := range items {
		if item.ID == id {
			conversationID = item.ConversationID
			break
		}
	}
	if conversationID == "" {
		util.ErrorResponse(c, http.StatusNotFound, "审批请求不存在", nil)
		return
	}
	if _, err := h.requireWebChatConversation(conversationID, spaceID); err != nil {
		util.ErrorResponse(c, response.NotFound, "会话不存在", nil)
		return
	}
	if err := h.approvalBroker.Resolve(id, body.Approved); err != nil {
		util.ErrorResponse(c, response.OperationFailed, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"id": id, "approved": body.Approved})
}
