package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/credential"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/middleware/security"
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
type TaskHeartbeatPayloadAdapter func(heartbeat protocol.TaskHeartbeatPayload) bool
type DeviceRevokedHandler func(context.Context, runtimeidentity.SpaceID, runtimeidentity.DeviceID) error

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
	Coordination                 *coordination.Service
	BusinessCoordinationReady    bool
	LANEndpoints                 func() []lan.Endpoint
	ProviderPath                 func() ([]string, error)
	Successor                    func(context.Context, string) (any, error)
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
	DeviceRevokedHandler         DeviceRevokedHandler
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
	authorized.GET("/provider/successor", func(c *gin.Context) {
		device, ok := deps.GetDeviceID(c)
		if !ok || device == "" {
			c.AbortWithStatus(401)
			return
		}
		if deps.Successor == nil {
			c.Status(http.StatusNoContent)
			return
		}
		result, err := deps.Successor(c.Request.Context(), device.String())
		if err != nil {
			c.JSON(503, gin.H{"message": err.Error()})
			return
		}
		if result == nil {
			c.Status(http.StatusNoContent)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, result)
	})
	authorized.POST("/pairing/successor-offers", func(c *gin.Context) {
		creator, ok := deps.GetDeviceID(c)
		if !ok || creator == "" {
			c.AbortWithStatus(401)
			return
		}
		var request struct {
			DeviceID    string `json:"deviceId"`
			Coordinated bool   `json:"coordinated"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
		if c.ShouldBindJSON(&request) != nil {
			c.JSON(400, gin.H{"message": "服务切换配对参数无效"})
			return
		}
		offer, token, err := deps.PairingSvc.CreateSuccessorOffer(c.Request.Context(), creator, runtimeidentity.ParseDeviceID(request.DeviceID), request.Coordinated)
		if err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"offerToken": token, "expiresAt": offer.ExpiresAt.UTC().Format(time.RFC3339Nano), "approvalRequired": true, "deviceId": request.DeviceID})
	})
	authorized.GET("/pairing/approvals", makeApprovalListHandler(deps))
	authorized.PUT("/pairing/approvals/:requestId", makeApprovalDecisionHandler(deps))
	authorized.GET("/devices", makeListDevicesHandler(deps))
	authorized.DELETE("/devices/:deviceId", makeRevokeDeviceHandler(deps))
	authorized.POST("/devices/:deviceId/runtimes/:runtimeId/probe", makeProbeHandler(deps))
	if deps.Coordination != nil {
		authorized.GET("/coordination/me", makePolicyHandler(deps))
		authorized.PUT("/coordination/me", makeModeHandler(deps))
		authorized.PUT("/devices/:deviceId/administrator", makeAdministratorHandler(deps))
	}

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
	OfferToken string       `json:"offerToken"`
	SetupCode  string       `json:"setupCode"`
	DeviceID   string       `json:"deviceId" binding:"required"`
	RuntimeID  string       `json:"runtimeId" binding:"required"`
	Platform   string       `json:"platform" binding:"required"`
	Label      string       `json:"label"`
	Proof      *proof.Proof `json:"proof,omitempty"`
}

func requirePairingAdministrator(c *gin.Context) bool {
	actor := security.GetActor(c)
	if actor == nil || !actor.HasPermission(auth.PermSystemAdmin) {
		c.AbortWithStatusJSON(403, gin.H{"message": "配对请求必须由服务提供设备或其管理员批准"})
		return false
	}
	return true
}

func makeApprovalListHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requirePairingAdministrator(c) {
			return
		}
		requests, err := deps.PairingSvc.PendingApprovals(c.Request.Context())
		if err != nil {
			c.JSON(503, gin.H{"message": "无法读取配对请求"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"requests": requests})
	}
}

func makeApprovalDecisionHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requirePairingAdministrator(c) {
			return
		}
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()
		var request struct {
			Allow            bool  `json:"allow"`
			ExpectedRevision int64 `json:"expectedRevision" binding:"required"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"message": "配对审批参数无效"})
			return
		}
		if err := deps.PairingSvc.DecideApproval(c.Request.Context(), c.Param("requestId"), request.ExpectedRevision, request.Allow); err != nil {
			c.JSON(409, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"saved": true})
	}
}

func makePairingStatusHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		trusted, first, err := deps.PairingSvc.Status(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"code": "mesh.pairing_status_failed", "message": err.Error()})
			return
		}
		path := []string{deps.PairingSvc.SpaceID().String()}
		if deps.ProviderPath != nil {
			path, err = deps.ProviderPath()
			if err != nil {
				c.JSON(503, gin.H{"message": "服务提供者拓扑暂不可用"})
				return
			}
		}
		c.JSON(200, gin.H{"spaceId": deps.PairingSvc.SpaceID().String(), "trustedDeviceCount": trusted, "firstDeviceSetupRequired": first, "providerPath": path})
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
			TTLSeconds int    `json:"ttlSeconds"`
			Endpoint   string `json:"endpoint"`
		}
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			c.JSON(400, gin.H{"code": "mesh.pairing_invalid", "message": "配对参数无效"})
			return
		}
		endpoint := strings.TrimSpace(req.Endpoint)
		if endpoint == "" {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			endpoint = scheme + "://" + c.Request.Host
		}
		parsedEndpoint, endpointErr := url.Parse(endpoint)
		if endpointErr != nil || parsedEndpoint.Host == "" || parsedEndpoint.User != nil || (parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") || parsedEndpoint.RawQuery != "" || parsedEndpoint.Fragment != "" || (parsedEndpoint.Path != "" && parsedEndpoint.Path != "/") {
			c.JSON(400, gin.H{"code": "mesh.pairing_invalid", "message": "服务提供者地址无效"})
			return
		}
		endpoint = parsedEndpoint.Scheme + "://" + parsedEndpoint.Host
		fingerprint := ""
		if deps.LANEndpoints != nil {
			for _, candidate := range deps.LANEndpoints() {
				if candidate.URL == endpoint && candidate.CoreID == deps.PairingSvc.SpaceID().String() {
					fingerprint = candidate.Fingerprint
					break
				}
			}
		}
		ttl := pairing.DefaultOfferTTL
		if req.TTLSeconds > 0 {
			ttl = time.Duration(req.TTLSeconds) * time.Second
		}
		offer, raw, err := deps.PairingSvc.CreateOffer(c.Request.Context(), deviceID, ttl, true)
		if err != nil {
			c.JSON(400, gin.H{"code": "mesh.pairing_offer_failed", "message": err.Error()})
			return
		}
		payload := "amitia://pair?endpoint=" + url.QueryEscape(endpoint) + "&offer=" + url.QueryEscape(raw)
		if fingerprint != "" {
			payload += "&fingerprint=" + url.QueryEscape(fingerprint) + "&core=" + url.QueryEscape(deps.PairingSvc.SpaceID().String())
		}
		png, err := qrcode.Encode(payload, qrcode.Medium, 320)
		if err != nil {
			c.JSON(500, gin.H{"code": "mesh.qr_failed", "message": "二维码生成失败"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{
			"offerId": offer.OfferID, "offerToken": raw,
			"qrPayload":        payload,
			"qrImage":          "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			"expiresAt":        offer.ExpiresAt.UTC().Format(time.RFC3339Nano),
			"approvalRequired": true,
		})
	}
}

func makePairingClaimHandler(deps *RouterDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req pairingClaimRequest
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"code": "mesh.pairing_invalid", "message": err.Error()})
			return
		}
		if c.Request.TLS != nil && req.Proof == nil {
			c.JSON(403, gin.H{"code": "pairing.identity_proof_required", "message": "局域网配对必须验证设备身份"})
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
			Platform: platform, Label: strings.TrimSpace(req.Label), Proof: req.Proof,
		})
		if err != nil {
			if status, code := pairingErrorResponse(err); code != "" {
				c.JSON(status, gin.H{"code": code, "message": err.Error()})
				return
			}
			c.JSON(400, gin.H{"code": "mesh.pairing_claim_failed", "message": err.Error()})
			return
		}
		if result.Pending != nil {
			c.JSON(202, gin.H{"pending": true, "requestId": result.Pending.RequestID, "message": "等待服务提供设备批准配对"})
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
	case errors.Is(err, pairing.ErrApprovalDenied):
		return http.StatusForbidden, "pairing.approval_denied"
	case errors.Is(err, pairing.ErrApprovalConflict):
		return http.StatusConflict, "pairing.approval_conflict"
	case errors.Is(err, proof.ErrProof):
		return http.StatusForbidden, "pairing.identity_proof_invalid"
	case errors.Is(err, proof.ErrIdentityCopy):
		return http.StatusForbidden, "pairing.identity_key_conflict"
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
			if deps.Coordination != nil {
				policy, policyErr := deps.Coordination.Get(c.Request.Context(), spaceID.String(), d.DeviceID.String())
				if policyErr != nil {
					c.JSON(503, gin.H{"code": "mesh.policy_unavailable", "message": policyErr.Error()})
					return
				}
				dev["coordination"] = policy
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
		if !canManageDevice(c, deviceID.String()) {
			return
		}
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()

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

		revoke := func(tx *sql.Tx) error {
			if err := deps.DeviceReg.RevokeDeviceTx(c.Request.Context(), tx, deviceID); err != nil {
				return err
			}
			if err := deps.CredentialSvc.RevokeAllForDeviceTx(c.Request.Context(), tx, spaceID, deviceID); err != nil {
				return err
			}
			return deps.PairingSvc.RevokeDevicePairingTx(c.Request.Context(), tx, deviceID)
		}
		var err error
		if deps.Coordination != nil {
			err = deps.Coordination.RevokeDevice(c.Request.Context(), spaceID.String(), deviceID.String(), revoke)
		} else {
			var tx *sql.Tx
			tx, err = deps.DB.BeginTx(c.Request.Context(), nil)
			if err == nil {
				defer tx.Rollback()
				err = revoke(tx)
				if err == nil {
					err = tx.Commit()
				}
			}
		}
		if err != nil {
			if errors.Is(err, coordination.ErrScopeExpired) {
				c.JSON(http.StatusConflict, gin.H{"code": "mesh.management_scope_changed", "message": err.Error()})
				return
			}
			c.JSON(500, gin.H{"code": "mesh.revoke_failed", "message": err.Error()})
			return
		}
		if deps.Hub != nil {
			deps.Hub.CloseDevice(spaceID, deviceID)
		}
		if deps.DeviceRevokedHandler != nil {
			cleanupContext, cancelCleanup := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 15*time.Second)
			defer cancelCleanup()
			if cleanupErr := deps.DeviceRevokedHandler(cleanupContext, spaceID, deviceID); cleanupErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":     "mesh.device_revoked_notification_cleanup_failed",
					"message":  cleanupErr.Error(),
					"deviceId": deviceID.String(),
					"revoked":  true,
				})
				return
			}
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
		finish, valid := security.BeginDeviceManagementIntent(c, deps.Coordination)
		if !valid {
			return
		}
		defer finish()

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
		if err := coordination.ValidateCurrent(c.Request.Context()); err != nil {
			c.JSON(409, gin.H{"code": "mesh.management_scope_changed", "message": err.Error()})
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
