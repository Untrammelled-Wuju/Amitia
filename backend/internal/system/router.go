// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/u-ai/backend/config"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/modelerror"
	"github.com/u-ai/backend/internal/profile"
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

func RegisterSystemRouter(r *gin.RouterGroup, ctx *app.AppContext, chatSvc chat.Service, unifiedEntry *interaction.UnifiedEntry, dataLifecycle *mindruntime.DataLifecycleCoordinator, reconciliation *mindruntime.ReconciliationEngine, memSvc memory.Service, profSvc profile.Service, epiSvc episodic.Service, graphSvc graph.Service, temporalSvc *temporal.Service, dpCoord *dataportability.Coordinator, artifactSvc *artifact.Service, continuityRepo *continuity.Repository, continuityCoordinator *continuity.WaitCoordinator, channelAccess ChannelAvailability, options ...SystemRouterOption) {
	svc := NewService(ctx, runtimeprofile.Profile(""))
	handler := NewHandler(svc, ctx.DB, chatSvc, dataLifecycle, unifiedEntry, reconciliation, memSvc)
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	handler.SetArtifactService(artifactSvc)
	handler.SetContinuityRuntime(continuityRepo, continuityCoordinator)
	handler.SetChannelAvailability(channelAccess)
	svc.AttachTemporalService(temporalSvc)
	chat.SetConversationTitleUpdatedPublisher(func(event chat.ConversationTitleUpdatedEvent) {
		channel := event.Channel
		if channel == "" {
			channel = "web"
		}
		GetMessageEventBus().PublishConversationUpdated(event.ConversationID, channel, map[string]interface{}{
			"title": event.Title,
		})
	})

	if dpCoord != nil {
		svc.SetDataPortabilityCoordinator(dpCoord)
	}

	modelerror.SetReporter(handler.publishModelError)

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

	r.GET("/continuity/threads", handler.ContinuityListThreads)
	r.POST("/continuity/threads", handler.ContinuityCreateThread)
	r.GET("/continuity/threads/:id", handler.ContinuityGetThread)
	r.PATCH("/continuity/threads/:id", handler.ContinuityUpdateThread)
	r.GET("/continuity/threads/:id/events", handler.ContinuityListEvents)
	r.GET("/continuity/threads/:id/waits", handler.ContinuityListWaits)
	r.POST("/continuity/threads/:id/waits", handler.ContinuityCreateWait)
	r.POST("/continuity/threads/:id/waits/:waitId/resolve", handler.ContinuityResolveWait)
	r.POST("/continuity/threads/:id/waits/:waitId/cancel", handler.ContinuityCancelWait)
	r.POST("/continuity/signals", handler.ContinuitySignal)

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

	r.POST("/safety/check-input", handler.CheckInputSafety)
	r.POST("/safety/check-output", handler.CheckOutputSafety)
	r.POST("/safety/check-import", handler.SafetyImportCheck)
	r.GET("/safety/events", sharedCoreAdminOnly(), handler.SafetyEvents)
	r.DELETE("/safety/events", sharedCoreAdminOnly(), handler.DeleteSafetyEvents)
	r.PUT("/safety/events/:id/handle", sharedCoreAdminOnly(), handler.HandleSafetyEvent)

	r.GET("/runtime/health", sharedCoreAdminOnly(), handler.RuntimeHealth)
	r.GET("/runtime/modules/health", sharedCoreAdminOnly(), handler.RuntimeModulesHealth)
	r.GET("/runtime/debug/snapshot", sharedCoreAdminOnly(), handler.RuntimeDebugSnapshot)
	r.POST("/runtime/check-db-integrity", sharedCoreAdminOnly(), handler.CheckDBIntegrity)
	r.POST("/runtime/check-now", sharedCoreAdminOnly(), handler.CheckNow)
	r.POST("/runtime/cleanup-temp", sharedCoreAdminOnly(), handler.CleanupTemp)
	r.POST("/runtime/mode/validate", sharedCoreAdminOnly(), handler.ValidateMode)
	r.POST("/runtime/rotate-logs", sharedCoreAdminOnly(), handler.RotateLogs)
	r.GET("/runtime/long-running/config", sharedCoreAdminOnly(), handler.LongRunningConfig)
	r.PUT("/runtime/long-running/config", sharedCoreAdminOnly(), handler.UpdateLongRunningConfig)
	r.GET("/runtime/long-running/status", sharedCoreAdminOnly(), handler.LongRunningStatus)
	r.PUT("/runtime/mode", sharedCoreAdminOnly(), handler.UpdateRuntimeMode)
	r.GET("/runtime/mode", sharedCoreAdminOnly(), handler.GetRuntimeMode)
	r.GET("/runtime/status", sharedCoreAdminOnly(), handler.RuntimeStatus)
	r.GET("/runtime/health-history", sharedCoreAdminOnly(), handler.HealthHistory)

	r.GET("/audit/actions", sharedCoreAdminOnly(), handler.AuditActions)
	r.GET("/audit/logs", sharedCoreAdminOnly(), handler.AuditLogs)
	r.DELETE("/audit/logs", sharedCoreAdminOnly(), handler.ClearAuditLogs)
	r.GET("/audit/settings", sharedCoreAdminOnly(), handler.AuditSettings)
	r.PUT("/audit/settings", sharedCoreAdminOnly(), handler.UpdateAuditSettings)
	r.GET("/audit/stats", sharedCoreAdminOnly(), handler.AuditStats)

	r.GET("/notifications/settings", handler.NotificationsSettings)
	r.PUT("/notifications/settings", handler.UpdateNotificationsSettings)
	r.GET("/notifications/status", handler.NotificationsStatus)
	r.POST("/notifications/subscribe", handler.NotificationsSubscribe)
	r.POST("/notifications/test", handler.NotificationsTest)
	r.POST("/notifications/unsubscribe", handler.NotificationsUnsubscribe)

	r.GET("/search/credentials", sharedCoreAdminOnly(), handler.SearchCredentialList)
	r.PUT("/search/credentials/:engineId", sharedCoreAdminOnly(), handler.SearchCredentialSave)
	r.DELETE("/search/credentials/:engineId", sharedCoreAdminOnly(), handler.SearchCredentialDelete)

	r.GET("/security/access-config", sharedCoreAdminOnly(), handler.SecurityAccessConfig)
	r.PUT("/security/access-config", sharedCoreAdminOnly(), handler.UpdateSecurityAccessConfig)
	r.GET("/security/access-status", sharedCoreAdminOnly(), handler.SecurityAccessStatus)
	r.GET("/security/identity-check", sharedCoreAdminOnly(), handler.SecurityIdentityCheck)
	r.GET("/security/exposure-check", sharedCoreAdminOnly(), handler.SecurityExposureCheck)
	r.GET("/security/status", sharedCoreAdminOnly(), handler.SecurityStatus)

	r.POST("/privacy/scan", sharedCoreAdminOnly(), handler.PrivacyScan)
	r.POST("/privacy/mask", sharedCoreAdminOnly(), handler.PrivacyMask)
	r.GET("/privacy/scan-results", sharedCoreAdminOnly(), handler.PrivacyScanResultsGet)
	r.GET("/privacy/scan-results/:id", sharedCoreAdminOnly(), handler.PrivacyScanResultByID)
	r.DELETE("/privacy/scan-results", sharedCoreAdminOnly(), handler.PrivacyScanResults)
	r.POST("/privacy/deletion/request", sharedCoreAdminOnly(), handler.PrivacyDeletionRequest)
	r.GET("/privacy/deletion/status/:id", sharedCoreAdminOnly(), handler.PrivacyDeletionStatus)
	r.GET("/privacy/deletion/stats", sharedCoreAdminOnly(), handler.PrivacyDeletionStats)
	r.POST("/privacy/deletion/cleanup", sharedCoreAdminOnly(), handler.PrivacyDeletionCleanup)
	r.POST("/privacy/deletion/security-tests", sharedCoreAdminOnly(), handler.PrivacyDeletionSecurityTests)

	r.POST("/update/check", sharedCoreAdminOnly(), handler.UpdateCheck)
	r.PUT("/update/config", sharedCoreAdminOnly(), handler.UpdateConfig_Update)
	r.GET("/update/config", sharedCoreAdminOnly(), handler.GetUpdateConfig)

	r.GET("/version", handler.Version)
	r.GET("/about", handler.About)

	// Storage backup/restore operates on the physical local Runtime database and
	// filesystem. Never expose it from a shared Cloud Core; cloud clients route
	// /api/storage to their retained Device Agent instead.
	if config.AppCfg != nil && config.AppCfg.Security.Mode == "local_single_user" {
		registerStorageRoutes(r, handler)
	}

	r.GET("/imports/batches", handler.ImportsBatches)
	r.GET("/imports/batches/:id", handler.ImportsBatchDetail)
	r.GET("/imports/batches/:id/summary", handler.ImportsBatchSummary)
	r.GET("/imports/batches/:id/memory-candidates", handler.ImportsBatchMemoryCandidates)
	r.DELETE("/imports/batches/:id", handler.ImportsBatchDelete)
	r.POST("/imports/batches/:id/generate-summary", handler.ImportsBatchGenerateSummary)
	r.POST("/imports/batches/:id/confirm-memories", handler.ImportsBatchConfirmMemories)
	r.POST("/imports/batches/:id/extract-memory-candidates", handler.ExtractImportsMemoryCandidates)
	r.POST("/imports/upload", handler.ImportsUpload)
	r.POST("/imports/parse-text", handler.ImportsParseText)
	r.POST("/imports/confirm", handler.ImportsConfirm)
	r.POST("/import", handler.ImportData)

	r.GET("/usage/overview", sharedCoreAdminOnly(), handler.UsageOverview)
	r.GET("/usage/daily", sharedCoreAdminOnly(), handler.UsageDaily)
	r.GET("/usage/periodic", sharedCoreAdminOnly(), handler.UsageDaily)
	r.GET("/usage/models", sharedCoreAdminOnly(), handler.UsageModels)
	r.GET("/usage/sources", sharedCoreAdminOnly(), handler.UsageSources)
	r.DELETE("/usage/clear", sharedCoreAdminOnly(), handler.UsageClear)

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

	r.GET("/proactive-sse", sse.SSEHandler)

	r.GET("/web-chat/conversations", handler.WebChatListConversations)
	r.GET("/web-chat/conversations/:id", handler.WebChatGetConv)
	r.GET("/web-chat/conversations/:id/turns", handler.WebChatListAssistantTurns)
	r.GET("/web-chat/conversations/:id/snapshot", handler.WebChatConversationSnapshot)
	r.GET("/web-chat/conversations/:id/events", handler.WebChatConversationEvents)
	r.POST("/web-chat/conversations/:id/turns/:turnId/interrupt", handler.WebChatInterruptTurn)
	r.POST("/web-chat/conversations/:id/turns/:turnId/steer", handler.WebChatSteerTurn)
	r.POST("/web-chat/conversations/:id/turns/:turnId/retry", handler.WebChatRetryTurn)
	r.POST("/web-chat/conversations/:id/turns/:turnId/approvals/:approvalId", handler.WebChatResolveTurnApproval)
	r.GET("/web-chat/sidebar", handler.WebChatConversationSidebar)
	r.GET("/web-chat/channels", handler.WebChatListChannelConversations)
	r.POST("/web-chat/projects", handler.WebChatCreateProject)
	r.PATCH("/web-chat/projects/:id", handler.WebChatUpdateProject)
	r.DELETE("/web-chat/projects/:id", handler.WebChatDeleteProject)
	r.GET("/web-chat/projects/:id/location", handler.WebChatProjectLocation)
	r.POST("/web-chat/realtime-conversations", handler.WebChatCreateRealtimeConversation)
	r.GET("/web-chat/conversations/:id/messages", handler.WebChatGetMessages)
	r.DELETE("/web-chat/conversations/:id", handler.WebChatDeleteConv)
	r.PUT("/web-chat/conversations/:id", handler.WebChatUpdateConv)
	r.DELETE("/web-chat/conversations/:id/messages", handler.WebChatDeleteConvMessages)
	r.PUT("/web-chat/messages/:id", handler.WebChatUpdateMessage)
	r.POST("/web-chat/messages", handler.WebChatSubmitMessage)
	r.POST("/web-chat/conversations/from-import", handler.WebChatFromImport)
	r.POST("/voice/upload", handler.VoiceUpload)
	r.POST("/image/upload", handler.ImageUpload)
	r.POST("/video/upload", handler.VideoUpload)
	r.POST("/voice/transcribe", handler.VoiceTranscribe)

	RegisterShadowRouter(r, handler)
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
