// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"time"

	"github.com/u-ai/backend/config"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/episodic"
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

func RegisterSystemRouter(r *gin.RouterGroup, ctx *app.AppContext, chatSvc chat.Service, unifiedEntry *interaction.UnifiedEntry, dataLifecycle *mindruntime.DataLifecycleCoordinator, reconciliation *mindruntime.ReconciliationEngine, memSvc memory.Service, profSvc profile.Service, epiSvc episodic.Service, graphSvc graph.Service, temporalSvc *temporal.Service, dpCoord *dataportability.Coordinator, artifactSvc *artifact.Service) {
	svc := NewService(ctx, runtimeprofile.Profile(""))
	handler := NewHandler(svc, ctx.DB, chatSvc, dataLifecycle, unifiedEntry, reconciliation, memSvc)
	handler.SetArtifactService(artifactSvc)
	svc.AttachTemporalService(temporalSvc)

	if dpCoord != nil {
		svc.SetDataPortabilityCoordinator(dpCoord)
	}

	modelerror.SetReporter(handler.publishModelError)

	chat.RegisterMessageCommitHook(func(event *chat.MessageCommitEvent) {
		bus := GetMessageEventBus()
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		channel := event.Channel
		if channel == "" {
			channel = "web"
		}
		if !event.IsInternal {
			metadata := map[string]interface{}{
				"userMessageId":       event.UserMessageID,
				"userMessageSequence": event.UserMessageSequence,
				"requestId":           event.RequestID,
			}
			if event.MessagePlan != nil {
				for _, item := range event.MessagePlan.Items {
					bus.PublishMessageCreated(event.ConversationID, item.MessageID, channel, "outbound", "assistant", item.Content, nowStr, event.Sequences[item.MessageID], metadata)
				}
				return
			}
			for i, msgID := range event.MessageIDs {
				content := ""
				if i < len(event.Lines) {
					content = event.Lines[i]
				}
				bus.PublishMessageCreated(event.ConversationID, msgID, channel, "outbound", "assistant", content, nowStr, event.Sequences[msgID], metadata)
			}
		}
	})

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

	r.GET("/wechat/bridge/status", sharedCoreAdminOnly(), handler.WechatBridgeStatus)
	r.GET("/wechat/bridge/status-detail", sharedCoreAdminOnly(), handler.WechatBridgeStatusDetail)
	r.GET("/wechat/bridge/config", sharedCoreAdminOnly(), handler.WechatBridgeConfig)
	r.PUT("/wechat/bridge/config", sharedCoreAdminOnly(), handler.UpdateWechatBridgeConfig)
	r.GET("/wechat/bridge/events", sharedCoreAdminOnly(), handler.WechatBridgeEvents)
	r.GET("/wechat/bridge/qrcode", sharedCoreAdminOnly(), handler.WechatBridgeQRCode)
	r.POST("/wechat/bridge/recover", sharedCoreAdminOnly(), handler.WechatBridgeRecover)
	r.GET("/qq/bridge/status", sharedCoreAdminOnly(), handler.QQBridgeStatus)
	r.GET("/qq/bridge/status-detail", sharedCoreAdminOnly(), handler.QQBridgeStatusDetail)
	r.GET("/qq/bridge/config", sharedCoreAdminOnly(), handler.QQBridgeConfig)
	r.GET("/qq/bridge/events", sharedCoreAdminOnly(), handler.QQBridgeEvents)
	r.POST("/qq/bridge/recover", sharedCoreAdminOnly(), handler.QQBridgeRecover)
	r.POST("/wechat/cloud-check/run", sharedCoreAdminOnly(), handler.WechatCloudCheckRun)
	r.GET("/wechat/cloud-check", sharedCoreAdminOnly(), handler.WechatCloudCheck)
	r.GET("/wechat/cloud-check/report", sharedCoreAdminOnly(), handler.WechatCloudCheckReport)
	r.GET("/wechat/cloud-check/risk-summary", sharedCoreAdminOnly(), handler.WechatCloudCheckRiskSummary)
	r.POST("/wechat/login/reconnect", sharedCoreAdminOnly(), handler.WechatLoginReconnect)
	r.POST("/wechat/login/rescan", sharedCoreAdminOnly(), handler.WechatLoginRescan)
	r.POST("/wechat/login/wait", sharedCoreAdminOnly(), handler.WechatLoginWait)
	r.GET("/wechat/login/start", sharedCoreAdminOnly(), handler.WechatLoginStart)
	r.GET("/wechat/status", sharedCoreAdminOnly(), handler.WechatStatus)
	r.GET("/wechat/events", sharedCoreAdminOnly(), handler.WechatEvents)
	r.POST("/wechat/reply-timing/recover", sharedCoreAdminOnly(), handler.WechatReplyTimingRecover)
	r.GET("/wechat/reply-timing/status", sharedCoreAdminOnly(), handler.WechatReplyTimingStatus)

	r.GET("/notifications/settings", handler.NotificationsSettings)
	r.PUT("/notifications/settings", handler.UpdateNotificationsSettings)
	r.GET("/notifications/status", handler.NotificationsStatus)
	r.POST("/notifications/subscribe", handler.NotificationsSubscribe)
	r.POST("/notifications/test", handler.NotificationsTest)
	r.POST("/notifications/unsubscribe", handler.NotificationsUnsubscribe)

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
	r.POST("/maintenance/restart-bridge", sharedCoreAdminOnly(), handler.MaintenanceRestartBridge)
	r.POST("/maintenance/restart-qq-bridge", sharedCoreAdminOnly(), handler.MaintenanceRestartQQBridge)

	r.GET("/release-check/latest", sharedCoreAdminOnly(), handler.ReleaseCheckLatest)
	r.GET("/release-check/history", sharedCoreAdminOnly(), handler.ReleaseCheckHistory)
	r.GET("/release-check/export", sharedCoreAdminOnly(), handler.ReleaseCheckExport)
	r.POST("/release-check/run", sharedCoreAdminOnly(), handler.ReleaseCheckRun)

	r.GET("/messages/stream", handler.MessagesStream)
	r.GET("/proactive-sse", sse.SSEHandler)

	r.GET("/messages/events", handler.MessagesEventsStream)

	r.GET("/web-chat/conversations", handler.WebChatListConversations)
	r.POST("/web-chat/conversations", handler.WebChatCreateConv)
	r.GET("/web-chat/conversations/:id/messages", handler.WebChatGetMessages)
	r.DELETE("/web-chat/conversations/:id", handler.WebChatDeleteConv)
	r.PUT("/web-chat/conversations/:id", handler.WebChatUpdateConv)
	r.GET("/web-chat/conversations/:id/workspace", handler.WebChatGetWorkspace)
	r.PUT("/web-chat/conversations/:id/workspace", handler.WebChatSetWorkspace)
	r.DELETE("/web-chat/conversations/:id/workspace", handler.WebChatClearWorkspace)
	r.DELETE("/web-chat/conversations/:id/messages", handler.WebChatDeleteConvMessages)
	r.POST("/web-chat/conversations/:id/regenerate", handler.WebChatRegenerate)
	r.POST("/web-chat/conversations/:id/reply-timing/force", handler.WebChatReplyTimingForce)
	r.POST("/web-chat/conversations/:id/reply-timing/hold", handler.WebChatReplyTimingHold)
	r.POST("/web-chat/conversations/:id/reply-timing/resume", handler.WebChatReplyTimingResume)
	r.GET("/web-chat/conversations/:id/reply-timing/status", handler.WebChatReplyTimingStatus)
	r.GET("/web-chat/message-status/:id", handler.WebChatMessageStatus)
	r.POST("/web-chat/send", handler.WebChatSend)
	r.POST("/web-chat/messages", handler.WebChatSubmitMessage)
	r.POST("/web-chat/send-stream", handler.WebChatSendStream)
	r.POST("/web-chat/conversations/from-import", handler.WebChatFromImport)
	r.GET("/web-chat/conversations/:id/generations/current/status", handler.WebChatGenerationStatus)
	r.POST("/web-chat/conversations/:id/generations/current/cancel", handler.WebChatCancelGeneration)
	r.POST("/web-chat/conversations/:id/generations/:generationId/cancel", handler.WebChatCancelGeneration)
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
