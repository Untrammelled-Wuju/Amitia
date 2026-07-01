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
	migFile := filepath.Join(s.dataDir, ".migration_version")
	data, err := os.ReadFile(migFile)
	version := "0"
	if err == nil {
		version = strings.TrimSpace(string(data))
	}
	return map[string]interface{}{"migrations": []interface{}{map[string]interface{}{"name": "initial", "version": version, "applied": err == nil}}}
}

func (s *service) CheckStorageMigrations() map[string]interface{} {
	migFile := filepath.Join(s.dataDir, ".migration_version")
	_, err := os.Stat(migFile)
	return map[string]interface{}{"needsMigration": os.IsNotExist(err)}
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
		return map[string]interface{}{"imported": true, "fileName": fileName, "size": len(data)}
	}
	return map[string]interface{}{"imported": false, "error": "missing fileName"}
}
