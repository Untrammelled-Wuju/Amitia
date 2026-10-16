package workbenchterminal

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) taskReceipts(c *gin.Context) {
	conversationID := strings.TrimSpace(c.Query("conversationId"))
	owner, ok := terminalOwner(c, conversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权读取任务终端凭据"})
		return
	}
	taskID := strings.TrimSpace(c.Query("taskId"))
	if !taskIDPattern.MatchString(taskID) {
		h.respondError(c, ErrInvalid)
		return
	}
	if h.taskVerifier == nil || h.manager == nil {
		h.respondError(c, ErrReceiptUnavailable)
		return
	}
	if h.manager.receiptStatus() != "enabled" {
		h.respondError(c, ErrReceiptUnavailable)
		return
	}
	receipts, err := h.manager.TaskReceipts(owner, conversationID, taskID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	result := make([]TaskReceipt, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt.ConversationID != conversationID || receipt.TaskID != taskID {
			continue
		}
		err = h.taskVerifier(c.Request.Context(), receipt.WorkspaceURI, taskID, conversationID)
		if errors.Is(err, ErrTaskBinding) {
			continue
		}
		if err != nil {
			h.respondError(c, err)
			return
		}
		result = append(result, receipt)
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"taskId": taskID, "conversationId": conversationID, "receipts": result, "summary": SummarizeTaskReceipts(result), "verification": "terminal-metadata-only-not-test-evidence"})
}
