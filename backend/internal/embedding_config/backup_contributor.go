// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package embedding_config

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const (
	ComponentIDEmbeddingConfigs = "embedding.configs"
)

type EmbeddingBackupContributor struct {
	DB *gorm.DB
}

func NewEmbeddingBackupContributor(db *gorm.DB) *EmbeddingBackupContributor {
	return &EmbeddingBackupContributor{DB: db}
}

func (c *EmbeddingBackupContributor) ID() string   { return "embedding" }
func (c *EmbeddingBackupContributor) Name() string { return "Embedding Config" }

func (c *EmbeddingBackupContributor) Dependencies() []string {
	return nil
}

type embeddingConfigV1 struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	ApiType            string `json:"apiType"`
	ModelName          string `json:"modelName"`
	BaseUrl            string `json:"baseUrl"`
	IsActive           int    `json:"isActive"`
	ProviderConfigJSON string `json:"providerConfigJSON"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

func (c *EmbeddingBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Model(&EmbeddingConfig{}).Count(&count)

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDEmbeddingConfigs,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "embedding.configs.v1",
			Required:      false,
			SourceOfTruth: true,
			Rebuildable:   false,
			ItemCount:     count,
			EstimatedSize: count * 512,
		},
	}, nil
}

func (c *EmbeddingBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	comp, err := out.CreateComponent(ComponentIDEmbeddingConfigs, "embedding.configs.v1", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create embedding.configs component: %w", err)
	}
	defer comp.Close()

	rows, err := c.DB.WithContext(ctx).Model(&EmbeddingConfig{}).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cfg EmbeddingConfig
		if err := c.DB.ScanRows(rows, &cfg); err != nil {
			continue
		}
		rec := embeddingConfigV1{
			ID:                 cfg.ID,
			Name:               cfg.Name,
			ApiType:            cfg.ApiType,
			ModelName:          cfg.ModelName,
			BaseUrl:            cfg.BaseUrl,
			IsActive:           cfg.IsActive,
			ProviderConfigJSON: cfg.ProviderConfigJSON,
			CreatedAt:          cfg.CreatedAt,
			UpdatedAt:          cfg.UpdatedAt,
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		comp.Write(data)
		comp.Write([]byte("\n"))
	}
	return rows.Err()
}

func (c *EmbeddingBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDEmbeddingConfigs,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "embedding.configs.v1",
		Collisions:  make([]dataportability.ComponentCollision, 0),
		Warnings:    make([]string, 0),
	}

	rc, err := in.ReadComponent(ComponentIDEmbeddingConfigs + ".v1")
	if err != nil {
		return []dataportability.ImportComponentPreview{preview}, nil
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		preview.ItemCount++

		var rec embeddingConfigV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		var existing struct{ ID int }
		c.DB.WithContext(ctx).Model(&EmbeddingConfig{}).Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != 0 {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   fmt.Sprintf("%d", rec.ID),
				TargetID:   fmt.Sprintf("%d", existing.ID),
				EntityType: "embedding_config",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *EmbeddingBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	rc, err := in.ReadComponent(ComponentIDEmbeddingConfigs + ".v1")
	if err != nil {
		return fmt.Errorf("import: embedding.configs component missing: %w", err)
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec embeddingConfigV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		exists := false
		var existing struct{ ID int }
		c.DB.WithContext(ctx).Model(&EmbeddingConfig{}).Select("id").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != 0 {
			exists = true
		}

		entity := EmbeddingConfig{
			Name:               rec.Name,
			ApiType:            rec.ApiType,
			ModelName:          rec.ModelName,
			BaseUrl:            rec.BaseUrl,
			IsActive:           rec.IsActive,
			ProviderConfigJSON: rec.ProviderConfigJSON,
		}

		if exists {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				c.DB.WithContext(ctx).Model(&EmbeddingConfig{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
					"name":                 rec.Name,
					"api_type":             rec.ApiType,
					"model_name":           rec.ModelName,
					"base_url":             rec.BaseUrl,
					"is_active":            rec.IsActive,
					"provider_config_json": rec.ProviderConfigJSON,
				})
				continue
			default:
				if err := c.DB.WithContext(ctx).Create(&entity).Error; err != nil {
					continue
				}
				continue
			}
		}

		if err := c.DB.WithContext(ctx).Create(&entity).Error; err != nil {
			continue
		}
	}

	return nil
}
