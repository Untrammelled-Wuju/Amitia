// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/accountsession"
	"github.com/u-ai/backend/internal/agent"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/device"
	"github.com/u-ai/backend/internal/desktoppet/doctor"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/maintenance"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/release/importer"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
	desktoppetsecurity "github.com/u-ai/backend/internal/desktoppet/security"
	devicemeshserver "github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/management"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/imagegen"
	iosnativebackground "github.com/u-ai/backend/internal/iosnative/background"
	"github.com/u-ai/backend/internal/mcpapi"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mood"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/realtime"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/sync"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/internal/user"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
)

func setupRouter(ctx *app.AppContext, services *AppServices, bootstrap *runtimeBootstrap) (*gin.Engine, error) {
	if config.AppCfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(middleware.TraceMiddleware())
	r.Use(security.CorsMiddleware(security.CorsConfig{
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))

	bootstrapTicketRepo := runtime.NewBootstrapTicketRepository(ctx.DB)

	tokenPath := filepath.Join(config.AppCfg.Storage.DataDir, "security", "local-token")
	localCredentialStore, err := security.NewLocalCredentialStore(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("initialize local credential store: %w", err)
	}

	r.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "alive"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		if services.RuntimeOrchestrator == nil {
			c.JSON(503, gin.H{"code": 503, "msg": "blocked", "data": gin.H{"status": "blocked", "reason": "orchestrator not initialized"}})
			return
		}
		if services.AccountSession == nil || services.AccountSession.Validator == nil {
			c.JSON(503, gin.H{"code": 503, "msg": "blocked", "data": gin.H{"status": "blocked", "reason": "accountsession validator not initialized"}})
			return
		}
		snap := services.RuntimeOrchestrator.Snapshot()
		overallStatus := "ready"
		if snap.IsBlocked() {
			overallStatus = "blocked"
		} else if snap.State == runtimeorchestrator.OrchestratorDegraded {
			overallStatus = "degraded"
		}
		result := gin.H{
			"status":        overallStatus,
			"state":         snap.State,
			"blockingCount": snap.BlockingCount,
			"degradedCount": snap.DegradedCount,
			"readyCount":    snap.ReadyCount,
			"failedCount":   snap.FailedCount,
			"timestamp":     snap.Timestamp,
		}
		httpStatus := 200
		if snap.IsBlocked() {
			httpStatus = 503
		}
		c.JSON(httpStatus, gin.H{"code": httpStatus, "msg": overallStatus, "data": result})
	})

	systemSvc := system.NewService(ctx, services.RuntimeProfile)
	systemHandler := system.NewHandler(systemSvc, ctx.DB, services.Chat, services.DataLifecycle, services.UnifiedEntry, services.Reconciliation, services.Memory)

	public := r.Group("/api/public")

	userRepo := user.NewRepository(ctx)
	userSvc := user.NewService(userRepo, ctx)
	userHandler := user.NewHandler(userSvc)

	accountSessionRuntime, err := BuildAccountSessionRuntime(ctx.DB, userRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to build accountsession runtime: %w", err)
	}
	services.AccountSession = accountSessionRuntime

	public.GET("/auth/status", userHandler.Status)

	accountsession.RegisterPublicRoutes(public, accountSessionRuntime.Handler)

	public.GET("/onboarding/status", systemHandler.OnboardingStatus)
	public.POST("/onboarding/complete", systemHandler.OnboardingComplete)
	public.GET("/health", systemHandler.Health)

	chatHandler := chat.NewHandlerWithUnifiedEntry(services.Chat, services.UnifiedEntry)
	public.POST("/model/detect-models", chatHandler.DetectModels)

	sessionSvc, err := security.NewDesktopSessionService(ctx.DB, config.AppCfg.Storage.DataDir, localCredentialStore)
	if err != nil {
		return nil, fmt.Errorf("initialize desktop session service: %w", err)
	}

	if err := sessionSvc.RecoverRotationJournals(context.Background()); err != nil {
		return nil, fmt.Errorf("recover local token rotation: %w", err)
	}

	if services.DesktopInstanceStore == nil && config.AppCfg.Security.Mode == "local_single_user" {
		return nil, fmt.Errorf("desktop instance store is required in local_single_user mode")
	}

	if services.DesktopInstanceStore != nil {
		localRoot :=
			r.Group("/api/local")

		localRoot.Use(
			security.AuthenticationMiddleware(
				security.AuthConfig{
					Mode:             config.AppCfg.Security.Mode,
					JWTSecret:        config.AppCfg.JWT.Secret,
					JWTIssuer:        config.AppCfg.JWT.Issuer,
					JWTAudience:      config.AppCfg.JWT.Audience,
					LocalCredentials: localCredentialStore,
					LocalUserID:      config.AppCfg.Security.LocalUserID,
					ListenAddress:    config.AppCfg.Server.Host,
					AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
				},
			),
		)

		localRoot.Use(
			security.RequireAuthMethod(
				security.AuthMethodLocalToken,
			),
		)
		{
			localRoot.POST(
				"/sessions",
				sessionSvc.CreateSession,
			)
		}
	}

	if services.DesktopInstanceStore != nil {
		localDesktop :=
			r.Group("/api/local")

		localDesktop.Use(
			security.AuthenticationMiddleware(
				security.AuthConfig{
					Mode:             config.AppCfg.Security.Mode,
					JWTSecret:        config.AppCfg.JWT.Secret,
					JWTIssuer:        config.AppCfg.JWT.Issuer,
					JWTAudience:      config.AppCfg.JWT.Audience,
					LocalCredentials: localCredentialStore,
					LocalUserID:      config.AppCfg.Security.LocalUserID,
					ListenAddress:    config.AppCfg.Server.Host,
					AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
					SessionService:   sessionSvc,
				},
			),
		)

		localDesktop.Use(
			security.RequireAuthMethod(
				security.AuthMethodDesktopSession,
			),
		)
		{
			localDesktop.POST("/devices/register", func(c *gin.Context) {
				actor := security.GetActor(c)
				if actor == nil || actor.UserID == "" {
					c.JSON(401, gin.H{"code": 401, "msg": "unauthorized"})
					return
				}
				var request struct {
					DeviceID          string `json:"deviceId" binding:"required"`
					DesktopInstanceID string `json:"desktopInstanceId"`
					Platform          string `json:"platform"`
					AppVersion        string `json:"appVersion"`
				}
				if err := c.ShouldBindJSON(&request); err != nil {
					c.JSON(400, gin.H{"code": 400, "msg": err.Error()})
					return
				}
				headerInstanceID :=
					strings.TrimSpace(
						c.GetHeader(
							"X-Amitia-Desktop-Instance",
						),
					)
				request.DesktopInstanceID =
					strings.TrimSpace(
						request.DesktopInstanceID,
					)
				if headerInstanceID == "" ||
					request.DesktopInstanceID == "" ||
					request.DesktopInstanceID !=
						headerInstanceID {
					c.JSON(
						http.StatusBadRequest,
						gin.H{
							"code": 400,
							"msg":  "desktopInstanceId mismatch",
						},
					)
					return
				}
				err := services.DeviceRepository.RegisterOrTouch(c.Request.Context(), device.Identity{
					UserID:            actor.UserID,
					DeviceID:          runtimeidentity.DeviceID(request.DeviceID),
					DesktopInstanceID: request.DesktopInstanceID,
					Platform:          runtimeidentity.Platform(request.Platform),
					AppVersion:        request.AppVersion,
				})
				if err != nil {
					log.Error("failed to register device identity", "error", err)
					c.JSON(500, gin.H{"code": 500, "msg": "failed to register device"})
					return
				}
				c.JSON(200, gin.H{"code": 200, "msg": "ok"})
			})
			localDesktop.POST("/devices/:deviceId/runtime-bootstrap-tickets", func(c *gin.Context) {
				actor := security.GetActor(c)
				if actor == nil || actor.UserID == "" {
					c.JSON(401, gin.H{"code": 401, "msg": "unauthorized"})
					return
				}
				deviceID := c.Param("deviceId")
				if deviceID == "" {
					c.JSON(400, gin.H{"code": 400, "msg": "deviceId is required"})
					return
				}
				var request struct {
					RuntimeID string `json:"runtimeId"`
				}
				if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.RuntimeID) == "" {
					c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "runtimeId is required"})
					return
				}
				runtimeID := strings.TrimSpace(request.RuntimeID)
				if err := services.DeviceRepository.RequireOwned(c.Request.Context(), string(actor.UserID), deviceID); err != nil {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
					return
				}
				rawTicket, ticket, err := bootstrapTicketRepo.Create(c.Request.Context(), string(actor.UserID), deviceID, runtimeID, 10*time.Minute)
				if err != nil {
					log.Error("failed to create bootstrap ticket", "error", err)
					c.JSON(500, gin.H{"code": 500, "msg": "failed to create bootstrap ticket"})
					return
				}
				c.JSON(200, gin.H{
					"code": 200,
					"msg":  "ok",
					"data": gin.H{
						"ticketId":   ticket.ID,
						"ticket":     rawTicket,
						"userId":     ticket.UserID,
						"deviceId":   ticket.DeviceID,
						"runtimeId":  ticket.RuntimeID,
						"expiresAt":  ticket.ExpiresAt,
						"ttlSeconds": 600,
					},
				})
			})
			localDesktop.DELETE("/devices/:deviceId/runtime-bootstrap-tickets", func(c *gin.Context) {
				actor := security.GetActor(c)
				if actor == nil || actor.UserID == "" {
					c.JSON(401, gin.H{"code": 401, "msg": "unauthorized"})
					return
				}
				deviceID := c.Param("deviceId")
				if deviceID == "" {
					c.JSON(400, gin.H{"code": 400, "msg": "deviceId is required"})
					return
				}
				affected, err := bootstrapTicketRepo.RevokeDeviceTickets(c.Request.Context(), string(actor.UserID), deviceID)
				if err != nil {
					log.Error("failed to revoke device bootstrap tickets", "error", err)
					c.JSON(500, gin.H{"code": 500, "msg": "failed to revoke tickets"})
					return
				}
				c.JSON(200, gin.H{"code": 200, "msg": "ok", "data": gin.H{"revoked": affected}})
			})
		}
	}

	if services.DesktopInstanceStore != nil {
		localAdmin := r.Group("/api/local/admin")
		localAdmin.Use(security.LocalAdminAuthenticationMiddleware(security.AuthConfig{
			Mode:                     config.AppCfg.Security.Mode,
			LocalCredentials:         localCredentialStore,
			LocalUserID:              config.AppCfg.Security.LocalUserID,
			ListenAddress:            config.AppCfg.Server.Host,
			AllowedOrigins:           config.AppCfg.Security.AllowedOrigins,
			DesktopInstanceValidator: services.DesktopInstanceStore.Validate,
		}))
		localAdmin.Use(security.RequirePermission("system.shutdown"))
		localAdmin.POST("/token/rotate", sessionSvc.RotateToken)
		localAdmin.POST("/shutdown", func(c *gin.Context) {
			actor := security.GetActor(c)
			if actor == nil || actor.AuthMethod != security.AuthMethodLocalAdminToken || !actor.IsLocalTrusted {
				c.JSON(403, gin.H{"code": 403, "msg": "local admin credential required"})
				return
			}
			log.Info("system.shutdown.requested", "actor", actor.UserID, "method", actor.AuthMethod)
			c.JSON(202, gin.H{"code": 202, "msg": "shutting down", "shutdownOperationId": generateShutdownOpID()})
			go func() {
				time.Sleep(300 * time.Millisecond)
				log.Info("system.shutdown.completed")
				if triggerShutdown != nil {
					triggerShutdown()
				}
			}()
		})
	}

	if services.MCPCompatibility != nil {
		mcpapi.RegisterOAuthCallback(r, services.MCPCompatibility.API)
	}

	apiGroup := r.Group("/api")
	apiGroup.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:             config.AppCfg.Security.Mode,
		JWTSecret:        config.AppCfg.JWT.Secret,
		JWTIssuer:        config.AppCfg.JWT.Issuer,
		JWTAudience:      config.AppCfg.JWT.Audience,
		LocalCredentials: localCredentialStore,
		LocalUserID:      config.AppCfg.Security.LocalUserID,
		ListenAddress:    config.AppCfg.Server.Host,
		AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
		SessionService:   sessionSvc,
		AccountSessions:  accountSessionRuntime.Validator,
	}))
	{
		accountsession.RegisterAuthenticatedRoutes(apiGroup, accountSessionRuntime.Handler)
		if services.MCPCompatibility != nil {
			mcpapi.RegisterRouter(apiGroup, ctx, services.MCPCompatibility.API)
		}
		user.RegisterUserRouter(apiGroup, ctx)
		if services.Sync != nil && services.Sync.ChangeLog != nil {
			character.RegisterCharacterRouterWithRecorder(apiGroup, ctx, services.Chat, services.Sync.ChangeLog)
		} else {
			character.RegisterCharacterRouter(apiGroup, ctx, services.Chat)
		}
		chat.RegisterChatRouterWithDelivery(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.ChatDeliveryAdapter)
		memHandler := memory.RegisterMemoryRouter(apiGroup, ctx, services.Graph)
		apiGroup.GET("/memory/retrieval/stats", memHandler.RetrieveStats)
		apiGroup.GET("/memory/pipeline/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "data": services.Chat.GetPipelineStatus(), "msg": "\u64cd\u4f5c\u6210\u529f"})
		})
		profile.RegisterProfileRouter(apiGroup, services.Profile)
		proHandler := proactive.RegisterProactiveRouterWithCompanion(apiGroup, ctx, services.Companion)
		proactive.RegisterRemindersRouter(apiGroup, proHandler)
		episodic.RegisterEpisodicRouter(apiGroup, services.Episodic)
		worldbook.RegisterWorldBookRouter(apiGroup, services.WorldBook)
		feedback.RegisterFeedbackRouter(apiGroup, ctx)
		graph.RegisterGraphRouter(apiGroup, config.AppCfg.Providers.GraphStore.SurrealDB)
		agent.RegisterAgentRouter(apiGroup, ctx, services.UnifiedEntry)
		system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.DataLifecycle, services.Reconciliation, services.Memory, services.Profile, services.Episodic, services.Graph, services.Temporal, services.DataPortability, services.Artifact.Service)
		companion.RegisterCompanionRouter(apiGroup, services.Companion)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
		embedding_config.RegisterEmbeddingConfigRouter(apiGroup, ctx)
		imagegen.RegisterImageGenRouter(apiGroup, ctx)
		desktoppet.RegisterDesktopPetRouter(apiGroup, ctx, services.PathRegistry)
		doctor.RegisterRouter(apiGroup, ctx.DB, services.Extension)
		readiness.RegisterRouter(apiGroup, ctx.DB, services.Extension)
		system.RegisterPsycheAPIRouter(apiGroup)
		system.RegisterPsycheSnapshotRouter(apiGroup, ctx.DB)
		system.RegisterHealthRouter(apiGroup, services.CircuitBreakers, services.DataLifecycle, services.Reconciliation)
		ttsRepo := tts.NewRepository(ctx.DB)
		ttsSvc := tts.NewService(ttsRepo)
		system.RegisterVoiceEntryRouter(apiGroup, services.VoiceEntry, ttsSvc, services.DeliveryStore)
		safety.RegisterSafetyRouter(apiGroup, ctx.DB)
		delivery.RegisterSubmitRouter(apiGroup, services.DeliveryStore)
		extension.RegisterRouter(apiGroup, ctx, services.Extension)
		if services.Artifact != nil && services.Artifact.Handler != nil {
			services.Artifact.Handler.Register(apiGroup)
		}
		if services.WorkspaceService != nil {
			workspace.NewHandler(services.WorkspaceService).RegisterRoutes(apiGroup)
		}
		if services.KernelContainer != nil {
			cardProvider := extension_center.NewKernelCardProvider(services.KernelContainer.DefinitionRepository, services.KernelContainer.InstallationRepository)
			centerSvc := extension_center.NewCenterService(cardProvider)
			centerHandler := extension_center.NewHTTPHandler(centerSvc)
			centerMux := http.NewServeMux()
			centerHandler.Register(centerMux)
			apiGroup.Any("/extension-center/*centerPath", func(c *gin.Context) {
				originalPath := c.Request.URL.Path
				c.Request.URL.Path = "/api/extension-center" + c.Param("centerPath")
				centerMux.ServeHTTP(c.Writer, c.Request)
				c.Request.URL.Path = originalPath
			})
		}
		if services.KernelContainer != nil && services.KernelContainer.WASMRuntimeFactory != nil {
			wasmService := wasm_runtime.NewAPIService(services.KernelContainer.WASMRuntimeFactory, services.KernelContainer.WASMDefinitionRepo)
			wasmHandler := wasm_runtime.NewHTTPHandler(wasmService)
			wasmMux := http.NewServeMux()
			wasmHandler.Register(wasmMux)
			apiGroup.Any("/wasm/*wasmPath", gin.WrapH(wasmMux))
		}
	if services.KernelContainer != nil && services.KernelContainer.GameHost != nil {
		log.Info("[GameCenter] GameHost detected, registering game-center routes")
		kernelReader := management.NewKernelReaderWithContributions(services.KernelContainer.DefinitionRepository, services.KernelContainer.InstallationRepository, services.KernelContainer.ContributionRepository)
		gameCenterSvc := management.NewProductionService(services.KernelContainer.GameHost, kernelReader)
		management.RegisterGameCenterRouter(apiGroup, gameCenterSvc)

		if services.Extension != nil {
			kernelMutation := management.NewKernelMutationFromFuncs(management.KernelMutationOptions{
				EnableFn: func(ctx context.Context, extensionID string) error {
					return services.Extension.Kernel.Enable(ctx, extensionID)
				},
				DisableFn: func(ctx context.Context, extensionID string) error {
					return services.Extension.Kernel.Disable(ctx, extensionID)
				},
			})
			var upgradeCoordinator management.PackageUpgradeCoordinator = nil
			if services.KernelContainer.GameHost != nil && services.KernelContainer.GameHost.UpgradeCoordinator != nil {
				upgradeCoordinator = &management.GameHostUpgradeCoordinatorAdapter{UC: services.KernelContainer.GameHost.UpgradeCoordinator}
			}
			packageSvc := management.NewProductionPackageMutationServiceFromKernelReader(kernelReader, management.NewGameHostPluginRegistryFromContainer(services.KernelContainer.GameHost), kernelMutation, upgradeCoordinator)
			runtimeSvc := management.NewProductionRuntimeMutationService(services.KernelContainer.GameHost)
			gameCenterMutationHandler := management.NewMutationHandler(packageSvc, runtimeSvc)
			management.RegisterGameCenterMutationRouter(apiGroup, gameCenterMutationHandler)

			if services.KernelContainer.GameHost != nil && services.KernelContainer.GameHost.TakeoverService != nil {
				controlHandler := management.NewControlHandlerFromFuncs(management.ControlServiceOptions{
					TakeoverFn: func(ctx context.Context, runtimeID string) (management.TakeoverResult, error) {
						_, err := services.KernelContainer.GameHost.TakeoverService.Takeover(ctx, control.TakeoverRequest{
							RuntimeID: domain.RuntimeInstanceID(runtimeID),
							Actor:     "game_center_user",
						})
						if err != nil {
							return management.TakeoverResult{}, err
						}
						return management.TakeoverResult{Success: true}, nil
					},
					ReleaseFn: func(ctx context.Context, runtimeID string, targetMode string, expectedEpoch uint64) (management.ReleaseResult, error) {
						_, err := services.KernelContainer.GameHost.TakeoverService.Release(ctx, control.ReleaseRequest{
							RuntimeID:     domain.RuntimeInstanceID(runtimeID),
							TargetMode:    domain.ControlMode(targetMode),
							Actor:         "game_center_user",
							ExpectedEpoch: expectedEpoch,
							UseExpected:   expectedEpoch > 0,
						})
						if err != nil {
							return management.ReleaseResult{}, err
						}
						return management.ReleaseResult{Success: true}, nil
					},
					EmergencyStopFn: func(ctx context.Context, runtimeID string) (control.EmergencyStopResult, error) {
						return services.KernelContainer.GameHost.EmergencyStopService.Execute(ctx, domain.RuntimeInstanceID(runtimeID))
					},
				})
				management.RegisterGameCenterControlRouter(apiGroup, controlHandler)
			}

			if services.KernelContainer.GameHost != nil && services.KernelContainer.GameHost.ControlPlane != nil {
				rpcInvoker := management.NewControlPlaneRPCInvoker(
					services.KernelContainer.GameHost.ControlPlane,
					management.NewGameHostTopologyStore(services.KernelContainer.GameHost.RuntimeTopologyStore),
					management.NewGameHostPluginRegistry(services.KernelContainer.GameHost.PluginRegistry),
				)
				rpcHandler := management.NewRPCHandler(rpcInvoker)
				management.RegisterRPCRouter(apiGroup, rpcHandler)

				debugHandler := management.NewDebugHandler(services.KernelContainer.GameHost)
				management.RegisterDebugRouter(apiGroup, debugHandler)
			}
		}
	}
	emote.RegisterRouter(apiGroup, services.Emote)
	temporal.RegisterRouter(apiGroup, services.Temporal, services.RelTimeCoordinator)
	mood.RegisterMoodRouter(apiGroup, ctx)
	if services.Sync != nil {
		syncHandler := sync.NewHandler(services.Sync, services.DeviceRepository)
		syncHandler.RegisterRoutes(apiGroup, security.AuthenticationMiddleware(security.AuthConfig{
			Mode:             config.AppCfg.Security.Mode,
			JWTSecret:        config.AppCfg.JWT.Secret,
			JWTIssuer:        config.AppCfg.JWT.Issuer,
			JWTAudience:      config.AppCfg.JWT.Audience,
			LocalCredentials: localCredentialStore,
			LocalUserID:      config.AppCfg.Security.LocalUserID,
			ListenAddress:    config.AppCfg.Server.Host,
			AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
			SessionService:   sessionSvc,
			AccountSessions:  accountSessionRuntime.Validator,
		}))
	}

		if services.DeviceMesh == nil || services.DeviceMesh.BootstrapSvc == nil || services.DeviceMesh.CredentialSvc == nil || services.DeviceMesh.Hub == nil || services.DeviceMesh.Handler == nil {
			return nil, fmt.Errorf("device mesh: required cloud runtime dependencies are not initialized")
		}
		{
			deviceMeshAuthMW := security.AuthenticationMiddleware(security.AuthConfig{
				Mode:             config.AppCfg.Security.Mode,
				JWTSecret:        config.AppCfg.JWT.Secret,
				JWTIssuer:        config.AppCfg.JWT.Issuer,
				JWTAudience:      config.AppCfg.JWT.Audience,
				LocalCredentials: localCredentialStore,
				LocalUserID:      config.AppCfg.Security.LocalUserID,
				ListenAddress:    config.AppCfg.Server.Host,
				AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
				SessionService:   sessionSvc,
				AccountSessions:  accountSessionRuntime.Validator,
			})
			meshSQLDB, meshDBErr := ctx.DB.DB()
			if meshDBErr != nil {
				return nil, fmt.Errorf("device mesh: resolve sql db: %w", meshDBErr)
			}
			if err := devicemeshserver.RegisterCloudRoutes(apiGroup, deviceMeshAuthMW, &devicemeshserver.RouterDeps{
				DB:            meshSQLDB,
				Sessions:      services.DeviceMesh.GetSessions(),
				BootstrapSvc:  services.DeviceMesh.BootstrapSvc,
				CredentialSvc: services.DeviceMesh.CredentialSvc,
				Hub:           services.DeviceMesh.Hub,
				Handler:       services.DeviceMesh.Handler,
				Probe:         services.DeviceMesh.Probe,
				DeviceReg:     services.DeviceMesh.DeviceReg,
				GetUserID: func(c *gin.Context) (runtimeidentity.UserID, bool) {
					actor := security.GetActor(c)
					if actor == nil || actor.UserID == "" {
						return "", false
					}
					return runtimeidentity.UserID(actor.UserID), true
				},
				InvocationResultHandler: devicemeshserver.InvocationResultHandler(func(result protocol.RuntimeResultPayload) {
					if services.DeviceMesh.PendingInvocations == nil {
						return
					}
					unifiedResult := capability.NewToolSuccessResult(result.InvocationID, "")
					unifiedResult.RuntimeSessionID = string(result.RuntimeSessionID)
					unifiedResult.DeviceID = string(result.DeviceID)
					unifiedResult.RuntimeID = string(result.RuntimeID)
					unifiedResult.Generation = result.ConnectionGeneration
					unifiedResult.Structured = result.Result
					services.DeviceMesh.PendingInvocations.Complete(result.InvocationID, unifiedResult)
				}),
				InvocationErrorHandler: devicemeshserver.InvocationErrorHandler(func(errResult protocol.RuntimeErrorPayload) {
					if services.DeviceMesh.PendingInvocations == nil {
						return
					}
					unifiedResult := capability.NewToolFailureResult(errResult.InvocationID, "", &capability.ToolError{
						Code:      errResult.ErrorCode,
						Message:   errResult.Message,
						Retryable: errResult.Retryable,
					})
					unifiedResult.RuntimeSessionID = string(errResult.RuntimeSessionID)
					unifiedResult.DeviceID = string(errResult.DeviceID)
					unifiedResult.RuntimeID = string(errResult.RuntimeID)
					unifiedResult.Generation = errResult.ConnectionGeneration
					services.DeviceMesh.PendingInvocations.Fail(errResult.InvocationID, unifiedResult)
				}),
				TaskClaimPayloadHandler: devicemeshserver.TaskClaimPayloadAdapter(func(claim protocol.TaskClaimPayload) bool {
					if services.DeviceMesh.PendingTasks == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
						return false
					}
					if !services.DeviceMesh.PendingTasks.ValidateBound(claim.TaskRunID, claim.AttemptID, claim.LeaseID, claim.RuntimeSessionID.String(), claim.ConnectionGeneration) {
						return false
					}
					leaseDuration := time.Duration(claim.LeaseDurationMs) * time.Millisecond
					if leaseDuration <= 0 {
						leaseDuration = 5 * time.Minute
					}
					if err := services.KernelContainer.TaskRuntimeService.HandleRemoteClaim(context.Background(), claim.TaskRunID, claim.AttemptID, claim.LeaseID, time.Now().UTC().Add(leaseDuration)); err != nil {
						return false
					}
					return services.DeviceMesh.PendingTasks.ClaimBound(
						claim.TaskRunID, claim.AttemptID, claim.LeaseID, claim.RuntimeSessionID.String(),
						claim.ConnectionGeneration, claim.WorkerID, leaseDuration,
					)
				}),
				TaskHeartbeatPayloadHandler: devicemeshserver.TaskHeartbeatPayloadAdapter(func(heartbeat protocol.TaskHeartbeatPayload) {
					if services.DeviceMesh.PendingTasks == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
						return
					}
					const leaseExtension = 5 * time.Minute
					if !services.DeviceMesh.PendingTasks.HeartbeatBound(heartbeat.TaskRunID, heartbeat.AttemptID, heartbeat.LeaseID, heartbeat.RuntimeSessionID.String(), heartbeat.ConnectionGeneration, heartbeat.Sequence, leaseExtension) {
						return
					}
					if err := services.KernelContainer.TaskRuntimeService.HeartbeatRemoteTask(context.Background(), heartbeat.TaskRunID, heartbeat.AttemptID, heartbeat.LeaseID, leaseExtension); err != nil {
						services.DeviceMesh.PendingTasks.Cancel(heartbeat.TaskRunID, "task runtime heartbeat persist failed")
						log.Error("device mesh task heartbeat persist failed", "taskRunId", heartbeat.TaskRunID, "error", err)
					}
				}),
				TaskProgressPayloadHandler: devicemeshserver.TaskProgressPayloadAdapter(func(progress protocol.TaskProgressPayload) {
					if services.DeviceMesh.PendingTasks == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
						return
					}
					if !services.DeviceMesh.PendingTasks.ValidateBound(progress.TaskRunID, progress.AttemptID, progress.LeaseID, progress.RuntimeSessionID.String(), progress.ConnectionGeneration) {
						return
					}
					if err := services.KernelContainer.TaskRuntimeService.HandleProgress(context.Background(), progress.TaskRunID, progress.AttemptID, progress.LeaseID, progress.Sequence, progress.Current, progress.Total, progress.Percentage, progress.Stage, progress.Message); err != nil {
						log.Error("device mesh task progress persist failed", "taskRunId", progress.TaskRunID, "error", err)
					}
				}),
				TaskCheckpointPayloadHandler: devicemeshserver.TaskCheckpointPayloadAdapter(func(checkpoint protocol.TaskCheckpointPayload) {
					if services.DeviceMesh.PendingTasks == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
						return
					}
					if !services.DeviceMesh.PendingTasks.ValidateBound(checkpoint.TaskRunID, checkpoint.AttemptID, checkpoint.LeaseID, checkpoint.RuntimeSessionID.String(), checkpoint.ConnectionGeneration) {
						return
					}
					if err := services.KernelContainer.TaskRuntimeService.HandleCheckpoint(context.Background(), checkpoint.TaskRunID, checkpoint.AttemptID, checkpoint.LeaseID, checkpoint.CheckpointID, checkpoint.Version, checkpoint.Payload, checkpoint.PayloadHash); err != nil {
						log.Error("device mesh task checkpoint persist failed", "taskRunId", checkpoint.TaskRunID, "error", err)
					}
				}),
				TaskCompletePayloadHandler: devicemeshserver.TaskCompletePayloadAdapter(func(complete protocol.TaskCompletePayload) {
					if services.DeviceMesh.PendingTasks == nil || services.KernelContainer == nil || services.KernelContainer.TaskRuntimeService == nil {
						return
					}
					if !services.DeviceMesh.PendingTasks.ValidateBound(complete.TaskRunID, complete.AttemptID, complete.LeaseID, complete.RuntimeSessionID.String(), complete.ConnectionGeneration) {
						return
					}
					if err := services.KernelContainer.TaskRuntimeService.HandleCompletion(context.Background(), complete.TaskRunID, complete.AttemptID, complete.LeaseID, complete.Success, complete.Result, complete.Error); err != nil {
						log.Error("device mesh task completion persist failed", "taskRunId", complete.TaskRunID, "error", err)
						return
					}
					services.DeviceMesh.PendingTasks.CompleteBound(complete.TaskRunID, complete.AttemptID, complete.LeaseID, complete.RuntimeSessionID.String(), complete.ConnectionGeneration, complete.Success, complete.Error)
				}),
				DisconnectHandler: devicemeshserver.DisconnectHandler(func(sessionID string, generation int64) {
					if services.DeviceMesh.PendingInvocations != nil {
						services.DeviceMesh.PendingInvocations.CancelAll(sessionID, "device connection closed")
					}
					if services.DeviceMesh.PendingTasks != nil {
						services.DeviceMesh.PendingTasks.CancelAll(sessionID, "device connection closed")
					}
				}),
			}); err != nil {
				return nil, fmt.Errorf("register device mesh routes: %w", err)
			}
		}

		if services.NativeBridgeRelay != nil && bootstrap != nil {
			tryRegisterAndroidBridge(services.NativeBridgeRelay, bootstrap)
			tryRegisterIOSBridge(services.NativeBridgeRelay, bootstrap)
			setupNativeBridgeRelayRoutes(services.NativeBridgeRelay, apiGroup)

			if services.KernelContainer != nil && services.KernelContainer.TaskRuntimeService != nil {
				eventSinkRouter := iosnativebackground.NewTaskRuntimeEventSinkRouter(services.KernelContainer.TaskRuntimeService)
				evtSink := nativebridge.NewNativeEventSinkAdapterWithRouter(services.KernelContainer.EventService, eventSinkRouter)
				services.NativeBridgeRelay.Handler().SetEventSink("android", evtSink)
				services.NativeBridgeRelay.Handler().SetEventSink("ios", evtSink)
			}
		}
	}

	desktopPetWriteGroup := r.Group("/api")
	desktopPetWriteGroup.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:             config.AppCfg.Security.Mode,
		JWTSecret:        config.AppCfg.JWT.Secret,
		JWTIssuer:        config.AppCfg.JWT.Issuer,
		JWTAudience:      config.AppCfg.JWT.Audience,
		LocalCredentials: localCredentialStore,
		LocalUserID:      config.AppCfg.Security.LocalUserID,
		ListenAddress:    config.AppCfg.Server.Host,
		AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
		SessionService:   sessionSvc,
		AccountSessions:  accountSessionRuntime.Validator,
	}))
	desktopPetWriteGroup.Use(readiness.RejectWritesWhenSafeMode(services.SafeMode))
	{
		desktoppet.RegisterDesktopPetWriteRouter(desktopPetWriteGroup, ctx, services.PathRegistry)
		processing.RegisterProcessingRouter(desktopPetWriteGroup, ctx, services.PathRegistry)
		editing.RegisterEditingRouterWithService(desktopPetWriteGroup, services.EditingService, services.OwnershipGuard, services.PathRegistry)
		quality.RegisterQualityRouter(desktopPetWriteGroup, services.QualityService, services.OwnershipGuard)
		installation.RegisterRoutes(desktopPetWriteGroup, services.InstallationCoordinator, services.InstallationRepo, services.OwnershipGuard)
		release.RegisterRoutes(desktopPetWriteGroup, services.NewReleaseService, services.OwnershipGuard)
		registerImportStagingRoutes(desktopPetWriteGroup, services.PathRegistry, services.ImportStagingRepo, services.OwnershipGuard, services.PackageImporter)
		behavior.RegisterRoutes(desktopPetWriteGroup, services.BehaviorService)
	}
	runtimev2.RegisterInternalRoutes(
		r,
		services.DesktopPetRuntimeV2,
		services.SafeMode,
		func(ctx context.Context, rawTicket string, runtimeID runtimeidentity.RuntimeID, deviceID runtimeidentity.DeviceID) (runtimeidentity.UserID, error) {
			ticket, err := bootstrapTicketRepo.ConsumeWithValidation(ctx, rawTicket, string(runtimeID), string(deviceID))
			if err != nil {
				return "", err
			}
			return runtimeidentity.UserID(ticket.UserID), nil
		},
	)
	runtimev2.RegisterUserRoutes(apiGroup, services.DesktopPetRuntimeV2)

	maintenanceAuthGroup := r.Group("/api")
	maintenanceAuthGroup.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:             "maintenance",
		JWTSecret:        config.AppCfg.JWT.Secret,
		LocalCredentials: localCredentialStore,
		LocalUserID:      config.AppCfg.Security.LocalUserID,
		ListenAddress:    config.AppCfg.Server.Host,
		AllowedOrigins:   config.AppCfg.Security.AllowedOrigins,
		AccountSessions:  accountSessionRuntime.Validator,
	}))
	maintenance.RegisterMaintenanceRouter(maintenanceAuthGroup, services.DesktopPetMaintenanceHandler)

	return r, nil
}

type packageImportAdapter struct {
	imp *importer.PackageImporter
}

func (a *packageImportAdapter) ImportPackage(ctx context.Context, req map[string]string) (string, string, string, error) {
	expectedRevision := int64(0)
	if revStr := req["expectedStagingRevision"]; revStr != "" {
		fmt.Sscanf(revStr, "%d", &expectedRevision)
	}
	result, err := a.imp.ImportPackage(ctx, &importer.ImportPackageRequest{
		UserID:                  req["userId"],
		ImportStagingID:         req["importStagingId"],
		SourceFilePath:          req["sourceFilePath"],
		IdempotencyKey:          req["idempotencyKey"],
		ExpectedStagingRevision: expectedRevision,
	})
	if err != nil {
		return "", "", "", err
	}
	return result.PetID, result.ReleaseID, result.OperationID, nil
}

func registerImportStagingRoutes(r *gin.RouterGroup, reg *desktoppetsecurity.PathRootRegistry, repo desktoppetsecurity.ImportStagingRepository, guard desktoppetsecurity.OwnershipGuard, packageImporter *importer.PackageImporter) {
	inspector := importer.NewImportInspector(reg, repo)
	handler := release.NewImportStagingHandler(reg, repo, guard, inspector, &packageImportAdapter{imp: packageImporter})
	g := r.Group("/import-stagings")
	{
		g.POST("/upload", handler.Upload)
		g.GET("/", handler.List)
		g.GET("/:stagingId", handler.Inspect)
		g.POST("/consume", handler.Consume)
		g.POST("/:stagingId/reject", handler.Reject)
	}
}

func generateShutdownOpID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "shutdown_fallback"
	}
	return "sd_" + base64.RawURLEncoding.EncodeToString(b)
}
