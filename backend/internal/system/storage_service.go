// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

func (s *service) GetStorageInfo() map[string]interface{} {
	dir := s.dataDir
	var totalSize int64
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

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
	return s.CreatePhysicalSafetySnapshotContext(context.Background())
}

func (s *service) CreatePhysicalSafetySnapshotContext(ctx context.Context) map[string]interface{} {
	if s.coordinator == nil {
		return map[string]interface{}{"ok": false, "error": "统一备份入口不可用"}
	}
	runner := dataportability.NewSQLiteAdapter(s.db, filepath.Join(s.dataDir, "backups"))
	result, err := s.coordinator.CreateBackup(ctx, dataportability.BackupRequest{Scope: dataportability.ScopeAll, Profile: dataportability.ProfileFull, Purpose: dataportability.PurposeUser}, filepath.Join(s.dataDir, "app.db"), runner)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	name := filepath.Base(result.Path)
	return map[string]interface{}{"ok": true, "name": name, "snapshotName": name, "size": result.SizeBytes, "sizeMB": result.SizeBytes / 1024 / 1024, "createdAt": time.Now().Format(time.DateTime)}
}

func (s *service) DeleteStorageBackup(name string) map[string]interface{} {
	clean := filepath.Base(name)
	if clean == "." || clean == "" || clean != name {
		return map[string]interface{}{"deleted": false, "error": "invalid backup name"}
	}
	path := filepath.Join(s.dataDir, "backups", clean)
	if err := os.Remove(path); err != nil {
		return map[string]interface{}{"deleted": false, "error": err.Error()}
	}
	return map[string]interface{}{"deleted": true, "name": clean}
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
	return s.RestorePhysicalSafetySnapshotContext(context.Background(), name)
}

func (s *service) RestorePhysicalSafetySnapshotContext(ctx context.Context, name string) map[string]interface{} {
	clean := filepath.Base(name)
	if clean == "." || clean == "" || clean != name {
		return map[string]interface{}{"ok": false, "error": "无效备份名称"}
	}
	if s.coordinator == nil {
		return map[string]interface{}{"ok": false, "error": "统一备份入口不可用"}
	}
	_, err := s.coordinator.ExecuteImport(ctx, filepath.Join(s.dataDir, "backups", clean), dataportability.ImportRequest{CharacterPolicy: dataportability.CollisionReplace, Purpose: dataportability.PurposeUser})
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "restored": true, "name": clean}
}

func sanitizeName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(s)
}

func (s *service) StorageExportAmitia(scope string, characterID string) map[string]interface{} {
	return s.StorageExportAmitiaContext(context.Background(), scope, characterID)
}

func (s *service) StorageExportAmitiaContext(ctx context.Context, scope string, characterID string) map[string]interface{} {
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
	result, err := coord.CreateBackup(ctx, req, dbPath, runner)
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}

	fileName := filepath.Base(result.Path)
	exportDir := filepath.Join(s.dataDir, "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	source, err := os.Open(result.Path)
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	defer source.Close()
	destination, err := os.Create(filepath.Join(exportDir, fileName))
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	if err := destination.Close(); err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}

	return map[string]interface{}{
		"exported": true,
		"file":     fileName,
		"size":     result.SizeBytes,
		"sizeKB":   result.SizeBytes / 1024,
	}
}

func (s *service) StorageImportUserData(body map[string]interface{}) map[string]interface{} {
	return s.StorageImportUserDataContext(context.Background(), body)
}

func (s *service) StorageImportUserDataContext(ctx context.Context, body map[string]interface{}) map[string]interface{} {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return map[string]interface{}{"imported": false, "error": "云端配置授权已变化，请重新加载后操作"}
	}
	coord := s.coordinator
	if coord == nil {
		return map[string]interface{}{"imported": false, "error": "coordinator not initialized"}
	}

	fileName, _ := body["fileName"].(string)
	if fileName == "" || filepath.Base(fileName) != fileName || fileName == "." || fileName == ".." {
		return map[string]interface{}{"imported": false, "error": "missing fileName"}
	}

	archivePath := filepath.Join(s.dataDir, "exports", fileName)

	preview, err := coord.PreviewImport(ctx, archivePath)
	if err != nil {
		return map[string]interface{}{"imported": false, "error": err.Error()}
	}

	importReq := dataportability.ImportRequest{
		CharacterPolicy: dataportability.CollisionReplace,
	}

	_, err = coord.ExecuteImport(ctx, archivePath, importReq)
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
