package main

import (
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/device"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/desktoppet/release"
	desktoppetruntime "github.com/u-ai/backend/internal/desktoppet/runtime"
	runtimev1 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/pkg/app"
)

var localMeshHandler *agent.LocalHandler

func setupDeviceAgentRouter(ctx *app.AppContext, services *AppServices) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.TraceMiddleware())
	r.Use(timeoutpolicy.Headers())
	r.Use(security.CorsMiddleware(security.CorsConfig{
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))

	r.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "alive"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if services == nil || services.RuntimeOrchestrator == nil {
			c.JSON(503, gin.H{"code": 503, "msg": "not_ready", "data": gin.H{"status": "not_ready", "reason": "orchestrator not initialized"}})
			return
		}
		if localMeshHandler == nil {
			c.JSON(503, gin.H{"code": 503, "msg": "not_ready", "data": gin.H{"status": "not_ready", "reason": "local mesh handler not initialized"}})
			return
		}
		if !localMeshHandler.TaskWorkerSet() {
			c.JSON(503, gin.H{"code": 503, "msg": "not_ready", "data": gin.H{"status": "not_ready", "reason": "task worker not set"}})
			return
		}
		if config.AppCfg.DesktopPetRuntime.Enabled {
			if err := deviceAgentDesktopPetReady(services); err != nil {
				c.JSON(503, gin.H{"code": 503, "msg": "not_ready", "data": gin.H{"status": "not_ready", "reason": err.Error()}})
				return
			}
		}
		c.JSON(200, gin.H{
			"code": 200,
			"msg":  "ready",
			"data": gin.H{"status": "ready"},
		})
	})

	tokenPath := configuredLocalCredentialPath(config.AppCfg.Storage.DataDir)
	localStore, err := security.NewLocalCredentialStore(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("initialize device-agent local credentials: %w", err)
	}

	localAuth := security.LocalRuntimeControlMiddleware(localStore, config.AppCfg.Server.Host)
	if localMeshHandler != nil {
		localMeshHandler.RegisterRoutes(r, localAuth)
	}

	if services != nil && services.NativeBridgeRelay != nil {
		localNativeBridge := r.Group("/api")
		localNativeBridge.Use(localAuth)
		setupNativeBridgeRelayBackendActionHandler(services.NativeBridgeRelay, services)
		setupNativeBridgeRelayRoutes(services.NativeBridgeRelay, localNativeBridge)
		setupNativeBridgeBackendActionRoutes(services, localNativeBridge)
	}

	if err := registerDeviceAgentDesktopPetRoutes(r, ctx, services, localStore); err != nil {
		return nil, err
	}

	return r, nil
}

// registerDeviceAgentDesktopPetRoutes exposes only the local desktop-pet body
// and the credentials required by that body. It deliberately does not enable
// FullHTTPAPI: cloud business/chat/memory traffic stays on CloudCore while pet
// packages, installation state, renderer commands and Runtime v1 stay local.
func registerDeviceAgentDesktopPetRoutes(
	r *gin.Engine,
	ctx *app.AppContext,
	services *AppServices,
	localStore *security.LocalCredentialStore,
) error {
	if services == nil {
		return fmt.Errorf("device-agent services are nil")
	}
	if services.DesktopInstanceStore == nil {
		return fmt.Errorf("device-agent desktop instance store is required")
	}

	sessionSvc, err := security.NewDesktopSessionService(
		ctx.DB,
		config.AppCfg.Storage.DataDir,
		localStore,
	)
	if err != nil {
		return fmt.Errorf("initialize device-agent desktop session service: %w", err)
	}
	if err := sessionSvc.RecoverRotationJournals(context.Background()); err != nil {
		return fmt.Errorf("recover device-agent desktop session rotation: %w", err)
	}

	authConfig := security.AuthConfig{
		Mode:             config.AppCfg.Security.Mode,
		LocalCredentials: localStore,
		SpaceID:          spaceidentity.DefaultSpaceID(),
		ListenAddress:    config.AppCfg.Server.Host,
		AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
		SessionService:   sessionSvc,
	}

	// Bootstrap a short-lived renderer/main-process Desktop Session from the
	// root local token. This is the same trust boundary used by local profile.
	localSession := r.Group("/api/local")
	localSession.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:             config.AppCfg.Security.Mode,
		LocalCredentials: localStore,
		SpaceID:          spaceidentity.DefaultSpaceID(),
		ListenAddress:    config.AppCfg.Server.Host,
		AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
	}))
	localSession.Use(security.RequireAuthMethod(security.AuthMethodLocalToken))
	localSession.POST("/sessions", sessionSvc.CreateSession)

	// Physical backup/import/export belongs to the retained device Runtime even
	// when business traffic is connected to Cloud Core. Expose the canonical
	// /api/storage surface only behind device-local credentials.
	deviceStorage := r.Group("/api")
	deviceStorage.Use(security.AuthenticationMiddleware(authConfig))
	deviceStorage.Use(security.RequireAuthMethod(
		security.AuthMethodDesktopSession,
		security.AuthMethodLocalToken,
	))
	system.RegisterDeviceStorageRoutes(deviceStorage, ctx, services.DataPortability)

	bootstrapTicketRepo := desktoppetruntime.NewBootstrapTicketRepository(ctx.DB)

	// Runtime v1 bootstrap requires a device ownership record. Electron uses a
	// short-lived Desktop Session; the trusted Android/iOS embedded host uses the
	// root loopback local token. Both credentials are device-local authorities.
	localDesktop := r.Group("/api/local")
	localDesktop.Use(security.AuthenticationMiddleware(authConfig))
	localDesktop.Use(security.RequireAuthMethod(
		security.AuthMethodDesktopSession,
		security.AuthMethodLocalToken,
	))
	if services.WorkspaceService != nil {
		// Workspace operations use the canonical /api/workspaces namespace. In
		// cloud mode clients classify that namespace as device-local, so the
		// Device Agent must expose the same API surface instead of letting those
		// calls fall through to Cloud Core's filesystem.
		deviceWorkspace := r.Group("/api")
		deviceWorkspace.Use(security.AuthenticationMiddleware(authConfig))
		deviceWorkspace.Use(security.RequireAuthMethod(
			security.AuthMethodDesktopSession,
			security.AuthMethodLocalToken,
		))
		workspace.NewHandler(services.WorkspaceService).RegisterRoutes(deviceWorkspace)
		if services.WorkspaceGitHandler != nil {
			services.WorkspaceGitHandler.RegisterRoutes(deviceWorkspace)
		}
	}
	localDesktop.POST("/devices/register", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "unauthorized"})
			return
		}
		var request struct {
			DeviceID          string `json:"deviceId" binding:"required"`
			DesktopInstanceID string `json:"desktopInstanceId"`
			Platform          string `json:"platform"`
			AppVersion        string `json:"appVersion"`
		}
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
			return
		}
		request.DeviceID = strings.TrimSpace(request.DeviceID)
		request.DesktopInstanceID = strings.TrimSpace(request.DesktopInstanceID)
		if request.DeviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "deviceId is required"})
			return
		}
		switch actor.AuthMethod {
		case security.AuthMethodDesktopSession:
			headerInstanceID := strings.TrimSpace(c.GetHeader("X-Amitia-Desktop-Instance"))
			if headerInstanceID == "" || request.DesktopInstanceID == "" || request.DesktopInstanceID != headerInstanceID {
				c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "desktopInstanceId mismatch"})
				return
			}
		case security.AuthMethodLocalToken:
			// Native mobile runtimes do not have an Electron desktop-instance id.
			// Bind ownership to a deterministic synthetic instance scoped to the
			// already authenticated loopback device instead of accepting caller
			// supplied desktop authority metadata.
			request.DesktopInstanceID = "mobile-native:" + request.DeviceID
		default:
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "unsupported local auth method"})
			return
		}
		if err := services.DeviceRepository.RegisterOrTouch(c.Request.Context(), device.Identity{
			SpaceID:           actor.SpaceID,
			DeviceID:          runtimeidentity.DeviceID(strings.TrimSpace(request.DeviceID)),
			DesktopInstanceID: request.DesktopInstanceID,
			Platform:          runtimeidentity.Platform(strings.TrimSpace(request.Platform)),
			AppVersion:        strings.TrimSpace(request.AppVersion),
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to register device"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "ok"})
	})
	localDesktop.POST("/devices/:deviceId/runtime-bootstrap-tickets", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.SpaceID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "unauthorized"})
			return
		}
		deviceID := strings.TrimSpace(c.Param("deviceId"))
		if deviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "deviceId is required"})
			return
		}
		var request struct {
			RuntimeID string `json:"runtimeId"`
		}
		if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.RuntimeID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId is required"})
			return
		}
		if err := services.DeviceRepository.RequireOwned(c.Request.Context(), string(actor.SpaceID), deviceID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
			return
		}
		rawTicket, ticket, err := bootstrapTicketRepo.Create(
			c.Request.Context(),
			string(actor.SpaceID),
			deviceID,
			strings.TrimSpace(request.RuntimeID),
			10*time.Minute,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to create bootstrap ticket"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "ok",
			"data": gin.H{
				"ticketId":   ticket.ID,
				"ticket":     rawTicket,
				"spaceId":    ticket.SpaceID,
				"deviceId":   ticket.DeviceID,
				"runtimeId":  ticket.RuntimeID,
				"expiresAt":  ticket.ExpiresAt,
				"ttlSeconds": 600,
			},
		})
	})

	// CoreManager uses this local-admin endpoint for orderly process shutdown in
	// both local and device-agent profiles. Keeping it here avoids force-killing
	// the device-agent while desktop-pet journals/runtime state are being flushed.
	localAdmin := r.Group("/api/local/admin")
	localAdmin.Use(security.LocalAdminAuthenticationMiddleware(security.AuthConfig{
		Mode:                     config.AppCfg.Security.Mode,
		LocalCredentials:         localStore,
		SpaceID:                  spaceidentity.DefaultSpaceID(),
		ListenAddress:            config.AppCfg.Server.Host,
		AllowedOrigins:           config.AppCfg.Security.AllowedOrigins,
		DesktopInstanceValidator: services.DesktopInstanceStore.Validate,
	}))
	localAdmin.Use(security.RequirePermission("system.shutdown"))
	localAdmin.POST("/shutdown", func(c *gin.Context) {
		actor := security.GetActor(c)
		if actor == nil || actor.AuthMethod != security.AuthMethodLocalAdminToken || !actor.IsLocalTrusted {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "local admin credential required"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"code":                202,
			"msg":                 "shutting down",
			"shutdownOperationId": generateShutdownOpID(),
		})
		go func() {
			time.Sleep(300 * time.Millisecond)
			if triggerShutdown != nil {
				triggerShutdown()
			}
		}()
	})

	// The device-agent HTTP surface is intentionally limited to the desktop-pet
	// namespace. Renderer traffic uses a short-lived Desktop Session; trusted
	// native mobile hosts may use the root local token returned by the runtime bridge.
	// Game packages are device execution artifacts. In cloud mode the desktop
	// uploads them directly to the loopback Device Agent using its Desktop
	// Session; only lightweight runtime control traverses Cloud Core/Device Mesh.
	deviceExtensions := r.Group("/api/extensions")
	deviceExtensions.Use(security.AuthenticationMiddleware(authConfig))
	deviceExtensions.Use(security.RequireAuthMethod(security.AuthMethodDesktopSession))
	deviceExtensions.Use(readiness.RejectWritesWhenSafeMode(services.SafeMode))
	extension.RegisterDeviceExecutionPackageRoutes(deviceExtensions, services.Extension)

	localWorkflows := r.Group("/api/local")
	localWorkflows.Use(security.AuthenticationMiddleware(authConfig))
	localWorkflows.Use(security.RequireAuthMethod(
		security.AuthMethodDesktopSession,
		security.AuthMethodLocalToken,
	))
	localWorkflows.Use(readiness.RejectWritesWhenSafeMode(services.SafeMode))
	extension.RegisterDeviceExecutionWorkflowRoutes(localWorkflows, services.Extension)

	petAPI := r.Group("/api")
	petAPI.Use(security.AuthenticationMiddleware(authConfig))
	petAPI.Use(security.RequireAuthMethod(
		security.AuthMethodDesktopSession,
		security.AuthMethodLocalToken,
	))
	petAPI.Use(readiness.RejectWritesWhenSafeMode(services.SafeMode))
	{
		desktoppet.RegisterDesktopPetRouter(petAPI, ctx, services.PathRegistry)
		desktoppet.RegisterDesktopPetWriteRouter(petAPI, ctx, services.PathRegistry)
		processing.RegisterProcessingRouter(petAPI, ctx, services.PathRegistry)
		editing.RegisterEditingRouterWithService(petAPI, services.EditingService, services.OwnershipGuard, services.PathRegistry)
		quality.RegisterQualityRouter(petAPI, services.QualityService, services.OwnershipGuard)
		installation.RegisterRoutes(petAPI, services.InstallationCoordinator, services.InstallationRepo, services.OwnershipGuard)
		release.RegisterRoutes(petAPI, services.NewReleaseService, services.OwnershipGuard)
		registerImportStagingRoutes(petAPI, services.PathRegistry, services.ImportStagingRepo, services.OwnershipGuard, services.PackageImporter)
		behavior.RegisterRoutes(petAPI, services.BehaviorService)
		runtimev1.RegisterUserRoutes(petAPI, services.DesktopPetRuntimeV1)
	}

	// Runtime v1 WebSocket is local and ticket-authenticated. It is not exposed
	// on the cloud core and does not accept cloud bearer credentials.
	runtimev1.RegisterInternalRoutes(
		r,
		services.DesktopPetRuntimeV1,
		services.SafeMode,
		func(ctx context.Context, rawTicket string, runtimeID runtimeidentity.RuntimeID, deviceID runtimeidentity.DeviceID) (runtimeidentity.SpaceID, error) {
			ticket, err := bootstrapTicketRepo.ConsumeWithValidation(ctx, rawTicket, string(runtimeID), string(deviceID))
			if err != nil {
				return "", err
			}
			return runtimeidentity.SpaceID(ticket.SpaceID), nil
		},
	)

	return nil
}

func deviceAgentDesktopPetReady(services *AppServices) error {
	if services == nil {
		return fmt.Errorf("desktop pet services not initialized")
	}
	if services.DesktopPetWorker == nil || !services.DesktopPetWorker.IsRunning() {
		return fmt.Errorf("desktop pet generation worker not running")
	}
	if services.ProcessingWorker == nil || !services.ProcessingWorker.IsRunning() {
		return fmt.Errorf("desktop pet processing worker not running")
	}
	if services.QualityWorker == nil || !services.QualityWorker.IsRunning() {
		return fmt.Errorf("desktop pet quality worker not running")
	}
	if services.RegenerationWorker == nil || !services.RegenerationWorker.IsRunning() {
		return fmt.Errorf("desktop pet regeneration worker not running")
	}
	if services.BridgeRecoveryWorker == nil || !services.BridgeRecoveryWorker.IsRunning() {
		return fmt.Errorf("desktop pet revision bridge recovery worker not running")
	}
	if services.InstallationProjectionBridge == nil || !services.InstallationProjectionBridge.IsRunning() {
		return fmt.Errorf("desktop pet installation projection bridge not running")
	}
	if services.InstallationDesiredOutbox == nil || !services.InstallationDesiredOutbox.IsRunning() {
		return fmt.Errorf("desktop pet installation desired outbox worker not running")
	}
	if services.InstallationRecoveryWorker == nil || !services.InstallationRecoveryWorker.IsRunning() {
		return fmt.Errorf("desktop pet installation recovery worker not running")
	}
	if services.ReleaseRecoveryWorker == nil || !services.ReleaseRecoveryWorker.IsRunning() {
		return fmt.Errorf("desktop pet release recovery worker not running")
	}
	if services.ReleaseEventOutboxDispatcher == nil || !services.ReleaseEventOutboxDispatcher.IsRunning() {
		return fmt.Errorf("desktop pet release event outbox dispatcher not running")
	}
	if services.BehaviorService == nil || !services.BehaviorService.IsRunning() {
		return fmt.Errorf("desktop pet behavior service not running")
	}
	if services.DesktopPetRuntimeV1 == nil || !services.DesktopPetRuntimeV1.IsStarted() {
		return fmt.Errorf("desktop pet runtime v1 not running")
	}
	if services.RuntimeDomainEventConsumer == nil || !services.RuntimeDomainEventConsumer.IsRunning() {
		return fmt.Errorf("desktop pet runtime domain event consumer not running")
	}
	return nil
}

func setLocalMeshHandler(h *agent.LocalHandler) {
	localMeshHandler = h
}
