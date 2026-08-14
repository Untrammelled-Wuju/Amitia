// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/system/dataportability"
)

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

func (s *service) CreatePhysicalSafetySnapshot() map[string]interface{} {
	backupDir := filepath.Join(s.dataDir, "backups")
	os.MkdirAll(backupDir, 0755)
	name := fmt.Sprintf("safety_%s.db", time.Now().Format("20060102_150405"))
	src := filepath.Join(s.dataDir, "app.db")
	srcData, err := os.ReadFile(src)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	err = os.WriteFile(filepath.Join(backupDir, name), srcData, 0644)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"snapshotName": name, "sizeMB": int64(len(srcData)) / 1024 / 1024}
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

func (s *service) RestorePhysicalSafetySnapshot(name string) map[string]interface{} {
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


func sanitizeName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(s)
}

func (s *service) StorageExportAmitia(scope string, characterID string) map[string]interface{} {
	coord := s.coordinator
	if coord == nil {
		return map[string]interface{}{"exported": false, "error": "coordinator not initialized"}
	}

	var dbPath string
	s.db.Raw("PRAGMA database_list").Row().Scan(nil, nil, &dbPath)
	if dbPath == "" {
		dbPath = filepath.Join(s.dataDir, "app.db")
	}

	bkpScope := dataportability.ScopeAll
	if scope == "character" {
		bkpScope = dataportability.ScopeCharacter
	}

	req := dataportability.BackupRequest{
		Scope:       bkpScope,
		Profile:     dataportability.ProfileFull,
		CharacterID: characterID,
		Purpose:     dataportability.PurposeUser,
	}

	runner := dataportability.NewSQLiteAdapter(s.db, filepath.Join(s.dataDir, "backups"))
	result, err := coord.CreateBackup(context.Background(), req, dbPath, runner)
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}

	return map[string]interface{}{
		"exported": true,
		"file":     filepath.Base(result.Path),
		"size":     result.SizeBytes,
		"sizeKB":   result.SizeBytes / 1024,
	}
}

func (s *service) StorageImportUserData(body map[string]interface{}) map[string]interface{} {
	coord := s.coordinator
	if coord == nil {
		return map[string]interface{}{"imported": false, "error": "coordinator not initialized"}
	}

	fileName, _ := body["fileName"].(string)
	if fileName == "" {
		return map[string]interface{}{"imported": false, "error": "missing fileName"}
	}

	archivePath := filepath.Join(s.dataDir, "exports", fileName)

	preview, err := coord.PreviewImport(context.Background(), archivePath)
	if err != nil {
		return map[string]interface{}{"imported": false, "error": err.Error()}
	}

	importReq := dataportability.ImportRequest{
		CharacterPolicy: dataportability.CollisionReplace,
	}

	_, err = coord.ExecuteImport(context.Background(), archivePath, importReq)
	if err != nil {
		return map[string]interface{}{"imported": false, "error": err.Error()}
	}

	return map[string]interface{}{
		"imported":       true,
		"fileName":       fileName,
		"backupId":       preview.Manifest.BackupID,
		"appVersion":     preview.Manifest.AppVersion,
		"schemaFinger":   preview.Manifest.SchemaFingerprint,
		"componentCount": len(preview.Components),
	}
}
