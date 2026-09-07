// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/requestidentity"
	"gorm.io/gorm"
)

type Repository interface {
	List() ([]TtsConfig, error)
	GetByID(id int) (*TtsConfig, error)
	Create(cfg *TtsConfig) error
	Update(id int, updates map[string]interface{}) error
	Delete(id int) error
	Activate(id int) error
	GetActive() (*TtsConfig, error)
	GetByCharacterID(userID, charID string) (*TtsConfig, error)
	ListProviders() []ProviderInfo
	ListClonedVoices(userID string) ([]ClonedVoice, error)
	GetClonedVoice(userID, speakerID string) (*ClonedVoice, error)
	GetClonedVoiceBySpeakerID(speakerID string) (*ClonedVoice, error)
	ResolveClonedVoiceConfig(userID, speakerID string) (*TtsConfig, *ClonedVoice, error)
	UpsertClonedVoice(voice *ClonedVoice) error
	DeleteClonedVoice(userID, speakerID string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) List() ([]TtsConfig, error) {
	var configs []TtsConfig
	err := r.db.Order("is_active DESC, created_at DESC").Find(&configs).Error
	if configs == nil {
		configs = []TtsConfig{}
	}
	return configs, err
}

func (r *repository) GetByID(id int) (*TtsConfig, error) {
	var cfg TtsConfig
	err := r.db.Where("id = ?", id).First(&cfg).Error
	return &cfg, err
}

func (r *repository) Create(cfg *TtsConfig) error {
	return r.db.Create(cfg).Error
}

func (r *repository) Update(id int, updates map[string]interface{}) error {
	return r.db.Model(&TtsConfig{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&TtsConfig{}).Error
}

func (r *repository) Activate(id int) error {
	r.db.Model(&TtsConfig{}).Where("is_active = 1").Update("is_active", 0)
	return r.db.Model(&TtsConfig{}).Where("id = ?", id).Update("is_active", 1).Error
}

func (r *repository) GetActive() (*TtsConfig, error) {
	var cfg TtsConfig
	err := r.db.Where("is_active = 1").First(&cfg).Error
	return &cfg, err
}

// resolveActiveConfig deliberately does not require an API key. Keyless TTS
// providers (for example Edge TTS and local CosyVoice) are valid active
// configurations and must not silently fall back to Volcengine.
func (r *repository) resolveActiveConfig() *TtsConfig {
	active, err := r.GetActive()
	if err == nil && active != nil {
		return active
	}
	var anyCfg TtsConfig
	if err2 := r.db.Order("created_at DESC").First(&anyCfg).Error; err2 == nil {
		return &anyCfg
	}
	return &TtsConfig{
		ApiType:    "volcengine",
		ResourceId: "seed-tts-2.0",
		VoiceType:  "zh_female_vv_uranus_bigtts",
		Speed:      1.0,
		Pitch:      1.0,
		Volume:     1.0,
	}
}

func ttsLocalSingleUserMode() bool {
	return config.AppCfg != nil && strings.EqualFold(strings.TrimSpace(config.AppCfg.Security.Mode), "local_single_user")
}

func ttsOwnerQuery(db *gorm.DB, column, userID string) *gorm.DB {
	owner := requestidentity.NormalizeUserID(userID)
	if ttsLocalSingleUserMode() {
		return db.Where(
			fmt.Sprintf("(%s = ? OR %s = '' OR %s IS NULL OR %s = ?)", column, column, column, column),
			owner,
			requestidentity.DefaultUserID,
		)
	}
	return db.Where(fmt.Sprintf("%s = ?", column), owner)
}

func ttsSameOwner(existingUserID, requestedUserID string) bool {
	existingUserID = strings.TrimSpace(existingUserID)
	requestedUserID = requestidentity.NormalizeUserID(requestedUserID)
	if existingUserID == requestedUserID {
		return true
	}
	return ttsLocalSingleUserMode() && (existingUserID == "" || existingUserID == requestidentity.DefaultUserID)
}

func (r *repository) GetByCharacterID(userID, charID string) (*TtsConfig, error) {
	var char struct {
		VoiceType       string
		VoiceSpeed      float64
		VoicePitch      float64
		VoiceVolume     float64
		CustomVoiceID   string
		VoiceMode       string
		VoiceConfigID   string
		Emotion         string
		EmotionScale    int
		SilenceDuration int
	}
	row := ttsOwnerQuery(r.db.Table("characters"), "user_id", userID).Select(
		"voice_type, voice_speed, voice_pitch, voice_volume, custom_voice_id, voice_mode, voice_config_id, emotion, emotion_scale, silence_duration",
	).Where("id = ?", strings.TrimSpace(charID)).Row()
	if err := row.Scan(
		&char.VoiceType,
		&char.VoiceSpeed,
		&char.VoicePitch,
		&char.VoiceVolume,
		&char.CustomVoiceID,
		&char.VoiceMode,
		&char.VoiceConfigID,
		&char.Emotion,
		&char.EmotionScale,
		&char.SilenceDuration,
	); err != nil {
		return nil, fmt.Errorf("角色不存在或不属于当前用户: %w", err)
	}

	base := r.resolveActiveConfig()
	if rawID := strings.TrimSpace(char.VoiceConfigID); rawID != "" {
		if id, err := strconv.Atoi(rawID); err == nil && id > 0 {
			if selected, getErr := r.GetByID(id); getErr == nil && selected != nil {
				base = selected
			}
		}
	}

	cloneID := ""
	if char.VoiceMode == "clone" && strings.TrimSpace(char.CustomVoiceID) != "" {
		cloneID = strings.TrimSpace(char.CustomVoiceID)
		cloneVoice, cloneErr := r.GetClonedVoice(userID, cloneID)
		if cloneErr != nil {
			// Local single-user deployments can have characters created before clone
			// metadata existed. Preserve that legacy path locally, but cloud mode
			// must never accept an unowned provider speaker id.
			if !ttsLocalSingleUserMode() {
				return nil, fmt.Errorf("角色复刻音色不存在或不属于当前用户")
			}
		} else if cloneVoice.TtsConfigID > 0 {
			boundConfig, boundErr := r.GetByID(cloneVoice.TtsConfigID)
			if boundErr != nil {
				return nil, fmt.Errorf("复刻音色绑定的 TTS 配置不存在")
			}
			base = boundConfig
		}
	}

	// Start from a full provider config. The previous implementation only copied
	// ApiKey/ResourceId, which discarded api_type/base_url/realtime credentials
	// and could silently route a character configured for another provider to
	// Volcengine.
	cfg := *base
	if char.VoiceType != "" {
		cfg.VoiceType = char.VoiceType
	}
	if char.VoiceSpeed != 0 {
		cfg.Speed = char.VoiceSpeed
	}
	if char.VoicePitch != 0 {
		cfg.Pitch = char.VoicePitch
	}
	if char.VoiceVolume != 0 {
		cfg.Volume = char.VoiceVolume
	}
	cfg.Emotion = char.Emotion
	cfg.EmotionScale = char.EmotionScale
	cfg.SilenceDuration = char.SilenceDuration

	if cloneID != "" {
		apiType := strings.ToLower(strings.TrimSpace(cfg.ApiType))
		if apiType != "" && apiType != "volcengine" {
			return nil, fmt.Errorf("角色复刻音色需要使用火山引擎 TTS 配置")
		}
		cfg.ApiType = "volcengine"
		cfg.VoiceType = cloneID
		cfg.ResourceId = "seed-icl-2.0"
	}

	if cfg.ApiType == "" {
		cfg.ApiType = "volcengine"
	}
	if cfg.VoiceType == "" {
		cfg.VoiceType = "zh_female_vv_uranus_bigtts"
	}
	if cfg.Speed == 0 {
		cfg.Speed = 1.0
	}
	if cfg.Pitch == 0 {
		cfg.Pitch = 1.0
	}
	if cfg.Volume == 0 {
		cfg.Volume = 1.0
	}
	return &cfg, nil
}

func (r *repository) ListClonedVoices(userID string) ([]ClonedVoice, error) {
	var voices []ClonedVoice
	err := ttsOwnerQuery(r.db.Model(&ClonedVoice{}), "user_id", userID).Order("created_at DESC").Find(&voices).Error
	if voices == nil {
		voices = []ClonedVoice{}
	}
	return voices, err
}

func (r *repository) GetClonedVoice(userID, speakerID string) (*ClonedVoice, error) {
	var voice ClonedVoice
	err := ttsOwnerQuery(r.db.Model(&ClonedVoice{}), "user_id", userID).Where("speaker_id = ?", speakerID).First(&voice).Error
	return &voice, err
}

func (r *repository) GetClonedVoiceBySpeakerID(speakerID string) (*ClonedVoice, error) {
	var voice ClonedVoice
	err := r.db.Where("speaker_id = ?", speakerID).First(&voice).Error
	return &voice, err
}

func (r *repository) ResolveClonedVoiceConfig(userID, speakerID string) (*TtsConfig, *ClonedVoice, error) {
	voice, err := r.GetClonedVoice(userID, speakerID)
	if err != nil {
		return nil, nil, err
	}
	if voice.TtsConfigID > 0 {
		cfg, cfgErr := r.GetByID(voice.TtsConfigID)
		if cfgErr != nil {
			return nil, voice, fmt.Errorf("复刻音色绑定的 TTS 配置不存在: %w", cfgErr)
		}
		return cfg, voice, nil
	}
	// Legacy metadata did not record its provider config. Preserve old local
	// installations by falling back to the current active config only for those
	// records; newly created clone voices always persist TtsConfigID.
	cfg, cfgErr := r.GetActive()
	if cfgErr != nil {
		return nil, voice, cfgErr
	}
	return cfg, voice, nil
}

func (r *repository) UpsertClonedVoice(voice *ClonedVoice) error {
	if voice == nil || strings.TrimSpace(voice.UserID) == "" || strings.TrimSpace(voice.SpeakerID) == "" {
		return gorm.ErrInvalidData
	}
	var existing ClonedVoice
	err := r.db.Where("speaker_id = ?", voice.SpeakerID).First(&existing).Error
	if err == nil {
		if !ttsSameOwner(existing.UserID, voice.UserID) {
			return fmt.Errorf("speakerId 已属于其他用户")
		}
		return r.db.Model(&ClonedVoice{}).Where("speaker_id = ?", voice.SpeakerID).Updates(map[string]interface{}{
			"user_id": requestidentity.NormalizeUserID(voice.UserID),
			"name":    voice.Name, "tts_config_id": voice.TtsConfigID, "language": voice.Language, "status": voice.Status, "updated_at": voice.UpdatedAt,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(voice).Error
}

func (r *repository) DeleteClonedVoice(userID, speakerID string) error {
	return ttsOwnerQuery(r.db.Model(&ClonedVoice{}), "user_id", userID).Where("speaker_id = ?", speakerID).Delete(&ClonedVoice{}).Error
}

func (r *repository) ListProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: "volcengine", Name: "火山引擎", DefaultBaseURL: "https://openspeech.bytedance.com", DefaultModel: "seed-tts-2.0"},
		{ID: "openai", Name: "OpenAI", DefaultBaseURL: "https://api.openai.com/v1", DefaultModel: "tts-1"},
		{ID: "azure", Name: "Azure Speech", DefaultBaseURL: "https://tts.speech.microsoft.com", DefaultModel: "azure-tts"},
		{ID: "edge", Name: "Edge TTS", DefaultBaseURL: "https://speech.platform.bing.com", DefaultModel: "edge-tts"},
		{ID: "elevenlabs", Name: "ElevenLabs", DefaultBaseURL: "https://api.elevenlabs.io/v1", DefaultModel: "eleven_multilingual_v2"},
		{ID: "minimax", Name: "MiniMax", DefaultBaseURL: "https://api.minimax.chat/v1", DefaultModel: "speech-01-hd"},
		{ID: "aliyun", Name: "阿里云", DefaultBaseURL: "https://nls-gateway.cn-shanghai.aliyuncs.com", DefaultModel: "nls-tts"},
		{ID: "cosyvoice", Name: "CosyVoice", DefaultBaseURL: "http://127.0.0.1:5000", DefaultModel: "cosyvoice-v2"},
	}
}
