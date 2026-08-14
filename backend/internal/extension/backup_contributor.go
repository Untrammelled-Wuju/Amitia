// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package extension

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

type ExtensionBackupContributor struct {
	DB *gorm.DB
}

func NewExtensionBackupContributor(db *gorm.DB) *ExtensionBackupContributor {
	return &ExtensionBackupContributor{DB: db}
}

func (c *ExtensionBackupContributor) ID() string   { return "extension" }
func (c *ExtensionBackupContributor) Name() string { return "Extension" }

type extensionSkillExportRecord struct {
	ID                   string `json:"id"`
	ExtensionID          string `json:"extension_id"`
	UserID               string `json:"user_id"`
	Name                 string `json:"name"`
	Description          string `json:"description"`
	License              string `json:"license"`
	Compatibility        string `json:"compatibility"`
	MetadataJSON         string `json:"metadata_json"`
	AllowedTools         string `json:"allowed_tools"`
	DisplayName          string `json:"display_name"`
	ShortDescription     string `json:"short_description"`
	DefaultPrompt        string `json:"default_prompt"`
	OpenAIMetadataJSON   string `json:"openai_metadata_json"`
	ScopeType            string `json:"scope_type"`
	ScopeID              string `json:"scope_id"`
	Source               string `json:"source"`
	CompatibilityStatus  string `json:"compatibility_status"`
	ContentHash          string `json:"content_hash"`
	RawFrontmatterJSON   string `json:"raw_frontmatter_json"`
	ExtraFrontmatterJSON string `json:"extra_frontmatter_json"`
	ToolMappingsJSON     string `json:"tool_mappings_json"`
	ScriptsPresent       int    `json:"scripts_present"`
	ScriptsRequired      int    `json:"scripts_required"`
	Enabled              int    `json:"enabled"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

const extensionComponentID = "extension.records.v1"
const extensionLogicalName = "extension.records"

func (c *ExtensionBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Model(&agentSkillMetadataRecord{}).Where("removed_at = ''").Count(&count)
	return []dataportability.BackupComponentPlan{
		{
			ID:            extensionComponentID,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   extensionLogicalName,
			Required:      false,
			SourceOfTruth: true,
			ItemCount:     count,
			EstimatedSize: count * 2048,
		},
	}, nil
}

func (c *ExtensionBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(extensionComponentID, extensionLogicalName, dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	query := c.DB.WithContext(ctx).Table("extension_agent_skill_metadata").Select(
		"id, extension_id, user_id, name, description, license, compatibility, metadata_json, allowed_tools, display_name, short_description, default_prompt, openai_metadata_json, scope_type, scope_id, source, compatibility_status, content_hash, raw_frontmatter_json, extra_frontmatter_json, tool_mappings_json, scripts_present, scripts_required, enabled, created_at, updated_at",
	).Where("removed_at = ''").Order("name ASC, scope_type ASC, scope_id ASC")

	rows, err := query.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec extensionSkillExportRecord
		if err := c.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		compW.Write(data)
		compW.Write([]byte("\n"))
	}
	return rows.Err()
}

func (c *ExtensionBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(extensionComponentID)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: extensionComponentID,
		Kind:        dataportability.KindNDJSON,
		LogicalName: extensionLogicalName,
	}

	reader := bufio.NewReader(rc)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			if len(line) == 0 {
				continue
			}
			var rec extensionSkillExportRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			preview.ItemCount++

			var existing struct {
				ID          string
				ExtensionID string
			}
			c.DB.WithContext(ctx).Table("extension_agent_skill_metadata").Select("id, extension_id").Where("extension_id = ?", rec.ExtensionID).Scan(&existing)
			if existing.ID != "" {
				preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
					SourceID:   rec.ExtensionID,
					TargetID:   existing.ExtensionID,
					EntityType: "extension",
					Policy:     dataportability.CollisionDuplicate,
				})
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *ExtensionBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(extensionComponentID)
	if err != nil {
		return err
	}
	defer rc.Close()

	reader := bufio.NewReader(rc)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			if len(line) == 0 {
				continue
			}
			var rec extensionSkillExportRecord
			if jsonErr := json.Unmarshal(line, &rec); jsonErr != nil {
				continue
			}

			metadata := agentSkillMetadataRecord{
				ID:                      rec.ID,
				ExtensionID:             rec.ExtensionID,
				UserID:                  rec.UserID,
				Name:                    rec.Name,
				Description:             rec.Description,
				License:                 rec.License,
				Compatibility:           rec.Compatibility,
				MetadataJSON:            rec.MetadataJSON,
				AllowedTools:            rec.AllowedTools,
				DisplayName:             rec.DisplayName,
				ShortDescription:        rec.ShortDescription,
				DefaultPrompt:           rec.DefaultPrompt,
				OpenAIMetadataJSON:      rec.OpenAIMetadataJSON,
				ScopeType:               rec.ScopeType,
				ScopeID:                 rec.ScopeID,
				Source:                  rec.Source,
				CompatibilityStatus:     rec.CompatibilityStatus,
				ContentHash:             rec.ContentHash,
				RawFrontmatterJSON:      rec.RawFrontmatterJSON,
				ExtraFrontmatterJSON:    rec.ExtraFrontmatterJSON,
				ToolMappingsJSON:        rec.ToolMappingsJSON,
				ScriptsPresent:          rec.ScriptsPresent,
				ScriptsRequired:         rec.ScriptsRequired,
				Enabled:                 0,
				CreatedAt:               rec.CreatedAt,
				UpdatedAt:               rec.UpdatedAt,
				CompatibilityReportJSON: "{}",
				ArtifactID:              uuid.NewString(),
				ResourceIndexJSON:       "[]",
			}

			var existing struct {
				ID          string
				ExtensionID string
			}
			c.DB.WithContext(ctx).Table("extension_agent_skill_metadata").Select("id, extension_id").Where("extension_id = ?", rec.ExtensionID).Scan(&existing)

			if existing.ID != "" {
				switch req.CharacterPolicy {
				case dataportability.CollisionSkip:
					continue
				case dataportability.CollisionReplace:
					c.DB.WithContext(ctx).Model(&agentSkillMetadataRecord{}).Where("extension_id = ?", rec.ExtensionID).Updates(map[string]interface{}{
						"name":                      metadata.Name,
						"description":               metadata.Description,
						"license":                   metadata.License,
						"compatibility":             metadata.Compatibility,
						"metadata_json":             metadata.MetadataJSON,
						"allowed_tools":             metadata.AllowedTools,
						"display_name":              metadata.DisplayName,
						"short_description":         metadata.ShortDescription,
						"default_prompt":            metadata.DefaultPrompt,
						"openai_metadata_json":      metadata.OpenAIMetadataJSON,
						"scope_type":                metadata.ScopeType,
						"scope_id":                  metadata.ScopeID,
						"source":                    metadata.Source,
						"compatibility_status":      metadata.CompatibilityStatus,
						"content_hash":              metadata.ContentHash,
						"raw_frontmatter_json":      metadata.RawFrontmatterJSON,
						"extra_frontmatter_json":    metadata.ExtraFrontmatterJSON,
						"tool_mappings_json":        metadata.ToolMappingsJSON,
						"scripts_present":           metadata.ScriptsPresent,
						"scripts_required":          metadata.ScriptsRequired,
						"compatibility_report_json": metadata.CompatibilityReportJSON,
						"resource_index_json":       metadata.ResourceIndexJSON,
						"updated_at":                metadata.UpdatedAt,
					})
				default:
					metadata.ID = uuid.NewString()
					metadata.ExtensionID = fmt.Sprintf("%s.import.%s", rec.ExtensionID, uuid.NewString()[:8])
					metadata.Enabled = 0
					c.DB.WithContext(ctx).Create(&metadata)
				}
			} else {
				metadata.ArtifactID = uuid.NewString()
				metadata.ResourceIndexJSON = "[]"
				metadata.CompatibilityReportJSON = "{}"
				metadata.Enabled = 0
				c.DB.WithContext(ctx).Create(&metadata)
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}
	}

	return nil
}
