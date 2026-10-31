// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
	"io"
	"strings"
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
func (c *ModelConfigBackupContributor) ComponentIDs() []string {
	return []string{ComponentIDModelConfigs}
}

func (c *ModelConfigBackupContributor) Dependencies() []string {
	return nil
}

type modelConfigExportRecord struct {
	ID                 int     `json:"id"`
	Name               string  `json:"name"`
	APIType            string  `json:"api_type"`
	Protocol           string  `json:"protocol"`
	BaseURL            string  `json:"base_url"`
	APIKey             string  `json:"api_key,omitempty"`
	ModelName          string  `json:"model_name"`
	Temperature        float64 `json:"temperature"`
	MaxTokens          int     `json:"max_tokens"`
	ContextWindow      int     `json:"context_window"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	BudgetRateVersion  string  `json:"budget_rate_version"`
	BudgetInputRate    int64   `json:"budget_input_rate"`
	BudgetOutputRate   int64   `json:"budget_output_rate"`
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
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil, nil
	}
	var count int64
	c.DB.WithContext(ctx).Model(&ModelConfig{}).Count(&count)
	var secretCount int64
	c.DB.WithContext(ctx).Table("model_configs").Where("api_key IS NOT NULL AND TRIM(api_key) <> ''").Count(&secretCount)
	var providerConfigs []string
	if err := c.DB.WithContext(ctx).Table("model_configs").Pluck("provider_config_json", &providerConfigs).Error; err != nil {
		return nil, err
	}
	for _, raw := range providerConfigs {
		if containsSensitiveModelProviderField(raw) {
			secretCount++
			break
		}
	}

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDModelConfigs,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "model.configs.v1",
			Required:      false,
			SourceOfTruth: false,
			ItemCount:     count,
			EstimatedSize: count * 1024,
			Sensitive:     secretCount > 0,
		},
	}, nil
}

func (c *ModelConfigBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil
	}
	compW, err := out.CreateComponent(ComponentIDModelConfigs, "model.configs.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table("model_configs").Select(
		"id, name, api_type, protocol, base_url, api_key, model_name, temperature, max_tokens, context_window, max_output_tokens, budget_rate_version, budget_input_rate, budget_output_rate, capabilities_json, provider_config_json, top_p, timeout_seconds, retry_count, is_active, last_test_status, last_test_message, last_test_at, created_at, updated_at",
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
		if !req.IncludeSecrets {
			rec.APIKey = ""
			rec.ProviderConfigJSON = redactModelProviderSecrets(rec.ProviderConfigJSON)
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
	missingAPIKeys := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec modelConfigExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++
		if strings.TrimSpace(rec.APIKey) == "" {
			missingAPIKeys++
		}

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
	if req.Manifest == nil || !req.Manifest.IncludesSecrets {
		preview.Warnings = append(preview.Warnings, "迁移包未包含模型 API Key，导入后相关模型需要重新授权或在目标设备补填")
	} else if missingAPIKeys > 0 {
		preview.Warnings = append(preview.Warnings, fmt.Sprintf("%d 条模型配置没有 API Key，导入后需要确认模型是否可使用", missingAPIKeys))
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
		IncludeSecrets:     req.Manifest != nil && req.Manifest.IncludesSecrets,
		AllowLegacySecrets: req.Manifest != nil && req.Manifest.SecretsPolicy == 0 && req.Manifest.Encrypted,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreModelConfigs(ctx, in, opts)
}

func (c *ModelConfigBackupContributor) RestoreModelConfigs(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
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
	return configwrite.Transaction(c.DB.WithContext(ctx), func(tx *gorm.DB) error {
		scoped := &ModelConfigBackupContributor{DB: tx}
		return scoped.restoreModelConfigLines(ctx, lines, opts)
	})
}

func (c *ModelConfigBackupContributor) restoreModelConfigLines(ctx context.Context, lines [][]byte, opts dataportability.RestoreOptions) error {
	activateImportedID := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec modelConfigExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("模型配置备份记录无效: %w", err)
		}
		if containsSensitiveModelProviderField(rec.ProviderConfigJSON) && !opts.IncludeSecrets && !opts.AllowLegacySecrets {
			return fmt.Errorf("模型配置 %s 包含未授权的供应商凭证", rec.Name)
		}

		var existing struct{ ID int }
		if err := c.DB.WithContext(ctx).Table("model_configs").Select("id").Where("id = ?", rec.ID).Scan(&existing).Error; err != nil {
			return err
		}

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
			"budget_rate_version":  rec.BudgetRateVersion,
			"budget_input_rate":    rec.BudgetInputRate,
			"budget_output_rate":   rec.BudgetOutputRate,
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
		if opts.IncludeSecrets && strings.TrimSpace(rec.APIKey) != "" {
			updates["api_key"] = rec.APIKey
		}

		if newID == rec.ID && existing.ID != 0 {
			if err := c.DB.WithContext(ctx).Table("model_configs").Where("id = ?", rec.ID).Updates(updates).Error; err != nil {
				return err
			}
		} else {
			updates["id"] = newID
			if err := c.DB.WithContext(ctx).Table("model_configs").Create(updates).Error; err != nil {
				return err
			}
		}
		if opts.ActivateImported && rec.IsActive != 0 && activateImportedID == 0 {
			activateImportedID = newID
		}
	}

	if activateImportedID != 0 {
		var imported struct {
			APIKey string
		}
		if err := c.DB.WithContext(ctx).Table("model_configs").Select("api_key").Where("id = ?", activateImportedID).Scan(&imported).Error; err != nil {
			return err
		}
		if strings.TrimSpace(imported.APIKey) != "" {
			if err := c.DB.WithContext(ctx).Table("model_configs").Where("1 = 1").Update("is_active", 0).Error; err != nil {
				return err
			}
			if err := c.DB.WithContext(ctx).Table("model_configs").Where("id = ?", activateImportedID).Update("is_active", 1).Error; err != nil {
				return err
			}
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

var modelProviderSecretMarkers = []string{
	"api_key",
	"apikey",
	"access_key",
	"private_key",
	"password",
	"passwd",
	"secret",
	"token",
	"credential",
}

func isSensitiveModelProviderKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, marker := range modelProviderSecretMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func containsSensitiveModelProviderField(raw string) bool {
	var value interface{}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "{}"
	}
	return valueContainsSensitiveModelProviderField(value)
}

func valueContainsSensitiveModelProviderField(value interface{}) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if isSensitiveModelProviderKey(key) || valueContainsSensitiveModelProviderField(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range typed {
			if valueContainsSensitiveModelProviderField(child) {
				return true
			}
		}
	}
	return false
}

func redactModelProviderSecrets(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	var value interface{}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return "{}"
	}
	redacted := redactModelProviderValue(value)
	data, err := json.Marshal(redacted)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func redactModelProviderValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			if isSensitiveModelProviderKey(key) {
				continue
			}
			result[key] = redactModelProviderValue(child)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, child := range typed {
			result = append(result, redactModelProviderValue(child))
		}
		return result
	default:
		return value
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

var _ dataportability.BackupContributor = (*ModelConfigBackupContributor)(nil)
