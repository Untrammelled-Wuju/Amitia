// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	amitialog "github.com/u-ai/backend/log"
)

type databaseIntegrityIssue struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

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
	issues := make([]databaseIntegrityIssue, 0)
	errors := make([]string, 0)
	record := func(kind string, err error) {
		message := err.Error()
		issues = append(issues, databaseIntegrityIssue{Type: kind, Message: message})
		errors = append(errors, message)
	}
	if s.db == nil {
		record("database", fmt.Errorf("数据库连接不可用"))
	} else if sqlDB, err := s.db.DB(); err != nil {
		record("connection", fmt.Errorf("获取数据库连接失败: %w", err))
	} else if err := sqlDB.PingContext(context.Background()); err != nil {
		record("connection", fmt.Errorf("数据库连接检查失败: %w", err))
	} else if s.db.Dialector.Name() == "sqlite" {
		var integrityRows []struct {
			Result string `gorm:"column:integrity_check"`
		}
		if err := s.db.WithContext(context.Background()).Raw("PRAGMA integrity_check").Scan(&integrityRows).Error; err != nil {
			record("integrity", fmt.Errorf("执行 SQLite integrity_check 失败: %w", err))
		} else if len(integrityRows) == 0 {
			record("integrity", fmt.Errorf("SQLite integrity_check 未返回结果"))
		} else {
			for _, row := range integrityRows {
				if row.Result != "ok" {
					record("integrity", fmt.Errorf("SQLite integrity_check 返回 %s", row.Result))
				}
			}
		}
		var foreignKeyRows []struct {
			Table  string `gorm:"column:table"`
			RowID  int64  `gorm:"column:rowid"`
			Parent string `gorm:"column:parent"`
			FKID   int64  `gorm:"column:fkid"`
		}
		if err := s.db.WithContext(context.Background()).Raw("PRAGMA foreign_key_check").Scan(&foreignKeyRows).Error; err != nil {
			record("foreign_key", fmt.Errorf("执行 SQLite foreign_key_check 失败: %w", err))
		} else {
			for _, row := range foreignKeyRows {
				record("foreign_key", fmt.Errorf("外键约束失败: table=%s rowid=%d parent=%s fkid=%d", row.Table, row.RowID, row.Parent, row.FKID))
			}
		}
	} else {
		record("integrity", fmt.Errorf("数据库方言 %s 不支持本地完整性检查", s.db.Dialector.Name()))
	}
	status := "ok"
	if len(errors) > 0 {
		status = "degraded"
	}
	return map[string]interface{}{"ok": len(errors) == 0, "errors": errors, "status": status, "issues": issues}
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
	current, err := amitialog.RotateNow()
	if err != nil {
		return map[string]interface{}{"rotated": []string{}, "skipped": []string{}, "error": err.Error()}
	}
	return map[string]interface{}{"rotated": []string{current}, "skipped": []string{}}
}

func (s *service) RunNow() map[string]interface{} {
	return map[string]interface{}{"started": true, "startedAt": time.Now().Format(time.DateTime)}
}
