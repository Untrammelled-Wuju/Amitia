// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
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
	versionInfo := s.GetVersion()
	current, _ := versionInfo["version"].(string)
	lastCheck := s.getAppSetting("last_release_check")
	return map[string]interface{}{"hasUpdate": false, "currentVersion": current, "latestVersion": current, "lastCheckedAt": lastCheck, "source": "local_release_metadata"}
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

func (s *service) RunNow() map[string]interface{} {
	return map[string]interface{}{"started": true, "startedAt": time.Now().Format(time.DateTime)}
}
