// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/u-ai/backend/config"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/system/dataportability"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/sse"
)

type SystemRouterOption func(*Handler)

func WithApprovalBroker(broker *execution.ApprovalBroker) SystemRouterOption {
	return func(handler *Handler) {
		handler.SetApprovalBroker(broker)
	}
}

func WithSafetyCoordination(coordinator *coordination.Service) SystemRouterOption {
	return func(handler *Handler) {
		handler.SetSafetyCoordination(coordinator)
	}
}

func RegisterSystemRouter(r *gin.RouterGroup, ctx *app.AppContext, svc Service, chatSvc chat.Service, unifiedEntry *interaction.UnifiedEntry, dataLifecycle *mindruntime.DataLifecycleCoordinator, reconciliation *mindruntime.ReconciliationEngine, memoryConsumer business.MemoryConsumerPort, temporalSvc *temporal.Service, dpCoord *dataportability.Coordinator, artifactSvc *artifact.Service, channelAccess ChannelAvailability, options ...SystemRouterOption) {
	if svc == nil {
		panic("system router requires the canonical system service")
	}
	handler := NewHandler(svc, ctx.DB, chatSvc, dataLifecycle, unifiedEntry, reconciliation, memoryConsumer)
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	handler.SetArtifactService(artifactSvc)
	handler.SetChannelAvailability(channelAccess)
	svc.AttachTemporalService(temporalSvc)

	if dpCoord != nil {
		svc.SetDataPortabilityCoordinator(dpCoord)
	}

	r.GET("/health", handler.Health)
	r.GET("/readyz", handler.Readyz)
	r.GET("/diagnostics", sharedCoreAdminOnly(), handler.Diagnostics)
	r.POST("/diagnostics/run", sharedCoreAdminOnly(), handler.RunDiagnostics)
	r.POST("/tools/route", handler.ToolRoute)

	r.GET("/setup/status", sharedCoreAdminOnly(), handler.SetupStatus)
	r.GET("/setup/checks", sharedCoreAdminOnly(), handler.SetupChecks)
	r.POST("/setup/finish", sharedCoreAdminOnly(), handler.SetupFinish)
	r.POST("/setup/reset", sharedCoreAdminOnly(), handler.SetupReset)
	r.POST("/setup/step", sharedCoreAdminOnly(), handler.SetupStep)
	r.GET("/onboarding/status", sharedCoreAdminOnly(), handler.OnboardingStatus)
	r.POST("/onboarding/complete", sharedCoreAdminOnly(), handler.OnboardingComplete)
	r.POST("/onboarding/reset", sharedCoreAdminOnly(), handler.OnboardingReset)
	r.GET("/runtime/capabilities", handler.RuntimeCapabilities)

	r.GET("/config", sharedCoreAdminOnly(), handler.AppConfig)
	r.PUT("/config", sharedCoreAdminOnly(), handler.UpdateConfig)
	r.GET("/config/mood-detection", sharedCoreAdminOnly(), handler.MoodDetectionConfig)
	r.PUT("/config/mood-detection", sharedCoreAdminOnly(), handler.MoodDetectionConfig)
	r.GET("/config/settings", sharedCoreAdminOnly(), handler.ConfigSettings)
	r.POST("/config/export", sharedCoreAdminOnly(), handler.ConfigExport)
	r.POST("/config/import/preview", sharedCoreAdminOnly(), handler.ConfigImportPreview)
	r.POST("/config/import/confirm", sharedCoreAdminOnly(), handler.ConfigImportConfirm)

	r.GET("/theme", sharedCoreAdminOnly(), handler.GetTheme)
	r.PUT("/theme", sharedCoreAdminOnly(), handler.UpdateTheme)
	r.GET("/theme/presets", sharedCoreAdminOnly(), handler.ThemePresets)

	r.GET("/safety/events", sharedCoreAdminOnly(), handler.SafetyEvents)
	r.DELETE("/safety/events", sharedCoreAdminOnly(), handler.DeleteSafetyEvents)
	r.PUT("/safety/events/:id/handle", sharedCoreAdminOnly(), handler.HandleSafetyEvent)

	r.GET("/runtime/health", sharedCoreAdminOnly(), handler.RuntimeHealth)
	r.GET("/runtime/modules/health", sharedCoreAdminOnly(), handler.RuntimeModulesHealth)
	r.GET("/runtime/debug/snapshot", sharedCoreAdminOnly(), handler.RuntimeDebugSnapshot)
	r.POST("/runtime/check-db-integrity", sharedCoreAdminOnly(), handler.CheckDBIntegrity)
	r.POST("/runtime/check-now", sharedCoreAdminOnly(), handler.CheckNow)
	r.POST("/runtime/cleanup-temp", sharedCoreAdminOnly(), handler.CleanupTemp)
	r.POST("/runtime/rotate-logs", sharedCoreAdminOnly(), handler.RotateLogs)
	r.GET("/runtime/tasks/config", sharedCoreAdminOnly(), handler.TaskMaintenanceConfig)
	r.GET("/runtime/timeout/config", sharedCoreAdminOnly(), handler.TimeoutSettings)
	r.PUT("/runtime/timeout/config", sharedCoreAdminOnly(), handler.UpdateTimeoutSettings)
	r.PUT("/runtime/tasks/config", sharedCoreAdminOnly(), handler.UpdateTaskMaintenanceConfig)
	r.GET("/runtime/tasks/status", sharedCoreAdminOnly(), handler.TaskMaintenanceStatus)
	r.GET("/runtime/status", sharedCoreAdminOnly(), handler.RuntimeStatus)
	r.GET("/runtime/health-history", sharedCoreAdminOnly(), handler.HealthHistory)

	r.GET("/audit/actions", sharedCoreAdminOnly(), handler.AuditActions)
	r.GET("/audit/logs", sharedCoreAdminOnly(), handler.AuditLogs)
	r.DELETE("/audit/logs", sharedCoreAdminOnly(), handler.ClearAuditLogs)
	r.GET("/audit/settings", sharedCoreAdminOnly(), handler.AuditSettings)
	r.PUT("/audit/settings", sharedCoreAdminOnly(), handler.UpdateAuditSettings)
	r.GET("/audit/stats", sharedCoreAdminOnly(), handler.AuditStats)

	r.GET("/search/credentials", sharedCoreAdminOnly(), handler.SearchCredentialList)
	r.PUT("/search/credentials/:engineId", sharedCoreAdminOnly(), handler.SearchCredentialSave)
	r.DELETE("/search/credentials/:engineId", sharedCoreAdminOnly(), handler.SearchCredentialDelete)

	r.GET("/security/access-config", sharedCoreAdminOnly(), handler.SecurityAccessConfig)
	r.PUT("/security/access-config", sharedCoreAdminOnly(), handler.UpdateSecurityAccessConfig)
	r.GET("/security/access-status", sharedCoreAdminOnly(), handler.SecurityAccessStatus)
	r.GET("/security/identity-check", sharedCoreAdminOnly(), handler.SecurityIdentityCheck)
	r.GET("/security/exposure-check", sharedCoreAdminOnly(), handler.SecurityExposureCheck)
	r.GET("/security/status", sharedCoreAdminOnly(), handler.SecurityStatus)

	r.POST("/privacy/scan", handler.PrivacyAuthority(), handler.PrivacyScan)
	r.POST("/privacy/mask", handler.PrivacyAuthority(), handler.PrivacyMask)
	r.GET("/privacy/scan-results", handler.PrivacyAuthority(), handler.PrivacyScanResultsGet)
	r.GET("/privacy/scan-results/:id", handler.PrivacyAuthority(), handler.PrivacyScanResultByID)
	r.DELETE("/privacy/scan-results", handler.PrivacyAuthority(), handler.PrivacyScanResults)
	r.POST("/privacy/deletion/preview", handler.PrivacyAuthority(), handler.PrivacyDeletionPreview)
	r.POST("/privacy/deletion/request", handler.PrivacyAuthority(), handler.PrivacyDeletionRequest)
	r.GET("/privacy/deletion/status/:id", handler.PrivacyAuthority(), handler.PrivacyDeletionStatus)
	r.GET("/privacy/deletion/stats", handler.PrivacyAuthority(), handler.PrivacyDeletionStats)
	r.POST("/privacy/deletion/cleanup", handler.PrivacyAuthority(), handler.PrivacyDeletionCleanup)
	r.POST("/privacy/deletion/security-tests", handler.PrivacyAuthority(), handler.PrivacyDeletionSecurityTests)

	r.POST("/update/check", sharedCoreAdminOnly(), handler.UpdateCheck)
	r.PUT("/update/config", sharedCoreAdminOnly(), handler.UpdateConfig_Update)
	r.GET("/update/config", sharedCoreAdminOnly(), handler.GetUpdateConfig)

	r.GET("/version", handler.Version)
	r.GET("/about", handler.About)

	registerDataPortabilityMigrationRoutes(r, handler)
	if config.AppCfg != nil && config.AppCfg.Security.Mode == "local_single_user" {
		registerStorageStatusRoutes(r, handler)
		registerDataPortabilityBackupRoutes(r, handler)
	}

	if runtimeprofile.CurrentProcessProfile() != runtimeprofile.ProfileCloudCore {
		registerChatImportRoutes(r, handler)
	}

	r.GET("/usage/overview", usageAccess, handler.UsageOverview)
	r.GET("/usage/daily", usageAccess, handler.UsageDaily)
	r.GET("/usage/periodic", usageAccess, handler.UsageDaily)
	r.GET("/usage/models", usageAccess, handler.UsageModels)
	r.GET("/usage/sources", usageAccess, handler.UsageSources)
	r.DELETE("/usage/clear", usageAccess, handler.UsageClear)

	r.GET("/logs/recent", sharedCoreAdminOnly(), handler.LogsRecent)
	r.GET("/logs/recent/errors", sharedCoreAdminOnly(), handler.LogsRecentErrors)
	r.GET("/logs/prompt-traces", sharedCoreAdminOnly(), handler.LogsPromptTraces)
	r.GET("/logs/files", sharedCoreAdminOnly(), handler.LogsFiles)
	r.GET("/logs/files/:name", sharedCoreAdminOnly(), handler.LogsFileContent)
	r.DELETE("/logs", sharedCoreAdminOnly(), handler.LogsDelete)
	r.GET("/logs/model-errors", sharedCoreAdminOnly(), handler.LogsModelErrors)
	r.DELETE("/logs/model-errors", sharedCoreAdminOnly(), handler.LogsDeleteModelErrors)

	r.GET("/maintenance/status", sharedCoreAdminOnly(), handler.MaintenanceStatus)
	r.POST("/maintenance/diagnose", sharedCoreAdminOnly(), handler.MaintenanceDiagnose)
	r.POST("/maintenance/export-diagnostic", sharedCoreAdminOnly(), handler.MaintenanceExportDiagnostic)
	r.POST("/maintenance/reload-config", sharedCoreAdminOnly(), handler.MaintenanceReloadConfig)
	r.GET("/release-check/latest", sharedCoreAdminOnly(), handler.ReleaseCheckLatest)
	r.GET("/release-check/history", sharedCoreAdminOnly(), handler.ReleaseCheckHistory)
	r.GET("/release-check/export", sharedCoreAdminOnly(), handler.ReleaseCheckExport)
	r.POST("/release-check/run", sharedCoreAdminOnly(), handler.ReleaseCheckRun)

	registerMediaAndEventRoutes(r, handler)
	RegisterShadowRouter(r, handler)
}

func registerMediaAndEventRoutes(r *gin.RouterGroup, handler *Handler) {
	r = r.Group("", sharedCoreAdminOnly())
	r.GET("/proactive-sse", sse.SSEHandler)

	r.POST("/voice/upload", handler.VoiceUpload)
	r.POST("/image/upload", handler.ImageUpload)
	r.POST("/video/upload", handler.VideoUpload)
	r.POST("/voice/transcribe", handler.VoiceTranscribe)

}

func RegisterShadowRouter(r *gin.RouterGroup, handler *Handler) {
	g := r.Group("/shadow")
	g.GET("/status", sharedCoreAdminOnly(), handler.ShadowModeStatus)
	g.POST("/start", sharedCoreAdminOnly(), handler.ShadowModeStart)
	g.POST("/stop", sharedCoreAdminOnly(), handler.ShadowModeStop)
	g.POST("/phase/advance", sharedCoreAdminOnly(), handler.ShadowModePhaseAdvance)
	g.GET("/thresholds", sharedCoreAdminOnly(), handler.ShadowModeThresholds)
	g.PUT("/thresholds", sharedCoreAdminOnly(), handler.ShadowModeUpdateThresholds)
	g.POST("/compare", sharedCoreAdminOnly(), handler.ShadowModeCompare)
	g.GET("/rollbacks", sharedCoreAdminOnly(), handler.ShadowModeRollbacks)
	g.POST("/load-sim", sharedCoreAdminOnly(), handler.ShadowModeLoadSim)
	g.POST("/longitudinal-sim", sharedCoreAdminOnly(), handler.ShadowModeLongitudinalSim)
}
