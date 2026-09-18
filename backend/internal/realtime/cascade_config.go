// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"fmt"
	"strings"
)

const (
	defaultCascadeASREndpoint   = "wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async"
	defaultCascadeASRResourceID = "volc.seedasr.sauc.duration"
	defaultCascadeTTSEndpoint   = "wss://openspeech.bytedance.com/api/v3/tts/bidirection"
	defaultCascadeTTSResourceID = "seed-tts-2.0"
	defaultCascadeTTSModel      = "seed-tts-2.0-expressive"
	defaultCascadeTTSVoice      = "zh_female_vv_uranus_bigtts"
)

type cascadeSpeechConfig struct {
	ApiKey     string
	AppID      string
	AccessKey  string
	ResourceID string
	Endpoint   string
	Model      string
	VoiceType  string
}

type cascadeASRSettings struct {
	ApiKey     string `gorm:"column:api_key"`
	BaseURL    string `gorm:"column:base_url"`
	ResourceID string `gorm:"column:resource_id"`
	ApiType    string `gorm:"column:api_type"`
}

type cascadeTTSSettings struct {
	ApiKey            string `gorm:"column:api_key"`
	BaseURL           string `gorm:"column:base_url"`
	ResourceID        string `gorm:"column:resource_id"`
	VoiceType         string `gorm:"column:voice_type"`
	ApiType           string `gorm:"column:api_type"`
	RealtimeAppID     string `gorm:"column:realtime_app_id"`
	RealtimeAccessKey string `gorm:"column:realtime_access_token"`
}

func resolveCascadeASRConfig() (cascadeSpeechConfig, error) {
	cfg := cascadeSpeechConfig{
		ResourceID: defaultCascadeASRResourceID,
		Endpoint:   defaultCascadeASREndpoint,
	}
	if dbInstance == nil {
		return cfg, fmt.Errorf("speech configuration storage unavailable")
	}
	var settings cascadeASRSettings
	if err := dbInstance.Table("asr_configs").Where("is_active = 1").First(&settings).Error; err != nil {
		return cfg, fmt.Errorf("load active asr config: %w", err)
	}
	cfg.ApiKey = strings.TrimSpace(settings.ApiKey)
	cfg.AppID = ""
	cfg.AccessKey = ""
	if endpoint := strings.TrimSpace(settings.BaseURL); strings.HasPrefix(endpoint, "wss://") || strings.HasPrefix(endpoint, "ws://") {
		cfg.Endpoint = endpoint
	}
	resourceID := strings.TrimSpace(settings.ResourceID)
	if resourceID != "" && !strings.HasPrefix(resourceID, "volc.seedasr.auc") {
		cfg.ResourceID = resourceID
	}
	if cfg.ApiKey == "" {
		return cfg, fmt.Errorf("active asr config has no api key")
	}
	return cfg, nil
}

func resolveCascadeTTSConfig(voiceOverride string) (cascadeSpeechConfig, error) {
	cfg := cascadeSpeechConfig{
		ResourceID: defaultCascadeTTSResourceID,
		Endpoint:   defaultCascadeTTSEndpoint,
		Model:      defaultCascadeTTSModel,
		VoiceType:  defaultCascadeTTSVoice,
	}
	if dbInstance == nil {
		return cfg, fmt.Errorf("speech configuration storage unavailable")
	}
	var settings cascadeTTSSettings
	if err := dbInstance.Table("tts_configs").Where("is_active = 1").First(&settings).Error; err != nil {
		return cfg, fmt.Errorf("load active tts config: %w", err)
	}
	cfg.ApiKey = strings.TrimSpace(settings.ApiKey)
	cfg.AppID = strings.TrimSpace(settings.RealtimeAppID)
	cfg.AccessKey = strings.TrimSpace(settings.RealtimeAccessKey)
	if endpoint := strings.TrimSpace(settings.BaseURL); strings.HasPrefix(endpoint, "wss://") || strings.HasPrefix(endpoint, "ws://") {
		cfg.Endpoint = endpoint
	}
	resourceID := strings.TrimSpace(settings.ResourceID)
	if resourceID != "" && !strings.HasPrefix(resourceID, "volc.speech.") {
		cfg.ResourceID = resourceID
	}
	if voice := strings.TrimSpace(settings.VoiceType); voice != "" {
		cfg.VoiceType = voice
	}
	if voice := strings.TrimSpace(voiceOverride); voice != "" {
		cfg.VoiceType = voice
	}
	if cfg.ApiKey == "" && (cfg.AppID == "" || cfg.AccessKey == "") {
		return cfg, fmt.Errorf("active tts config has no usable credentials")
	}
	return cfg, nil
}

func cascadeTTSVoiceID(voiceID string) string {
	voiceID = strings.TrimSpace(voiceID)
	switch strings.ToLower(voiceID) {
	case "":
		return defaultCascadeTTSVoice
	case "zh_female_xiaohe_jupiter_bigtts":
		return "zh_female_xiaohe_uranus_bigtts"
	case "zh_female_vv_jupiter_bigtts":
		return "zh_female_vv_uranus_bigtts"
	default:
		return voiceID
	}
}

func cascadeTTSResourceID(resourceID, voiceID string) string {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID != "" && !strings.HasPrefix(resourceID, "volc.speech.") {
		return resourceID
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(voiceID)), "S_") {
		return "seed-icl-2.0"
	}
	return defaultCascadeTTSResourceID
}

func cascadeTTSLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh", "zh-cn", "zh_cn", "cn":
		return "zh-cn"
	case "en", "en-us", "en_us":
		return "en"
	case "ja", "ja-jp", "ja_jp":
		return "ja"
	case "ko", "ko-kr", "ko_kr":
		return "ko"
	case "es", "es-mx", "es_mx":
		return "es-mx"
	case "id", "id-id", "id_id":
		return "id"
	case "pt", "pt-br", "pt_br":
		return "pt-br"
	default:
		return ""
	}
}
