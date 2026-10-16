package workbenchterminal

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (h *Handler) export(c *gin.Context) {
	conversationID := c.Query("conversationId")
	owner, ok := terminalOwner(c, conversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权导出终端输出"})
		return
	}
	if err := h.validateTask(c.Request.Context(), owner, c.Param("id"), conversationID); err != nil {
		h.respondError(c, err)
		return
	}
	result, err := h.manager.Export(owner, c.Param("id"))
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
