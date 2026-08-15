// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package system

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
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

func (c *SettingsBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var count int64
	c.DB.WithContext(ctx).Table("app_settings").Count(&count)

	estimatedSize := count * 256

	return []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDSettings,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "settings.records.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   false,
			Sensitive:     false,
			ItemCount:     count,
			EstimatedSize: estimatedSize,
		},
	}, nil
}

func (c *SettingsBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	compW, err := out.CreateComponent(ComponentIDSettings, "settings.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	defer compW.Close()

	rows, err := c.DB.WithContext(ctx).Table("app_settings").Select("key, value, updated_at").Rows()
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
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreSettings(ctx, in, opts)
}

func (c *SettingsBackupContributor) RestoreSettings(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	rc, err := in.ReadComponent(ComponentIDSettings)
	if err != nil {
		return err
	}
	defer rc.Close()

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

		var existing struct{ Key string }
		c.DB.WithContext(ctx).Table("app_settings").Select("key").Where("key = ?", rec.Key).Scan(&existing)

		if existing.Key != "" {
			c.DB.WithContext(ctx).Table("app_settings").Where("key = ?", rec.Key).Updates(map[string]interface{}{
				"value":      rec.Value,
				"updated_at": rec.UpdatedAt,
			})
		} else {
			c.DB.WithContext(ctx).Table("app_settings").Create(map[string]interface{}{
				"key":        rec.Key,
				"value":      rec.Value,
				"updated_at": rec.UpdatedAt,
			})
		}
	}

	return scanner.Err()
}
