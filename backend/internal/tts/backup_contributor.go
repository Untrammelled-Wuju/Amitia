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
	"io"
)

type VoiceBackupContributor struct {
	DB *gorm.DB
}

func NewVoiceBackupContributor(db *gorm.DB) *VoiceBackupContributor {
	return &VoiceBackupContributor{DB: db}
}

func (c *VoiceBackupContributor) ID() string   { return "voice" }
func (c *VoiceBackupContributor) Name() string { return "Voice Config" }

func (c *VoiceBackupContributor) Dependencies() []string {
	return []string{"character"}
}

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
	RealtimeAccessToken string  `json:"realtimeAccessToken,omitempty"`
	RealtimeSecretKey   string  `json:"realtimeSecretKey,omitempty"`
	SecretRef           string  `json:"secretRef"`
	RebindRequired      bool    `json:"rebindRequired"`
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

type clonedVoiceExportRecord struct {
	SpaceID       string `json:"spaceId"`
	SpeakerID     string `json:"speakerId"`
	Name          string `json:"name"`
	TtsConfigID   int    `json:"voiceConfigId,omitempty"`
	TtsConfigName string `json:"voiceConfigName,omitempty"`
	Language      int    `json:"language"`
	Status        string `json:"status"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func (c *VoiceBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	var ttsCount, asrCount, clonedVoiceCount int64
	c.DB.WithContext(ctx).Table("tts_configs").Count(&ttsCount)
	c.DB.WithContext(ctx).Table("asr_configs").Count(&asrCount)
	c.DB.WithContext(ctx).Table("tts_cloned_voices").Count(&clonedVoiceCount)

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
		{
			ID:            "voice.clones.v1",
			Kind:          dataportability.KindDataset,
			LogicalName:   "voice.clones.v1",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   false,
			Sensitive:     false,
			ItemCount:     clonedVoiceCount,
			EstimatedSize: clonedVoiceCount * 512,
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
		if rec.RealtimeAccessToken != "" || rec.RealtimeSecretKey != "" {
			rec.SecretRef = fmt.Sprintf("tts:%d", rec.ID)
			rec.RebindRequired = true
			rec.RealtimeAccessToken = ""
			rec.RealtimeSecretKey = ""
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

	cloneW, err := out.CreateComponent("voice.clones.v1", "voice.clones.v1", dataportability.KindDataset)
	if err != nil {
		return err
	}
	defer cloneW.Close()

	cloneRows, err := c.DB.WithContext(ctx).Table("tts_cloned_voices AS cv").Select(
		"cv.space_id, cv.speaker_id, cv.name, cv.tts_config_id, COALESCE(tc.name, '') AS tts_config_name, cv.language, cv.status, cv.created_at, cv.updated_at",
	).Joins(
		"LEFT JOIN tts_configs AS tc ON tc.id = cv.tts_config_id",
	).Rows()
	if err != nil {
		return err
	}
	defer cloneRows.Close()

	for cloneRows.Next() {
		var rec clonedVoiceExportRecord
		if err := c.DB.ScanRows(cloneRows, &rec); err != nil {
			continue
		}
		data, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		cloneW.Write(data)
		cloneW.Write([]byte("\n"))
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

	cloneRC, err := in.ReadComponent("voice.clones.v1")
	if err == nil {
		clonePreview := dataportability.ImportComponentPreview{
			ComponentID: "voice.clones.v1",
			Kind:        dataportability.KindDataset,
			LogicalName: "voice.clones.v1",
		}
		c.previewClonedVoices(ctx, cloneRC, &clonePreview)
		previews = append(previews, clonePreview)
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

func (c *VoiceBackupContributor) previewClonedVoices(ctx context.Context, rc io.ReadCloser, preview *dataportability.ImportComponentPreview) {
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec clonedVoiceExportRecord
		if err := json.Unmarshal(line, &rec); err != nil || rec.SpeakerID == "" {
			continue
		}
		preview.ItemCount++

		spaceID := rec.SpaceID
		if spaceID == "" {
			spaceID = "local_user"
		}
		var existing struct {
			SpaceID   string
			SpeakerID string
		}
		c.DB.WithContext(ctx).Table("tts_cloned_voices").Select("space_id, speaker_id").Where("speaker_id = ?", rec.SpeakerID).Scan(&existing)
		if existing.SpeakerID != "" {
			preview.Collisions = append(preview.Collisions, dataportability.ComponentCollision{
				SourceID:   spaceID + ":" + rec.SpeakerID,
				TargetID:   existing.SpaceID + ":" + existing.SpeakerID,
				EntityType: "tts_cloned_voice",
				Policy:     dataportability.CollisionDuplicate,
			})
		}
	}
}

func (c *VoiceBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	opts := dataportability.RestoreOptions{
		OperationID:        req.OperationID,
		Purpose:            dataportability.RestorePurposeOrdinary,
		CharacterPolicy:    req.CharacterPolicy,
		DefaultCharacterID: req.DefaultCharacterID,
		ActivateImported:   req.ActivateImported,
		IdentityMap:        req.IdentityMap,
		SecretProvider:     req.SecretProvider,
	}
	return c.RestoreVoices(ctx, in, opts)
}

func (c *VoiceBackupContributor) RestoreVoices(ctx context.Context, in dataportability.BackupReader, opts dataportability.RestoreOptions) error {
	ttsRC, err := in.ReadComponent("voice.tts.v1")
	if err == nil {
		c.restoreTTS(ctx, ttsRC, opts)
	}

	asrRC, err := in.ReadComponent("voice.asr.v1")
	if err == nil {
		c.restoreASR(ctx, asrRC, opts)
	}

	cloneRC, err := in.ReadComponent("voice.clones.v1")
	if err == nil {
		c.restoreClonedVoices(ctx, cloneRC, opts)
	}

	return nil
}

func (c *VoiceBackupContributor) restoreTTS(ctx context.Context, rc io.ReadCloser, opts dataportability.RestoreOptions) {
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
			switch opts.CharacterPolicy {
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
					"realtime_access_token": "",
					"realtime_secret_key":   "",
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
				"realtime_access_token": "",
				"realtime_secret_key":   "",
				"created_at":            now,
				"updated_at":            now,
			})
			_ = result
		}
	}
}

func (c *VoiceBackupContributor) restoreASR(ctx context.Context, rc io.ReadCloser, opts dataportability.RestoreOptions) {
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
			switch opts.CharacterPolicy {
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

func (c *VoiceBackupContributor) restoreClonedVoices(ctx context.Context, rc io.ReadCloser, opts dataportability.RestoreOptions) {
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec clonedVoiceExportRecord
		if err := json.Unmarshal(line, &rec); err != nil || rec.SpeakerID == "" {
			continue
		}

		spaceID := rec.SpaceID
		if spaceID == "" {
			spaceID = "local_user"
		}
		ttsConfigID := c.resolveRestoredCloneTTSConfigID(ctx, rec)
		var existing struct {
			SpaceID   string
			SpeakerID string
		}
		c.DB.WithContext(ctx).Table("tts_cloned_voices").Select("space_id, speaker_id").Where("speaker_id = ?", rec.SpeakerID).Scan(&existing)
		if existing.SpeakerID != "" {
			// Provider speaker IDs are globally unique within the provider account.
			// Never transfer an existing provider identity to another user during restore.
			if existing.SpaceID != spaceID {
				continue
			}
			switch opts.CharacterPolicy {
			case dataportability.CollisionSkip:
				continue
			case dataportability.CollisionReplace:
				c.DB.WithContext(ctx).Table("tts_cloned_voices").Where("space_id = ? AND speaker_id = ?", spaceID, rec.SpeakerID).Updates(map[string]interface{}{
					"name": rec.Name, "tts_config_id": ttsConfigID, "language": rec.Language, "status": rec.Status, "updated_at": rec.UpdatedAt,
				})
				continue
			default:
				// A cloned voice is keyed by the provider-issued speaker ID and cannot
				// be duplicated locally under a fabricated identity. Keep the target
				// record when the generic import policy asks for duplication.
				continue
			}
		}

		createdAt := rec.CreatedAt
		if createdAt == "" {
			createdAt = "2025-01-01 00:00:00"
		}
		updatedAt := rec.UpdatedAt
		if updatedAt == "" {
			updatedAt = createdAt
		}
		status := rec.Status
		if status == "" {
			status = "ready"
		}
		c.DB.WithContext(ctx).Table("tts_cloned_voices").Create(map[string]interface{}{
			"space_id": spaceID, "speaker_id": rec.SpeakerID, "name": rec.Name, "tts_config_id": ttsConfigID, "language": rec.Language, "status": status,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
}

func (c *VoiceBackupContributor) resolveRestoredCloneTTSConfigID(ctx context.Context, rec clonedVoiceExportRecord) int {
	if rec.TtsConfigName != "" {
		var target struct{ ID int }
		c.DB.WithContext(ctx).Table("tts_configs").Select("id").Where("name = ?", rec.TtsConfigName).Limit(1).Scan(&target)
		if target.ID > 0 {
			return target.ID
		}
	}
	if rec.TtsConfigID > 0 {
		var target struct{ ID int }
		c.DB.WithContext(ctx).Table("tts_configs").Select("id").Where("id = ?", rec.TtsConfigID).Limit(1).Scan(&target)
		if target.ID > 0 {
			return target.ID
		}
	}
	return 0
}

func (c *VoiceBackupContributor) importTTS(ctx context.Context, req dataportability.ImportRequest, rc io.ReadCloser) {
	opts := dataportability.RestoreOptions{
		CharacterPolicy: req.CharacterPolicy,
	}
	c.restoreTTS(ctx, rc, opts)
}

func (c *VoiceBackupContributor) importASR(ctx context.Context, req dataportability.ImportRequest, rc io.ReadCloser) {
	opts := dataportability.RestoreOptions{
		CharacterPolicy: req.CharacterPolicy,
	}
	c.restoreASR(ctx, rc, opts)
}

var _ dataportability.VoiceRestorePort = (*VoiceBackupContributor)(nil)
