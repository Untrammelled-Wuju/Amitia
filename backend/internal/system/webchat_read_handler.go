package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type webChatReadService interface {
	MarkConversationReadForSpace(conversationID, spaceID string) error
}

func (h *Handler) WebChatMarkConversationRead(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Param("id"))
	if conversationID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	scoped, ok := h.chatSvc.(webChatReadService)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "chat service does not provide read state operations", nil)
		return
	}
	if err := scoped.MarkConversationReadForSpace(conversationID, webChatSpaceID(c)); err != nil {
		util.ErrorResponse(c, response.OperationFailed, "更新已读状态失败", nil)
		return
	}
	util.SuccessResponse(c, gin.H{"conversationId": conversationID})
}
