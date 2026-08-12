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
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/management"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/mcpapi"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/middleware"
	"github.com/u-ai/backend/internal/middleware/security"
	"github.com/u-ai/backend/internal/mood"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/realtime"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/internal/user"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/worldbook"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
)

func setupRouter(ctx *app.AppContext, services *AppServices) (*gin.Engine, error) {
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

	systemSvc := system.NewService(ctx)
	systemHandler := system.NewHandler(systemSvc, ctx.DB, services.Chat, services.DataLifecycle, services.UnifiedEntry, services.Reconciliation)

	public := r.Group("/api/public")
	{
		userRepo := user.NewRepository(ctx)
		userSvc := user.NewService(userRepo, ctx)
		userHandler := user.NewHandler(userSvc)

		public.GET("/auth/status", userHandler.Status)
		public.POST("/auth/setup", userHandler.Setup)
		public.POST("/auth/login", userHandler.Login)

		public.GET("/onboarding/status", systemHandler.OnboardingStatus)
		public.POST("/onboarding/complete", systemHandler.OnboardingComplete)
	}

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
					DeviceID:          request.DeviceID,
					DesktopInstanceID: request.DesktopInstanceID,
					Platform:          request.Platform,
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
				if err := services.DeviceRepository.RequireOwned(c.Request.Context(), actor.UserID, deviceID); err != nil {
					c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "device not found"})
					return
				}
				rawTicket, ticket, err := bootstrapTicketRepo.Create(c.Request.Context(), actor.UserID, deviceID, runtimeID, 10*time.Minute)
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
				affected, err := bootstrapTicketRepo.RevokeDeviceTickets(c.Request.Context(), actor.UserID, deviceID)
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
	}))
	{
		user.RegisterUserRouter(apiGroup, ctx)
		character.RegisterCharacterRouter(apiGroup, ctx, services.Chat)
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
		system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.DataLifecycle, services.Reconciliation, services.Memory, services.Profile, services.Episodic, services.Graph, services.Temporal)
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
		if services.KernelContainer != nil {
			cardProvider := extension_center.NewKernelCardProvider(services.KernelContainer.DefinitionRepository, services.KernelContainer.InstallationRepository)
			centerSvc := extension_center.NewCenterService(cardProvider)
			centerHandler := extension_center.NewHTTPHandler(centerSvc)
			centerMux := http.NewServeMux()
			centerHandler.Register(centerMux)
			apiGroup.Any("/api/extension-center/*centerPath", gin.WrapH(centerMux))
		}
		if services.KernelContainer != nil && services.KernelContainer.WASMRuntimeFactory != nil {
			wasmService := wasm_runtime.NewAPIService(services.KernelContainer.WASMRuntimeFactory, services.KernelContainer.WASMDefinitionRepo)
			wasmHandler := wasm_runtime.NewHTTPHandler(wasmService)
			wasmMux := http.NewServeMux()
			wasmHandler.Register(wasmMux)
			apiGroup.Any("/wasm/*wasmPath", gin.WrapH(wasmMux))
		}
	if services.KernelContainer != nil && services.KernelContainer.GameHost != nil {
		kernelReader := management.NewKernelReaderWithContributions(services.KernelContainer.DefinitionRepository, services.KernelContainer.InstallationRepository, services.KernelContainer.ContributionRepository)
		gameCenterSvc := management.NewProductionService(services.KernelContainer.GameHost, kernelReader)
		management.RegisterGameCenterRouter(apiGroup, gameCenterSvc)

		if services.Extension != nil {
			kernelMutation := management.NewKernelMutationFromFuncs(management.KernelMutationOptions{
				InstallFn: func(ctx context.Context, archivePath string) (management.KernelInstalledExtension, error) {
					installed, err := services.Extension.Kernel.Install(ctx, archivePath)
					if err != nil {
						return management.KernelInstalledExtension{}, err
					}
					return management.KernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
				},
				UpdateFn: func(ctx context.Context, archivePath string) (management.KernelInstalledExtension, error) {
					installed, err := services.Extension.Kernel.Update(ctx, archivePath)
					if err != nil {
						return management.KernelInstalledExtension{}, err
					}
					return management.KernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
				},
				EnableFn:    func(ctx context.Context, extensionID string) error { return services.Extension.Kernel.Enable(ctx, extensionID) },
				DisableFn:   func(ctx context.Context, extensionID string) error { return services.Extension.Kernel.Disable(ctx, extensionID) },
				UninstallFn: func(ctx context.Context, extensionID string) error { return services.Extension.Kernel.Uninstall(ctx, extensionID) },
			})
			packageSvc := management.NewProductionPackageMutationServiceFromKernelReader(kernelReader, management.NewGameHostPluginRegistryFromContainer(services.KernelContainer.GameHost), kernelMutation)
			runtimeSvc := management.NewProductionRuntimeMutationService(services.KernelContainer.GameHost, nil)
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
					EmergencyStopFn: func(ctx context.Context, runtimeID string) error {
						return nil
					},
				})
				management.RegisterGameCenterControlRouter(apiGroup, controlHandler)
			}
		}
	}
	emote.RegisterRouter(apiGroup, services.Emote)
		mcpapi.RegisterRouter(apiGroup, ctx, mcpapi.Services{Repository: services.MCPRepository, Connections: services.MCPConnections, Auth: services.MCPAuth, Discovery: services.MCPDiscovery, Skills: services.MCPSkills, Secrets: services.MCPSecrets, Extensions: services.Extension, Features: services.MCPFeatures, Dependencies: services.MCPDependencies, Interactions: services.MCPInteractions})
		temporal.RegisterRouter(apiGroup, services.Temporal, services.RelTimeCoordinator)
		mood.RegisterMoodRouter(apiGroup, ctx)
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
		func(ctx context.Context, rawTicket, runtimeID, deviceID string) (string, error) {
			ticket, err := bootstrapTicketRepo.ConsumeWithValidation(ctx, rawTicket, runtimeID, deviceID)
			if err != nil {
				return "", err
			}
			return ticket.UserID, nil
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
