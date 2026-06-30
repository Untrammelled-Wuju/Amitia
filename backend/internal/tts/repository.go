// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
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
	GetByCharacterID(charID string) (*TtsConfig, error)
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

func (r *repository) GetByCharacterID(charID string) (*TtsConfig, error) {
	var char struct {
		VoiceType       string
		VoiceSpeed      float64
		VoicePitch      float64
		VoiceVolume     float64
		CustomVoiceID   string
		VoiceMode       string
		Emotion         string
		EmotionScale    int
		SilenceDuration int
	}
	err := r.db.Table("characters").Select("voice_type, voice_speed, voice_pitch, voice_volume, custom_voice_id, voice_mode, emotion, emotion_scale, silence_duration").Where("id = ?", charID).Row().Scan(&char.VoiceType, &char.VoiceSpeed, &char.VoicePitch, &char.VoiceVolume, &char.CustomVoiceID, &char.VoiceMode, &char.Emotion, &char.EmotionScale, &char.SilenceDuration)
	if err != nil {
		return r.GetActive()
	}
	active, _ := r.GetActive()
	if active == nil {
		active = &TtsConfig{VoiceType: "zh_female_vv_uranus_bigtts", Speed: 1.0, Pitch: 1.0, Volume: 1.0}
	}
	cfg := &TtsConfig{
		ApiKey:          active.ApiKey,
		ResourceId:      active.ResourceId,
		VoiceType:       char.VoiceType,
		Speed:           char.VoiceSpeed,
		Pitch:           char.VoicePitch,
		Volume:          char.VoiceVolume,
		Emotion:         char.Emotion,
		EmotionScale:    char.EmotionScale,
		SilenceDuration: char.SilenceDuration,
	}
	if cfg.VoiceType == "" {
		cfg.VoiceType = active.VoiceType
	}
	if cfg.Speed == 0 {
		cfg.Speed = active.Speed
	}
	if cfg.Pitch == 0 {
		cfg.Pitch = active.Pitch
	}
	if cfg.Volume == 0 {
		cfg.Volume = active.Volume
	}
	if char.VoiceMode == "clone" && char.CustomVoiceID != "" {
		cfg.VoiceType = char.CustomVoiceID
		cfg.ResourceId = "seed-icl-2.0"
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
	return cfg, nil
}
