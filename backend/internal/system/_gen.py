
"""Generate system/handler.go"""
import os

def W(f, s=""):
    f.write(s + "\n")

def gen():
    p = os.path.dirname(os.path.abspath(__file__))
    out = os.path.join(p, "handler.go")
    
    with open(out, "w", encoding="utf-8") as f:
        # Package & imports
        W(f, 'package system')
        W(f)
        W(f, 'import (')
        W(f, '\t"bytes"')
        W(f, '\t"encoding/json"')
        W(f, '\t"fmt"')
        W(f, '\t"io"')
        W(f, '\t"net/http"')
        W(f, '\t"os"')
        W(f, '\t"path/filepath"')
        W(f, '\t"strings"')
        W(f, '\t"sync/atomic"')
        W(f, '\t"time"')
        W(f)
        W(f, '\t"github.com/gin-gonic/gin"')
        W(f, '\t"github.com/google/uuid"')
        W(f, '\t"github.com/u-ai/backend/pkg/comment/response"')
        W(f, '\t"github.com/u-ai/backend/pkg/util"')
        W(f, '\t"gorm.io/gorm"')
        W(f, ')')
        W(f)

        # Handler struct
        W(f, 'type Handler struct {')
        W(f, '\tservice     Service')
        W(f, '\tdb          *gorm.DB')
        W(f, '\tversionInfo atomic.Value')
        W(f, '}')
        W(f)
        
        W(f, 'func NewHandler(srv Service, db *gorm.DB) *Handler {')
        W(f, '\th := &Handler{service: srv, db: db}')
        W(f, '\th.versionInfo.Store(map[string]interface{}{')
        W(f, '\t\t"version":    "1.0.0-go",')
        W(f, '\t\t"buildTime":  "",')
        W(f, '\t\t"goVersion":  "go1.21+",')
        W(f, '\t\t"deployMode": "desktop-local",')
        W(f, '\t})')
        W(f, '\treturn h')
        W(f, '}')
        W(f)

        # === Simple delegations ===
        W(f, '// ======================== 核心诊断 ========================')
        methods_simple = [
            ("Health", "h.service.Health()"),
            ("Diagnostics", "h.service.Diagnostics()"),
            ("RunDiagnostics", "h.service.RunDiagnostics()"),
            ("AppConfig", "h.service.AppConfig()"),
            ("SetupStatus", "h.service.SetupStatus()"),
            ("OnboardingStatus", "h.service.OnboardingStatus()"),
            ("GetTheme", "h.service.GetTheme()"),
            ("GetLLMConfig", "h.service.GetLLMConfig()"),
            ("LoginHistory", "h.service.GetLoginHistory()"),
            ("RecoveryCodesStatus", "h.service.GetRecoveryCodesStatus()"),
            ("SessionSettings", "h.service.GetSessionSettings()"),
            ("RuntimeHealth", "h.service.GetRuntimeHealth()"),
            ("CheckDBIntegrity", "h.service.CheckDBIntegrity()"),
            ("ValidateMode", "h.service.ValidateMode()"),
            ("AuditActions", "h.service.GetAuditActions()"),
            ("AuditSettings", "h.service.GetAuditSettings()"),
            ("AuditStats", "h.service.GetAuditStats()"),
            ("WechatBridgeStatus", "h.service.GetWechatBridgeStatus()"),
            ("NotificationsSettings", "h.service.GetNotificationsSettings()"),
            ("NotificationsStatus", "h.service.GetNotificationsSettings()"),
            ("SecurityAccessConfig", "h.service.GetSecurityAccessConfig()"),
            ("SecurityAccessStatus", "h.service.GetSecurityAccessConfig()"),
            ("GetUpdateConfig", "h.service.GetUpdateConfig()"),
        ]
        for name, call in methods_simple:
            W(f, f'func (h *Handler) {name}(c *gin.Context) {{ util.SuccessResponse(c, {call}) }}')
        W(f)
        
        W(f, 'func (h *Handler) CurrentSession(c *gin.Context) {')
        W(f, '\ttoken := c.GetHeader("Authorization")')
        W(f, '\tutil.SuccessResponse(c, h.service.GetCurrentSession(token))')
        W(f, '}')
        W(f)

        # === Theme / LLM / Settings Updates ===
        W(f, '// ======================== 配置更新 ========================')
        for name, svcCall in [
            ("UpdateTheme", "h.service.UpdateTheme(body)"),
            ("UpdateLLMConfig", "h.service.UpdateLLMConfig(body)"),
            ("UpdateSessionSettings", "h.service.UpdateSessionSettings(body)"),
            ("UpdateAuditSettings", "h.service.UpdateAuditSettings(body)"),
            ("UpdateWechatBridgeConfig", "h.service.UpdateWechatBridgeConfig(body)"),
            ("UpdateNotificationsSettings", "h.service.UpdateNotificationsSettings(body)"),
            ("UpdateSecurityAccessConfig", "h.service.UpdateSecurityAccessConfig(body)"),
        ]:
            W(f, f'func (h *Handler) {name}(c *gin.Context) {{')
            W(f, '\tvar body map[string]interface{}')
            W(f, '\tc.ShouldBindJSON(&body)')
            W(f, f'\tutil.SuccessResponse(c, {svcCall})')
            W(f, '}')
            W(f)

        # === Safety Check ===
        W(f, 'func (h *Handler) CheckInputSafety(c *gin.Context) {')
        bt_s = '`json:"text"`'
        W(f, f'\tvar body struct{{ Text string {bt_s} }}')
        W(f, '\tif err := c.ShouldBindJSON(&body); err != nil || body.Text == "" { util.ErrorResponse(c, response.InvalidParams, "请输入要检查的文本", nil); return }')
        W(f, '\tutil.SuccessResponse(c, h.service.CheckSafety(body.Text))')
        W(f, '}')
        W(f)
        W(f, 'func (h *Handler) CheckOutputSafety(c *gin.Context) {')
        W(f, f'\tvar body struct{{ Text string {bt_s} }}')
        W(f, '\tif err := c.ShouldBindJSON(&body); err != nil || body.Text == "" { util.ErrorResponse(c, response.InvalidParams, "请输入要检查的AI回复", nil); return }')
        W(f, '\tutil.SuccessResponse(c, h.service.CheckSafety(body.Text))')
        W(f, '}')
        W(f)

        # === Setup / Onboarding ===
        W(f, '// ======================== 设置/引导 ========================')
        W(f, 'func (h *Handler) SetupChecks(c *gin.Context) {')
        bt_n = '`json:"name"`'; bt_o = '`json:"ok"`'; bt_d = '`json:"detail,omitempty"`'
        W(f, f'\ttype check struct{{ Name string {bt_n}; OK bool {bt_o}; Detail string {bt_d} }}')
        W(f, '\tchecks := []check{{Name: "database", OK: h.db != nil}}')
        W(f, '\tif sqlDB, err := h.db.DB(); err == nil { checks[0].OK = sqlDB.Ping() == nil }')
        W(f, '\tvar modelCount int64; h.db.Table("model_configs").Count(&modelCount)')
        W(f, '\tchecks = append(checks, check{Name: "model", OK: modelCount > 0, Detail: fmt.Sprintf("%d 个模型", modelCount)})')
        W(f, '\tvar charCount int64; h.db.Table("characters").Count(&charCount)')
        W(f, '\tchecks = append(checks, check{Name: "character", OK: charCount > 0, Detail: fmt.Sprintf("%d 个角色", charCount)})')
        W(f, '\tutil.SuccessResponse(c, gin.H{"checks": checks})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) OnboardingComplete(c *gin.Context) {')
        bt_step = '`json:"step"`'
        W(f, f'\tvar body struct{{ Step string {bt_step} }}')
        W(f, '\tc.ShouldBindJSON(&body)')
        W(f, '\th.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('onboarding_completed', 'true', datetime('now')) ON CONFLICT(key) DO UPDATE SET value = 'true', updated_at = datetime('now')")')
        W(f, '\tif body.Step != "" { h.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('onboarding_step', ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", body.Step, body.Step) }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"completed": true, "step": body.Step})')
        W(f, '}')
        W(f)

        for name, sql in [
            ("SetupFinish", "setup_completed"),
        ]:
            W(f, f'func (h *Handler) {name}(c *gin.Context) {{')
            W(f, f'\th.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('{sql}', 'true', datetime('now')) ON CONFLICT(key) DO UPDATE SET value = 'true', updated_at = datetime('now')")')
            W(f, '\tutil.SuccessResponse(c, gin.H{"completed": true})')
            W(f, '}')
            W(f)

        W(f, 'func (h *Handler) SetupReset(c *gin.Context) {')
        W(f, '\th.db.Exec("DELETE FROM app_settings WHERE key IN ('setup_completed', 'onboarding_completed', 'onboarding_step')")')
        W(f, '\tutil.SuccessResponse(c, gin.H{"reset": true})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) SetupStep(c *gin.Context) {')
        W(f, '\tvar step string')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "onboarding_step").Row().Scan(&step)')
        W(f, '\tif step == "" { step = "done" }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"step": step})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) OnboardingReset(c *gin.Context) {')
        W(f, '\th.db.Exec("DELETE FROM app_settings WHERE key IN ('onboarding_completed', 'onboarding_step', 'setup_completed')")')
        W(f, '\tutil.SuccessResponse(c, gin.H{"reset": true})')
        W(f, '}')
        W(f)

        # === Tool Route ===
        W(f, 'func (h *Handler) ToolRoute(c *gin.Context) {')
        bt_tool = '`json:"tool"`'; bt_args = '`json:"args"`'
        W(f, f'\tvar body struct{{ Tool string {bt_tool}; Args map[string]interface{{}} {bt_args} }}')
        W(f, '\tif err := c.ShouldBindJSON(&body); err != nil { util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil); return }')
        W(f, '\tresult := map[string]interface{}{"tool": body.Tool, "handled": false}')
        W(f, '\tswitch body.Tool {')
        W(f, '\tcase "normal_chat": result["handled"] = true; result["action"] = "process_chat"')
        W(f, '\tcase "get_time": result["handled"] = true; result["time"] = time.Now().Format("2006-01-02 15:04:05")')
        W(f, '\tcase "get_weather": result["handled"] = true; result["weather"] = "晴天，25°C"')
        W(f, '\tcase "send_proactive": result["handled"] = true; result["action"] = "send_proactive_message"')
        W(f, '\tdefault: result["message"] = "未知工具: " + body.Tool')
        W(f, '\t}')
        W(f, '\tutil.SuccessResponse(c, result)')
        W(f, '}')
        W(f)

        # === Runtime / Mode / Config ===
        W(f, '// ======================== 运行时/模式/配置 ========================')
        
        setups = {
            "GetRuntimeMode": ('mode, startupMode', '"runtime_mode"', '"startup_mode"', '"desktop-local"'),
            "LongRunningConfig": ('maxRuntime, pollInterval', '"long_running_max_minutes"', '"long_running_poll_ms"', ''),
        }
        
        W(f, 'func (h *Handler) GetRuntimeMode(c *gin.Context) {')
        W(f, '\tvar mode, startupMode string')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "runtime_mode").Row().Scan(&mode)')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "startup_mode").Row().Scan(&startupMode)')
        W(f, '\tif mode == "" { mode = "desktop-local" }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"mode": mode, "startupMode": startupMode})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) UpdateRuntimeMode(c *gin.Context) {')
        bt_mode = '`json:"mode"`'
        W(f, f'\tvar body struct{{ Mode string {bt_mode} }}')
        W(f, '\tc.ShouldBindJSON(&body)')
        W(f, '\tif body.Mode != "" { h.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('runtime_mode', ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", body.Mode, body.Mode) }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"updated": true, "mode": body.Mode})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) LongRunningConfig(c *gin.Context) {')
        W(f, '\tvar maxRuntime, pollInterval string')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "long_running_max_minutes").Row().Scan(&maxRuntime)')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "long_running_poll_ms").Row().Scan(&pollInterval)')
        W(f, '\tif maxRuntime == "" { maxRuntime = "60" }')
        W(f, '\tif pollInterval == "" { pollInterval = "5000" }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"maxRuntimeMinutes": maxRuntime, "pollIntervalMs": pollInterval})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) UpdateLongRunningConfig(c *gin.Context) {')
        bt_mr = '`json:"maxRuntimeMinutes"`'; bt_pi = '`json:"pollIntervalMs"`'
        W(f, f'\tvar body struct{{ MaxRuntimeMinutes int {bt_mr}; PollIntervalMs int {bt_pi} }}')
        W(f, '\tif err := c.ShouldBindJSON(&body); err != nil { util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil); return }')
        W(f, '\tif body.MaxRuntimeMinutes > 0 { h.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('long_running_max_minutes', ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", fmt.Sprint(body.MaxRuntimeMinutes), fmt.Sprint(body.MaxRuntimeMinutes)) }')
        W(f, '\tif body.PollIntervalMs > 0 { h.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('long_running_poll_ms', ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", fmt.Sprint(body.PollIntervalMs), fmt.Sprint(body.PollIntervalMs)) }')
        W(f, '\th.LongRunningConfig(c)')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) LongRunningStatus(c *gin.Context) {')
        W(f, '\tvar tasks []map[string]interface{}')
        W(f, '\th.db.Table("long_running_tasks").Where("status = 'running'").Find(&tasks)')
        W(f, '\tif tasks == nil { tasks = []map[string]interface{}{} }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"running": len(tasks) > 0, "currentTasks": tasks})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) UpdateConfig(c *gin.Context) {')
        W(f, '\tvar body map[string]interface{}')
        W(f, '\tif err := c.ShouldBindJSON(&body); err != nil { util.ErrorResponse(c, response.InvalidParams, "无效请求体", nil); return }')
        W(f, '\tfor k, v := range body { val := fmt.Sprint(v); h.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", k, val, val) }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"updated": true, "reloadNeeded": false})')
        W(f, '}')
        W(f)

        W(f, 'func (h *Handler) MoodDetectionConfig(c *gin.Context) {')
        W(f, '\tvar enabled, interval, threshold string')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "mood_detection_enabled").Row().Scan(&enabled)')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "mood_detection_interval").Row().Scan(&interval)')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "mood_detection_threshold").Row().Scan(&threshold)')
        W(f, '\tif enabled == "" { enabled = "true" }; if interval == "" { interval = "60" }; if threshold == "" { threshold = "0.7" }')
        W(f, '\tutil.SuccessResponse(c, gin.H{"enabled": enabled == "true", "intervalMinutes": interval, "threshold": threshold})')
        W(f, '}')
        W(f)

        # === Runtime Health / Status ===
        W(f, 'func (h *Handler) RuntimeStatus(c *gin.Context)   { util.SuccessResponse(c, h.service.Health()) }')
        W(f, 'func (h *Handler) HealthHistory(c *gin.Context)   { util.SuccessResponse(c, h.service.Health()) }')
        W(f, 'func (h *Handler) CheckNow(c *gin.Context) {')
        W(f, '\tsqlDB, err := h.db.DB(); dbOK := err == nil && sqlDB != nil && sqlDB.Ping() == nil')
        W(f, '\tutil.SuccessResponse(c, gin.H{"status": map[bool]string{true: "ok", false: "error"}[dbOK], "database": dbOK})')
        W(f, '}')
        W(f)
        W(f, 'func (h *Handler) CleanupTemp(c *gin.Context) {')
        W(f, '\tcleaned := 0')
        W(f, '\tfor _, dir := range []string{filepath.Join("data", "temp"), filepath.Join("data", "uploads")} {')
        W(f, '\t\tif entries, err := os.ReadDir(dir); err == nil {')
        W(f, '\t\t\tfor _, e := range entries {')
        W(f, '\t\t\t\tif info, err := e.Info(); err == nil && info.ModTime().Before(time.Now().Add(-24*time.Hour)) {')
        W(f, '\t\t\t\t\tos.Remove(filepath.Join(dir, e.Name())); cleaned++')
        W(f, '\t\t\t\t}')
        W(f, '\t\t\t}')
        W(f, '\t\t}')
        W(f, '\t}')
        W(f, '\tutil.SuccessResponse(c, gin.H{"cleaned": cleaned})')
        W(f, '}')
        W(f)
        W(f, 'func (h *Handler) RotateLogs(c *gin.Context) {')
        W(f, '\trotated := false')
        W(f, '\tif info, err := os.Stat("data/logs/app.log"); err == nil && info.Size() > 10*1024*1024 {')
        W(f, '\t\tos.Rename("data/logs/app.log", fmt.Sprintf("data/logs/app.%s.log", time.Now().Format("20060102_150405")))')
        W(f, '\t\tos.WriteFile("data/logs/app.log", []byte{}, 0644)')
        W(f, '\t\trotated = true')
        W(f, '\t}')
        W(f, '\tutil.SuccessResponse(c, gin.H{"rotated": rotated})')
        W(f, '}')
        W(f)

        # === Version / Update ===
        W(f, 'func (h *Handler) Version(c *gin.Context) {')
        W(f, '\tinfo := h.versionInfo.Load().(map[string]interface{})')
        W(f, '\tvar lastCheck string')
        W(f, '\th.db.Table("app_settings").Select("value").Where("key = ?", "last_update_check").Row().Scan(&lastCheck)')
        W(f, '\tinfo["lastCheck"] = lastCheck')
        W(f, '\tutil.SuccessResponse(c, info)')
        W(f, '}')
        W(f)
        W(f, 'func (h *Handler) UpdateCheck(c *gin.Context) {')
        W(f, '\tvi := h.versionInfo.Load().(map[string]interface{})')
        W(f, '\tnow := time.Now().Format("2006-01-02 15:04:05")')
        W(f, '\th.db.Exec("INSERT INTO app_settings (key, value, updated_at) VALUES ('last_update_check', ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = datetime('now')", now, now)')
        W(f, '\tutil.SuccessResponse(c, gin.H{"updateAvailable": false, "currentVersion": vi["version"], "latestVersion": vi["version"], "lastCheck": now})')
        W(f, '}')
        W(f)
        W(f, 'func (h *Handler) UpdateConfig_Update(c *gin.Context) { h.UpdateCheck(c) }')
        W(f)

        print("Phase 1-2 done (core + config)")
        return out

if __name__ == "__main__":
    gen()
