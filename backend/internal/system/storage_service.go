// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	amitialog "log"
)

func (s *service) GetStorageInfo() map[string]interface{} {
	dir := s.dataDir
	var totalSize int64
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	}); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}

	dbPath := filepath.Join(dir, "app.db")
	var dbSize int64
	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		dbSize = info.Size()
	}

	result := map[string]interface{}{
		"totalMB":           totalSize / 1024 / 1024,
		"usedMB":            totalSize / 1024 / 1024,
		"freeMB":            0,
		"path":              dir,
		"dbSize":            formatStorageBytes(dbSize),
		"dbSizeBytes":       dbSize,
		"messageCount":      s.storageTableCount("messages"),
		"conversationCount": s.storageTableCount("conversations"),
	}
	if s.memoryConsumer == nil {
		result["memoryError"] = "统一记忆统计入口不可用"
		return result
	}
	entries, err := s.memoryConsumer.Read(context.Background(), "", "memory", true)
	if err != nil {
		result["memoryError"] = err.Error()
		return result
	}
	result["memoryCount"] = len(entries)
	return result
}

func (s *service) ownedMemoryUsage() (map[string]int64, error) {
	if s.memoryConsumer == nil {
		return nil, fmt.Errorf("统一记忆统计入口不可用")
	}
	entries, err := s.memoryConsumer.Read(context.Background(), "", "memory", true)
	if err != nil {
		return nil, err
	}
	counts := map[string]int64{}
	for _, entry := range entries {
		document, content, err := business.MemoryContent(entry)
		if err != nil {
			return nil, err
		}
		created := business.MemoryText(content, "createdAt")
		if created == "" {
			created = business.MemoryText(document, "createdAt")
		}
		if len(created) >= 10 {
			counts[created[:10]]++
		}
	}
	return counts, nil
}

func (s *service) storageTableCount(table string) int64 {
	if s.db == nil || !s.db.Migrator().HasTable(table) {
		return 0
	}
	var count int64
	if err := s.db.Table(table).Count(&count).Error; err != nil {
		return 0
	}
	return count
}

func formatStorageBytes(size int64) string {
	const (
		kb = int64(1024)
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case size >= gb:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func (s *service) GetStorageMigrations() map[string]interface{} {
	legacyVersion, hasLegacy := s.readLegacyMigrationVersion()
	records, err := s.readSchemaMigrationRecords()
	if err == nil && len(records) > 0 {
		migrations := make([]interface{}, 0, len(records))
		currentVersion := "0"
		appliedCount := 0
		for _, record := range records {
			applied := record.Status == "applied"
			if applied {
				appliedCount++
				if record.Version > currentVersion {
					currentVersion = record.Version
				}
			}
			migrations = append(migrations, map[string]interface{}{
				"name":       record.Name,
				"version":    record.Version,
				"status":     record.Status,
				"applied":    applied,
				"startedAt":  record.StartedAt,
				"finishedAt": record.FinishedAt,
				"source":     "schema_migrations",
			})
		}
		result := map[string]interface{}{
			"migrations":      migrations,
			"currentVersion":  currentVersion,
			"totalMigrations": len(records),
			"appliedCount":    appliedCount,
			"pendingCount":    len(records) - appliedCount,
			"source":          "schema_migrations",
		}
		if hasLegacy {
			result["legacyVersion"] = legacyVersion
		}
		return result
	}
	return legacyMigrationResult(legacyVersion, hasLegacy)
}

func (s *service) CheckStorageMigrations() map[string]interface{} {
	legacyVersion, hasLegacy := s.readLegacyMigrationVersion()
	records, err := s.readSchemaMigrationRecords()
	if err == nil && len(records) > 0 {
		needsMigration := false
		for _, record := range records {
			if record.Status != "applied" {
				needsMigration = true
				break
			}
		}
		result := map[string]interface{}{"needsMigration": needsMigration, "source": "schema_migrations"}
		if hasLegacy {
			result["legacyVersion"] = legacyVersion
		}
		return result
	}
	return map[string]interface{}{"needsMigration": !hasLegacy, "source": "legacy_file", "legacyVersion": legacyVersion}
}

type storageMigrationRecord struct {
	Version    string `gorm:"column:version"`
	Name       string `gorm:"column:name"`
	Status     string `gorm:"column:status"`
	StartedAt  string `gorm:"column:started_at"`
	FinishedAt string `gorm:"column:finished_at"`
}

func (s *service) readSchemaMigrationRecords() ([]storageMigrationRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var tableCount int64
	if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", "schema_migrations").Scan(&tableCount).Error; err != nil {
		return nil, err
	}
	if tableCount == 0 {
		return nil, fmt.Errorf("schema_migrations table not found")
	}
	var records []storageMigrationRecord
	err := s.db.Table("schema_migrations").Select("version, name, status, started_at, finished_at").Order("version ASC").Scan(&records).Error
	return records, err
}

func (s *service) readLegacyMigrationVersion() (string, bool) {
	data, err := os.ReadFile(filepath.Join(s.dataDir, ".migration_version"))
	if err != nil {
		return "0", false
	}
	return strings.TrimSpace(string(data)), true
}

func legacyMigrationResult(version string, applied bool) map[string]interface{} {
	return map[string]interface{}{
		"migrations": []interface{}{map[string]interface{}{
			"name":    "initial",
			"version": version,
			"applied": applied,
			"source":  "legacy_file",
		}},
		"currentVersion":  version,
		"totalMigrations": 1,
		"appliedCount":    boolToInt(applied),
		"pendingCount":    boolToInt(!applied),
		"source":          "legacy_file",
		"legacyVersion":   version,
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
