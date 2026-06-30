// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"fmt"
	applog "github.com/u-ai/backend/log"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/tts"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
	"gorm.io/gorm"
	"strconv"
)

// SystemFormatInstruction is injected into every LLM call to enforce WeChat-style line splitting.
// It is NOT part of any character prompt and cannot be modified per character.
const SystemFormatInstruction = `【回复格式 - 系统固定规则】

每句话必须单独一行，用换行符分隔。
每句话尽量短，像微信连续消息一样。
能一句说完就一句，不要写长段落。
不要把多句话连成一段。
不要用句号连接多个意思。`

type Handler struct {
	service     Service
	db          *gorm.DB
	chatSvc     chat.Service
	versionInfo atomic.Value
}

func NewHandler(srv Service, db *gorm.DB, chatSvc chat.Service) *Handler {
	h := &Handler{service: srv, db: db, chatSvc: chatSvc}
	h.versionInfo.Store(srv.GetVersion())
	return h
}

// ============= 核心健康/诊断 =============

func (h *Handler) Health(c *gin.Context)         { util.SuccessResponse(c, h.service.Health()) }
func (h *Handler) Diagnostics(c *gin.Context)    { util.SuccessResponse(c, h.service.Diagnostics()) }
func (h *Handler) RunDiagnostics(c *gin.Context) { util.SuccessResponse(c, h.service.RunDiagnostics()) }
func (h *Handler) ToolRoute(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ToolRoute(body))
}
func (h *Handler) AppConfig(c *gin.Context) { util.SuccessResponse(c, h.service.AppConfig()) }
func (h *Handler) UpdateConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateAppConfig(body))
}
func (h *Handler) ConfigSettings(c *gin.Context) { util.SuccessResponse(c, h.service.ConfigSettings()) }
func (h *Handler) ConfigExport(c *gin.Context)   { util.SuccessResponse(c, h.service.ConfigExport()) }
func (h *Handler) ConfigImportPreview(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ImportData(body))
}
func (h *Handler) ConfigImportConfirm(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ConfirmImports(body))
}
func (h *Handler) GetLLMConfig(c *gin.Context) { util.SuccessResponse(c, h.service.GetLLMConfig()) }
func (h *Handler) UpdateLLMConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateLLMConfig(body))
}
func (h *Handler) MoodDetectionConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.MoodDetectionConfig())
}
func (h *Handler) Version(c *gin.Context) { util.SuccessResponse(c, h.service.GetVersion()) }
func (h *Handler) About(c *gin.Context)   { util.SuccessResponse(c, h.service.GetAbout()) }

// ============= 设置/引导 =============

func (h *Handler) SetupStatus(c *gin.Context) { util.SuccessResponse(c, h.service.SetupStatus()) }
func (h *Handler) SetupChecks(c *gin.Context) { util.SuccessResponse(c, h.service.SetupChecks()) }
func (h *Handler) SetupFinish(c *gin.Context) { util.SuccessResponse(c, h.service.SetupFinish()) }
func (h *Handler) SetupReset(c *gin.Context)  { util.SuccessResponse(c, h.service.SetupReset()) }
func (h *Handler) SetupStep(c *gin.Context) {
	var body struct {
		Step string `json:"step"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.SetupStep(body.Step))
}
func (h *Handler) OnboardingStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingStatus())
}
func (h *Handler) OnboardingComplete(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingComplete())
}
func (h *Handler) OnboardingReset(c *gin.Context) {
	util.SuccessResponse(c, h.service.OnboardingReset())
}

// ============= 主题 =============

func (h *Handler) GetTheme(c *gin.Context) { util.SuccessResponse(c, h.service.GetTheme()) }
func (h *Handler) UpdateTheme(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateTheme(body))
}
func (h *Handler) ThemePresets(c *gin.Context) { util.SuccessResponse(c, h.service.GetThemePresets()) }

// ============= 安全 =============

func (h *Handler) CheckInputSafety(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.CheckSafety(body.Text))
}
func (h *Handler) CheckOutputSafety(c *gin.Context) {
	var body struct {
		Text string `json:"text"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.CheckSafety(body.Text))
}
func (h *Handler) SafetyImportCheck(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.SafetyImportCheck(body))
}
func (h *Handler) SafetyEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	util.SuccessResponse(c, h.service.SafetyEvents(page, pageSize))
}

// ============= 会话 =============

func (h *Handler) CurrentSession(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetCurrentSession(c.GetHeader("Authorization")))
}
func (h *Handler) LoginHistory(c *gin.Context) { util.SuccessResponse(c, h.service.GetLoginHistory()) }
func (h *Handler) RecoveryCodesStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRecoveryCodesStatus())
}
func (h *Handler) GenerateRecoveryCodes(c *gin.Context) {
	util.SuccessResponse(c, h.service.GenerateRecoveryCodes())
}
func (h *Handler) VerifyRecoveryCode(c *gin.Context) {
	var body struct {
		Code string `json:"code"`
	}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.VerifyRecoveryCode(body.Code))
}
func (h *Handler) SessionSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSessionSettings())
}
func (h *Handler) UpdateSessionSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateSessionSettings(body))
}

// ============= 运行时 =============

func (h *Handler) RuntimeHealth(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeHealth())
}
func (h *Handler) HealthHistory(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeHealthHistory())
}
func (h *Handler) RuntimeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetRuntimeStatus())
}
func (h *Handler) CheckDBIntegrity(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckDBIntegrity())
}
func (h *Handler) CheckNow(c *gin.Context)     { util.SuccessResponse(c, h.service.RunNow()) }
func (h *Handler) CleanupTemp(c *gin.Context)  { util.SuccessResponse(c, h.service.CleanupTemp()) }
func (h *Handler) ValidateMode(c *gin.Context) { util.SuccessResponse(c, h.service.ValidateMode()) }
func (h *Handler) RotateLogs(c *gin.Context)   { util.SuccessResponse(c, h.service.RotateLogs()) }
func (h *Handler) LongRunningConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLongRunningConfig())
}
func (h *Handler) UpdateLongRunningConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateLongRunningConfig(body))
}
func (h *Handler) LongRunningStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLongRunningStatus())
}
func (h *Handler) GetRuntimeMode(c *gin.Context) { util.SuccessResponse(c, h.service.GetRuntimeMode()) }
func (h *Handler) UpdateRuntimeMode(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateRuntimeMode(body))
}

// ============= 审计 =============

func (h *Handler) AuditActions(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditActions()) }
func (h *Handler) AuditSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetAuditSettings())
}
func (h *Handler) UpdateAuditSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateAuditSettings(body))
}
func (h *Handler) AuditStats(c *gin.Context) { util.SuccessResponse(c, h.service.GetAuditStats()) }

// ============= 微信 =============

func (h *Handler) WechatBridgeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeStatus())
}
func (h *Handler) WechatBridgeStatusDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeStatusDetail())
}
func (h *Handler) WechatBridgeConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeConfig())
}
func (h *Handler) UpdateWechatBridgeConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateWechatBridgeConfig(body))
}
func (h *Handler) WechatBridgeEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeEvents())
}
func (h *Handler) WechatBridgeQRCode(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetWechatBridgeQRCode())
}
func (h *Handler) QQBridgeStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeStatus())
}
func (h *Handler) QQBridgeStatusDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeStatusDetail())
}
func (h *Handler) QQBridgeConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeConfig())
}
func (h *Handler) QQBridgeEvents(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetQQBridgeEvents())
}
func (h *Handler) QQBridgeRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.QQBridgeRecover())
}
func (h *Handler) MaintenanceRestartQQBridge(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceRestartQQBridge())
}

func (h *Handler) WechatBridgeRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatBridgeRecover())
}
func (h *Handler) WechatCloudCheckRun(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckRun())
}
func (h *Handler) WechatCloudCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheck())
}
func (h *Handler) WechatCloudCheckReport(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckReport())
}
func (h *Handler) WechatCloudCheckRiskSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatCloudCheckRiskSummary())
}
func (h *Handler) WechatLoginReconnect(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginReconnect())
}
func (h *Handler) WechatLoginRescan(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginRescan())
}
func (h *Handler) WechatLoginStart(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginStart())
}
func (h *Handler) WechatLoginWait(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatLoginWait())
}
func (h *Handler) WechatStatus(c *gin.Context) { util.SuccessResponse(c, h.service.GetWechatStatus()) }
func (h *Handler) WechatEvents(c *gin.Context) { util.SuccessResponse(c, h.service.GetWechatEvents()) }
func (h *Handler) WechatReplyTimingRecover(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatReplyTimingRecover())
}
func (h *Handler) WechatReplyTimingStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.WechatReplyTimingStatus())
}

// ============= 通知 =============

func (h *Handler) NotificationsSettings(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetNotificationsSettings())
}
func (h *Handler) UpdateNotificationsSettings(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateNotificationsSettings(body))
}
func (h *Handler) NotificationsStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetNotificationsStatus())
}
func (h *Handler) NotificationsSubscribe(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.NotificationsSubscribe(body))
}
func (h *Handler) NotificationsTest(c *gin.Context) {
	util.SuccessResponse(c, h.service.NotificationsTest())
}
func (h *Handler) NotificationsUnsubscribe(c *gin.Context) {
	util.SuccessResponse(c, h.service.NotificationsUnsubscribe())
}

// ============= 安全配置 =============

func (h *Handler) SecurityAccessConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityAccessConfig())
}
func (h *Handler) UpdateSecurityAccessConfig(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateSecurityAccessConfig(body))
}
func (h *Handler) SecurityAccessStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityAccessStatus())
}
func (h *Handler) SecurityAccountCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.SecurityAccountCheck())
}
func (h *Handler) SecurityExposureCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.SecurityExposureCheck())
}
func (h *Handler) SecurityStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetSecurityStatus())
}

// ============= 隐私 =============

func (h *Handler) PrivacyMask(c *gin.Context) { util.SuccessResponse(c, h.service.PrivacyMask()) }
func (h *Handler) PrivacyScan(c *gin.Context) { util.SuccessResponse(c, h.service.PrivacyScan()) }
func (h *Handler) PrivacyScanResults(c *gin.Context) {
	util.SuccessResponse(c, h.service.PrivacyScanResults())
}
func (h *Handler) PrivacyScanResultsGet(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetPrivacyScanResult(c.Param("id")))
}

// ============= 更新 =============

func (h *Handler) UpdateCheck(c *gin.Context) { util.SuccessResponse(c, h.service.CheckUpdate()) }
func (h *Handler) GetUpdateConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetUpdateConfig())
}
func (h *Handler) UpdateConfig_Update(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UpdateUpdateConfig(body))
}

// ============= 回复时序 =============

func (h *Handler) ReplyTimingOverview(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReplyTimingOverview())
}
func (h *Handler) ReplyTimingBuffers(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReplyTimingBuffers())
}
func (h *Handler) ReplyTimingForce(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingForce())
}
func (h *Handler) ReplyTimingCancelBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingCancelBuffer(c.Param("id")))
}
func (h *Handler) ReplyTimingForceBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingForceBuffer(c.Param("id")))
}
func (h *Handler) ReplyTimingResumeBuffer(c *gin.Context) {
	util.SuccessResponse(c, h.service.ReplyTimingResumeBuffer(c.Param("id")))
}

// ============= 存储 =============

func (h *Handler) StorageBackup(c *gin.Context) { util.SuccessResponse(c, h.service.StorageBackup()) }
func (h *Handler) StorageBackupEncrypted(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageBackupEncrypted())
}
func (h *Handler) StorageBackups(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageBackups())
}
func (h *Handler) StorageDeleteBackup(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteStorageBackup(c.Param("name")))
}
func (h *Handler) StorageDeleteAll(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteAllStorage())
}
func (h *Handler) StorageRestore(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageRestore(c.Param("name")))
}
func (h *Handler) StorageRestoreEncrypted(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageRestoreEncrypted(body))
}
func (h *Handler) StorageRestoreVerify(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageRestoreVerify(body))
}
func (h *Handler) StorageExportUserData(c *gin.Context) {
	util.SuccessResponse(c, h.service.StorageExportUserData())
}
func (h *Handler) StorageImportUserData(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.StorageImportUserData(body))
}
func (h *Handler) StorageInfo(c *gin.Context) { util.SuccessResponse(c, h.service.GetStorageInfo()) }
func (h *Handler) StorageMigrations(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetStorageMigrations())
}
func (h *Handler) StorageMigrationsCheck(c *gin.Context) {
	util.SuccessResponse(c, h.service.CheckStorageMigrations())
}

// ============= 导入 =============

func (h *Handler) ImportsBatches(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatches())
}
func (h *Handler) ImportsBatchDetail(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchDetail(c.Param("id")))
}
func (h *Handler) ImportsBatchSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchSummary(c.Param("id")))
}
func (h *Handler) ImportsBatchMemoryCandidates(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetImportsBatchMemoryCandidates(c.Param("id")))
}
func (h *Handler) ImportsBatchDelete(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteImportsBatch(c.Param("id")))
}
func (h *Handler) ImportsBatchGenerateSummary(c *gin.Context) {
	util.SuccessResponse(c, h.service.GenerateImportsBatchSummary(c.Param("id")))
}
func (h *Handler) ImportsBatchConfirmMemories(c *gin.Context) {
	util.SuccessResponse(c, h.service.ConfirmImportsBatchMemories(c.Param("id")))
}
func (h *Handler) ImportsUpload(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.UploadImports(body))
}
func (h *Handler) ImportsParseText(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ParseImportsText(body))
}
func (h *Handler) ImportsConfirm(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ConfirmImports(body))
}
func (h *Handler) ImportData(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, h.service.ImportData(body))
}

// ============= 用量 =============

func (h *Handler) UsageOverview(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetUsageOverview())
}
func (h *Handler) UsageDaily(c *gin.Context)   { util.SuccessResponse(c, h.service.GetUsageDaily()) }
func (h *Handler) UsageModels(c *gin.Context)  { util.SuccessResponse(c, h.service.GetUsageModels()) }
func (h *Handler) UsageSources(c *gin.Context) { util.SuccessResponse(c, h.service.GetUsageSources()) }
func (h *Handler) UsageClear(c *gin.Context)   { util.SuccessResponse(c, h.service.ClearUsage()) }

// ============= 日志 =============

func (h *Handler) LogsRecent(c *gin.Context) { util.SuccessResponse(c, h.service.GetLogsRecent(50)) }
func (h *Handler) LogsRecentErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsRecentErrors(20))
}
func (h *Handler) LogsFiles(c *gin.Context) { util.SuccessResponse(c, h.service.GetLogsFiles()) }
func (h *Handler) LogsFileContent(c *gin.Context) {
	c.String(200, h.service.GetLogsFileContent(c.Param("name")))
}
func (h *Handler) LogsDelete(c *gin.Context) { util.SuccessResponse(c, h.service.DeleteLogs()) }
func (h *Handler) LogsModelErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetLogsModelErrors())
}
func (h *Handler) LogsDeleteModelErrors(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteLogsModelErrors())
}

// ============= 维护 =============

func (h *Handler) MaintenanceStatus(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetMaintenanceStatus())
}
func (h *Handler) MaintenanceDiagnose(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceDiagnose())
}
func (h *Handler) MaintenanceExportDiagnostic(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceExportDiagnostic())
}
func (h *Handler) MaintenanceReloadConfig(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceReloadConfig())
}
func (h *Handler) MaintenanceRestartBridge(c *gin.Context) {
	util.SuccessResponse(c, h.service.MaintenanceRestartBridge())
}

// ============= 版本 =============

func (h *Handler) ReleaseCheckLatest(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReleaseCheckLatest())
}
func (h *Handler) ReleaseCheckHistory(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetReleaseCheckHistory())
}
func (h *Handler) ReleaseCheckExport(c *gin.Context) {
	util.SuccessResponse(c, h.service.ExportReleaseCheck())
}
func (h *Handler) ReleaseCheckRun(c *gin.Context) {
	util.SuccessResponse(c, h.service.RunReleaseCheck())
}

// ============= 心情 =============

func (h *Handler) GetCompanionMoods(c *gin.Context) { util.SuccessResponse(c, h.service.GetMoods()) }
func (h *Handler) GetCompanionMoodsByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.GetMoodsByConversation(c.Param("id")))
}
func (h *Handler) DeleteCompanionMood(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteMood(c.Param("id")))
}
func (h *Handler) DeleteCompanionMoodsByConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.DeleteMoodsByConversation(c.Param("id")))
}

// ============= 旧版兼容 =============

func (h *Handler) LegacyListConversations(c *gin.Context) {
	util.SuccessResponse(c, h.service.LegacyListConversations())
}
func (h *Handler) LegacyGetMessages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	util.SuccessResponse(c, h.service.LegacyGetMessages(c.Param("id"), page, pageSize))
}
func (h *Handler) LegacyDeleteConversation(c *gin.Context) {
	util.SuccessResponse(c, h.service.LegacyDeleteConversation(c.Param("id")))
}

// ============= Web Chat 核心 (保留原实现) =============

func (h *Handler) WebChatSend(c *gin.Context) {
	var body struct {
		ConversationID string  `json:"conversationId"`
		Content        string  `json:"content"`
		Message        string  `json:"message"`
		CharacterID    string  `json:"characterId"`
		VoiceMessage   bool    `json:"voiceMessage"`
		AudioUrl       string  `json:"audioUrl"`
		AudioDuration  float64 `json:"audioDuration"`
		ImageUrl       string  `json:"imageUrl"`
		VideoUrl       string  `json:"videoUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	msgContent := body.Content
	if msgContent == "" {
		msgContent = body.Message
	}
	if msgContent == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息不能为空", nil)
		return
	}

	convID := body.ConversationID
	if convID == "" {
		convID = "web-" + uuid.New().String()[:8]
	}

	applog.Info(fmt.Sprintf("[Webhook] ImageUrl=%s VideoUrl=%s", body.ImageUrl[:min(len(body.ImageUrl), 60)], body.VideoUrl[:min(len(body.VideoUrl), 60)]))
	chat.GetBuffer().AnalyzeImage(convID, body.ImageUrl)
	chat.GetBuffer().AnalyzeVideo(convID, body.VideoUrl)

	bufferedMsgs, bufErr := chat.GetBuffer().Buffer(convID, msgContent)
	if bufErr != nil {
		util.SuccessResponse(c, gin.H{"status": "queued", "conversationId": convID})
		return
	}

	mergedContent := strings.Join(bufferedMsgs, "\n")
	imageCtx := chat.GetBuffer().GetImageContexts(convID)
	applog.Info(fmt.Sprintf("[Webhook] imageCtx len=%d content=%s", len(imageCtx), imageCtx[:min(len(imageCtx), 200)]))

	result, err := h.chatSvc.ProcessMessage(&chat.ProcessMessageRequest{
		CharacterID: body.CharacterID, Message: mergedContent,
		ConversationID: convID, Channel: "web", Source: "manual",
		AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
		VoiceMessage: body.VoiceMessage,
		ImageUrl:     body.ImageUrl,
		VideoUrl:     body.VideoUrl,
		ImageContext: imageCtx,
	})
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	util.SuccessResponse(c, gin.H{"conversationId": result.ConversationID, "reply": result.Reply, "messageIds": result.MessageIDs, "characterName": result.CharacterName})
}
func (h *Handler) WebChatSendStream(c *gin.Context) {
	var body struct {
		ConversationID string  `json:"conversationId"`
		Content        string  `json:"content"`
		Message        string  `json:"message"`
		CharacterID    string  `json:"characterId"`
		VoiceMessage   bool    `json:"voiceMessage"`
		AudioUrl       string  `json:"audioUrl"`
		AudioDuration  float64 `json:"audioDuration"`
		ImageUrl       string  `json:"imageUrl"`
		VideoUrl       string  `json:"videoUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	msgContent := body.Content
	if msgContent == "" {
		msgContent = body.Message
	}
	if msgContent == "" {
		util.ErrorResponse(c, response.InvalidParams, "消息不能为空", nil)
		return
	}

	convID := body.ConversationID
	if convID == "" {
		convID = "web-" + uuid.New().String()[:8]
	}

	applog.Info(fmt.Sprintf("[Webhook] ImageUrl=%s VideoUrl=%s", body.ImageUrl[:min(len(body.ImageUrl), 60)], body.VideoUrl[:min(len(body.VideoUrl), 60)]))
	chat.GetBuffer().AnalyzeImage(convID, body.ImageUrl)
	chat.GetBuffer().AnalyzeVideo(convID, body.VideoUrl)

	bufferedMsgs, bufErr := chat.GetBuffer().Buffer(convID, msgContent)
	if bufErr != nil {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"status": "queued", "conversationId": convID}})
		return
	}

	mergedContent := strings.Join(bufferedMsgs, "\n")
	imageCtx := chat.GetBuffer().GetImageContexts(convID)
	applog.Info(fmt.Sprintf("[Webhook] imageCtx len=%d content=%s", len(imageCtx), imageCtx[:min(len(imageCtx), 200)]))

	result, err := h.chatSvc.ProcessMessage(&chat.ProcessMessageRequest{
		CharacterID: body.CharacterID, Message: mergedContent,
		ConversationID: convID, Channel: "web", Source: "manual",
		AudioUrl: body.AudioUrl, AudioDuration: body.AudioDuration,
		VoiceMessage: body.VoiceMessage,
		ImageUrl:     body.ImageUrl,
		VideoUrl:     body.VideoUrl,
		ImageContext: imageCtx,
	})
	if err != nil {
		util.ErrorResponse(c, response.InternalError, err.Error(), nil)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		util.ErrorResponse(c, response.InternalError, "SSE not supported", nil)
		return
	}
	lines := strings.Split(strings.TrimSpace(result.Reply), "\n")
	msgIDIdx := 0

	voiceChance := 0.20
	if body.VoiceMessage {
		voiceChance = 0.80
	}
	if result.ForceVoice {
		voiceChance = 1.0
	}
	applog.Info(fmt.Sprintf("[Voice] voiceMessage=%v forceVoice=%v voiceChance=%.0f%% replyLen=%d", body.VoiceMessage, result.ForceVoice, voiceChance*100, len(result.Reply)))
	var ttsCfg *tts.TtsConfig
	if (rand.Float64() < voiceChance || result.ForceVoice) && result.Reply != "" {
		ttsRepo := tts.NewRepository(h.db)
		charCfg, cfgErr := ttsRepo.GetByCharacterID(body.CharacterID)
		if cfgErr != nil {
			applog.Info(fmt.Sprintf("[Voice] GetByCharacterID err: %v", cfgErr))
		}
		if cfgErr == nil && charCfg.ApiKey != "" {
			cfg := &tts.TtsConfig{ApiKey: charCfg.ApiKey, ResourceId: charCfg.ResourceId, VoiceType: charCfg.VoiceType, Speed: charCfg.Speed, Pitch: charCfg.Pitch, Volume: charCfg.Volume}
			if cfg.ResourceId == "" {
				cfg.ResourceId = "seed-tts-2.0"
			}
			if cfg.VoiceType == "" {
				cfg.VoiceType = "zh_female_vv_uranus_bigtts"
			}
			if cfg.Speed == 0 {
				cfg.Speed = 1.0
			}
			if cfg.Pitch == 0 {
				cfg.Pitch = 1.0
			}
			if cfg.Volume == 0 {
				cfg.Volume = 1.0
			}
			ttsCfg = cfg
		} else if charCfg != nil && charCfg.ApiKey == "" {
			applog.Info("[Voice] TTS ApiKey empty")
		}
	} else {
		applog.Info(fmt.Sprintf("[Voice] skipped: chance=%.2f forceVoice=%v reply=%v", voiceChance, result.ForceVoice, result.Reply != ""))
	}

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || isReasoningLine(line) {
			continue
		}
		if i > 0 {
			delayMs := 300 + len([]rune(line))*80
			if delayMs > 3000 {
				delayMs = 3000
			}
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
		msgID := uuid.New().String()
		if msgIDIdx < len(result.MessageIDs) {
			msgID = result.MessageIDs[msgIDIdx]
		}
		msgIDIdx++

		var audioURL string
		var audioDuration float64
		if ttsCfg != nil {
			applog.Info(fmt.Sprintf("[Voice] TTS part: %s", line[:min(len(line), 30)]))
			synthResult, synthErr := ttsSynthesizeWithTimeout(ttsCfg, line, 8*time.Second)
			if synthErr != nil {
				applog.Info(fmt.Sprintf("[Voice] TTS err: %v", synthErr))
			} else {
				audioURL = synthResult.AudioURL
				audioDuration = synthResult.Duration
				h.db.Table("messages").Where("id = ?", msgID).Updates(map[string]interface{}{
					"audio_url":      audioURL,
					"audio_duration": audioDuration,
				})
			}
		}

		if ttsCfg != nil && audioURL != "" {
			audioData := gin.H{"messageId": msgID, "conversationId": result.ConversationID, "role": "assistant", "content": line, "createdAt": time.Now().Format("2006-01-02 15:04:05"), "audioUrl": audioURL, "duration": audioDuration}
			ad, _ := json.Marshal(audioData)
			applog.Info(fmt.Sprintf("[Voice] sending voice_audio: %s", audioURL))
			fmt.Fprintf(c.Writer, "event: voice_audio\ndata: %s\n\n", string(ad))
			flusher.Flush()
		} else if ttsCfg == nil {
			msg := gin.H{"id": msgID, "conversationId": result.ConversationID, "role": "assistant", "content": line, "createdAt": time.Now().Format("2006-01-02 15:04:05")}
			b, _ := json.Marshal(msg)
			fmt.Fprintf(c.Writer, "event: token\ndata: %s\n\n", string(b))
			flusher.Flush()
		}
	}
	doneData := gin.H{"conversationId": result.ConversationID}
	db, _ := json.Marshal(doneData)
	fmt.Fprintf(c.Writer, "event: done\ndata: %s\n\n", string(db))
	flusher.Flush()
}
func (h *Handler) WebChatCreateConv(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		CharacterID string `json:"characterId"`
		Channel     string `json:"channel"`
		Source      string `json:"source"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if body.Title == "" {
		body.Title = "新对话"
		if body.CharacterID != "" {
			var charName string
			h.db.Table("characters").Select("name").Where("id = ?", body.CharacterID).Limit(1).Row().Scan(&charName)
			if charName != "" {
				body.Title = charName
			}
		}
	}
	if body.Channel == "" {
		body.Channel = "web"
	}
	if body.Source == "" {
		body.Source = "manual"
	}
	if body.CharacterID != "" {
		var existingConvID string
		h.db.Table("characters").Select("conversation_id").Where("id = ?", body.CharacterID).Limit(1).Row().Scan(&existingConvID)
		if existingConvID != "" {
			var conv struct {
				ID, Title, Channel, Source, CharacterID string
			}
			h.db.Table("conversations").Select("id, title, channel, source, character_id").Where("id = ?", existingConvID).Limit(1).Row().Scan(&conv.ID, &conv.Title, &conv.Channel, &conv.Source, &conv.CharacterID)
			if conv.ID != "" {
				util.SuccessResponse(c, gin.H{"id": conv.ID, "title": conv.Title, "channel": conv.Channel, "source": conv.Source, "characterId": conv.CharacterID})
				return
			}
		}
	}
	if body.Channel == "wechat" || body.Channel == "qq" {
		var existing []map[string]interface{}
		h.db.Table("conversations").Where("channel = ? AND character_id = ?", body.Channel, body.CharacterID).Limit(1).Find(&existing)
		if len(existing) > 0 {
			util.SuccessResponse(c, gin.H{"id": existing[0]["id"], "title": existing[0]["title"], "channel": body.Channel, "source": existing[0]["source"], "characterId": existing[0]["character_id"]})
			return
		}
	}
	convID := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")
	h.db.Exec("INSERT INTO conversations (id, title, character_id, channel, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", convID, body.Title, body.CharacterID, body.Channel, body.Source, now, now)
	h.db.Exec("UPDATE characters SET conversation_id = ?, updated_at = ? WHERE id = ? AND (conversation_id IS NULL OR conversation_id = '')", convID, now, body.CharacterID)
	util.SuccessResponse(c, gin.H{"id": convID, "title": body.Title, "channel": body.Channel, "source": body.Source, "characterId": body.CharacterID})
}

func (h *Handler) WebChatUpdateConv(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		CharacterID string `json:"characterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil)
		return
	}
	if body.CharacterID != "" {
		h.db.Exec("UPDATE conversations SET character_id = ?, updated_at = ? WHERE id = ?", body.CharacterID, time.Now(), id)
	}
	util.SuccessResponse(c, gin.H{"updated": true, "id": id})
}

func (h *Handler) WebChatDeleteConv(c *gin.Context) {
	id := c.Param("id")
	h.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	h.db.Exec("DELETE FROM conversations WHERE id = ?", id)
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatDeleteConvMessages(c *gin.Context) {
	id := c.Param("id")
	h.db.Exec("DELETE FROM messages WHERE conversation_id = ?", id)
	util.SuccessResponse(c, gin.H{"deleted": true})
}

func (h *Handler) WebChatRegenerate(c *gin.Context) {
	convID := c.Param("id")
	if convID == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少会话ID", nil)
		return
	}
	type lastMsg struct {
		Role    string
		Content string
	}
	var msg lastMsg
	if err := h.db.Table("messages").Select("role, content").Where("conversation_id = ?", convID).Order("created_at DESC").Limit(1).Row().Scan(&msg.Role, &msg.Content); err != nil || msg.Role != "user" {
		util.ErrorResponse(c, response.DataNotFound, "没有可重新生成的消息", nil)
		return
	}
	h.db.Exec("DELETE FROM messages WHERE id = (SELECT id FROM messages WHERE conversation_id = ? AND role = 'assistant' ORDER BY created_at DESC LIMIT 1)", convID)
	h.WebChatSend(c)
}

func isReasoningLine(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "（推理") || strings.HasPrefix(s, "(推理")
}

func (h *Handler) getDBPath() string {
	var dbPath string
	h.db.Raw("PRAGMA database_list").Row().Scan(nil, nil, &dbPath)
	if dbPath == "" {
		dbPath = filepath.Join("data", "app.db")
	}
	return dbPath
}

func (h *Handler) readLogTail(path string, limit int) []map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return []map[string]interface{}{}
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > limit {
		start = len(lines) - limit
	}
	result := []map[string]interface{}{}
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		entry := map[string]interface{}{"line": i + 1, "content": line, "timestamp": time.Now().Format("2006-01-02 15:04:05")}
		if strings.Contains(line, "error") {
			entry["level"] = "error"
		} else if strings.Contains(line, "warn") {
			entry["level"] = "warn"
		} else {
			entry["level"] = "info"
		}
		result = append(result, entry)
	}
	return result
}

func (h *Handler) MessagesStream(c *gin.Context) {
	convID := c.Query("conversationId")
	if convID == "" {
		c.Header("Content-Type", "text/event-stream")
		c.Writer.WriteString("event: error\ndata: missing conversationId\n\n")
		c.Writer.Flush()
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	sinceID := c.Query("since")
	sinceCreatedAt := ""
	if sinceID != "" {
		h.db.Table("messages").Select("created_at").Where("id = ?", sinceID).Row().Scan(&sinceCreatedAt)
	}
	if sinceCreatedAt == "" {
		sinceCreatedAt = "0001-01-01"
	}
	for {
		var msgs []map[string]interface{}
		rows, _ := h.db.Table("messages").Where("conversation_id = ? AND (created_at > ? OR (created_at = ? AND id > ?))", convID, sinceCreatedAt, sinceCreatedAt, sinceID).Order("created_at ASC, id ASC").Rows()
		for rows.Next() {
			var m map[string]interface{}
			h.db.ScanRows(rows, &m)
			msgs = append(msgs, m)
		}
		rows.Close()
		for _, m := range msgs {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			if role == "tool" {
				continue
			}
			if role == "assistant" && content == "" {
				continue
			}
			c.SSEvent("message", m)
			if ca, ok := m["created_at"].(string); ok {
				sinceCreatedAt = ca
			}
			if id, ok := m["id"].(string); ok {
				sinceID = id
			}
			c.Writer.Flush()
		}
		select {
		case <-c.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// RemindersStream pushes events when reminders table changes (create/update/delete/trigger)
func (h *Handler) RemindersStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	var lastCount int64
	var lastUpdated string
	h.db.Table("reminders").Count(&lastCount)
	h.db.Table("reminders").Select("MAX(updated_at)").Row().Scan(&lastUpdated)

	c.SSEvent("status", map[string]interface{}{"count": lastCount})
	c.Writer.Flush()

	for {
		select {
		case <-c.Done():
			return
		case <-time.After(5 * time.Second):
		}

		var curCount int64
		var curUpdated string
		h.db.Table("reminders").Count(&curCount)
		h.db.Table("reminders").Select("MAX(updated_at)").Row().Scan(&curUpdated)

		if curCount != lastCount || curUpdated != lastUpdated {
			lastCount = curCount
			lastUpdated = curUpdated
			c.SSEvent("changed", map[string]interface{}{"count": curCount, "updatedAt": curUpdated})
			c.Writer.Flush()
		}
	}
}

func (h *Handler) WebChatReplyTimingForce(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"forced": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingHold(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"held": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingResume(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"resumed": true, "id": c.Param("id")})
}

func (h *Handler) WebChatReplyTimingStatus(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"id": c.Param("id"), "status": "idle"})
}

func (h *Handler) WebChatMessageStatus(c *gin.Context) {
	util.SuccessResponse(c, map[string]interface{}{"id": c.Param("id"), "status": "sent"})
}

func (h *Handler) WebChatFromImport(c *gin.Context) {
	var body map[string]interface{}
	c.ShouldBindJSON(&body)
	util.SuccessResponse(c, map[string]interface{}{"imported": true, "conversationId": ""})
}

func (h *Handler) VoiceUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少音频文件", nil)
		return
	}
	defer file.Close()

	voiceDir := filepath.Join("data", "voice_msg")
	if err := os.MkdirAll(voiceDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".webm"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(voiceDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	audioUrl := "/voice/" + filename
	util.SuccessResponse(c, gin.H{"audioUrl": audioUrl, "duration": 0})
}

func (h *Handler) ImageUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少图片文件", nil)
		return
	}
	defer file.Close()

	imageDir := filepath.Join("data", "images")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".png"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(imageDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	imageUrl := "/images/" + filename
	util.SuccessResponse(c, gin.H{"imageUrl": imageUrl})
}
func (h *Handler) VideoUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "缺少视频文件", nil)
		return
	}
	defer file.Close()

	videoDir := filepath.Join("data", "videos")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		util.ErrorResponse(c, response.InternalError, "创建目录失败", nil)
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".mp4"
	}
	filename := uuid.New().String() + ext
	savePath := filepath.Join(videoDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		util.ErrorResponse(c, response.InternalError, "保存文件失败", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		util.ErrorResponse(c, response.InternalError, "写入文件失败", nil)
		return
	}

	videoUrl := "/videos/" + filename
	util.SuccessResponse(c, gin.H{"videoUrl": videoUrl})
}

func (h *Handler) VoiceTranscribe(c *gin.Context) {
	var body struct {
		AudioUrl string `json:"audioUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.AudioUrl == "" {
		util.ErrorResponse(c, response.InvalidParams, "缺少audioUrl", nil)
		return
	}

	asrRepo := asr.NewRepository(h.db)
	activeCfg, cfgErr := asrRepo.GetActive()
	apiKey := ""
	if cfgErr == nil && activeCfg.ApiKey != "" {
		apiKey = activeCfg.ApiKey
	}
	if apiKey == "" {
		util.SuccessResponse(c, gin.H{"text": "", "status": "no_asr_key"})
		return
	}

	fullAudioUrl := "http://127.0.0.1:8899" + body.AudioUrl

	taskID, submitErr := asr.SubmitTask(apiKey, fullAudioUrl, "zh-CN")
	if submitErr != nil {
		util.SuccessResponse(c, gin.H{"text": "", "status": "asr_failed"})
		return
	}

	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		result, queryErr := asr.QueryTask(apiKey, taskID)
		if queryErr != nil {
			continue
		}
		if result.Status == "done" || result.Status == "success" {
			util.SuccessResponse(c, gin.H{"text": result.Result, "status": "ok"})
			return
		}
		if result.Status == "failed" {
			util.SuccessResponse(c, gin.H{"text": "", "status": "asr_failed"})
			return
		}
	}

	util.SuccessResponse(c, gin.H{"text": "", "status": "timeout"})
}

func ttsSynthesizeWithTimeout(cfg *tts.TtsConfig, text string, timeout time.Duration) (*tts.SynthesizeResponse, error) {
	type result struct {
		res *tts.SynthesizeResponse
		err error
	}
	ch := make(chan result, 1)
	go func() {
		r, e := tts.Synthesize(cfg, text)
		ch <- result{r, e}
	}()
	select {
	case res := <-ch:
		return res.res, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("tts timeout after %v", timeout)
	}
}
