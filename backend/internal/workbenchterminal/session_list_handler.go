package workbenchterminal

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) listSessions(c *gin.Context) {
	owner, ok := terminalOwner(c, c.Query("conversationId"))
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权列出终端会话"})
		return
	}
	list := h.manager.List(owner)
	result := make([]SessionSummary, 0, len(list))
	for _, summary := range list {
		if err := h.validateTask(c.Request.Context(), owner, summary.ID, c.Query("conversationId")); err == nil {
			result = append(result, summary)
		}
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
