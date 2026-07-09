package system

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	applog "github.com/u-ai/backend/log"
)

func (h *Handler) MessagesEventsStream(c *gin.Context) {
	channel := c.Query("channel")
	if channel == "" {
		channel = "web"
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(400, gin.H{"code": 400, "msg": "SSE not supported"})
		return
	}

	bus := GetMessageEventBus()
	subID := fmt.Sprintf("sse-%d", c.Request.Context().Value(nil))
	if sID, exists := c.Get("X-Session-ID"); exists {
		subID = fmt.Sprintf("sse-%v", sID)
	} else {
		subID = fmt.Sprintf("sse-%p", c.Request)
	}

	sub := bus.Subscribe(subID, []string{channel})
	defer bus.Unsubscribe(subID)

	applog.Info(fmt.Sprintf("[SSE] messages events stream connected: %s channel=%s", subID, channel))

	c.SSEvent("connected", gin.H{"channel": channel})
	flusher.Flush()

	for {
		select {
		case <-c.Done():
			applog.Info(fmt.Sprintf("[SSE] messages events stream disconnected: %s", subID))
			return
		case event, ok := <-sub.Events:
			if !ok {
				return
			}
			eventData := gin.H{
				"conversationId": event.ConversationID,
				"messageId":      event.MessageID,
				"channel":        event.Channel,
				"direction":      event.Direction,
				"role":           event.Role,
				"content":        event.Content,
				"createdAt":      event.CreatedAt,
				"status":         event.Status,
			}
			if event.Data != nil {
				eventData["data"] = event.Data
			}
			payload, err := json.Marshal(eventData)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(payload))
			flusher.Flush()
		}
	}
}
