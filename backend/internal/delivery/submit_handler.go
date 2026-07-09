package delivery

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

type SubmitIntentRequest struct {
	Channel       string `json:"channel"`
	PeerID        string `json:"peerId"`
	Text          string `json:"text"`
	InteractionID string `json:"interactionId"`
	MessageID     string `json:"messageId"`
}

type SubmitHandler struct {
	store *SQLiteDeliveryStore
}

func NewSubmitHandler(store *SQLiteDeliveryStore) *SubmitHandler {
	return &SubmitHandler{store: store}
}

func (h *SubmitHandler) Submit(c *gin.Context) {
	var req SubmitIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if req.Channel == "" || req.PeerID == "" || req.Text == "" {
		util.ErrorResponse(c, response.InvalidParams, "channel, peerId, text 不能为空", nil)
		return
	}
	interactionID := req.InteractionID
	if interactionID == "" {
		interactionID = "sidecar-" + req.MessageID
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"messageId": req.MessageID,
		"content":   req.Text,
	})
	intent := NewDeliveryIntent(interactionID, req.Channel, req.PeerID, "text", payload)
	inserted, err := h.store.SubmitIntent(intent)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "提交失败: "+err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{
		"inserted":   inserted,
		"id":         intent.ID,
		"status":     string(intent.Status),
		"maxRetries": intent.MaxRetries,
		"createdAt":  intent.CreatedAt.Format(time.RFC3339),
	})
}

func RegisterSubmitRouter(r *gin.RouterGroup, store *SQLiteDeliveryStore) {
	handler := NewSubmitHandler(store)
	r.POST("/delivery/submit", handler.Submit)
}
