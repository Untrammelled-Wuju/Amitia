package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/requestidentity"
)

var _ = nativebridge.HealthReady

type nativeBridgeRelay struct {
	handler *nativebridge.RelayHandler
}

func newNativeBridgeRelay() *nativeBridgeRelay {
	return &nativeBridgeRelay{
		handler: nativebridge.NewRelayHandler(),
	}
}

func (r *nativeBridgeRelay) Handler() *nativebridge.RelayHandler {
	return r.handler
}

func (r *nativeBridgeRelay) RegisterAndroidBridge(bridge *nativebridge.AndroidTransportBridge) {
	r.handler.RegisterBridge("android", bridge)
}

func registerNativeBridgeTransports(relay *nativeBridgeRelay, bootstrap *runtimeBootstrap) {
	if relay == nil || bootstrap == nil {
		return
	}
	tryRegisterAndroidBridge(relay, bootstrap)
	tryRegisterIOSBridge(relay, bootstrap)
}

func setupNativeBridgeRelayRoutes(r *nativeBridgeRelay, routerGroup *gin.RouterGroup) {
	if r == nil || r.handler == nil || routerGroup == nil {
		return
	}
	routerGroup.GET("/native-bridge/relay", r.handler.HandleWebSocket)
}

func setupNativeBridgeRelayBackendActionHandler(r *nativeBridgeRelay, services *AppServices) {
	if r == nil || r.handler == nil || services == nil {
		return
	}
	handler := newNativeShortcutBackendActionHandler(services)
	r.handler.SetBackendActionHandler(func(ctx context.Context, platform string, payload json.RawMessage) (json.RawMessage, error) {
		return handler(ctx, requestidentity.CanonicalSpaceID(), platform, payload)
	})
}

func setupNativeBridgeBackendActionRoutes(services *AppServices, routerGroup *gin.RouterGroup) {
	if services == nil || routerGroup == nil {
		return
	}
	handler := newNativeShortcutBackendActionHandler(services)
	routerGroup.POST("/native-bridge/backend-action", func(c *gin.Context) {
		var body struct {
			Platform  string         `json:"platform"`
			RequestID string         `json:"requestId"`
			ActionID  string         `json:"actionId"`
			Payload   map[string]any `json:"payload"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": gin.H{"code": "INVALID_ARGUMENT", "message": err.Error()}})
			return
		}
		if strings.TrimSpace(body.Platform) != "ios" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": gin.H{"code": "PLATFORM_NOT_SUPPORTED", "message": "backend actions are only accepted from ios"}})
			return
		}
		raw, err := json.Marshal(nativeShortcutActionRequest{
			RequestID: strings.TrimSpace(body.RequestID),
			ActionID:  strings.TrimSpace(body.ActionID),
			Payload:   body.Payload,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": gin.H{"code": "INVALID_ARGUMENT", "message": err.Error()}})
			return
		}
		// Full-runtime requests carry an authenticated ActorContext. Device Agent
		// requests are authenticated by LocalRuntimeControlMiddleware, which
		// intentionally does not synthesize a cloud/user actor. ResolveGin keeps
		// both paths on the canonical single-user identity without trusting a
		// user ID supplied in the request body.
		spaceID := requestidentity.ResolveGin(c)
		result, err := handler(c.Request.Context(), spaceID, "ios", raw)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"status": "error", "error": gin.H{"code": "BACKEND_ACTION_FAILED", "message": err.Error()}})
			return
		}
		var response any
		if err := json.Unmarshal(result, &response); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": gin.H{"code": "INVALID_BACKEND_ACTION_RESPONSE", "message": err.Error()}})
			return
		}
		c.JSON(http.StatusOK, response)
	})
}
