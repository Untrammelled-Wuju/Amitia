// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (s *service) GetRuntimeStatus() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	pid := os.Getpid()
	return map[string]interface{}{
		"status":         "running",
		"pid":            pid,
		"runtimeProfile": s.runtimeProfile.String(),
		"memory":         map[string]interface{}{"rssMB": memStats.Alloc / 1024 / 1024},
		"cpu":            runtime.NumCPU(),
		"uptime":         int(time.Since(s.startTime).Seconds()),
	}
}

func (s *service) GetRuntimeHealth() map[string]interface{} {
	result := s.Health()
	result["runtimeProfile"] = s.runtimeProfile.String()
	return result
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
	return map[string]interface{}{
		"mode":           mode,
		"runtimeProfile": s.runtimeProfile.String(),
	}
}

func (s *service) UpdateRuntimeMode(body map[string]interface{}) map[string]interface{} {
	if _, ok := body["runtimeProfile"]; ok {
		return map[string]interface{}{
			"mode":            s.getAppSetting("runtime_mode"),
			"runtimeProfile":  s.runtimeProfile.String(),
			"requiresRestart": true,
		}
	}
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
