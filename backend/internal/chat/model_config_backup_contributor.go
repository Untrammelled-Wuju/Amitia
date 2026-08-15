// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
	"io"
)

const ComponentIDModelConfigs = "model.configs"

type ModelConfigBackupContributor struct {
	DB *gorm.DB
}

func NewModelConfigBackupContributor(db *gorm.DB) *ModelConfigBackupContributor {
	return &ModelConfigBackupContributor{DB: db}
}

func (c *ModelConfigBackupContributor) ID() string   { return "model" }
func (c *ModelConfigBackupContributor) Name() string { return "Model Config" }

func (c *ModelConfigBackupContributor) Dependencies() []string {
	return nil
}

type modelConfigExportRecord struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	APIType            string  `json:"api_type"`
	Protocol           string  `json:"protocol"`
	BaseURL            string  `json:"base_url"`
	ModelName          string  `json:"model_name"`
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"max_tokens"`
	ContextWindow      int     `json:"context_window"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	CapabilitiesJSON   string  `json:"capabilities_json"`
	ProviderConfigJSON string  `json:"provider_config_json"`
	TopP               float64 `json:"top_p"`
	TimeoutSeconds     int     `json:"timeout_seconds"`
	RetryCount         int     `json:"retry_count"`
	IsActive           int     `json:"is_active"`
	LastTestStatus     string  `json:"last_test_status"`
	LastTestMessage    string  `json:"last_test_message"`
	LastTestAt         string  `json:"last_test_at"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

func (c *ModelConfigBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Model(&ModelConfig{}).Count(&count)

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDModelConfigs,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "model.configs.v1",
			Required:      false,
			SourceOfTruth: false,
			ItemCount:     count,
			EstimatedSize: count * 1024,
		},
	}, nil
}

func (c *ModelConfigBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDModelConfigs, "model.configs.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table("model_configs").Select(
		"id, name, api_type, protocol, base_url, model_name, temperature, max_tokens, context_window, max_output_tokens, capabilities_json, provider_config_json, top_p, timeout_seconds, retry_count, is_active, last_test_status, last_test_message, last_test_at, created_at, updated_at",
	).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec modelConfigExportRecord
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

func (c *ModelConfigBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDModelConfigs)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDModelConfigs,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "model.configs.v1",
	}

	data, err := readAllModelConfig(rc)
	if err != nil {
		return nil, err
	}

	lines := splitModelConfigLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec modelConfigExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct {
			ID   int
			Name string
		}
		c.DB.WithContext(ctx).Table("model_configs").Select("id, name").Where("id = ?", rec.ID).Scan(&existing)
		if existing.ID != 0 {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   fmt.Sprintf("%d", rec.ID),
				TargetID:   fmt.Sprintf("%d", existing.ID),
				EntityType: "model_config",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *ModelConfigBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            dataportability.RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreModelConfigs(ctx, in, opts)
}

func (c *ModelConfigBackupContributor) RestoreModelConfigs(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	rc, err := in.ReadComponent(ComponentIDModelConfigs)
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := readAllModelConfig(rc)
	if err != nil {
		return err
	}

	lines := splitModelConfigLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec modelConfigExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		var existing struct{ ID int }
		c.DB.WithContext(ctx).Table("model_configs").Select("id").Where("id = ?", rec.ID).Scan(&existing)

		newID := rec.ID
		if existing.ID != 0 {
			switch opts.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				newID = rec.ID
			default:
				newID = int(uuid.New().ID())
			}
		}

		updates := map[string]interface{}{
			"name":                 rec.Name,
			"api_type":             rec.APIType,
			"protocol":             rec.Protocol,
			"base_url":             rec.BaseURL,
			"model_name":           rec.ModelName,
			"temperature":          rec.Temperature,
			"max_tokens":           rec.MaxTokens,
			"context_window":       rec.ContextWindow,
			"max_output_tokens":    rec.MaxOutputTokens,
			"capabilities_json":    rec.CapabilitiesJSON,
			"provider_config_json": rec.ProviderConfigJSON,
			"top_p":                rec.TopP,
			"timeout_seconds":      rec.TimeoutSeconds,
			"retry_count":          rec.RetryCount,
			"is_active":            0,
			"last_test_status":     rec.LastTestStatus,
			"last_test_message":    rec.LastTestMessage,
			"last_test_at":         rec.LastTestAt,
		}

		if newID == rec.ID && existing.ID != 0 {
			c.DB.WithContext(ctx).Table("model_configs").Where("id = ?", rec.ID).Updates(updates)
		} else {
			updates["id"] = newID
			c.DB.WithContext(ctx).Table("model_configs").Create(updates)
		}
	}

	return nil
}

func readAllModelConfig(r io.Reader) ([]byte, error) {
	result := make([]byte, 0, 4096)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return result, nil
			}
			return result, err
		}
	}
}

func splitModelConfigLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

var _ dataportability.ModelConfigRestorePort = (*ModelConfigBackupContributor)(nil)
var _ dataportability.BackupContributor = (*ModelConfigBackupContributor)(nil)
