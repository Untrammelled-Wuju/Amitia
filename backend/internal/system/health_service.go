// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"runtime"
	"time"
)

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
		"wechat_running": s.isWechatSidecarRunning(), "qq_running": s.isQQSidecarRunning(),
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
