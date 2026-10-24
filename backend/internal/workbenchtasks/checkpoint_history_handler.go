package workbenchtasks

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) checkpoints(c *gin.Context) {
	coordinationID := c.Query("coordinationId")
	if !validConversation.MatchString(coordinationID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的任务或对话标识"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	var out TaskCheckpointHistory
	h.readConversation(c, func(ctx context.Context, query QueryContext) error {
		var err error
		out, err = ListCheckpoints(ctx, scopedRecordReader{reader: h.records, role: query.ExecutionScope.RoleID, space: query.ExecutionScope.SpaceID, canRead: query.CanReadRecord}, query.ExecutionScope.SpaceID, query.ConversationID, coordinationID)
		out.QueryContext = &query
		return err
	})
	if c.Writer.Written() || out.QueryContext == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	if len(out.Records) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "当前对话下未找到任务检查点"})
		return
	}
	c.JSON(http.StatusOK, out)
}
