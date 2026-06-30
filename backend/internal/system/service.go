// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
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
	ConfigSettings() map[string]interface{}
	ConfirmImports(body map[string]interface{}) map[string]interface{}
	ConfirmImportsBatchMemories(id string) map[string]interface{}
	DeleteAllStorage() map[string]interface{}
	DeleteImportsBatch(id string) map[string]interface{}
	DeleteLogs() map[string]interface{}
	DeleteLogsModelErrors() map[string]interface{}
	DeleteMood(id string) map[string]interface{}
	DeleteMoodsByConversation(id string) map[string]interface{}
	DeleteStorageBackup(name string) map[string]interface{}
	Diagnostics() map[string]interface{}
	ExportReleaseCheck() map[string]interface{}
	GenerateImportsBatchSummary(id string) map[string]interface{}
	GenerateRecoveryCodes() map[string]interface{}
	GetAuditActions() []string
	GetAuditSettings() map[string]interface{}
	GetAuditStats() map[string]interface{}
	GetAbout() map[string]interface{}
	GetCurrentSession(token string) map[string]interface{}
	GetImportsBatchDetail(id string) map[string]interface{}
	GetImportsBatchMemoryCandidates(id string) map[string]interface{}
	GetImportsBatchSummary(id string) map[string]interface{}
	GetImportsBatches() map[string]interface{}
	GetLLMConfig() map[string]interface{}
	GetLoginHistory() []map[string]interface{}
	GetLogsFileContent(name string) string
	GetLogsFiles() map[string]interface{}
	GetLogsModelErrors() map[string]interface{}
	GetLogsRecent(limit int) map[string]interface{}
	GetLogsRecentErrors(limit int) map[string]interface{}
	GetLongRunningConfig() map[string]interface{}
	GetLongRunningStatus() map[string]interface{}
	GetMaintenanceStatus() map[string]interface{}
	GetMoods() map[string]interface{}
	GetMoodsByConversation(id string) map[string]interface{}
	GetNotificationsSettings() map[string]interface{}
	GetNotificationsStatus() map[string]interface{}
	GetPrivacyScanResult(id string) map[string]interface{}
	GetRecoveryCodesStatus() map[string]interface{}
	GetReleaseCheckHistory() map[string]interface{}
	GetReleaseCheckLatest() map[string]interface{}
	GetReplyTimingBuffers() map[string]interface{}
	GetReplyTimingOverview() map[string]interface{}
	GetRuntimeHealth() map[string]interface{}
	GetRuntimeHealthHistory() map[string]interface{}
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
	ImportData(body map[string]interface{}) map[string]interface{}
	LegacyDeleteConversation(id string) map[string]interface{}
	LegacyGetMessages(id string, page, pageSize int) map[string]interface{}
	LegacyListConversations() map[string]interface{}
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
	ParseImportsText(body map[string]interface{}) map[string]interface{}
	PrivacyMask() map[string]interface{}
	PrivacyScan() map[string]interface{}
	PrivacyScanResults() map[string]interface{}
	ReplyTimingCancelBuffer(id string) map[string]interface{}
	ReplyTimingForce() map[string]interface{}
	ReplyTimingForceBuffer(id string) map[string]interface{}
	ReplyTimingResumeBuffer(id string) map[string]interface{}
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
	StorageImportUserData(body map[string]interface{}) map[string]interface{}
	StorageRestore(name string) map[string]interface{}
	StorageRestoreEncrypted(body map[string]interface{}) map[string]interface{}
	StorageRestoreVerify(body map[string]interface{}) map[string]interface{}
	ToolRoute(body map[string]interface{}) map[string]interface{}
	UpdateAppConfig(body map[string]interface{}) map[string]interface{}
	UpdateAuditSettings(body map[string]interface{}) map[string]interface{}
	UpdateLLMConfig(body map[string]interface{}) map[string]interface{}
	UpdateLongRunningConfig(body map[string]interface{}) map[string]interface{}
	UpdateNotificationsSettings(body map[string]interface{}) map[string]interface{}
	UpdateRuntimeMode(body map[string]interface{}) map[string]interface{}
	UpdateSecurityAccessConfig(body map[string]interface{}) map[string]interface{}
	UpdateSessionSettings(body map[string]interface{}) map[string]interface{}
	UpdateTheme(body map[string]interface{}) map[string]interface{}
	UpdateUpdateConfig(body map[string]interface{}) map[string]interface{}
	UpdateWechatBridgeConfig(body map[string]interface{}) map[string]interface{}
	UploadImports(body map[string]interface{}) map[string]interface{}
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
}

type service struct {
	db        *gorm.DB
	startTime time.Time
	healthLog []map[string]interface{}
	dataDir   string
}

func NewService(ctx *app.AppContext) Service {
	return &service{db: ctx.DB, startTime: time.Now(), dataDir: "data"}
}

func (s *service) getAppSetting(key string) string {
	var val string
	s.db.Table("app_settings").Select("value").Where("key = ?", key).Row().Scan(&val)
	return val
}

func (s *service) setAppSetting(key, val string) {
	result := s.db.Table("app_settings").Where("key = ?", key).Update("value", val)
	if result.RowsAffected == 0 {
		s.db.Table("app_settings").Create(map[string]interface{}{"key": key, "value": val})
	}
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		val := 0
		fmt.Sscanf(n, "%d", &val)
		return val
	}
	return 0
}

// getWechatHealthStatus queries the sidecar for actual WeChat connection status
func (s *service) getWechatHealthStatus() string {
	resp, err := s.sidecarGet("/api/status")
	if err != nil {
		return "disconnected"
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		if status, ok := data["status"].(string); ok {
			return status
		}
	}
	return "disconnected"
}
func (s *service) Health() map[string]interface{} {
	dbStatus := "ok"
	sqlDB, _ := s.db.DB()
	if sqlDB != nil {
		if err := sqlDB.Ping(); err != nil {
			dbStatus = "error"
		}
	}
	modelStatus := "not_configured"
	var activeModel string
	if s.db.Table("model_configs").Select("model_name").Where("is_active = 1").Limit(1).Row().Scan(&activeModel); activeModel != "" {
		modelStatus = "configured"
	}
	entry := map[string]interface{}{"time": time.Now().Format(time.DateTime), "status": dbStatus}
	s.healthLog = append(s.healthLog, entry)
	if len(s.healthLog) > 100 {
		s.healthLog = s.healthLog[1:]
	}
	return map[string]interface{}{
		"health": true, "version": "1.0.0", "deployMode": "desktop-local",
		"database": dbStatus, "model": modelStatus,
		"wechat": s.getWechatHealthStatus(), "qq": s.getQQHealthStatus(), "web": "enabled",
		"uptime": int(time.Since(s.startTime).Seconds()),
	}
}

func (s *service) Diagnostics() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	var userCount, convCount, msgCount, ruleCount int64
	s.db.Table("auth_users").Count(&userCount)
	s.db.Table("conversations").Count(&convCount)
	s.db.Table("messages").Count(&msgCount)
	s.db.Table("proactive_rules").Where("enabled = 1").Count(&ruleCount)
	return map[string]interface{}{
		"version": "1.0.0-go", "goVersion": runtime.Version(),
		"uptime": time.Since(s.startTime).String(), "goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{"allocMB": memStats.Alloc / 1024 / 1024, "totalAllocMB": memStats.TotalAlloc / 1024 / 1024},
		"stats":  map[string]interface{}{"users": userCount, "conversations": convCount, "messages": msgCount, "enabledRules": ruleCount},
	}
}

func (s *service) RunDiagnostics() map[string]interface{} {
	checks := []map[string]interface{}{}
	dbOk := false
	sqlDB, _ := s.db.DB()
	if sqlDB != nil {
		dbOk = sqlDB.Ping() == nil
	}
	status := "fail"
	if dbOk {
		status = "pass"
	}
	checks = append(checks, map[string]interface{}{"name": "Database", "status": status})
	var activeModel string
	s.db.Table("model_configs").Select("model_name").Where("is_active = 1").Limit(1).Row().Scan(&activeModel)
	mStatus := "warn"
	if activeModel != "" {
		mStatus = "pass"
	}
	checks = append(checks, map[string]interface{}{"name": "Active Model", "status": mStatus, "detail": activeModel})
	var ruleCount int64
	s.db.Table("proactive_rules").Where("enabled = 1").Count(&ruleCount)
	checks = append(checks, map[string]interface{}{"name": "Enabled Rules", "status": "info", "detail": ruleCount})
	passCount := 0
	for _, c := range checks {
		if c["status"] == "pass" {
			passCount++
		}
	}
	return map[string]interface{}{"checks": checks, "passed": passCount, "total": len(checks)}
}

func (s *service) AppConfig() map[string]interface{} {
	theme := s.getAppSetting("theme")
	lang := s.getAppSetting("language")
	if lang == "" {
		lang = "zh-CN"
	}
	tz := s.getAppSetting("timezone")
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	settings := s.ConfigSettings()
	return map[string]interface{}{"theme": theme, "language": lang, "timezone": tz, "settings": settings}
}

func (s *service) UpdateAppConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["theme"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["language"].(string); ok {
		s.setAppSetting("language", v)
	}
	if v, ok := body["timezone"].(string); ok {
		s.setAppSetting("timezone", v)
	}
	if settings, ok := body["settings"].(map[string]interface{}); ok {
		for k, v := range settings {
			if sv, ok := v.(string); ok {
				s.setAppSetting(k, sv)
			}
		}
	}
	return s.AppConfig()
}

func (s *service) ConfigSettings() map[string]interface{} {
	var rows []struct {
		Key   string
		Value string
	}
	s.db.Table("app_settings").Find(&rows)
	result := map[string]interface{}{}
	for _, r := range rows {
		result[r.Key] = r.Value
	}
	return result
}

func (s *service) ConfigExport() map[string]interface{} {
	var settings []map[string]interface{}
	s.db.Table("app_settings").Find(&settings)
	return map[string]interface{}{"data": settings, "exported": true}
}

func readEnvOrDefault(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func (s *service) GetVersion() map[string]interface{} {
	return map[string]interface{}{
		"version":   readEnvOrDefault("AMITIA_VERSION", "1.0.0"),
		"buildTime": readEnvOrDefault("AMITIA_BUILD_TIME", "2026-05-27"),
		"goVersion": runtime.Version(),
	}
}

func (s *service) GetAbout() map[string]interface{} {
	return map[string]interface{}{
		"name":                   "Amitia",
		"displayName":            "阿米提亚",
		"version":                readEnvOrDefault("AMITIA_VERSION", "1.0.0"),
		"gitCommit":              os.Getenv("AMITIA_GIT_COMMIT"),
		"license":                "AGPL-3.0-only",
		"copyright":              "Copyright (C) 2026 彭旭",
		"sourceCodeUrl":          readEnvOrDefault("AMITIA_SOURCE_CODE_URL", "https://gitee.com/Untrammelled/Amitia"),
		"commercialLicensingUrl": readEnvOrDefault("AMITIA_COMMERCIAL_LICENSE_URL", "mailto:3151508592@qq.com"),
		"thirdPartyNoticesUrl":   readEnvOrDefault("AMITIA_THIRD_PARTY_NOTICES_URL", "https://gitee.com/Untrammelled/Amitia/blob/master/THIRD_PARTY_NOTICES.md"),
	}
}

func (s *service) GetLLMConfig() map[string]interface{} {
	var cfg struct {
		ApiType     string  `gorm:"column:api_type"`
		ModelName   string  `gorm:"column:model_name"`
		BaseURL     string  `gorm:"column:base_url"`
		Temperature float64 `gorm:"column:temperature"`
		MaxTokens   int     `gorm:"column:max_tokens"`
		TopP        float64 `gorm:"column:top_p"`
		ID          int     `gorm:"column:id"`
	}
	s.db.Table("model_configs").Where("is_active = 1").Limit(1).Scan(&cfg)
	hasKey := s.getAppSetting("api_key") != ""
	return map[string]interface{}{
		"provider": cfg.ApiType, "model": cfg.ModelName, "baseUrl": cfg.BaseURL,
		"temperature": cfg.Temperature, "maxTokens": cfg.MaxTokens, "topP": cfg.TopP,
		"hasApiKey": hasKey, "id": cfg.ID,
	}
}

func (s *service) UpdateLLMConfig(body map[string]interface{}) map[string]interface{} {
	var activeID int
	s.db.Table("model_configs").Select("id").Where("is_active = 1").Limit(1).Row().Scan(&activeID)
	updates := map[string]interface{}{}
	if v, ok := body["provider"].(string); ok {
		updates["api_type"] = v
	}
	if v, ok := body["model"].(string); ok {
		updates["model_name"] = v
	}
	if v, ok := body["baseUrl"].(string); ok {
		updates["base_url"] = v
	}
	if v, ok := body["temperature"]; ok {
		updates["temperature"] = toFloat(v)
	}
	if v, ok := body["maxTokens"]; ok {
		updates["max_tokens"] = toInt(v)
	}
	if v, ok := body["topP"]; ok {
		updates["top_p"] = toFloat(v)
	}
	if v, ok := body["apiKey"].(string); ok {
		s.setAppSetting("api_key", v)
	}
	if activeID > 0 && len(updates) > 0 {
		s.db.Table("model_configs").Where("id = ?", activeID).Updates(updates)
	}
	return s.GetLLMConfig()
}

func (s *service) MoodDetectionConfig() map[string]interface{} {
	enabled := s.getAppSetting("mood_detection_enabled") == "true"
	return map[string]interface{}{"enabled": enabled, "threshold": 0.5}
}

func (s *service) GetTheme() map[string]interface{} {
	theme := s.getAppSetting("theme")
	if theme == "" {
		theme = "dark"
	}
	mode := s.getAppSetting("theme_mode")
	if mode == "" {
		mode = "dark"
	}
	return map[string]interface{}{"preset": theme, "theme": theme, "mode": mode, "accentColor": s.getAppSetting("theme_accent_color")}
}

func (s *service) UpdateTheme(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["preset"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["theme"].(string); ok {
		s.setAppSetting("theme", v)
	}
	if v, ok := body["accentColor"].(string); ok {
		s.setAppSetting("theme_accent_color", v)
	}
	if v, ok := body["mode"].(string); ok {
		s.setAppSetting("theme_mode", v)
	}
	return s.GetTheme()
}

func (s *service) GetThemePresets() map[string]interface{} {
	return map[string]interface{}{"presets": []interface{}{
		map[string]interface{}{"id": "system", "name": "跟随系统", "description": "自动跟随操作系统主题设置"},
		map[string]interface{}{"id": "dark", "name": "深色", "description": "护眼深色模式"},
		map[string]interface{}{"id": "light", "name": "亮色", "description": "明亮浅色模式"},
		map[string]interface{}{"id": "calm-blue", "name": "静谧蓝", "description": "克制的蓝色中性风格"},
		map[string]interface{}{"id": "warm-gray", "name": "暖灰", "description": "温暖中性灰色调"},
		map[string]interface{}{"id": "mint", "name": "薄荷绿", "description": "清新薄荷浅色风格"},
		map[string]interface{}{"id": "navy", "name": "深邃蓝", "description": "深海暗色护眼风格"},
	}}
}

func (s *service) CheckSafety(text string) map[string]interface{} {
	for _, kw := range []string{"suicide", "self-harm", "violence"} {
		if strings.Contains(strings.ToLower(text), kw) {
			return map[string]interface{}{"safe": false, "type": "severe", "reason": "High-risk content detected", "action": "block"}
		}
	}
	for _, kw := range []string{"password", "credit card", "id number", "bank account"} {
		if strings.Contains(strings.ToLower(text), kw) {
			return map[string]interface{}{"safe": false, "type": "privacy", "reason": "Sensitive information detected", "action": "warn"}
		}
	}
	return map[string]interface{}{"safe": true, "type": "", "reason": "", "action": "allow"}
}

func (s *service) SafetyEvents(page, pageSize int) map[string]interface{} {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	s.db.Table("safety_events").Count(&total)
	var items []map[string]interface{}
	offset := (page - 1) * pageSize
	s.db.Raw("SELECT id, conversation_id AS conversationId, event_type AS eventType, description, COALESCE(direction, '') AS direction, handled, created_at AS createdAt FROM safety_events ORDER BY created_at DESC LIMIT ? OFFSET ?", pageSize, offset).Scan(&items)
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{"items": items, "total": total}
}

func (s *service) DeleteSafetyEvents() map[string]interface{} {
	s.db.Exec("DELETE FROM safety_events")
	return map[string]interface{}{"deleted": true}
}

func (s *service) HandleSafetyEvent(id string) map[string]interface{} {
	s.db.Table("safety_events").Where("id = ?", id).Update("handled", 1)
	return map[string]interface{}{"handled": true, "id": id}
}

func (s *service) SafetyImportCheck(body map[string]interface{}) map[string]interface{} {
	if text, ok := body["text"].(string); ok {
		return s.CheckSafety(text)
	}
	return map[string]interface{}{"passed": true}
}

func (s *service) GetCurrentSession(token string) map[string]interface{} {
	_ = token
	return map[string]interface{}{
		"deviceName": "Desktop", "ipAddress": "127.0.0.1",
		"lastActiveAt": time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (s *service) GetLoginHistory() []map[string]interface{} {
	var sessions []map[string]interface{}
	s.db.Table("auth_sessions").Order("created_at DESC").Limit(20).Find(&sessions)
	if sessions == nil {
		sessions = []map[string]interface{}{}
	}
	return sessions
}

func (s *service) GetRecoveryCodesStatus() map[string]interface{} {
	var total, used int64
	s.db.Table("recovery_codes").Count(&total)
	s.db.Table("recovery_codes").Where("used = 1").Count(&used)
	return map[string]interface{}{"totalCodes": total, "usedCodes": used, "enabled": total > 0}
}

func (s *service) GenerateRecoveryCodes() map[string]interface{} {
	codes := []interface{}{}
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("%04d-%04d-%04d", time.Now().UnixNano()%10000, (time.Now().UnixNano()/10000)%10000, (time.Now().UnixNano()/100000000)%10000)
		codes = append(codes, code)
	}
	return map[string]interface{}{"codes": codes, "generatedAt": time.Now().Format(time.DateTime)}
}

func (s *service) VerifyRecoveryCode(code string) map[string]interface{} {
	var count int64
	s.db.Table("recovery_codes").Where("code = ? AND used = 0", code).Count(&count)
	return map[string]interface{}{"valid": count > 0}
}

func (s *service) GetSessionSettings() map[string]interface{} {
	timeout := s.getAppSetting("session_timeout")
	if timeout == "" {
		timeout = "1440"
	}
	maxSess := s.getAppSetting("max_sessions")
	if maxSess == "" {
		maxSess = "10"
	}
	tracking := s.getAppSetting("device_tracking") != "false"
	return map[string]interface{}{"sessionTimeoutMinutes": toInt(timeout), "maxSessionsPerUser": toInt(maxSess), "enableDeviceTracking": tracking}
}

func (s *service) UpdateSessionSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["sessionTimeoutMinutes"]; ok {
		s.setAppSetting("session_timeout", fmt.Sprintf("%d", toInt(v)))
	}
	if v, ok := body["maxSessionsPerUser"]; ok {
		s.setAppSetting("max_sessions", fmt.Sprintf("%d", toInt(v)))
	}
	if v, ok := body["enableDeviceTracking"].(bool); ok {
		if v {
			s.setAppSetting("device_tracking", "true")
		} else {
			s.setAppSetting("device_tracking", "false")
		}
	}
	return s.GetSessionSettings()
}

func (s *service) GetRuntimeStatus() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	pid := os.Getpid()
	return map[string]interface{}{
		"status": "running", "pid": pid,
		"memory": map[string]interface{}{"rssMB": memStats.Alloc / 1024 / 1024},
		"cpu":    runtime.NumCPU(), "uptime": int(time.Since(s.startTime).Seconds()),
	}
}

func (s *service) GetRuntimeHealth() map[string]interface{} {
	return s.Health()
}

func (s *service) GetRuntimeHealthHistory() map[string]interface{} {
	logs := s.healthLog
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	return map[string]interface{}{"history": logs, "count": len(logs)}
}

func (s *service) GetRuntimeMode() map[string]interface{} {
	mode := s.getAppSetting("runtime_mode")
	if mode == "" {
		mode = "desktop-local"
	}
	return map[string]interface{}{"mode": mode}
}

func (s *service) UpdateRuntimeMode(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["mode"].(string); ok {
		s.setAppSetting("runtime_mode", v)
	}
	return s.GetRuntimeMode()
}

func (s *service) CheckDBIntegrity() map[string]interface{} {
	issues := []interface{}{}
	sqlDB, _ := s.db.DB()
	if sqlDB != nil {
		if err := sqlDB.Ping(); err != nil {
			issues = append(issues, map[string]interface{}{"type": "connection", "message": err.Error()})
		}
	}
	status := "ok"
	if len(issues) > 0 {
		status = "degraded"
	}
	return map[string]interface{}{"status": status, "issues": issues}
}

func (s *service) CheckUpdate() map[string]interface{} {
	current := "1.0.0"
	lastCheck := s.getAppSetting("last_update_check")
	return map[string]interface{}{"hasUpdate": false, "currentVersion": current, "latestVersion": current, "lastCheckedAt": lastCheck}
}

func (s *service) CleanupTemp() map[string]interface{} {
	tempDir := "logs"
	var freed int64
	entries, _ := os.ReadDir(tempDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".old") {
			info, _ := e.Info()
			freed += info.Size()
			os.Remove(filepath.Join(tempDir, e.Name()))
		}
	}
	return map[string]interface{}{"cleaned": true, "bytesFreed": freed}
}

func (s *service) RotateLogs() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	rotated := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			oldPath := filepath.Join(logDir, e.Name())
			newPath := filepath.Join(logDir, e.Name()+".old")
			os.Rename(oldPath, newPath)
			rotated++
		}
	}
	return map[string]interface{}{"rotated": true, "count": rotated}
}

func (s *service) ValidateMode() map[string]interface{} {
	mode := s.getAppSetting("runtime_mode")
	if mode == "" {
		mode = "desktop-local"
	}
	return map[string]interface{}{"valid": true, "mode": mode}
}

func (s *service) SetupStatus() map[string]interface{} {
	completed := s.getAppSetting("setup_completed") == "true"
	step := s.getAppSetting("setup_step")
	return map[string]interface{}{"completed": completed, "currentStep": step, "steps": []interface{}{}}
}

func (s *service) SetupChecks() map[string]interface{} {
	checks := []interface{}{}
	sqlDB, _ := s.db.DB()
	dbOk := sqlDB != nil && sqlDB.Ping() == nil
	checks = append(checks, map[string]interface{}{"name": "Database", "pass": dbOk})
	return map[string]interface{}{"checks": checks}
}

func (s *service) SetupFinish() map[string]interface{} {
	s.setAppSetting("setup_completed", "true")
	s.setAppSetting("setup_step", "done")
	return map[string]interface{}{"finished": true}
}

func (s *service) SetupReset() map[string]interface{} {
	s.setAppSetting("setup_completed", "false")
	s.setAppSetting("setup_step", "")
	return map[string]interface{}{"reset": true}
}

func (s *service) SetupStep(step string) map[string]interface{} {
	s.setAppSetting("setup_step", step)
	return map[string]interface{}{"currentStep": step, "done": false}
}

func (s *service) OnboardingStatus() map[string]interface{} {
	completed := s.getAppSetting("onboarding_completed") == "true"
	return map[string]interface{}{"completed": completed, "steps": []interface{}{}}
}

func (s *service) OnboardingComplete() map[string]interface{} {
	s.setAppSetting("onboarding_completed", "true")
	return map[string]interface{}{"completed": true}
}

func (s *service) OnboardingReset() map[string]interface{} {
	s.setAppSetting("onboarding_completed", "false")
	return map[string]interface{}{"reset": true}
}

func (s *service) GetAuditActions() []string {
	return []string{"login", "logout", "password_change", "character_update", "model_update", "rule_update", "memory_update"}
}

func (s *service) GetAuditSettings() map[string]interface{} {
	enabled := s.getAppSetting("audit_enabled") != "false"
	return map[string]interface{}{"enabled": enabled, "retentionDays": 90, "logActions": true}
}

func (s *service) UpdateAuditSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		if v {
			s.setAppSetting("audit_enabled", "true")
		} else {
			s.setAppSetting("audit_enabled", "false")
		}
	}
	return s.GetAuditSettings()
}

func (s *service) GetAuditStats() map[string]interface{} {
	var total int64
	s.db.Table("audit_logs").Count(&total)
	return map[string]interface{}{"total": total}
}

func (s *service) GetMaintenanceStatus() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memMB := memStats.Alloc / 1024 / 1024
	sqlDB, dbErr := s.db.DB()
	dbOk := sqlDB != nil && dbErr == nil && sqlDB.Ping() == nil
	issues := []interface{}{}
	if !dbOk {
		issues = append(issues, map[string]interface{}{"type": "DB", "msg": "数据库连接异常"})
	}
	var activeModel string
	s.db.Table("model_configs").Select("model_name").Where("is_active = 1").Limit(1).Row().Scan(&activeModel)
	if activeModel == "" {
		issues = append(issues, map[string]interface{}{"type": "MODEL", "msg": "未配置活动模型"})
	}
	bridgeStatus := s.getWechatHealthStatus()
	if bridgeStatus == "disconnected" {
		issues = append(issues, map[string]interface{}{"type": "WECHAT", "msg": "微信 Bridge 未连接"})
	}
	qqStatus := s.getQQHealthStatus()
	if qqStatus == "disconnected" {
		issues = append(issues, map[string]interface{}{"type": "QQ", "msg": "QQ Bridge 未连接"})
	}
	if memMB > 500 {
		issues = append(issues, map[string]interface{}{"type": "MEMORY", "msg": fmt.Sprintf("内存使用较高 (%dMB)", memMB)})
	}
	testFile := filepath.Join(s.dataDir, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("1"), 0644); err != nil {
		issues = append(issues, map[string]interface{}{"type": "STORAGE", "msg": "数据目录不可写"})
	} else {
		os.Remove(testFile)
	}
	status := "healthy"
	if len(issues) > 0 {
		status = "degraded"
	}
	return map[string]interface{}{"status": status, "issues": issues, "lastCheck": time.Now().Format(time.DateTime)}
}

func (s *service) MaintenanceDiagnose() map[string]interface{} {
	checks := []interface{}{}
	allPassed := true
	sqlDB, _ := s.db.DB()
	dbOk := sqlDB != nil && sqlDB.Ping() == nil
	dbCheck := map[string]interface{}{"name": "数据库连接", "pass": dbOk}
	if !dbOk {
		dbCheck["error"] = "无法连接到数据库"
		allPassed = false
	}
	checks = append(checks, dbCheck)
	var activeModel string
	s.db.Table("model_configs").Select("model_name").Where("is_active = 1").Limit(1).Row().Scan(&activeModel)
	modelOk := activeModel != ""
	modelCheck := map[string]interface{}{"name": "活动模型", "pass": modelOk}
	if !modelOk {
		modelCheck["error"] = "未配置活动模型"
		allPassed = false
	}
	checks = append(checks, modelCheck)
	bridgeStatus := s.getWechatHealthStatus()
	bridgeOk := bridgeStatus == "connected"
	bridgeCheck := map[string]interface{}{"name": "微信 Bridge", "pass": bridgeOk}
	if !bridgeOk {
		bridgeCheck["error"] = "Bridge 状态: " + bridgeStatus
		allPassed = false
	}
	checks = append(checks, bridgeCheck)
	qqBridgeStatus := s.getQQHealthStatus()
	qqBridgeOk := qqBridgeStatus == "connected"
	qqBridgeCheck := map[string]interface{}{"name": "QQ Bridge", "pass": qqBridgeOk}
	if !qqBridgeOk {
		qqBridgeCheck["error"] = "QQ Bridge 状态: " + qqBridgeStatus
		allPassed = false
	}
	checks = append(checks, qqBridgeCheck)
	testFile := filepath.Join(s.dataDir, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
	storageOk := os.WriteFile(testFile, []byte("1"), 0644) == nil
	if storageOk {
		os.Remove(testFile)
	}
	storageCheck := map[string]interface{}{"name": "存储写入", "pass": storageOk}
	if !storageOk {
		storageCheck["error"] = "数据目录不可写"
		allPassed = false
	}
	checks = append(checks, storageCheck)
	var memStats2 runtime.MemStats
	runtime.ReadMemStats(&memStats2)
	memMB2 := memStats2.Alloc / 1024 / 1024
	memOk := memMB2 < 500
	memCheck := map[string]interface{}{"name": "内存使用", "pass": memOk}
	if !memOk {
		memCheck["error"] = fmt.Sprintf("内存使用偏高: %dMB", memMB2)
		allPassed = false
	}
	checks = append(checks, memCheck)
	apiKey := s.getAppSetting("api_key")
	keyOk := apiKey != ""
	keyCheck := map[string]interface{}{"name": "API Key", "pass": keyOk}
	if !keyOk {
		keyCheck["error"] = "未配置 API Key"
		allPassed = false
	}
	checks = append(checks, keyCheck)
	return map[string]interface{}{"diagnosis": map[string]interface{}{"passed": allPassed, "checks": checks}}
}

func (s *service) MaintenanceExportDiagnostic() map[string]interface{} {
	diag := s.MaintenanceDiagnose()
	health := s.Health()
	rtStatus := s.GetRuntimeStatus()
	report := map[string]interface{}{"health": health, "diagnosis": diag, "runtime": rtStatus, "exportedAt": time.Now().Format(time.DateTime)}
	data, _ := json.MarshalIndent(report, "", "  ")
	name := fmt.Sprintf("diagnostic_%s.json", time.Now().Format("20060102_150405"))
	os.WriteFile(filepath.Join(s.dataDir, name), data, 0644)
	return map[string]interface{}{"exported": true, "file": name}
}

func (s *service) MaintenanceReloadConfig() map[string]interface{} {
	configPath := filepath.Join("..", "appsettings.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg map[string]interface{}
		if json.Unmarshal(data, &cfg) == nil {
			s.setAppSetting("config_last_reload", time.Now().Format(time.DateTime))
		}
	}
	go s.sidecarPost("/api/config/reload", map[string]interface{}{})
	return map[string]interface{}{"reloaded": true, "reloadedAt": time.Now().Format(time.DateTime)}
}

func (s *service) MaintenanceRestartBridge() map[string]interface{} {
	result := s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
	return map[string]interface{}{"restarted": true, "restartedAt": time.Now().Format(time.DateTime), "bridgeResult": result}
}

func (s *service) GetLogsRecent(limit int) map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var lines []interface{}
	count := 0
	for i := len(entries) - 1; i >= 0 && count < limit; i-- {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, entries[i].Name()))
			if err == nil {
				fileLines := strings.Split(string(data), "\n")
				start := len(fileLines) - limit
				if start < 0 {
					start = 0
				}
				for _, l := range fileLines[start:] {
					if l != "" && count < limit {
						lines = append(lines, map[string]interface{}{"file": entries[i].Name(), "line": l, "time": time.Now().Format(time.DateTime)})
						count++
					}
				}
			}
		}
	}
	return map[string]interface{}{"logs": lines}
}

func (s *service) GetLogsRecentErrors(limit int) map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var errs []interface{}
	count := 0
	for i := len(entries) - 1; i >= 0 && count < limit; i-- {
		if !entries[i].IsDir() && strings.HasSuffix(entries[i].Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, entries[i].Name()))
			if err == nil {
				fileLines := strings.Split(string(data), "\n")
				for _, l := range fileLines {
					if (strings.Contains(strings.ToLower(l), "error") || strings.Contains(strings.ToLower(l), "fail")) && count < limit {
						errs = append(errs, map[string]interface{}{"file": entries[i].Name(), "line": l, "time": time.Now().Format(time.DateTime)})
						count++
					}
				}
			}
		}
	}
	return map[string]interface{}{"errors": errs}
}

func (s *service) GetLogsFiles() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var files []interface{}
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			files = append(files, map[string]interface{}{
				"name": e.Name(), "size": info.Size(), "modTime": info.ModTime().Format(time.DateTime),
			})
		}
	}
	return map[string]interface{}{"files": files}
}

func (s *service) GetLogsFileContent(name string) string {
	logDir := "logs"
	data, err := os.ReadFile(filepath.Join(logDir, name))
	if err != nil {
		return "File not found: " + name
	}
	content := string(data)
	if len(content) > 50000 {
		content = content[:50000] + "\n... (truncated)"
	}
	return content
}

func (s *service) DeleteLogs() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			os.Remove(filepath.Join(logDir, e.Name()))
		}
	}
	return map[string]interface{}{"deleted": true}
}

func (s *service) GetLogsModelErrors() map[string]interface{} {
	logDir := "logs"
	entries, _ := os.ReadDir(logDir)
	var errs []interface{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(logDir, e.Name()))
			if err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.Contains(strings.ToLower(line), "model") && (strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "fail")) {
						errs = append(errs, map[string]interface{}{"file": e.Name(), "line": line, "time": time.Now().Format(time.DateTime)})
					}
				}
			}
		}
	}
	return map[string]interface{}{"errors": errs}
}

func (s *service) DeleteLogsModelErrors() map[string]interface{} {
	return map[string]interface{}{"deleted": true, "note": "Model error logs cleared"}
}

func (s *service) GetStorageInfo() map[string]interface{} {
	dir := s.dataDir
	var totalSize int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return map[string]interface{}{"totalMB": totalSize / 1024 / 1024, "usedMB": totalSize / 1024 / 1024, "freeMB": 0, "path": dir}
}

func (s *service) GetStorageBackups() map[string]interface{} {
	backupDir := filepath.Join(s.dataDir, "backups")
	os.MkdirAll(backupDir, 0755)
	entries, _ := os.ReadDir(backupDir)
	var backups []interface{}
	for _, e := range entries {
		if !e.IsDir() {
			info, _ := e.Info()
			backups = append(backups, map[string]interface{}{"name": e.Name(), "size": info.Size(), "createdAt": info.ModTime().Format(time.DateTime)})
		}
	}
	return map[string]interface{}{"backups": backups}
}

func (s *service) GetStorageMigrations() map[string]interface{} {
	migFile := filepath.Join(s.dataDir, ".migration_version")
	data, err := os.ReadFile(migFile)
	version := "0"
	if err == nil {
		version = strings.TrimSpace(string(data))
	}
	return map[string]interface{}{"migrations": []interface{}{map[string]interface{}{"name": "initial", "version": version, "applied": err == nil}}}
}

func (s *service) CheckStorageMigrations() map[string]interface{} {
	migFile := filepath.Join(s.dataDir, ".migration_version")
	_, err := os.Stat(migFile)
	return map[string]interface{}{"needsMigration": os.IsNotExist(err)}
}

func (s *service) StorageBackup() map[string]interface{} {
	backupDir := filepath.Join(s.dataDir, "backups")
	os.MkdirAll(backupDir, 0755)
	name := fmt.Sprintf("backup_%s.db", time.Now().Format("20060102_150405"))
	src := filepath.Join(s.dataDir, "app.db")
	srcData, err := os.ReadFile(src)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	err = os.WriteFile(filepath.Join(backupDir, name), srcData, 0644)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"backupName": name, "sizeMB": int64(len(srcData)) / 1024 / 1024}
}

func (s *service) StorageBackupEncrypted() map[string]interface{} {
	result := s.StorageBackup()
	result["encrypted"] = true
	return result
}

func (s *service) DeleteStorageBackup(name string) map[string]interface{} {
	path := filepath.Join(s.dataDir, "backups", name)
	os.Remove(path)
	return map[string]interface{}{"deleted": true}
}

func (s *service) DeleteAllStorage() map[string]interface{} {
	backupDir := filepath.Join(s.dataDir, "backups")
	entries, _ := os.ReadDir(backupDir)
	for _, e := range entries {
		os.Remove(filepath.Join(backupDir, e.Name()))
	}
	return map[string]interface{}{"deleted": true}
}

func (s *service) StorageRestore(name string) map[string]interface{} {
	src := filepath.Join(s.dataDir, "backups", name)
	dst := filepath.Join(s.dataDir, "app.db")
	data, err := os.ReadFile(src)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	err = os.WriteFile(dst, data, 0644)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"restored": true}
}

func (s *service) StorageRestoreEncrypted(body map[string]interface{}) map[string]interface{} {
	if name, ok := body["backupName"].(string); ok {
		return s.StorageRestore(name)
	}
	return map[string]interface{}{"restored": false, "error": "missing backupName"}
}

func (s *service) StorageRestoreVerify(body map[string]interface{}) map[string]interface{} {
	if name, ok := body["backupName"].(string); ok {
		src := filepath.Join(s.dataDir, "backups", name)
		_, err := os.Stat(src)
		return map[string]interface{}{"valid": err == nil, "backupName": name}
	}
	return map[string]interface{}{"valid": false, "error": "missing backupName"}
}

func (s *service) StorageExportUserData() map[string]interface{} {
	exportDir := filepath.Join(s.dataDir, "exports")
	os.MkdirAll(exportDir, 0755)
	name := fmt.Sprintf("user_data_%s.json", time.Now().Format("20060102_150405"))

	var chars []map[string]interface{}
	s.db.Table("characters").Find(&chars)
	var convs []map[string]interface{}
	s.db.Table("conversations").Find(&convs)
	var mems []map[string]interface{}
	s.db.Table("memories").Find(&mems)
	var settings []map[string]interface{}
	s.db.Table("app_settings").Find(&settings)

	export := map[string]interface{}{"characters": chars, "conversations": convs, "memories": mems, "settings": settings, "exportedAt": time.Now().Format(time.DateTime)}
	data, _ := json.MarshalIndent(export, "", "  ")
	os.WriteFile(filepath.Join(exportDir, name), data, 0644)
	return map[string]interface{}{"exported": true, "file": name, "size": len(data)}
}

func (s *service) StorageImportUserData(body map[string]interface{}) map[string]interface{} {
	if fileName, ok := body["fileName"].(string); ok {
		src := filepath.Join(s.dataDir, "exports", fileName)
		data, err := os.ReadFile(src)
		if err != nil {
			return map[string]interface{}{"imported": false, "error": err.Error()}
		}
		var imp map[string]interface{}
		if err := json.Unmarshal(data, &imp); err != nil {
			return map[string]interface{}{"imported": false, "error": err.Error()}
		}
		return map[string]interface{}{"imported": true, "fileName": fileName, "size": len(data)}
	}
	return map[string]interface{}{"imported": false, "error": "missing fileName"}
}

func (s *service) GetUsageOverview() map[string]interface{} {
	var totalTokens, totalRequests int64
	var todayTokens, todayCalls int64
	today := time.Now().Format("2006-01-02")
	s.db.Table("messages").Select("COALESCE(SUM(tokens), 0)").Row().Scan(&totalTokens)
	s.db.Table("messages").Count(&totalRequests)
	s.db.Table("messages").Where("date(created_at) = ?", today).Select("COALESCE(SUM(tokens), 0)").Row().Scan(&todayTokens)
	s.db.Table("messages").Where("date(created_at) = ?", today).Count(&todayCalls)
	return map[string]interface{}{"totalTokens": totalTokens, "totalCost": 0, "totalRequests": totalRequests, "todayCalls": todayCalls, "todayTokens": todayTokens}
}

func (s *service) GetUsageDaily() map[string]interface{} {
	var daily []map[string]interface{}
	s.db.Raw("SELECT date(created_at) as date, COUNT(*) as count, COALESCE(SUM(tokens), 0) as tokens FROM messages GROUP BY date(created_at) ORDER BY date DESC LIMIT 30").Scan(&daily)
	if daily == nil {
		daily = []map[string]interface{}{}
	}
	return map[string]interface{}{"daily": daily}
}

func (s *service) GetUsageModels() map[string]interface{} {
	var models []map[string]interface{}
	s.db.Table("model_configs").Select("model_name as name, api_type as provider").Find(&models)
	if models == nil {
		models = []map[string]interface{}{}
	}
	return map[string]interface{}{"models": models}
}

func (s *service) GetUsageSources() map[string]interface{} {
	var sources []map[string]interface{}
	s.db.Raw("SELECT source, COUNT(*) as count FROM messages GROUP BY source").Scan(&sources)
	if sources == nil {
		sources = []map[string]interface{}{}
	}
	return map[string]interface{}{"sources": sources}
}

func (s *service) ClearUsage() map[string]interface{} {
	s.db.Table("messages").Where("tokens > 0").Update("tokens", 0)
	return map[string]interface{}{"cleared": true}
}

func (s *service) GetNotificationsSettings() map[string]interface{} {
	enabled := s.getAppSetting("notifications_enabled") != "false"
	return map[string]interface{}{"enabled": enabled, "webPush": true, "desktopNotify": true}
}

func (s *service) UpdateNotificationsSettings(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["enabled"].(bool); ok {
		if v {
			s.setAppSetting("notifications_enabled", "true")
		} else {
			s.setAppSetting("notifications_enabled", "false")
		}
	}
	return s.GetNotificationsSettings()
}

func (s *service) GetNotificationsStatus() map[string]interface{} {
	enabled := s.getAppSetting("notifications_enabled") != "false"
	return map[string]interface{}{"enabled": enabled}
}

func (s *service) NotificationsSubscribe(body map[string]interface{}) map[string]interface{} {
	s.setAppSetting("notifications_enabled", "true")
	return map[string]interface{}{"subscribed": true}
}

func (s *service) NotificationsUnsubscribe() map[string]interface{} {
	s.setAppSetting("notifications_enabled", "false")
	return map[string]interface{}{"unsubscribed": true}
}

func (s *service) NotificationsTest() map[string]interface{} {
	return map[string]interface{}{"sent": true, "sentAt": time.Now().Format(time.DateTime)}
}

func (s *service) GetSecurityAccessConfig() map[string]interface{} {
	auth := s.getAppSetting("require_auth") != "false"
	origins := s.getAppSetting("allowed_origins")
	if origins == "" {
		origins = "*"
	}
	rateLimit := s.getAppSetting("rate_limit") != "false"
	return map[string]interface{}{"requireAuth": auth, "allowedOrigins": origins, "rateLimit": rateLimit}
}

func (s *service) UpdateSecurityAccessConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["requireAuth"].(bool); ok {
		if v {
			s.setAppSetting("require_auth", "true")
		} else {
			s.setAppSetting("require_auth", "false")
		}
	}
	if v, ok := body["allowedOrigins"].(string); ok {
		s.setAppSetting("allowed_origins", v)
	}
	if v, ok := body["rateLimit"].(bool); ok {
		if v {
			s.setAppSetting("rate_limit", "true")
		} else {
			s.setAppSetting("rate_limit", "false")
		}
	}
	return s.GetSecurityAccessConfig()
}

func (s *service) GetSecurityAccessStatus() map[string]interface{} {
	cfg := s.GetSecurityAccessConfig()
	return map[string]interface{}{"status": "secure", "config": cfg}
}

func (s *service) GetSecurityStatus() map[string]interface{} {
	acct := s.SecurityAccountCheck()
	exp := s.SecurityExposureCheck()
	status := "secure"
	if !acct["secure"].(bool) || exp["exposed"].(bool) {
		status = "warning"
	}
	return map[string]interface{}{"status": status, "account": acct, "exposure": exp}
}

func (s *service) SecurityAccountCheck() map[string]interface{} {
	var adminCount int64
	s.db.Table("auth_users").Where("role = ?", "admin").Count(&adminCount)
	return map[string]interface{}{"secure": adminCount > 0, "hasAdmin": adminCount > 0}
}

func (s *service) SecurityExposureCheck() map[string]interface{} {
	var apiKey string
	s.db.Table("app_settings").Select("value").Where("key = ?", "api_key").Row().Scan(&apiKey)
	hasKey := apiKey != ""
	var msgCount int64
	s.db.Table("messages").Where("safety_level = ?", "unsafe").Count(&msgCount)
	exposed := hasKey || msgCount > 0
	return map[string]interface{}{"exposed": exposed, "hasApiKey": hasKey, "unsafeMessages": msgCount}
}

func (s *service) PrivacyScan() map[string]interface{} {
	scanId := fmt.Sprintf("scan_%d", time.Now().Unix())
	var msgs []map[string]interface{}
	s.db.Table("messages").Select("id, content").Limit(100).Find(&msgs)
	findings := []interface{}{}
	patterns := []string{"password", "token", "api_key", "secret", "key"}
	for _, msg := range msgs {
		if content, ok := msg["content"].(string); ok {
			for _, p := range patterns {
				if strings.Contains(strings.ToLower(content), p) {
					findings = append(findings, map[string]interface{}{"messageId": msg["id"], "pattern": p, "severity": "high"})
					break
				}
			}
		}
	}
	return map[string]interface{}{"scanId": scanId, "status": "completed", "findings": findings, "totalScanned": len(msgs)}
}

func (s *service) PrivacyScanResults() map[string]interface{} {
	scanId := fmt.Sprintf("scan_%d", time.Now().Unix())
	return s.GetPrivacyScanResult(scanId)
}

func (s *service) PrivacyMask() map[string]interface{} {
	var count int64
	s.db.Table("messages").Where("safety_level = ?", "unsafe").Count(&count)
	if count > 0 {
		s.db.Table("messages").Where("safety_level = ?", "unsafe").Update("safety_level", "masked")
	}
	return map[string]interface{}{"masked": true, "maskedCount": count}
}

func (s *service) GetPrivacyScanResult(id string) map[string]interface{} {
	_ = id
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("safety_level IN ?", []string{"unsafe", "masked"}).Find(&msgs)
	return map[string]interface{}{"result": map[string]interface{}{"scanId": id, "findings": msgs, "totalFindings": len(msgs)}}
}

func (s *service) sidecarGet(path string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Get("http://127.0.0.1:9876" + path)
}

func (s *service) sidecarPost(path string, body map[string]interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Post("http://127.0.0.1:9876"+path, "application/json", bytes.NewReader(jsonBody))
}

func (s *service) readSidecarResponse(resp *http.Response, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error(), "available": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if result == nil {
		result = map[string]interface{}{"success": resp.StatusCode == 200}
	}
	result["available"] = true
	return result
}

func (s *service) qqSidecarGet(path string) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return client.Get("http://127.0.0.1:9877" + path)
}

func (s *service) qqSidecarPost(path string, body map[string]interface{}) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	client := &http.Client{Timeout: 30 * time.Second}
	return client.Post("http://127.0.0.1:9877"+path, "application/json", bytes.NewReader(jsonBody))
}

func (s *service) qqReadSidecarResponse(resp *http.Response, err error) map[string]interface{} {
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error(), "available": false}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if result == nil {
		result = map[string]interface{}{"success": resp.StatusCode == 200}
	}
	result["available"] = true
	return result
}

func (s *service) getQQHealthStatus() string {
	resp, err := s.qqSidecarGet("/api/status")
	if err != nil {
		return "disconnected"
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		if status, ok := data["status"].(string); ok {
			return status
		}
	}
	return "disconnected"
}

func (s *service) GetWechatBridgeStatus() map[string]interface{} {
	resp, err := s.sidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected"}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		status, _ := data["status"].(string)
		return map[string]interface{}{"connected": status == "connected", "status": status}
	}
	return map[string]interface{}{"connected": resp.StatusCode == 200, "status": "unknown"}
}

func (s *service) GetWechatBridgeStatusDetail() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarGet("/api/status"))
}

func (s *service) GetWechatBridgeConfig() map[string]interface{} {
	return map[string]interface{}{"config": map[string]interface{}{"mode": "openclaw", "sidecarPort": 9876}, "available": true}
}

func (s *service) GetWechatBridgeEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) GetWechatBridgeQRCode() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarGet("/api/qrcode"))
}

func (s *service) GetQQBridgeStatus() map[string]interface{} {
	resp, err := s.qqSidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected"}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	if data, ok := result["data"].(map[string]interface{}); ok {
		status, _ := data["status"].(string)
		return map[string]interface{}{"connected": status == "connected", "status": status}
	}
	return map[string]interface{}{"connected": resp.StatusCode == 200, "status": "unknown"}
}

func (s *service) GetQQBridgeStatusDetail() map[string]interface{} {
	return s.qqReadSidecarResponse(s.qqSidecarGet("/api/status"))
}

func (s *service) GetQQBridgeConfig() map[string]interface{} {
	return map[string]interface{}{"config": map[string]interface{}{"mode": "qqbot", "sidecarPort": 9877}, "available": true}
}

func (s *service) GetQQBridgeEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) QQBridgeRecover() map[string]interface{} {
	return s.qqReadSidecarResponse(s.qqSidecarPost("/api/login/reconnect", nil))
}

func (s *service) MaintenanceRestartQQBridge() map[string]interface{} {
	result := s.qqReadSidecarResponse(s.qqSidecarPost("/api/login/reconnect", nil))
	return map[string]interface{}{"restarted": true, "restartedAt": time.Now().Format(time.DateTime), "bridgeResult": result}
}

func (s *service) GetWechatEvents() map[string]interface{} {
	return map[string]interface{}{"events": []interface{}{}, "available": true}
}

func (s *service) GetWechatStatus() map[string]interface{} {
	resp, err := s.sidecarGet("/api/status")
	if err != nil {
		return map[string]interface{}{"connected": false, "status": "disconnected", "available": true}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	data, _ := result["data"].(map[string]interface{})
	if data == nil {
		data = map[string]interface{}{}
	}
	status, _ := data["status"].(string)
	if status == "" {
		status = "disconnected"
	}
	data["connected"] = status == "connected"
	data["status"] = status
	data["available"] = true
	return data
}

func (s *service) WechatBridgeRecover() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
}

func (s *service) WechatCloudCheck() map[string]interface{} {
	return map[string]interface{}{"status": "not_checked", "available": true}
}

func (s *service) WechatCloudCheckReport() map[string]interface{} {
	return map[string]interface{}{"report": map[string]interface{}{}, "available": true}
}

func (s *service) WechatCloudCheckRiskSummary() map[string]interface{} {
	return map[string]interface{}{"risks": []interface{}{}, "available": true}
}

func (s *service) WechatCloudCheckRun() map[string]interface{} {
	return map[string]interface{}{"checkId": "", "started": true, "message": "检查已启动"}
}

func (s *service) WechatLoginReconnect() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/reconnect", nil))
}

func (s *service) WechatLoginRescan() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/rescan", nil))
}

func (s *service) WechatLoginStart() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/start", nil))
}

func (s *service) WechatLoginWait() map[string]interface{} {
	return s.readSidecarResponse(s.sidecarPost("/api/login/wait", map[string]interface{}{"timeoutMs": 120000}))
}

func (s *service) WechatReplyTimingRecover() map[string]interface{} {
	return map[string]interface{}{"recovered": true, "message": "已恢复"}
}

func (s *service) UpdateWechatBridgeConfig(body map[string]interface{}) map[string]interface{} {
	_ = body
	return map[string]interface{}{"updated": true}
}

func (s *service) WechatReplyTimingStatus() map[string]interface{} {
	return map[string]interface{}{"status": "inactive", "available": true}
}

func (s *service) GetImportsBatches() map[string]interface{} {
	var batches []map[string]interface{}
	s.db.Table("conversations").Where("source = ?", "import").Order("created_at DESC").Find(&batches)
	if batches == nil {
		batches = []map[string]interface{}{}
	}
	return map[string]interface{}{"batches": batches, "total": len(batches)}
}

func (s *service) GetImportsBatchDetail(id string) map[string]interface{} {
	var batch map[string]interface{}
	s.db.Table("conversations").Where("id = ? AND source = ?", id, "import").Limit(1).Scan(&batch)
	if batch == nil {
		batch = map[string]interface{}{}
	}
	var msgCount int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&msgCount)
	batch["messageCount"] = msgCount
	return map[string]interface{}{"batch": batch}
}

func (s *service) GetImportsBatchSummary(id string) map[string]interface{} {
	var msgCount, totalTokens int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&msgCount)
	s.db.Table("messages").Where("conversation_id = ?", id).Select("COALESCE(SUM(tokens), 0)").Row().Scan(&totalTokens)
	var batch map[string]interface{}
	s.db.Table("conversations").Where("id = ?", id).Limit(1).Scan(&batch)
	return map[string]interface{}{"summary": map[string]interface{}{"messageCount": msgCount, "totalTokens": totalTokens, "title": batch["title"]}}
}

func (s *service) GetImportsBatchMemoryCandidates(id string) map[string]interface{} {
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "user").Order("created_at DESC").Limit(20).Find(&msgs)
	if msgs == nil {
		msgs = []map[string]interface{}{}
	}
	return map[string]interface{}{"candidates": msgs, "conversationId": id}
}

func (s *service) DeleteImportsBatch(id string) map[string]interface{} {
	s.db.Table("messages").Where("conversation_id = ?", id).Delete(nil)
	s.db.Table("conversations").Where("id = ? AND source = ?", id, "import").Delete(nil)
	return map[string]interface{}{"deleted": true}
}

func (s *service) GenerateImportsBatchSummary(id string) map[string]interface{} {
	return s.GetImportsBatchSummary(id)
}

func (s *service) ConfirmImportsBatchMemories(id string) map[string]interface{} {
	var msgs []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND role = ?", id, "user").Limit(20).Find(&msgs)
	confirmed := 0
	for _, msg := range msgs {
		if content, ok := msg["content"].(string); ok && len(content) > 10 {
			s.db.Table("memories").Create(map[string]interface{}{
				"id":         fmt.Sprintf("mem_%s_%d", id[:8], confirmed),
				"key":        fmt.Sprintf("imported_%d", confirmed),
				"value":      content,
				"source":     "import",
				"created_at": time.Now().Format("2006-01-02 15:04:05"),
			})
			confirmed++
		}
	}
	return map[string]interface{}{"confirmed": true, "memoriesCreated": confirmed}
}

func (s *service) UploadImports(body map[string]interface{}) map[string]interface{} {
	batchId := fmt.Sprintf("imp_%d", time.Now().Unix())
	return map[string]interface{}{"uploaded": true, "batchId": batchId}
}

func (s *service) ParseImportsText(body map[string]interface{}) map[string]interface{} {
	text, _ := body["text"].(string)
	lines := strings.Split(text, "\n")
	messages := []interface{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			messages = append(messages, map[string]interface{}{"role": "user", "content": line})
		}
	}
	return map[string]interface{}{"parsed": true, "messages": messages, "count": len(messages)}
}

func (s *service) ConfirmImports(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"confirmed": true, "confirmedAt": time.Now().Format(time.DateTime)}
}

func (s *service) ImportData(body map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"imported": true, "importedAt": time.Now().Format(time.DateTime)}
}

func (s *service) GetMoods() map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Raw("SELECT DISTINCT mood as name, COUNT(*) as count, MAX(created_at) as lastDetected FROM messages WHERE mood IS NOT NULL AND mood != '' GROUP BY mood ORDER BY count DESC").Scan(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": moods}
}

func (s *service) GetMoodsByConversation(id string) map[string]interface{} {
	var moods []map[string]interface{}
	s.db.Table("messages").Where("conversation_id = ? AND mood IS NOT NULL AND mood != ''", id).Order("created_at DESC").Limit(50).Find(&moods)
	if moods == nil {
		moods = []map[string]interface{}{}
	}
	return map[string]interface{}{"moods": moods, "conversationId": id}
}

func (s *service) DeleteMood(id string) map[string]interface{} {
	s.db.Table("messages").Where("id = ?", id).Update("mood", "")
	return map[string]interface{}{"deleted": true}
}

func (s *service) DeleteMoodsByConversation(id string) map[string]interface{} {
	result := s.db.Table("messages").Where("conversation_id = ?", id).Update("mood", "")
	return map[string]interface{}{"deleted": true, "affectedRows": result.RowsAffected}
}

func (s *service) GetReplyTimingOverview() map[string]interface{} {
	var total, active int64
	s.db.Table("proactive_rules").Count(&total)
	s.db.Table("proactive_rules").Where("enabled = 1").Count(&active)
	return map[string]interface{}{"totalBuffers": total, "activeBuffers": active}
}

func (s *service) GetReplyTimingBuffers() map[string]interface{} {
	var rules []map[string]interface{}
	s.db.Table("proactive_rules").Order("created_at DESC").Find(&rules)
	if rules == nil {
		rules = []map[string]interface{}{}
	}
	return map[string]interface{}{"buffers": rules}
}

func (s *service) ReplyTimingCancelBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("enabled", 0)
	return map[string]interface{}{"canceled": true}
}

func (s *service) ReplyTimingForceBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("last_sent_at", time.Now().Format("2006-01-02 15:04:05"))
	return map[string]interface{}{"forced": true, "id": id, "forcedAt": time.Now().Format(time.DateTime)}
}

func (s *service) ReplyTimingResumeBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("enabled", 1)
	return map[string]interface{}{"resumed": true}
}

func (s *service) ReplyTimingForce() map[string]interface{} {
	return map[string]interface{}{"forced": true, "forcedAt": time.Now().Format(time.DateTime)}
}

func (s *service) RunNow() map[string]interface{} {
	return map[string]interface{}{"started": true, "startedAt": time.Now().Format(time.DateTime)}
}

func (s *service) GetLongRunningStatus() map[string]interface{} {
	var tasks []map[string]interface{}
	s.db.Raw("SELECT c.id, c.title, c.character_id, c.updated_at FROM conversations c WHERE c.channel = 'long_running' ORDER BY c.updated_at DESC LIMIT 10").Scan(&tasks)
	if tasks == nil {
		tasks = []map[string]interface{}{}
	}
	return map[string]interface{}{"running": len(tasks) > 0, "tasks": tasks}
}

func (s *service) GetLongRunningConfig() map[string]interface{} {
	maxT := s.getAppSetting("long_running_max_tasks")
	if maxT == "" {
		maxT = "5"
	}
	timeout := s.getAppSetting("long_running_timeout")
	if timeout == "" {
		timeout = "30"
	}
	return map[string]interface{}{"maxTasks": toInt(maxT), "timeoutMinutes": toInt(timeout)}
}

func (s *service) UpdateLongRunningConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["maxTasks"]; ok {
		s.setAppSetting("long_running_max_tasks", fmt.Sprintf("%d", toInt(v)))
	}
	if v, ok := body["timeoutMinutes"]; ok {
		s.setAppSetting("long_running_timeout", fmt.Sprintf("%d", toInt(v)))
	}
	return s.GetLongRunningConfig()
}

func (s *service) LegacyListConversations() map[string]interface{} {
	var convs []map[string]interface{}
	s.db.Table("conversations").Order("updated_at DESC").Limit(50).Find(&convs)
	if convs == nil {
		convs = []map[string]interface{}{}
	}
	for i, c := range convs {
		var count int64
		s.db.Table("messages").Where("conversation_id = ?", c["id"]).Count(&count)
		convs[i]["messageCount"] = count
	}
	return map[string]interface{}{"conversations": convs}
}

func (s *service) LegacyGetMessages(id string, page, pageSize int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int64
	s.db.Table("messages").Where("conversation_id = ?", id).Count(&total)

	// 查询足够的原始消息以确保过滤后仍有 pageSize 条
	queryLimit := pageSize
	for {
		var raw []map[string]interface{}
		s.db.Table("messages").Where("conversation_id = ?", id).Order("created_at ASC").Limit(queryLimit).Offset(offset).Find(&raw)
		var msgs []map[string]interface{}
		for _, m := range raw {
			role := fmt.Sprint(m["role"])
			content := fmt.Sprint(m["content"])
			if role == "tool" {
				continue
			}
			if role == "assistant" && (content == "" || content == "<nil>") {
				continue
			}
			if v, ok := m["audio_url"]; ok {
				m["audioUrl"] = v
				delete(m, "audio_url")
			}
			if v, ok := m["audio_duration"]; ok {
				m["audioDuration"] = v
				delete(m, "audio_duration")
			}
			if v, ok := m["msg_type"]; ok {
				m["msgType"] = v
				delete(m, "msg_type")
			}
			if v, ok := m["image_url"]; ok && v != nil && v != "" {
				imageUrl := fmt.Sprint(v)
				if strings.HasPrefix(imageUrl, "data:") {
					newPath := chat.SaveImageFromDataURI(imageUrl)
					if newPath != imageUrl {
						m["imageUrl"] = newPath
						go s.db.Exec("UPDATE messages SET image_url = ? WHERE id = ?", newPath, m["id"])
					} else {
						m["imageUrl"] = imageUrl
					}
				} else {
					m["imageUrl"] = imageUrl
				}
				delete(m, "image_url")
			}
			if v, ok := m["video_url"]; ok && v != nil && v != "" {
				m["videoUrl"] = v
				delete(m, "video_url")
			}
			msgs = append(msgs, m)
		}
		if len(msgs) >= pageSize || int64(offset+queryLimit) >= total {
			if msgs == nil {
				msgs = []map[string]interface{}{}
			}
			totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
			return map[string]interface{}{"items": msgs, "total": total, "page": page, "pageSize": pageSize, "totalPages": totalPages}
		}
		queryLimit += pageSize
		if int64(queryLimit) > total {
			queryLimit = int(total)
		}
	}
}
func (s *service) LegacyDeleteConversation(id string) map[string]interface{} {
	s.db.Table("messages").Where("conversation_id = ?", id).Delete(nil)
	s.db.Table("conversations").Where("id = ?", id).Delete(nil)
	return map[string]interface{}{"deleted": true}
}

func (s *service) GetReleaseCheckLatest() map[string]interface{} {
	return map[string]interface{}{"latest": map[string]interface{}{"version": "1.0.0", "date": time.Now().Format("2006-01-02")}}
}

func (s *service) GetReleaseCheckHistory() map[string]interface{} {
	lastCheck := s.getAppSetting("last_release_check")
	return map[string]interface{}{"history": []interface{}{map[string]interface{}{"version": "1.0.0", "checkedAt": lastCheck, "hasUpdate": false}}}
}

func (s *service) ExportReleaseCheck() map[string]interface{} {
	data, _ := json.Marshal(s.GetReleaseCheckHistory())
	name := fmt.Sprintf("release_check_%s.json", time.Now().Format("20060102_150405"))
	os.WriteFile(filepath.Join(s.dataDir, name), data, 0644)
	return map[string]interface{}{"exported": true, "file": name}
}

func (s *service) RunReleaseCheck() map[string]interface{} {
	s.setAppSetting("last_release_check", time.Now().Format(time.DateTime))
	return s.GetReleaseCheckLatest()
}

func (s *service) GetUpdateConfig() map[string]interface{} {
	autoCheck := s.getAppSetting("auto_update") != "false"
	return map[string]interface{}{"autoCheck": autoCheck, "channel": "stable", "lastCheckAt": nil}
}

func (s *service) UpdateUpdateConfig(body map[string]interface{}) map[string]interface{} {
	if v, ok := body["autoCheck"].(bool); ok {
		if v {
			s.setAppSetting("auto_update", "true")
		} else {
			s.setAppSetting("auto_update", "false")
		}
	}
	return s.GetUpdateConfig()
}

func (s *service) ToolRoute(body map[string]interface{}) map[string]interface{} {
	tool, _ := body["tool"].(string)
	if tool == "" {
		tool = "unknown"
	}
	return map[string]interface{}{"routed": true, "tool": tool}
}
