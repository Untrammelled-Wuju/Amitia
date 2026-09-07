package delivery

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/middleware/security"
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
		util.ErrorResponse(c, response.InvalidParams, err.Error(), nil)
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
	r.POST("/delivery/submit", security.SharedCoreAdminOnly(), handler.Submit)
}

// RegisterBridgeSubmitRouter exposes the sidecar-only delivery ingress outside
// the user-authenticated /api group. It is intentionally restricted to
// loopback callers presenting BRIDGE_API_TOKEN so managed channel sidecars do
// not need to impersonate a user JWT.
func RegisterBridgeSubmitRouter(r *gin.Engine, store *SQLiteDeliveryStore) {
	if r == nil || store == nil {
		return
	}
	handler := NewSubmitHandler(store)
	r.POST("/internal/delivery/submit", bridgeServiceOnly(), handler.Submit)
}

func bridgeServiceOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(strings.TrimSpace(c.Request.RemoteAddr))
		if err != nil {
			host = strings.Trim(strings.TrimSpace(c.Request.RemoteAddr), "[]")
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "msg": "delivery bridge requires loopback access"})
			return
		}

		expected := strings.TrimSpace(os.Getenv("BRIDGE_API_TOKEN"))
		if expected == "" || expected == "change-me-bridge-token" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": http.StatusServiceUnavailable, "msg": "delivery bridge token is not configured"})
			return
		}

		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.Fields(authz)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "msg": "invalid delivery bridge credential"})
			return
		}
		c.Next()
	}
}
