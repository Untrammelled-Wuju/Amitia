package proactive

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/pkg/sse"
)

func (h *Handler) RemindersStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	userID := normalizeProactiveOwner(requestidentity.ResolveGin(c, ""))
	client := sse.Global.SubscribeScoped(c.Query("clientId"), userID)
	defer sse.Global.UnsubscribeClient(client)

	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-client.Events:
			if !ok {
				return
			}
			eventName, _ := msg["event"].(string)
			data, _ := msg["data"].(map[string]interface{})
			jsonData, _ := json.Marshal(data)
			c.SSEvent(eventName, string(jsonData))
			c.Writer.Flush()
		case <-ticker.C:
			c.SSEvent("ping", "{}")
			c.Writer.Flush()
		case <-c.Done():
			return
		}
	}
}
