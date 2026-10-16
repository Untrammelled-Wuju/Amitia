package workbenchterminal

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (h *Handler) lifecycle(c *gin.Context) {
	conversationID := c.Query("conversationId")
	owner, ok := terminalOwner(c, conversationID)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权读取终端事件"})
		return
	}
	after, err := strconv.ParseUint(strings.TrimSpace(c.DefaultQuery("after", "0")), 10, 64)
	if err != nil {
		h.respondError(c, ErrInvalid)
		return
	}
	if err = h.validateTask(c.Request.Context(), owner, c.Param("id"), conversationID); err != nil {
		h.respondError(c, err)
		return
	}
	result, err := h.manager.Lifecycle(owner, c.Param("id"), after)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, result)
}
