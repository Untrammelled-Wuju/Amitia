// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

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
	if !s.isWechatSidecarRunning() {
		issues = append(issues, map[string]interface{}{"type": "WECHAT", "msg": "微信侧车未启动"})
	} else if s.getWechatHealthStatus() != "connected" {
		issues = append(issues, map[string]interface{}{"type": "WECHAT", "msg": "微信 Bridge 未连接"})
	}
	if !s.isQQSidecarRunning() {
		issues = append(issues, map[string]interface{}{"type": "QQ", "msg": "QQ侧车未启动"})
	} else if s.getQQHealthStatus() != "connected" {
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
	bridgeRunning := s.isWechatSidecarRunning()
	bridgeOk := bridgeRunning && bridgeStatus == "connected"
	bridgeCheck := map[string]interface{}{"name": "微信 Bridge", "pass": bridgeOk}
	if !bridgeOk {
		if !bridgeRunning {
			bridgeCheck["error"] = "微信侧车未启动"
		} else {
			bridgeCheck["error"] = "Bridge 状态: " + bridgeStatus
		}
		allPassed = false
	}
	checks = append(checks, bridgeCheck)
	qqBridgeStatus := s.getQQHealthStatus()
	qqBridgeRunning := s.isQQSidecarRunning()
	qqBridgeOk := qqBridgeRunning && qqBridgeStatus == "connected"
	qqBridgeCheck := map[string]interface{}{"name": "QQ Bridge", "pass": qqBridgeOk}
	if !qqBridgeOk {
		if !qqBridgeRunning {
			qqBridgeCheck["error"] = "QQ侧车未启动"
		} else {
			qqBridgeCheck["error"] = "QQ Bridge 状态: " + qqBridgeStatus
		}
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
