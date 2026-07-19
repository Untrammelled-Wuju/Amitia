// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/episodic"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/profile"
	"github.com/u-ai/backend/internal/temporal"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/sse"
	"time"
)

func RegisterSystemRouter(r *gin.RouterGroup, ctx *app.AppContext, chatSvc chat.Service, unifiedEntry *interaction.UnifiedEntry, dataLifecycle *mindruntime.DataLifecycleCoordinator, memSvc memory.Service, profSvc profile.Service, epiSvc episodic.Service, graphSvc graph.Service, temporalSvc *temporal.Service) {
	svc := NewService(ctx)
	handler := NewHandler(svc, ctx.DB, chatSvc, dataLifecycle, unifiedEntry)
	svc.AttachTemporalService(temporalSvc)

	chat.RegisterMessageCommitHook(func(event *chat.MessageCommitEvent) {
		bus := GetMessageEventBus()
		nowStr := time.Now().Format("2006-01-02 15:04:05")
		channel := event.Channel
		if channel == "" {
			channel = "web"
		}
		bus.PublishMessageCreated(event.ConversationID, event.UserMessageID, channel, "inbound", "user", event.UserMessage, nowStr)
		if event.MessagePlan != nil {
			for _, item := range event.MessagePlan.Items {
				bus.PublishMessageCreated(event.ConversationID, item.MessageID, channel, "outbound", "assistant", item.Content, nowStr)
			}
			return
		}
		for i, msgID := range event.MessageIDs {
			content := ""
			if i < len(event.Lines) {
				content = event.Lines[i]
			}
			bus.PublishMessageCreated(event.ConversationID, msgID, channel, "outbound", "assistant", content, nowStr)
		}
	})

	r.GET("/health", handler.Health)
	r.GET("/diagnostics", handler.Diagnostics)
	r.POST("/diagnostics/run", handler.RunDiagnostics)
	r.POST("/tools/route", handler.ToolRoute)

	r.GET("/setup/status", handler.SetupStatus)
	r.GET("/setup/checks", handler.SetupChecks)
	r.POST("/setup/finish", handler.SetupFinish)
	r.POST("/setup/reset", handler.SetupReset)
	r.POST("/setup/step", handler.SetupStep)
	r.GET("/onboarding/status", handler.OnboardingStatus)
	r.POST("/onboarding/complete", handler.OnboardingComplete)
	r.POST("/onboarding/reset", handler.OnboardingReset)

	r.GET("/config", handler.AppConfig)
	r.PUT("/config", handler.UpdateConfig)
	r.GET("/config/mood-detection", handler.MoodDetectionConfig)
	r.PUT("/config/mood-detection", handler.MoodDetectionConfig)
	r.GET("/config/settings", handler.ConfigSettings)
	r.POST("/config/export", handler.ConfigExport)
	r.POST("/config/import/preview", handler.ConfigImportPreview)
	r.POST("/config/import/confirm", handler.ConfigImportConfirm)

	r.GET("/theme", handler.GetTheme)
	r.PUT("/theme", handler.UpdateTheme)
	r.GET("/theme/presets", handler.ThemePresets)

	r.POST("/safety/check-input", handler.CheckInputSafety)
	r.POST("/safety/check-output", handler.CheckOutputSafety)
	r.POST("/safety/check-import", handler.SafetyImportCheck)
	r.GET("/safety/events", handler.SafetyEvents)
	r.PUT("/safety/events/:id/handle", handler.CheckInputSafety)

	r.GET("/auth/current-session", handler.CurrentSession)
	r.GET("/auth/login-history", handler.LoginHistory)
	r.GET("/auth/recovery-codes/status", handler.RecoveryCodesStatus)
	r.POST("/auth/recovery-codes/generate", handler.GenerateRecoveryCodes)
	r.POST("/auth/recovery-codes/verify", handler.VerifyRecoveryCode)
	r.GET("/auth/session-settings", handler.SessionSettings)
	r.PUT("/auth/session-settings", handler.UpdateSessionSettings)

	r.GET("/runtime/health", handler.RuntimeHealth)
	r.GET("/runtime/modules/health", handler.RuntimeModulesHealth)
	r.POST("/runtime/check-db-integrity", handler.CheckDBIntegrity)
	r.POST("/runtime/check-now", handler.CheckNow)
	r.POST("/runtime/cleanup-temp", handler.CleanupTemp)
	r.POST("/runtime/mode/validate", handler.ValidateMode)
	r.POST("/runtime/rotate-logs", handler.RotateLogs)
	r.GET("/runtime/long-running/config", handler.LongRunningConfig)
	r.PUT("/runtime/long-running/config", handler.UpdateLongRunningConfig)
	r.GET("/runtime/long-running/status", handler.LongRunningStatus)
	r.PUT("/runtime/mode", handler.UpdateRuntimeMode)
	r.GET("/runtime/mode", handler.GetRuntimeMode)
	r.GET("/runtime/status", handler.RuntimeStatus)
	r.GET("/runtime/health-history", handler.HealthHistory)

	r.GET("/audit/actions", handler.AuditActions)
	r.GET("/audit/logs", handler.CurrentSession)
	r.DELETE("/audit/logs", handler.CheckNow)
	r.GET("/audit/settings", handler.AuditSettings)
	r.PUT("/audit/settings", handler.UpdateAuditSettings)
	r.GET("/audit/stats", handler.AuditStats)

	r.GET("/wechat/bridge/status", handler.WechatBridgeStatus)
	r.GET("/wechat/bridge/status-detail", handler.WechatBridgeStatusDetail)
	r.GET("/wechat/bridge/config", handler.WechatBridgeConfig)
	r.PUT("/wechat/bridge/config", handler.UpdateWechatBridgeConfig)
	r.GET("/wechat/bridge/events", handler.WechatBridgeEvents)
	r.GET("/wechat/bridge/qrcode", handler.WechatBridgeQRCode)
	r.POST("/wechat/bridge/recover", handler.WechatBridgeRecover)
	r.GET("/qq/bridge/status", handler.QQBridgeStatus)
	r.GET("/qq/bridge/status-detail", handler.QQBridgeStatusDetail)
	r.GET("/qq/bridge/config", handler.QQBridgeConfig)
	r.GET("/qq/bridge/events", handler.QQBridgeEvents)
	r.POST("/qq/bridge/recover", handler.QQBridgeRecover)
	r.POST("/wechat/cloud-check/run", handler.WechatCloudCheckRun)
	r.GET("/wechat/cloud-check", handler.WechatCloudCheck)
	r.GET("/wechat/cloud-check/report", handler.WechatCloudCheckReport)
	r.GET("/wechat/cloud-check/risk-summary", handler.WechatCloudCheckRiskSummary)
	r.POST("/wechat/login/reconnect", handler.WechatLoginReconnect)
	r.POST("/wechat/login/rescan", handler.WechatLoginRescan)
	r.POST("/wechat/login/wait", handler.WechatLoginWait)
	r.GET("/wechat/login/start", handler.WechatLoginStart)
	r.GET("/wechat/status", handler.WechatStatus)
	r.GET("/wechat/events", handler.WechatEvents)
	r.POST("/wechat/reply-timing/recover", handler.WechatReplyTimingRecover)
	r.GET("/wechat/reply-timing/status", handler.WechatReplyTimingStatus)

	r.PUT("/notifications/settings", handler.UpdateNotificationsSettings)
	r.GET("/notifications/status", handler.NotificationsStatus)
	r.POST("/notifications/subscribe", handler.NotificationsSubscribe)
	r.POST("/notifications/test", handler.NotificationsTest)
	r.POST("/notifications/unsubscribe", handler.NotificationsUnsubscribe)

	r.PUT("/security/access-config", handler.UpdateSecurityAccessConfig)
	r.GET("/security/access-status", handler.SecurityAccessStatus)
	r.GET("/security/account-check", handler.SecurityAccountCheck)
	r.GET("/security/exposure-check", handler.SecurityExposureCheck)
	r.GET("/security/status", handler.SecurityStatus)

	r.POST("/privacy/scan", handler.PrivacyScan)
	r.POST("/privacy/mask", handler.PrivacyMask)
	r.GET("/privacy/scan-results", handler.PrivacyScanResultsGet)
	r.DELETE("/privacy/scan-results", handler.PrivacyScanResults)
	r.POST("/privacy/deletion/request", handler.PrivacyDeletionRequest)
	r.GET("/privacy/deletion/status/:id", handler.PrivacyDeletionStatus)
	r.GET("/privacy/deletion/stats", handler.PrivacyDeletionStats)
	r.POST("/privacy/deletion/cleanup", handler.PrivacyDeletionCleanup)
	r.POST("/privacy/deletion/security-tests", handler.PrivacyDeletionSecurityTests)

	r.POST("/update/check", handler.UpdateCheck)
	r.PUT("/update/config", handler.UpdateConfig_Update)
	r.GET("/update/config", handler.GetUpdateConfig)

	r.GET("/version", handler.Version)
	r.GET("/about", handler.About)


	r.POST("/storage/backup", handler.StorageBackup)
	r.POST("/storage/backup/encrypted", handler.StorageBackupEncrypted)
	r.GET("/storage/backups", handler.StorageBackups)
	r.DELETE("/storage/backups/:name", handler.StorageDeleteBackup)
	r.DELETE("/storage/all", handler.StorageDeleteAll)
	r.POST("/storage/restore", handler.StorageRestore)
	r.POST("/storage/restore/encrypted", handler.StorageRestoreEncrypted)
	r.POST("/storage/restore/verify", handler.StorageRestoreVerify)
	r.POST("/storage/export-user-data", handler.StorageExportUserData)
	r.POST("/storage/export-amitia", handler.StorageExportAmitia)
	r.GET("/storage/export-download/:filename", handler.StorageExportDownload)
	r.POST("/storage/import-user-data", handler.StorageImportUserData)
	r.POST("/storage/import-amitia", handler.StorageImportAmitia)
	r.GET("/storage/info", handler.StorageInfo)
	r.GET("/storage/migrations", handler.StorageMigrations)
	r.POST("/storage/migrations/check", handler.StorageMigrationsCheck)

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

	r.GET("/usage/overview", handler.UsageOverview)
	r.GET("/usage/daily", handler.UsageDaily)
	r.GET("/usage/periodic", handler.UsageDaily)
	r.GET("/usage/models", handler.UsageModels)
	r.GET("/usage/sources", handler.UsageSources)
	r.DELETE("/usage/clear", handler.UsageClear)

	r.GET("/logs/recent", handler.LogsRecent)
	r.GET("/logs/recent/errors", handler.LogsRecentErrors)
	r.GET("/logs/files", handler.LogsFiles)
	r.GET("/logs/files/:name", handler.LogsFileContent)
	r.DELETE("/logs", handler.LogsDelete)
	r.GET("/logs/model-errors", handler.LogsModelErrors)
	r.DELETE("/logs/model-errors", handler.LogsDeleteModelErrors)

	r.GET("/maintenance/status", handler.MaintenanceStatus)
	r.POST("/maintenance/diagnose", handler.MaintenanceDiagnose)
	r.POST("/maintenance/export-diagnostic", handler.MaintenanceExportDiagnostic)
	r.POST("/maintenance/reload-config", handler.MaintenanceReloadConfig)
	r.POST("/maintenance/restart-bridge", handler.MaintenanceRestartBridge)
	r.POST("/maintenance/restart-qq-bridge", handler.MaintenanceRestartQQBridge)

	r.GET("/release-check/latest", handler.ReleaseCheckLatest)
	r.GET("/release-check/history", handler.ReleaseCheckHistory)
	r.GET("/release-check/export", handler.ReleaseCheckExport)
	r.POST("/release-check/run", handler.ReleaseCheckRun)

	r.GET("/messages/stream", handler.MessagesStream)
	r.GET("/proactive-sse", sse.SSEHandler)

	r.GET("/messages/events", handler.MessagesEventsStream)



	r.GET("/web-chat/conversations", handler.WebChatListConversations)
	r.POST("/web-chat/conversations", handler.WebChatCreateConv)
	r.GET("/web-chat/conversations/:id/messages", handler.WebChatGetMessages)
	r.DELETE("/web-chat/conversations/:id", handler.WebChatDeleteConv)
	r.PUT("/web-chat/conversations/:id", handler.WebChatUpdateConv)
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
	r.POST("/web-chat/conversations/:id/generations/:generationId/cancel", handler.WebChatCancelGeneration)
	r.POST("/voice/upload", handler.VoiceUpload)
	r.POST("/image/upload", handler.ImageUpload)
	r.POST("/video/upload", handler.VideoUpload)
	r.POST("/voice/transcribe", handler.VoiceTranscribe)

}

func RegisterShadowRouter(r *gin.RouterGroup, handler *Handler) {
	g := r.Group("/shadow")
	g.GET("/status", handler.ShadowModeStatus)
	g.POST("/start", handler.ShadowModeStart)
	g.POST("/stop", handler.ShadowModeStop)
	g.GET("/thresholds", handler.ShadowModeThresholds)
	g.POST("/compare", handler.ShadowModeCompare)
	g.POST("/load-sim", handler.ShadowModeLoadSim)
}
