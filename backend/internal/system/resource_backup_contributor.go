// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package system

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const ComponentIDResources = "resource.files"

type ResourceBackupContributor struct {
	DB      *gorm.DB
	DataDir string
}

func NewResourceBackupContributor(db *gorm.DB, dataDir string) *ResourceBackupContributor {
	return &ResourceBackupContributor{DB: db, DataDir: dataDir}
}

func (c *ResourceBackupContributor) ID() string   { return "resource" }
func (c *ResourceBackupContributor) Name() string { return "Resource" }

func (c *ResourceBackupContributor) Dependencies() []string {
	return nil
}

type resourceFileV1 struct {
	RelativePath string `json:"relative_path"`
	SizeBytes    int64  `json:"size_bytes"`
	ModTime      string `json:"mod_time"`
	MimeType     string `json:"mime_type"`
	Category     string `json:"category"`
}

func (c *ResourceBackupContributor) tableExists(name string) bool {
	var count int64
	c.DB.Raw(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name,
	).Scan(&count)
	return count > 0
}

func (c *ResourceBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64

	if c.tableExists("resource_files") {
		c.DB.WithContext(ctx).Table("resource_files").Count(&count)
	} else if c.tableExists("attachments") {
		c.DB.WithContext(ctx).Table("attachments").Count(&count)
	} else {
		resDir := filepath.Join(c.DataDir, "resources")
		count = c.countFiles(resDir)
	}

	estimatedSize := count * 512

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDResources,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "resource.files.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   true,
			Sensitive:     false,
			ItemCount:     count,
			EstimatedSize: estimatedSize,
		},
	}, nil
}

func (c *ResourceBackupContributor) countFiles(dir string) int64 {
	var count int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if entry.IsDir() {
			count += c.countFiles(filepath.Join(dir, entry.Name()))
			continue
		}
		count++
	}
	return count
}

func (c *ResourceBackupContributor) exportFromTable(ctx context.Context, tableName string, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDResources, "resource.files.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table(tableName).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	writer := bufio.NewWriter(compW)
	for rows.Next() {
		var rec resourceFileV1
		if err := c.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		writer.Write(data)
		writer.WriteByte('\n')
	}
	writer.Flush()

	return rows.Err()
}

func (c *ResourceBackupContributor) exportFromFilesystem(ctx context.Context, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDResources, "resource.files.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	resDir := filepath.Join(c.DataDir, "resources")
	writer := bufio.NewWriter(compW)

	c.walkResources(resDir, resDir, writer)
	writer.Flush()

	return nil
}

func (c *ResourceBackupContributor) walkResources(baseDir, currentDir string, writer *bufio.Writer) {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		fullPath := filepath.Join(currentDir, entry.Name())
		if entry.IsDir() {
			c.walkResources(baseDir, fullPath, writer)
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(baseDir, fullPath)
		if err != nil {
			continue
		}
		rec := resourceFileV1{
			RelativePath: filepath.ToSlash(relPath),
			SizeBytes:    info.Size(),
			ModTime:      info.ModTime().UTC().Format(time.RFC3339),
			MimeType:     detectMime(relPath),
			Category:     detectCategory(relPath),
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		writer.Write(data)
		writer.WriteByte('\n')
	}
}

func (c *ResourceBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if c.tableExists("resource_files") {
		return c.exportFromTable(ctx, "resource_files", out)
	}
	if c.tableExists("attachments") {
		return c.exportFromTable(ctx, "attachments", out)
	}
	return c.exportFromFilesystem(ctx, out)
}

func (c *ResourceBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDResources)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDResources,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "resource.files.v1",
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec resourceFileV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		if !c.tableExists("resource_files") && !c.tableExists("attachments") {
			continue
		}

		tableName := "resource_files"
		if !c.tableExists("resource_files") && c.tableExists("attachments") {
			tableName = "attachments"
		}
		var existing struct{ RelativePath string }
		c.DB.WithContext(ctx).Table(tableName).Select("relative_path").Where("relative_path = ?", rec.RelativePath).Scan(&existing)
		if existing.RelativePath != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.RelativePath,
				TargetID:   existing.RelativePath,
				EntityType: "resource_file",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, scanner.Err()
}

func (c *ResourceBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	useTable := c.tableExists("resource_files") || c.tableExists("attachments")

	rc, err := in.ReadComponent(ComponentIDResources)
	if err != nil {
		return err
	}
	defer rc.Close()

	tableName := "resource_files"
	if !c.tableExists("resource_files") && c.tableExists("attachments") {
		tableName = "attachments"
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec resourceFileV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		if !useTable {
			continue
		}

		var existing struct{ RelativePath string }
		c.DB.WithContext(ctx).Table(tableName).Select("relative_path").Where("relative_path = ?", rec.RelativePath).Scan(&existing)

		if existing.RelativePath != "" {
			c.DB.WithContext(ctx).Table(tableName).Where("relative_path = ?", rec.RelativePath).Updates(map[string]interface{}{
				"size_bytes": rec.SizeBytes,
				"mod_time":   rec.ModTime,
				"mime_type":  rec.MimeType,
				"category":   rec.Category,
			})
		} else {
			c.DB.WithContext(ctx).Table(tableName).Create(map[string]interface{}{
				"relative_path": rec.RelativePath,
				"size_bytes":    rec.SizeBytes,
				"mod_time":      rec.ModTime,
				"mime_type":     rec.MimeType,
				"category":      rec.Category,
			})
		}
	}

	return scanner.Err()
}

func detectMime(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".json":
		return "application/json"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func detectCategory(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return "image"
	case ".mp3", ".wav", ".ogg", ".flac":
		return "audio"
	case ".mp4", ".webm", ".avi", ".mov":
		return "video"
	case ".pdf", ".doc", ".docx", ".txt", ".md":
		return "document"
	case ".zip", ".tar", ".gz", ".7z":
		return "archive"
	default:
		return "other"
	}
}
