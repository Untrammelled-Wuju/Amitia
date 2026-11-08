// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"context"
	"fmt"
	"github.com/u-ai/backend/internal/devicemesh/business"
	amitialog "log"
	"os"
	"path/filepath"
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
		"messageCount":      int64(0),
		"conversationCount": int64(0),
		"scope":             "all-roles",
	}
	if s.memoryConsumer == nil {
		result["memoryError"] = "统一记忆统计入口不可用"
		return result
	}
	if storage, ok := s.memoryConsumer.(business.OwnedStorageStatsPort); ok {
		stats, err := storage.OwnedStorageStats(context.Background())
		if err != nil {
			result["ownedStorageError"] = err.Error()
		} else {
			result["messageCount"] = stats.MessageCount
			result["conversationCount"] = stats.ConversationCount
			result["memoryCount"] = stats.MemoryCount
			dbSize += stats.DatabaseBytes
			result["dbSize"] = formatStorageBytes(dbSize)
			result["dbSizeBytes"] = dbSize
			result["kernelDbSizeBytes"] = stats.DatabaseBytes
		}
	} else {
		result["ownedStorageError"] = "统一存储统计入口不可用"
	}
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
