// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"github.com/u-ai/backend/internal/temporal"
	"os"
	"strconv"
	"time"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	AppConfig() map[string]interface{}
	CheckDBIntegrity() map[string]interface{}
	CheckSafety(text string) map[string]interface{}
	CheckStorageMigrations() map[string]interface{}
	CheckUpdate() map[string]interface{}
	CleanupTemp() map[string]interface{}
	ClearUsage() map[string]interface{}
	ConfigExport() map[string]interface{}
	ConfigImportPreviewService(body map[string]interface{}) map[string]interface{}
	ConfigImportConfirmService(body map[string]interface{}) map[string]interface{}
	ConfigSettings() map[string]interface{}
	DeleteAllStorage() map[string]interface{}
	DeleteLogs() map[string]interface{}
	DeleteLogsModelErrors() map[string]interface{}
	DeleteStorageBackup(name string) map[string]interface{}
	Diagnostics() map[string]interface{}
	ExportReleaseCheck() map[string]interface{}
	GenerateRecoveryCodes() map[string]interface{}
	GetAuditActions() []string
	GetAuditSettings() map[string]interface{}
	GetAuditStats() map[string]interface{}
	GetAbout() map[string]interface{}
	GetCurrentSession(token string) map[string]interface{}
	GetLoginHistory() []map[string]interface{}
	GetLogsFileContent(name string) string
	GetLogsFiles() map[string]interface{}
	GetLogsModelErrors() map[string]interface{}
	GetLogsRecent(limit int) map[string]interface{}
	GetLogsRecentErrors(limit int) map[string]interface{}
	GetLongRunningConfig() map[string]interface{}
	GetLongRunningStatus() map[string]interface{}
	GetMaintenanceStatus() map[string]interface{}
	GetNotificationsSettings() map[string]interface{}
	GetNotificationsStatus() map[string]interface{}
	GetPrivacyScanResult(id string) map[string]interface{}
	GetRecoveryCodesStatus() map[string]interface{}
	GetReleaseCheckHistory() map[string]interface{}
	GetReleaseCheckLatest() map[string]interface{}
	GetRuntimeHealth() map[string]interface{}
	GetRuntimeHealthHistory() map[string]interface{}
	GetIdentityCore(characterID string) map[string]interface{}
	GetRuntimeMode() map[string]interface{}
	GetRuntimeStatus() map[string]interface{}
	GetSecurityAccessConfig() map[string]interface{}
	GetSecurityAccessStatus() map[string]interface{}
	GetSecurityStatus() map[string]interface{}
	GetSessionSettings() map[string]interface{}
	GetStorageBackups() map[string]interface{}
	GetStorageInfo() map[string]interface{}
	GetStorageMigrations() map[string]interface{}
	GetTheme() map[string]interface{}
	GetThemePresets() map[string]interface{}
	GetUpdateConfig() map[string]interface{}
	GetUsageDaily() map[string]interface{}
	GetUsageModels() map[string]interface{}
	GetUsageOverview() map[string]interface{}
	GetUsageSources() map[string]interface{}
	GetVersion() map[string]interface{}
	GetWechatBridgeConfig() map[string]interface{}
	GetWechatBridgeEvents() map[string]interface{}
	GetWechatBridgeQRCode() map[string]interface{}
	GetWechatBridgeStatus() map[string]interface{}
	GetWechatBridgeStatusDetail() map[string]interface{}
	GetQQBridgeStatus() map[string]interface{}
	GetQQBridgeStatusDetail() map[string]interface{}
	GetQQBridgeConfig() map[string]interface{}
	GetQQBridgeEvents() map[string]interface{}
	GetWechatEvents() map[string]interface{}
	GetWechatStatus() map[string]interface{}
	Health() map[string]interface{}
	MaintenanceDiagnose() map[string]interface{}
	MaintenanceExportDiagnostic() map[string]interface{}
	MaintenanceReloadConfig() map[string]interface{}
	MaintenanceRestartBridge() map[string]interface{}
	MaintenanceRestartQQBridge() map[string]interface{}
	MoodDetectionConfig() map[string]interface{}
	NotificationsSubscribe(body map[string]interface{}) map[string]interface{}
	NotificationsTest() map[string]interface{}
	NotificationsUnsubscribe() map[string]interface{}
	OnboardingComplete() map[string]interface{}
	OnboardingReset() map[string]interface{}
	OnboardingStatus() map[string]interface{}
	PrivacyMask() map[string]interface{}
	PrivacyScan() map[string]interface{}
	PrivacyScanResults() map[string]interface{}
	RotateLogs() map[string]interface{}
	RunDiagnostics() map[string]interface{}
	RunNow() map[string]interface{}
	RunReleaseCheck() map[string]interface{}
	SafetyEvents(page, pageSize int) map[string]interface{}
	DeleteSafetyEvents() map[string]interface{}
	HandleSafetyEvent(id string) map[string]interface{}
	SafetyImportCheck(body map[string]interface{}) map[string]interface{}
	SecurityAccountCheck() map[string]interface{}
	SecurityExposureCheck() map[string]interface{}
	SetupChecks() map[string]interface{}
	SetupFinish() map[string]interface{}
	SetupReset() map[string]interface{}
	SetupStatus() map[string]interface{}
	SetupStep(step string) map[string]interface{}
	StorageBackup() map[string]interface{}
	StorageBackupEncrypted() map[string]interface{}
	StorageExportUserData() map[string]interface{}
	StorageExportAmitia(scope string, characterID string) map[string]interface{}
	StorageImportUserData(body map[string]interface{}) map[string]interface{}
	StorageRestore(name string) map[string]interface{}
	StorageRestoreEncrypted(body map[string]interface{}) map[string]interface{}
	StorageRestoreVerify(body map[string]interface{}) map[string]interface{}
	ToolRoute(body map[string]interface{}) map[string]interface{}
	UpdateAppConfig(body map[string]interface{}) map[string]interface{}
	UpdateAuditSettings(body map[string]interface{}) map[string]interface{}
	UpdateLongRunningConfig(body map[string]interface{}) map[string]interface{}
	ValidateIdentityCorePatch(characterID string, body map[string]interface{}) map[string]interface{}
	UpdateNotificationsSettings(body map[string]interface{}) map[string]interface{}
	UpdateRuntimeMode(body map[string]interface{}) map[string]interface{}
	UpdateSecurityAccessConfig(body map[string]interface{}) map[string]interface{}
	UpdateSessionSettings(body map[string]interface{}) map[string]interface{}
	UpdateTheme(body map[string]interface{}) map[string]interface{}
	UpdateUpdateConfig(body map[string]interface{}) map[string]interface{}
	UpdateWechatBridgeConfig(body map[string]interface{}) map[string]interface{}
	ValidateMode() map[string]interface{}
	VerifyRecoveryCode(code string) map[string]interface{}
	WechatBridgeRecover() map[string]interface{}
	QQBridgeRecover() map[string]interface{}
	WechatCloudCheck() map[string]interface{}
	WechatCloudCheckReport() map[string]interface{}
	WechatCloudCheckRiskSummary() map[string]interface{}
	WechatCloudCheckRun() map[string]interface{}
	WechatLoginReconnect() map[string]interface{}
	WechatLoginRescan() map[string]interface{}
	WechatLoginStart() map[string]interface{}
	WechatLoginWait() map[string]interface{}
	WechatReplyTimingRecover() map[string]interface{}
	WechatReplyTimingStatus() map[string]interface{}
	AttachTemporalService(temporalSvc *temporal.Service)
}

type service struct {
	db          *gorm.DB
	startTime   time.Time
	healthLog   []map[string]interface{}
	dataDir     string
	temporalSvc *temporal.Service
}

func NewService(ctx *app.AppContext) Service {
	return &service{db: ctx.DB, startTime: time.Now(), dataDir: "data"}
}

func (s *service) AttachTemporalService(temporalSvc *temporal.Service) {
	s.temporalSvc = temporalSvc
}
func (s *service) MaintenanceRestartBridge() map[string]interface{} {
	result := s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
	return map[string]interface{}{"restarted": true, "restartedAt": time.Now().Format(time.DateTime), "bridgeResult": result}
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case json.Number:
		i, _ := val.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	}
	return 0
}

func readEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
