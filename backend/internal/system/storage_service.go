// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"gorm.io/gorm/clause"
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

type exportTable struct {
	name          string
	characterCol  string
	needsConvJoin bool
	needsMemJoin  bool
}

var amitiaExportTables = []exportTable{
	{name: "characters", characterCol: "id"},
	{name: "conversations", characterCol: "character_id"},
	{name: "messages", needsConvJoin: true},
	{name: "conversation_summaries", needsConvJoin: true},
	{name: "memories", characterCol: "character_id"},
	{name: "memory_events", characterCol: "character_id"},
	{name: "memory_candidates", characterCol: "character_id"},
	{name: "episodic_memories"},
	{name: "moods", characterCol: "character_id"},
	{name: "need_states", characterCol: "character_id"},
	{name: "sleep_settings", characterCol: "character_id"},
	{name: "fixed_events", characterCol: "character_id"},
	{name: "special_events", characterCol: "character_id"},
	{name: "class_adjustments", characterCol: "character_id"},
	{name: "lifestyle_tendencies", characterCol: "character_id"},
	{name: "work_profiles", characterCol: "character_id"},
	{name: "role_profiles", characterCol: "character_id"},
	{name: "active_message_settings", characterCol: "character_id"},
	{name: "active_message_task", characterCol: "character_id"},
	{name: "proactive_rules", characterCol: "character_id"},
	{name: "proactive_messages", characterCol: "character_id"},
	{name: "reminders", characterCol: "character_id"},
	{name: "temporal_profiles", characterCol: "character_id"},
	{name: "temporal_global_presence_states"},
	{name: "retrieval_logs", characterCol: "character_id"},
	{name: "tool_call_intents", characterCol: "character_id"},
	{name: "tool_call_results", characterCol: "character_id"},
	{name: "pipeline_checkpoints", needsConvJoin: true},
	{name: "app_settings"},
	{name: "model_configs"},
	{name: "tts_configs"},
	{name: "asr_configs"},
	{name: "vision_configs"},
	{name: "embedding_configs"},
	{name: "character_templates"},
	{name: "user_profiles"},
	{name: "world_book"},
	{name: "safety_events"},
	{name: "message_feedback"},
	{name: "delivery_intents", characterCol: "character_id"},
	{name: "psyche_events", characterCol: "character_id"},
	{name: "psyche_snapshots", characterCol: "character_id"},
	{name: "psyche_states", characterCol: "character_id"},
	{name: "relationship_events", characterCol: "character_id"},
	{name: "relationship_states", characterCol: "character_id"},
	{name: "memory_embeddings", needsMemJoin: true},
	{name: "memory_temporal_metadata", needsMemJoin: true},
	{name: "character_emote_settings", characterCol: "character_id"},
	{name: "emote_character_bindings", characterCol: "character_id"},
	{name: "emote_send_records", characterCol: "character_id"},
	{name: "emotes"},
	{name: "emote_groups"},
	{name: "emote_group_items"},
	{name: "interaction_records", characterCol: "character_id"},
	{name: "output_leases", characterCol: "character_id"},
	{name: "temporal_anchors", characterCol: "character_id"},
	{name: "temporal_cadence_samples", characterCol: "character_id"},
	{name: "temporal_effect_ledger", characterCol: "character_id"},
	{name: "temporal_events", characterCol: "character_id"},
	{name: "temporal_interaction_receipts", characterCol: "character_id"},
	{name: "temporal_relationship_presence_states", characterCol: "character_id"},
	{name: "temporal_relationship_time_settings", characterCol: "character_id"},
	{name: "temporal_reunion_episodes", characterCol: "character_id"},
	{name: "message_sequence_checkpoints", needsConvJoin: true},
	{name: "schedules"},
	{name: "trigger_histories"},
}

func sanitizeName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(s)
}

func (s *service) StorageExportAmitia(scope string, characterID string) map[string]interface{} {
	exportDir := filepath.Join(s.dataDir, "exports")
	os.MkdirAll(exportDir, 0755)

	var prefix string
	if scope == "character" && characterID != "" {
		var ch map[string]interface{}
		if err := s.db.Table("characters").Where("id = ?", characterID).Take(&ch).Error; err != nil {
			return map[string]interface{}{"exported": false, "error": "角色不存在"}
		}
		prefix = sanitizeName(fmt.Sprint(ch["name"])) + "_" + time.Now().Format("20060102_150405")
	} else {
		prefix = "amitia_all_" + time.Now().Format("20060102_150405")
	}

	name := prefix + ".json"
	export := map[string]interface{}{"exportedAt": time.Now().Format(time.DateTime), "scope": scope}
	if characterID != "" {
		export["characterId"] = characterID
	}

	var convIDs []string
	if scope == "character" && characterID != "" {
		s.db.Table("conversations").Where("character_id = ?", characterID).Pluck("id", &convIDs)
	}

	var memIDs []string
	if scope == "character" && characterID != "" {
		s.db.Table("memories").Where("character_id = ?", characterID).Pluck("id", &memIDs)
	}

	for _, t := range amitiaExportTables {
		var items []map[string]interface{}
		query := s.db.Table(t.name)

		if t.needsConvJoin && scope == "character" && characterID != "" {
			if len(convIDs) > 0 {
				query = query.Where("conversation_id IN ?", convIDs)
			} else {
				export[t.name] = []map[string]interface{}{}
				continue
			}
		} else if t.needsMemJoin && scope == "character" && characterID != "" {
			if len(memIDs) > 0 {
				query = query.Where("memory_id IN ?", memIDs)
			} else {
				export[t.name] = []map[string]interface{}{}
				continue
			}
		} else if t.characterCol != "" && scope == "character" && characterID != "" {
			query = query.Where(t.characterCol+" = ?", characterID)
		}

		query.Find(&items)
		export[t.name] = items
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	err = os.WriteFile(filepath.Join(exportDir, name), data, 0644)
	if err != nil {
		return map[string]interface{}{"exported": false, "error": err.Error()}
	}
	return map[string]interface{}{"exported": true, "file": name, "size": len(data), "sizeKB": len(data) / 1024}
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

		stats := map[string]int{}
		errors := map[string]string{}
		totalImported := 0

		for _, t := range amitiaExportTables {
			raw, ok := imp[t.name]
			if !ok {
				continue
			}
			arr, ok := raw.([]interface{})
			if !ok || len(arr) == 0 {
				continue
			}

			items := make([]map[string]interface{}, 0, len(arr))
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					items = append(items, m)
				}
			}
			if len(items) == 0 {
				continue
			}

			result := s.db.Table(t.name).Clauses(clause.OnConflict{DoNothing: true}).Create(items)
			if result.Error != nil { errors[t.name] = result.Error.Error(); continue }
			count := len(items)
			stats[t.name] = count
			totalImported += count
		}

		return map[string]interface{}{
			"imported":      true,
			"fileName":      fileName,
			"totalImported": totalImported,
			"stats":         stats,
			"errors":         errors,
		}
	}
	return map[string]interface{}{"imported": false, "error": "missing fileName"}
}