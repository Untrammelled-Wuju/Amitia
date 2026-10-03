package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type BootstrapValidatorAdapter struct {
	svc *bootstrap.Service
}

func (a *BootstrapValidatorAdapter) Validate(ctx context.Context, rawTicket string) (*credential.TicketSnapshot, error) {
	snapshot, err := a.svc.Validate(ctx, rawTicket)
	if err != nil {
		return nil, err
	}
	return &credential.TicketSnapshot{
		SpaceID:   snapshot.SpaceID,
		DeviceID:  snapshot.DeviceID,
		RuntimeID: snapshot.RuntimeID,
		ExpiresAt: snapshot.ExpiresAt,
	}, nil
}

type TaskClaimAdapter func(taskRunID string, workerID string, leaseDuration time.Duration) bool
type TaskCompleteAdapter func(taskRunID string, success bool, errMsg string)
type TaskClaimPayloadAdapter func(claim protocol.TaskClaimPayload) bool
type TaskCompletePayloadAdapter func(complete protocol.TaskCompletePayload)
type TaskProgressPayloadAdapter func(progress protocol.TaskProgressPayload)
type TaskCheckpointPayloadAdapter func(checkpoint protocol.TaskCheckpointPayload)
type TaskHeartbeatPayloadAdapter func(heartbeat protocol.TaskHeartbeatPayload)

type RouterDeps struct {
	DB                           *sql.DB
	Sessions                     *deviceruntime.Service
	BootstrapSvc                 *bootstrap.Service
	CredentialSvc                *credential.Service
	Hub                          *ConnectionHub
	Handler                      *Handler
	Probe                        *ProbeService
	DeviceReg                    *host_registry.Registry
	PairingSvc                   *pairing.Service
	GetSpaceID                   func(c *gin.Context) (runtimeidentity.SpaceID, bool)
	GetDeviceID                  func(c *gin.Context) (runtimeidentity.DeviceID, bool)
	InvocationResultHandler      InvocationResultHandler
	InvocationErrorHandler       InvocationErrorHandler
	TaskClaimHandler             TaskClaimAdapter
	TaskCompleteHandler          TaskCompleteAdapter
	TaskClaimPayloadHandler      TaskClaimPayloadAdapter
	TaskCompletePayloadHandler   TaskCompletePayloadAdapter
	TaskProgressPayloadHandler   TaskProgressPayloadAdapter
	TaskCheckpointPayloadHandler TaskCheckpointPayloadAdapter
	TaskHeartbeatPayloadHandler  TaskHeartbeatPayloadAdapter
	DisconnectHandler            DisconnectHandler
}

func RegisterCloudRoutes(router gin.IRouter, authMW gin.HandlerFunc, webAccessMW gin.HandlerFunc, publicWebAccessMW gin.HandlerFunc, deps *RouterDeps) error {
	if deps == nil {
		return fmt.Errorf("devicemesh: router dependencies are required")
	}
	if deps.Sessions == nil || deps.BootstrapSvc == nil || deps.CredentialSvc == nil || deps.Hub == nil || deps.Probe == nil || deps.DeviceReg == nil || deps.PairingSvc == nil || deps.GetSpaceID == nil || deps.GetDeviceID == nil {
		return fmt.Errorf("devicemesh: incomplete cloud router dependencies")
	}
	if deps.InvocationResultHandler == nil || deps.InvocationErrorHandler == nil ||
		deps.TaskClaimPayloadHandler == nil || deps.TaskHeartbeatPayloadHandler == nil ||
		deps.TaskProgressPayloadHandler == nil || deps.TaskCheckpointPayloadHandler == nil ||
		deps.TaskCompletePayloadHandler == nil || deps.DisconnectHandler == nil {
		return fmt.Errorf("devicemesh: incomplete invocation/task callback wiring")
	}
	handler := deps.Handler
	if handler == nil {
		handler = NewHandler(deps.Sessions, deps.Hub)
	}

	if deps.InvocationResultHandler != nil {
		handler.SetOnInvocationResult(deps.InvocationResultHandler)
	}
	if deps.InvocationErrorHandler != nil {
		handler.SetOnInvocationError(deps.InvocationErrorHandler)
	}
	if deps.TaskClaimPayloadHandler != nil {
		handler.SetOnTaskClaimPayload(TaskClaimPayloadHandler(deps.TaskClaimPayloadHandler))
	}
	if deps.TaskCompletePayloadHandler != nil {
		handler.SetOnTaskCompletePayload(TaskCompletePayloadHandler(deps.TaskCompletePayloadHandler))
	}
	if deps.TaskProgressPayloadHandler != nil {
		handler.SetOnTaskProgressPayload(TaskProgressPayloadHandler(deps.TaskProgressPayloadHandler))
	}
	if deps.TaskCheckpointPayloadHandler != nil {
		handler.SetOnTaskCheckpointPayload(TaskCheckpointPayloadHandler(deps.TaskCheckpointPayloadHandler))
	}
	if deps.TaskHeartbeatPayloadHandler != nil {
		handler.SetOnTaskHeartbeatPayload(TaskHeartbeatPayloadHandler(deps.TaskHeartbeatPayloadHandler))
	}
	if deps.TaskClaimHandler != nil {
		handler.SetOnTaskClaim(func(taskRunID string, attemptID string, workerID string, leaseDuration time.Duration) bool {
			return deps.TaskClaimHandler(taskRunID, workerID, leaseDuration)
		})
	}
	if deps.TaskCompleteHandler != nil {
		handler.SetOnTaskComplete(func(taskRunID string, attemptID string, success bool, result json.RawMessage, errMsg string) {
			deps.TaskCompleteHandler(taskRunID, success, errMsg)
		})
	}
	if deps.DisconnectHandler != nil {
		handler.SetOnDisconnect(deps.DisconnectHandler)
	}

	public := router.Group("/api/public/device-mesh/v1")
	if publicWebAccessMW != nil {
		public.Use(publicWebAccessMW)
	}
	public.GET("/pairing/status", makePairingStatusHandler(deps))
	public.POST("/pairing/claim", makePairingClaimHandler(deps))

	authorized := router.Group("/api/device-mesh/v1")
	authorized.Use(authMW)
	if webAccessMW != nil {
		authorized.Use(webAccessMW)
	}
	authorized.POST("/pairing/offers", makePairingOfferHandler(deps))
	authorized.GET("/devices", makeListDevicesHandler(deps))
	authorized.DELETE("/devices/:deviceId", makeRevokeDeviceHandler(deps))
	authorized.POST("/devices/:deviceId/runtimes/:runtimeId/probe", makeProbeHandler(deps))

	bootstrapHandlers := []gin.HandlerFunc{}
	if publicWebAccessMW != nil {
		bootstrapHandlers = append(bootstrapHandlers, publicWebAccessMW)
	}
	bootstrapHandlers = append(bootstrapHandlers, credential.BootstrapTicketAuthMiddleware(deps.CredentialSvc, &BootstrapValidatorAdapter{svc: deps.BootstrapSvc}), makeExchangeHandler(deps))
	router.POST("/api/public/device-mesh/v1/bootstrap/exchange", bootstrapHandlers...)
	router.GET("/api/device-mesh/v1/runtime/ws", credential.DeviceAuthCredentialWS(deps.CredentialSvc, deps.DeviceReg, handler))
	return nil
}

type pairingClaimRequest struct {
	OfferToken string `json:"offerToken"`
	SetupCode  string `json:"setupCode"`
	DeviceID   string `json:"deviceId" binding:"required"`
	RuntimeID  string `json:"runtimeId" binding:"required"`
	Platform   string `json:"platform" binding:"required"`
	Label      string `json:"label"`
}

func makePairingStatusHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		trusted, first, err := deps.PairingSvc.Status(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"code": "mesh.pairing_status_failed", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"spaceId": deps.PairingSvc.SpaceID().String(), "trustedDeviceCount": trusted, "firstDeviceSetupRequired": first})
	}
}

func makePairingOfferHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID, ok := deps.GetDeviceID(c)
		if !ok || deviceID == "" {
			c.JSON(401, gin.H{"code": "mesh.unauthorized", "message": "trusted device required"})
			return
		}
		var req struct {
			TTLSeconds int `json:"ttlSeconds"`
		}
		_ = c.ShouldBindJSON(&req)
		ttl := pairing.DefaultOfferTTL
		if req.TTLSeconds > 0 {
			ttl = time.Duration(req.TTLSeconds) * time.Second
		}
		offer, raw, err := deps.PairingSvc.CreateOffer(c.Request.Context(), deviceID, ttl)
		if err != nil {
			c.JSON(400, gin.H{"code": "mesh.pairing_offer_failed", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"offerId": offer.OfferID, "offerToken": raw,
			"qrPayload": "amitia://pair?offer=" + url.QueryEscape(raw),
			"expiresAt": offer.ExpiresAt.UTC().Format(time.RFC3339Nano),
		})
	}
}

func makePairingClaimHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pairingClaimRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": "mesh.pairing_invalid", "message": err.Error()})
			return
		}
		platform, err := runtimeidentity.ParsePlatform(req.Platform)
		if err != nil || platform == runtimeidentity.PlatformUnknown {
			c.JSON(400, gin.H{"code": "mesh.pairing_invalid", "message": "invalid platform"})
			return
		}
		result, err := deps.PairingSvc.Claim(c.Request.Context(), pairing.ClaimRequest{
			OfferToken: strings.TrimSpace(req.OfferToken), SetupCode: strings.TrimSpace(req.SetupCode),
			DeviceID: runtimeidentity.ParseDeviceID(req.DeviceID), RuntimeID: runtimeidentity.ParseRuntimeID(req.RuntimeID),
			Platform: platform, Label: strings.TrimSpace(req.Label),
		})
		if err != nil {
			if status, code := pairingErrorResponse(err); code != "" {
				c.JSON(status, gin.H{"code": code, "message": err.Error()})
				return
			}
			c.JSON(400, gin.H{"code": "mesh.pairing_claim_failed", "message": err.Error()})
			return
		}
		c.JSON(200, gin.H{
			"ticketId": result.Ticket.TicketID, "ticket": result.RawTicket,
			"spaceId": result.Ticket.SpaceID.String(), "deviceId": result.Ticket.DeviceID.String(),
			"runtimeId": result.Ticket.RuntimeID.String(), "expiresAt": result.Ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
			"ttlSeconds": meshprotocol.BootstrapTicketTTL,
		})
	}
}

func pairingErrorResponse(err error) (int, string) {
	switch {
	case errors.Is(err, pairing.ErrSelfPairing):
		return http.StatusConflict, "pairing.self"
	case errors.Is(err, pairing.ErrAlreadyPaired):
		return http.StatusConflict, "pairing.already_paired"
	case errors.Is(err, pairing.ErrPairingPending):
		return http.StatusConflict, "pairing.pending"
	case errors.Is(err, pairing.ErrOfferExpired):
		return http.StatusGone, "pairing.offer_expired"
	case errors.Is(err, pairing.ErrOfferConsumed):
		return http.StatusConflict, "pairing.offer_consumed"
	case errors.Is(err, pairing.ErrOfferNotFound):
		return http.StatusBadRequest, "pairing.offer_invalid"
	default:
		return 0, ""
	}
}

type exchangeRequest struct {
	DeviceID  string `json:"deviceId" binding:"required"`
	RuntimeID string `json:"runtimeId" binding:"required"`
	Platform  string `json:"platform" binding:"required"`
}

func makeExchangeHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req exchangeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": "mesh.bootstrap_invalid", "message": err.Error()})
			return
		}

		principal, ok := credential.GinPrincipal(c)
		if !ok {
			c.JSON(401, gin.H{"code": "mesh.credential_invalid", "message": "unauthorized"})
			return
		}

		_, ticketOk := credential.GinBootstrapTicket(c)
		if !ticketOk {
			c.JSON(401, gin.H{"code": "mesh.bootstrap_required", "message": "bootstrap ticket not validated"})
			return
		}

		deviceID := runtimeidentity.ParseDeviceID(req.DeviceID)
		runtimeID := runtimeidentity.ParseRuntimeID(req.RuntimeID)

		if deviceID != principal.DeviceID || runtimeID != principal.RuntimeID {
			c.JSON(400, gin.H{"code": "mesh.identity_mismatch", "message": "request identity mismatch"})
			return
		}

		header := c.GetHeader("Authorization")
		rawTicket := strings.TrimPrefix(header, "AmitiaBootstrap ")

		credID, raw, credentialExpiresAt, err := deps.BootstrapSvc.Exchange(c.Request.Context(), rawTicket, deviceID, runtimeID)
		if err != nil {
			code := "mesh.bootstrap_invalid"
			if strings.Contains(err.Error(), "already consumed") {
				code = "mesh.bootstrap_consumed"
			} else if strings.Contains(err.Error(), "expired") {
				code = "mesh.bootstrap_expired"
			} else if strings.Contains(err.Error(), "not active") {
				code = "mesh.bootstrap_invalid"
			}
			c.JSON(400, gin.H{"code": code, "message": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"credentialId":    credID,
			"credential":      raw,
			"spaceId":         principal.SpaceID.String(),
			"deviceId":        principal.DeviceID.String(),
			"runtimeId":       principal.RuntimeID.String(),
			"expiresAt":       credentialExpiresAt.UTC().Format(time.RFC3339Nano),
			"protocol":        meshprotocol.ProtocolName,
			"envelopeVersion": meshprotocol.EnvelopeVersion,
			"schemaVersion":   meshprotocol.SchemaVersion,
			"websocketPath":   meshprotocol.WebSocketPath,
		})
	}
}

func makeListDevicesHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		spaceID, ok := deps.GetSpaceID(c)
		if !ok {
			c.JSON(401, gin.H{"code": "mesh.unauthorized", "message": "unauthorized"})
			return
		}
		if deps.DeviceReg == nil {
			c.JSON(200, gin.H{"devices": []gin.H{}})
			return
		}

		devices, err := deps.DeviceReg.ListDevicesBySpace(c.Request.Context(), spaceID)
		if err != nil {
			c.JSON(200, gin.H{"devices": []gin.H{}})
			return
		}

		response := make([]gin.H, 0, len(devices))
		for _, d := range devices {
			dev := gin.H{
				"deviceId":   d.DeviceID.String(),
				"platform":   d.Platform.String(),
				"label":      d.Label,
				"trustState": string(d.TrustState),
			}

			presence, err := deps.DeviceReg.GetDevicePresence(c.Request.Context(), spaceID, d.DeviceID)
			if err == nil {
				dev["presence"] = string(presence.State)
				dev["lastHeartbeat"] = presence.LastHeartbeat.UTC().Format("2006-01-02T15:04:05.000Z")
			}

			runtimes, _ := deps.DeviceReg.ListRuntimePresenceByDevice(c.Request.Context(), spaceID, d.DeviceID)
			rtList := make([]gin.H, 0, len(runtimes))
			for _, rt := range runtimes {
				conn, connected := deps.Hub.GetByRuntime(spaceID, d.DeviceID, rt.RuntimeID)
				rtEntry := gin.H{
					"runtimeId": rt.RuntimeID.String(),
					"presence":  string(rt.State),
				}
				if connected {
					rtEntry["runtimeSessionId"] = conn.SessionID.String()
					rtEntry["connectionGeneration"] = conn.Generation
				}
				rtList = append(rtList, rtEntry)
			}
			dev["runtimes"] = rtList
			response = append(response, dev)
		}

		c.JSON(200, gin.H{"devices": response})
	}
}

func makeRevokeDeviceHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		spaceID, ok := deps.GetSpaceID(c)
		if !ok {
			c.JSON(401, gin.H{"code": "mesh.unauthorized", "message": "unauthorized"})
			return
		}

		deviceID := runtimeidentity.ParseDeviceID(c.Param("deviceId"))

		if err := deps.DeviceReg.RequireDeviceOwnedBy(c.Request.Context(), spaceID, deviceID); err != nil {
			if err == host_registry.ErrDeviceNotFound {
				c.JSON(404, gin.H{"code": "mesh.device_not_found", "message": "device not found"})
				return
			}
			if err == host_registry.ErrDeviceOwnedByOther {
				c.JSON(403, gin.H{"code": "mesh.device_owned_by_other", "message": "forbidden"})
				return
			}
			c.JSON(500, gin.H{"code": "mesh.error", "message": err.Error()})
			return
		}

		tx, err := deps.DB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}
		defer tx.Rollback()
		if err := deps.DeviceReg.RevokeDeviceTx(c.Request.Context(), tx, deviceID); err != nil {
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}
		if err := deps.CredentialSvc.RevokeAllForDeviceTx(c.Request.Context(), tx, spaceID, deviceID); err != nil {
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}
		if err := deps.PairingSvc.RevokeDevicePairingTx(c.Request.Context(), tx, deviceID); err != nil {
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}
		if err := tx.Commit(); err != nil {
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}

		c.JSON(200, gin.H{"ok": true, "deviceId": deviceID.String()})
	}
}

func makeProbeHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		spaceID, ok := deps.GetSpaceID(c)
		if !ok {
			c.JSON(401, gin.H{"code": "mesh.unauthorized", "message": "unauthorized"})
			return
		}

		deviceID := runtimeidentity.ParseDeviceID(c.Param("deviceId"))
		runtimeID := runtimeidentity.ParseRuntimeID(c.Param("runtimeId"))

		if err := deps.DeviceReg.RequireDeviceOwnedBy(c.Request.Context(), spaceID, deviceID); err != nil {
			c.JSON(403, gin.H{"code": "mesh.device_owned_by_other", "message": err.Error()})
			return
		}

		latency, err := deps.Probe.ProbeRuntime(c.Request.Context(), spaceID, deviceID, runtimeID)
		if err != nil {
			c.JSON(503, gin.H{"code": "mesh.connection_unavailable", "message": err.Error()})
			return
		}

		conn, _ := deps.Hub.GetByRuntime(spaceID, deviceID, runtimeID)
		result := gin.H{
			"reachable": true,
			"latencyMs": latency.Milliseconds(),
		}
		if conn != nil {
			result["runtimeSessionId"] = conn.SessionID.String()
			result["connectionGeneration"] = conn.Generation
		}

		c.JSON(200, result)
	}
}
