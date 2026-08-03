// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
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
	"github.com/u-ai/backend/internal/desktoppet/doctor"
	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/internal/desktoppet/readiness"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/runtime"
	"github.com/u-ai/backend/internal/embedding_config"
	"github.com/u-ai/backend/internal/emote"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/feedback"
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

func setupRouter(ctx *app.AppContext, services *AppServices) *gin.Engine {
	if config.AppCfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(middleware.TraceMiddleware())
	r.Use(security.CorsMiddleware(security.CorsConfig{
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))

	resolveLocalToken := func() string {
		if config.AppCfg.Security.LocalToken != "" {
			return config.AppCfg.Security.LocalToken
		}
		if config.AppCfg.Security.LocalTokenFile != "" {
			data, err := os.ReadFile(config.AppCfg.Security.LocalTokenFile)
			if err == nil {
				return strings.TrimSpace(string(data))
			}
		}
		return ""
	}

	r.GET("/livez", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "alive"})
	})

	readinessSvc := readiness.NewStartupReadinessService(ctx.DB, services.Extension)

	r.GET("/readyz", func(c *gin.Context) {
		if readinessSvc == nil {
			c.JSON(503, gin.H{"code": 503, "msg": "blocked", "data": gin.H{"status": "blocked", "reason": "readiness service not initialized"}})
			return
		}
		snapshot := readinessSvc.Snapshot()
		result := gin.H{
			"status":        snapshot.OverallStatus,
			"blockingCount": snapshot.BlockingCount,
			"degradedCount": snapshot.DegradedCount,
			"timestamp":     snapshot.Timestamp,
		}
		httpStatus := 200
		if snapshot.OverallStatus == readiness.StatusBlocked {
			httpStatus = 503
		}
		c.JSON(httpStatus, gin.H{"code": httpStatus, "msg": string(snapshot.OverallStatus), "data": result})
	})


	public := r.Group("/api/public")
	{
		userRepo := user.NewRepository(ctx)
		userSvc := user.NewService(userRepo, ctx)
		userHandler := user.NewHandler(userSvc)

		public.GET("/auth/status", userHandler.Status)
		public.POST("/auth/setup", userHandler.Setup)
		public.POST("/auth/login", userHandler.Login)
	}

	sessionSvc, err := security.NewDesktopSessionService(ctx.DB, config.AppCfg.Storage.DataDir)
	if err != nil {
		log.Error("failed to init session service", "error", err)
	}

	bootstrapTicketRepo := runtime.NewBootstrapTicketRepository(ctx.DB)
	services.DesktopPetRuntime.SetBootstrapTicketRepo(bootstrapTicketRepo)

	local := r.Group("/api/local")
	local.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:           config.AppCfg.Security.Mode,
		JWTSecret:      config.AppCfg.JWT.Secret,
		JWTIssuer:      config.AppCfg.JWT.Issuer,
		JWTAudience:    config.AppCfg.JWT.Audience,
		LocalToken:     resolveLocalToken(),
		LocalUserID:    config.AppCfg.Security.LocalUserID,
		ListenAddress:  config.AppCfg.Server.Host,
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))
	{
		local.POST("/sessions", func(c *gin.Context) {
			if sessionSvc == nil {
				c.JSON(500, gin.H{"code": 500, "msg": "session service not available"})
				return
			}
			sessionSvc.CreateSession(c)
		})
		local.POST("/token/rotate", func(c *gin.Context) {
			if sessionSvc == nil {
				c.JSON(500, gin.H{"code": 500, "msg": "session service not available"})
				return
			}
			sessionSvc.RotateToken(c)
		})
		local.POST("/devices/:deviceId/runtime-bootstrap-tickets", func(c *gin.Context) {
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
			rawTicket, ticket, err := bootstrapTicketRepo.Create(c.Request.Context(), actor.UserID, deviceID, "", 10*time.Minute)
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
					"deviceId":   ticket.DeviceID,
					"expiresAt":  ticket.ExpiresAt,
					"ttlSeconds": 600,
				},
			})
		})
		local.DELETE("/devices/:deviceId/runtime-bootstrap-tickets", func(c *gin.Context) {
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

	localAdmin := r.Group("/api/local/admin")
	localAdmin.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:           config.AppCfg.Security.Mode,
		JWTSecret:      config.AppCfg.JWT.Secret,
		JWTIssuer:      config.AppCfg.JWT.Issuer,
		JWTAudience:    config.AppCfg.JWT.Audience,
		LocalToken:     resolveLocalToken(),
		LocalUserID:    config.AppCfg.Security.LocalUserID,
		ListenAddress:  config.AppCfg.Server.Host,
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))
	localAdmin.Use(security.RequirePermission("system.shutdown"))
	localAdmin.POST("/shutdown", func(c *gin.Context) {
		actor := security.GetActor(c)
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

	apiGroup := r.Group("/api")
	apiGroup.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:           config.AppCfg.Security.Mode,
		JWTSecret:      config.AppCfg.JWT.Secret,
		JWTIssuer:      config.AppCfg.JWT.Issuer,
		JWTAudience:    config.AppCfg.JWT.Audience,
		LocalToken:     resolveLocalToken(),
		LocalUserID:    config.AppCfg.Security.LocalUserID,
		ListenAddress:  config.AppCfg.Server.Host,
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
		SessionService: sessionSvc,
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
		graph.RegisterGraphRouter(apiGroup, config.AppCfg.Surreal)
		agent.RegisterAgentRouter(apiGroup, ctx, services.UnifiedEntry)
		system.RegisterSystemRouter(apiGroup, ctx, services.Chat, services.UnifiedEntry, services.DataLifecycle, services.Memory, services.Profile, services.Episodic, services.Graph, services.Temporal)
		companion.RegisterCompanionRouter(apiGroup, services.Companion)
		qq.RegisterQQRouter(apiGroup, ctx)
		tts.RegisterTtsRouter(apiGroup, ctx)
		asr.RegisterAsrRouter(apiGroup, ctx)
		realtime.RegisterRealtimeRouter(apiGroup, ctx)
		vision.RegisterVisionRouter(apiGroup, ctx)
		embedding_config.RegisterEmbeddingConfigRouter(apiGroup, ctx)
		imagegen.RegisterImageGenRouter(apiGroup, ctx)
		desktoppet.RegisterDesktopPetRouter(apiGroup, ctx)
		doctor.RegisterRouter(apiGroup, ctx.DB, services.Extension)
		readiness.RegisterRouter(apiGroup, ctx.DB, services.Extension)
		processing.RegisterProcessingRouter(apiGroup, ctx)
		editing.RegisterEditingRouterWithService(apiGroup, services.EditingService, services.OwnershipGuard)
		quality.RegisterQualityRouter(apiGroup, services.QualityService, services.OwnershipGuard)
		installation.RegisterRoutes(apiGroup, services.InstallationService, services.OwnershipGuard)
		release.RegisterRoutes(apiGroup, services.NewReleaseService, services.OwnershipGuard)
		behavior.RegisterRoutes(apiGroup, services.BehaviorService)
		system.RegisterPsycheAPIRouter(apiGroup)
		system.RegisterPsycheSnapshotRouter(apiGroup, ctx.DB)
		system.RegisterHealthRouter(apiGroup, services.CircuitBreakers, services.DataLifecycle, services.Reconciliation)
		ttsRepo := tts.NewRepository(ctx.DB)
		ttsSvc := tts.NewService(ttsRepo)
		system.RegisterVoiceEntryRouter(apiGroup, services.VoiceEntry, ttsSvc, services.DeliveryStore)
		safety.RegisterSafetyRouter(apiGroup, ctx.DB)
		delivery.RegisterSubmitRouter(apiGroup, services.DeliveryStore)
		extension.RegisterRouter(apiGroup, ctx, services.Extension)
		if services.KernelContainer != nil && services.KernelContainer.WASMRuntimeFactory != nil {
			wasmService := wasm_runtime.NewAPIService(services.KernelContainer.WASMRuntimeFactory, services.KernelContainer.WASMDefinitionRepo)
			wasmHandler := wasm_runtime.NewHTTPHandler(wasmService)
			wasmMux := http.NewServeMux()
			wasmHandler.Register(wasmMux)
			apiGroup.Any("/wasm/*wasmPath", gin.WrapH(wasmMux))
		}
		emote.RegisterRouter(apiGroup, services.Emote)
		mcpapi.RegisterRouter(apiGroup, ctx, mcpapi.Services{Repository: services.MCPRepository, Connections: services.MCPConnections, Auth: services.MCPAuth, Discovery: services.MCPDiscovery, Skills: services.MCPSkills, Secrets: services.MCPSecrets, Extensions: services.Extension, Features: services.MCPFeatures, Dependencies: services.MCPDependencies, Interactions: services.MCPInteractions})
		temporal.RegisterRouter(apiGroup, services.Temporal, services.RelTimeCoordinator)
		mood.RegisterMoodRouter(apiGroup, ctx)
	}
	runtime.RegisterInternalRoutes(r, services.DesktopPetRuntime)
	runtime.RegisterUserRoutes(apiGroup, services.DesktopPetRuntime)

	maintenance := r.Group("/api/maintenance")
	maintenance.Use(security.AuthenticationMiddleware(security.AuthConfig{
		Mode:           "maintenance",
		JWTSecret:      config.AppCfg.JWT.Secret,
		LocalToken:     resolveLocalToken(),
		LocalUserID:    config.AppCfg.Security.LocalUserID,
		ListenAddress:  config.AppCfg.Server.Host,
		AllowedOrigins: config.AppCfg.Security.AllowedOrigins,
	}))
	{
		maintenance.GET("/doctor", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "msg": "doctor not implemented"})
		})
		maintenance.GET("/readiness", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "msg": "readiness not implemented"})
		})
		maintenance.GET("/migrations", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "msg": "migrations not implemented"})
		})
		maintenance.POST("/backup", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "msg": "backup not implemented"})
		})
		maintenance.POST("/export", func(c *gin.Context) {
			c.JSON(200, gin.H{"code": 200, "msg": "export not implemented"})
		})
	}

	return r
}

func generateShutdownOpID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "shutdown_fallback"
	}
	return "sd_" + base64.RawURLEncoding.EncodeToString(b)
}
