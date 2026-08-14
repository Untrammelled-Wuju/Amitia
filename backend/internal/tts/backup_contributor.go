// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package tts

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

type VoiceBackupContributor struct {
	DB *gorm.DB
}

func NewVoiceBackupContributor(db *gorm.DB) *VoiceBackupContributor {
	return &VoiceBackupContributor{DB: db}
}

func (c *VoiceBackupContributor) ID() string   { return "voice" }
func (c *VoiceBackupContributor) Name() string { return "Voice Config" }

type ttsExportRecord struct {
	ID                  int     `json:"id"`
	Name                string  `json:"name"`
	ApiType             string  `json:"apiType"`
	BaseURL             string  `json:"baseUrl"`
	ResourceId          string  `json:"resourceId"`
	VoiceType           string  `json:"voiceType"`
	Emotion             string  `json:"emotion"`
	Speed               float64 `json:"speed"`
	Pitch               float64 `json:"pitch"`
	Volume              float64 `json:"volume"`
	IsActive            int     `json:"isActive"`
	IsCustom            int     `json:"isCustom"`
	CustomVoiceID       string  `json:"customVoiceId"`
	CloneResourceId     string  `json:"cloneResourceId"`
	RealtimeAppId       string  `json:"realtimeAppId"`
	RealtimeAccessToken string  `json:"realtimeAccessToken"`
	RealtimeSecretKey   string  `json:"realtimeSecretKey"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
}

type asrExportRecord struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ApiType    string `json:"apiType"`
	BaseURL    string `json:"baseUrl"`
	ResourceId string `json:"resourceId"`
	IsActive   int    `json:"isActive"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

func (c *VoiceBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var ttsCount, asrCount int64
	c.DB.WithContext(ctx).Table("tts_configs").Count(&ttsCount)
	c.DB.WithContext(ctx).Table("asr_configs").Count(&asrCount)

	return []dataportability.BackupComponentPlan{
		{
			ID:            "voice.tts.v1",
			Kind:          dataportability.KindDataset,
			LogicalName:   "voice.tts.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   false,
			Sensitive:     true,
			ItemCount:     ttsCount,
			EstimatedSize: ttsCount * 1024,
		},
		{
			ID:            "voice.asr.v1",
			Kind:          dataportability.KindDataset,
			LogicalName:   "voice.asr.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   false,
			Sensitive:     true,
			ItemCount:     asrCount,
			EstimatedSize: asrCount * 512,
		},
	}, nil
}

func (c *VoiceBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	ttsW, err := out.CreateComponent("voice.tts.v1", "voice.tts.v1", dataportability.KindDataset)
	if err != nil {
		return err
	}
	defer ttsW.Close()

	rows, err := c.DB.WithContext(ctx).Table("tts_configs").Select(
		"id, name, api_type, base_url, resource_id, voice_type, emotion, speed, pitch, volume, is_active, is_custom, custom_voice_id, clone_resource_id, realtime_app_id, realtime_access_token, realtime_secret_key, created_at, updated_at",
	).Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rec ttsExportRecord
		if err := c.DB.ScanRows(rows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		ttsW.Write(data)
		ttsW.Write([]byte("\n"))
	}

	asrW, err := out.CreateComponent("voice.asr.v1", "voice.asr.v1", dataportability.KindDataset)
	if err != nil {
		return err
	}
	defer asrW.Close()

	asrRows, err := c.DB.WithContext(ctx).Table("asr_configs").Select(
		"id, name, api_type, base_url, resource_id, is_active, created_at, updated_at",
	).Rows()
	if err != nil {
		return err
	}
	defer asrRows.Close()

	for asrRows.Next() {
		var rec asrExportRecord
		if err := c.DB.ScanRows(asrRows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		asrW.Write(data)
		asrW.Write([]byte("\n"))
	}

	return nil
}

func (c *VoiceBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	var previews []dataportability.ImportComponentPreview

	ttsRC, err := in.ReadComponent("voice.tts.v1")
	if err == nil {
		ttsPreview := dataportability.ImportComponentPreview{
			ComponentID: "voice.tts.v1",
			Kind:        dataportability.KindDataset,
			LogicalName: "voice.tts.v1",
		}
		c.previewTTS(ctx, ttsRC, &ttsPreview)
		previews = append(previews, ttsPreview)
	}

	asrRC, err := in.ReadComponent("voice.asr.v1")
	if err == nil {
		asrPreview := dataportability.ImportComponentPreview{
			ComponentID: "voice.asr.v1",
			Kind:        dataportability.KindDataset,
			LogicalName: "voice.asr.v1",
		}
		c.previewASR(ctx, asrRC, &asrPreview)
		previews = append(previews, asrPreview)
	}

	return previews, nil
}

func (c *VoiceBackupContributor) previewTTS(ctx context.Context, rc io.ReadCloser, preview *dataportability.ImportComponentPreview) {
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec ttsExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct {
			ID   int
			Name string
		}
		c.DB.WithContext(ctx).Table("tts_configs").Select("id, name").Where("name = ?", rec.Name).Scan(&existing)
		if existing.ID != 0 {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   fmt.Sprintf("%d", rec.ID),
				TargetID:   fmt.Sprintf("%d", existing.ID),
				EntityType: "tts_config",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}
}

func (c *VoiceBackupContributor) previewASR(ctx context.Context, rc io.ReadCloser, preview *dataportability.ImportComponentPreview) {
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec asrExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		preview.ItemCount++

		var existing struct {
			ID   int
			Name string
		}
		c.DB.WithContext(ctx).Table("asr_configs").Select("id, name").Where("name = ?", rec.Name).Scan(&existing)
		if existing.ID != 0 {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   fmt.Sprintf("%d", rec.ID),
				TargetID:   fmt.Sprintf("%d", existing.ID),
				EntityType: "asr_config",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}
}

func (c *VoiceBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	ttsRC, err := in.ReadComponent("voice.tts.v1")
	if err == nil {
		c.importTTS(ctx, req, ttsRC)
	}

	asrRC, err := in.ReadComponent("voice.asr.v1")
	if err == nil {
		c.importASR(ctx, req, asrRC)
	}

	return nil
}

func (c *VoiceBackupContributor) importTTS(ctx context.Context, req dataportability.ImportRequest, rc io.ReadCloser) {
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec ttsExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		var existing struct {
			ID   int
			Name string
		}
		c.DB.WithContext(ctx).Table("tts_configs").Select("id, name").Where("name = ?", rec.Name).Scan(&existing)

		newID := 0
		if existing.ID != 0 {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				updates := map[string]interface{}{
					"api_type":              rec.ApiType,
					"base_url":              rec.BaseURL,
					"resource_id":           rec.ResourceId,
					"voice_type":            rec.VoiceType,
					"emotion":               rec.Emotion,
					"speed":                 rec.Speed,
					"pitch":                 rec.Pitch,
					"volume":                rec.Volume,
					"is_active":             0,
					"is_custom":             rec.IsCustom,
					"custom_voice_id":       rec.CustomVoiceID,
					"clone_resource_id":     rec.CloneResourceId,
					"realtime_app_id":       rec.RealtimeAppId,
					"realtime_access_token": rec.RealtimeAccessToken,
					"realtime_secret_key":   rec.RealtimeSecretKey,
					"updated_at":            rec.UpdatedAt,
				}
				c.DB.WithContext(ctx).Table("tts_configs").Where("id = ?", existing.ID).Updates(updates)
				continue
			default:
			}
		}

		if newID == 0 {
			now := rec.CreatedAt
			if now == "" {
				now = "2025-01-01 00:00:00"
			}
			result := c.DB.WithContext(ctx).Table("tts_configs").Create(map[string]interface{}{
				"name":                  rec.Name,
				"api_type":              rec.ApiType,
				"base_url":              rec.BaseURL,
				"resource_id":           rec.ResourceId,
				"voice_type":            rec.VoiceType,
				"emotion":               rec.Emotion,
				"speed":                 rec.Speed,
				"pitch":                 rec.Pitch,
				"volume":                rec.Volume,
				"is_active":             0,
				"is_custom":             rec.IsCustom,
				"custom_voice_id":       rec.CustomVoiceID,
				"clone_resource_id":     rec.CloneResourceId,
				"realtime_app_id":       rec.RealtimeAppId,
				"realtime_access_token": rec.RealtimeAccessToken,
				"realtime_secret_key":   rec.RealtimeSecretKey,
				"created_at":            now,
				"updated_at":            now,
			})
			_ = result
		}
	}
}

func (c *VoiceBackupContributor) importASR(ctx context.Context, req dataportability.ImportRequest, rc io.ReadCloser) {
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec asrExportRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		var existing struct {
			ID   int
			Name string
		}
		c.DB.WithContext(ctx).Table("asr_configs").Select("id, name").Where("name = ?", rec.Name).Scan(&existing)

		if existing.ID != 0 {
			switch req.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				updates := map[string]interface{}{
					"api_type":    rec.ApiType,
					"base_url":    rec.BaseURL,
					"resource_id": rec.ResourceId,
					"is_active":   0,
					"updated_at":  rec.UpdatedAt,
				}
				c.DB.WithContext(ctx).Table("asr_configs").Where("id = ?", existing.ID).Updates(updates)
				continue
			default:
			}
		}

		now := rec.CreatedAt
		if now == "" {
			now = "2025-01-01 00:00:00"
		}
		c.DB.WithContext(ctx).Table("asr_configs").Create(map[string]interface{}{
			"name":        rec.Name,
			"api_type":    rec.ApiType,
			"base_url":    rec.BaseURL,
			"resource_id": rec.ResourceId,
			"is_active":   0,
			"created_at":  now,
			"updated_at":  now,
		})
	}
}
