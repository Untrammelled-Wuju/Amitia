// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package system

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/configwrite"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
	"io"
	"strings"
)

const ComponentIDSettings = "settings.records"

type SettingsBackupContributor struct {
	DB *gorm.DB
}

func NewSettingsBackupContributor(db *gorm.DB) *SettingsBackupContributor {
	return &SettingsBackupContributor{DB: db}
}

func (c *SettingsBackupContributor) ID() string   { return "settings" }
func (c *SettingsBackupContributor) Name() string { return "Settings" }
func (c *SettingsBackupContributor) ComponentIDs() []string {
	return []string{ComponentIDSettings}
}

func (c *SettingsBackupContributor) Dependencies() []string {
	return nil
}

type settingsRecordV1 struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

var settingsDenylist = map[string]bool{
	"last_release_check": true,
	"last_update_check":  true,
}

func isSettingsKeyPortable(key string) bool {
	return !settingsDenylist[key]
}

var settingsSecretMarkers = []string{
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

func isSettingsKeySensitive(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, marker := range settingsSecretMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func (c *SettingsBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil, nil
	}
	var keys []string
	if err := c.DB.WithContext(ctx).Table("app_settings").Where("deleted_at IS NULL").Pluck("key", &keys).Error; err != nil {
		return nil, err
	}
	var count int64
	hasSensitive := false
	for _, key := range keys {
		if !isSettingsKeyPortable(key) {
			continue
		}
		sensitive := isSettingsKeySensitive(key)
		hasSensitive = hasSensitive || sensitive
		if sensitive && !req.IncludeSecrets {
			continue
		}
		count++
	}

	estimatedSize := count * 256

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDSettings,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "settings.records.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   false,
			Sensitive:     hasSensitive,
			ItemCount:     count,
			EstimatedSize: estimatedSize,
		},
	}, nil
}

func (c *SettingsBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if req.EffectiveScope() != dataportability.ScopeAll && req.EffectiveScope() != dataportability.ScopeSettings {
		return nil
	}
	compW, err := out.CreateComponent(ComponentIDSettings, "settings.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table("app_settings").Select("key, value, updated_at").Where("deleted_at IS NULL").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	writer := bufio.NewWriter(compW)
	for rows.Next() {
		var rec settingsRecordV1
		if err := c.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		if !isSettingsKeyPortable(rec.Key) {
			continue
		}
		if isSettingsKeySensitive(rec.Key) && !req.IncludeSecrets {
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

func (c *SettingsBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	rc, err := in.ReadComponent(ComponentIDSettings)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	preview := dataportability.ImportComponentPreview{
		ComponentID: ComponentIDSettings,
		Kind:        dataportability.KindNDJSON,
		LogicalName: "settings.records.v1",
	}
	if req.Manifest != nil && !req.Manifest.IncludesSecrets {
		preview.Warnings = append(preview.Warnings, "迁移包未包含 API Key、令牌或口令等敏感设置，导入后需要在目标设备重新配置")
	}

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec settingsRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct{ Key string }
		c.DB.WithContext(ctx).Table("app_settings").Select("key").Where("key = ?", rec.Key).Scan(&existing)
		if existing.Key != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   rec.Key,
				TargetID:   existing.Key,
				EntityType: "setting",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}

	return []dataportability.ImportComponentPreview{preview}, scanner.Err()
}

func (c *SettingsBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
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
	return c.RestoreSettings(ctx, in, opts)
}

func (c *SettingsBackupContributor) RestoreSettings(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	rc, err := in.ReadComponent(ComponentIDSettings)
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return configwrite.Transaction(c.DB.WithContext(ctx), func(tx *gorm.DB) error {
		store := NewSettingsStore(tx)
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var rec settingsRecordV1
			if err := json.Unmarshal(line, &rec); err != nil {
				return fmt.Errorf("设置备份记录无效: %w", err)
			}
			if rec.Key == "" {
				return fmt.Errorf("设置备份缺少配置键")
			}
			if isSettingsKeySensitive(rec.Key) && !opts.IncludeSecrets && !opts.AllowLegacySecrets {
				return fmt.Errorf("敏感设置 %s 未获得迁移授权", rec.Key)
			}
			if _, err := store.Upsert(rec.Key, rec.Value); err != nil {
				return err
			}
		}

		return scanner.Err()
	})
}
