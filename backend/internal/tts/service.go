// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Service interface {
	List() ([]TtsConfig, error)
	GetByID(id int) (*TtsConfig, error)
	GetProviderConfigByID(id int) (*TtsConfig, error)
	Create(req *CreateTtsConfigRequest) (*TtsConfig, error)
	Update(id int, updates map[string]interface{}) (*TtsConfig, error)
	Delete(id int) error
	Activate(id int) (*TtsConfig, error)
	GetActive() (*TtsConfig, error)
	GetAvailableVoices() []VoicePreset
	GetEmotions() []string
	Test(id int) error
	Synthesize(voiceID int, text string) (*SynthesizeResponse, error)
	SynthesizePreview(req *SynthesizePreviewRequest) (*SynthesizeResponse, error)
	SynthesizeForCharacter(spaceID, charID string, text string) (*SynthesizeResponse, error)
	SynthesizeWithSpeaker(spaceID, speakerID, text string) (*SynthesizeResponse, error)
	SynthesizeWithActive(text string) (*SynthesizeResponse, error)
	ListProviders() []ProviderInfo
	ListClonedVoices(spaceID string) ([]ClonedVoice, error)
	GetClonedVoice(spaceID, speakerID string) (*ClonedVoice, error)
	GetClonedVoiceBySpeakerID(speakerID string) (*ClonedVoice, error)
	ResolveClonedVoiceProviderConfig(spaceID, speakerID string) (*TtsConfig, *ClonedVoice, error)
	SaveClonedVoice(voice *ClonedVoice) error
	DeleteClonedVoiceMetadata(spaceID, speakerID string) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func redactConfigForResponse(cfg *TtsConfig) {
	if cfg == nil {
		return
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	cfg.ApiKey = ""
	cfg.RealtimeAccessToken = ""
	cfg.RealtimeSecretKey = ""
}
func (s *service) List() ([]TtsConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		redactConfigForResponse(&configs[i])
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*TtsConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("音色配置不存在")
	}
	redactConfigForResponse(cfg)
	return cfg, nil
}

func (s *service) GetProviderConfigByID(id int) (*TtsConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("音色配置不存在")
	}
	return cfg, nil
}

func (s *service) Create(req *CreateTtsConfigRequest) (*TtsConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	if req.ApiType == "" {
		req.ApiType = "volcengine"
	}
	if req.VoiceType == "" {
		req.VoiceType = "zh_female_cancan_mars_bigtts"
	}
	if req.ResourceId == "" {
		req.ResourceId = "seed-tts-2.0"
	}
	if req.Speed == 0 {
		req.Speed = 1.0
	}
	if req.Pitch == 0 {
		req.Pitch = 1.0
	}
	if req.Volume == 0 {
		req.Volume = 1.0
	}
	if req.CloneResourceId == "" {
		req.CloneResourceId = "volc.megatts.timbre"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &TtsConfig{
		Name: req.Name, ApiType: req.ApiType, ApiKey: req.ApiKey, BaseURL: req.BaseURL, ResourceId: req.ResourceId,
		VoiceType: req.VoiceType, Emotion: req.Emotion,
		Speed: req.Speed, Pitch: req.Pitch, Volume: req.Volume,
		CloneResourceId:     req.CloneResourceId,
		RealtimeAppId:       req.RealtimeAppId,
		RealtimeAccessToken: req.RealtimeAccessToken,
		RealtimeSecretKey:   req.RealtimeSecretKey,
		IsActive:            req.IsActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	redactConfigForResponse(cfg)
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*TtsConfig, error) {
	if err := s.repo.Update(id, normalizeConfigUpdates(updates)); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		redactConfigForResponse(cfg)
	}
	return cfg, nil
}

func (s *service) Delete(id int) error { return s.repo.Delete(id) }
func (s *service) Activate(id int) (*TtsConfig, error) {
	if err := s.repo.Activate(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		redactConfigForResponse(cfg)
	}
	return cfg, nil
}

func (s *service) GetActive() (*TtsConfig, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, err
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) GetAvailableVoices() []VoicePreset { return GetAvailableVoices() }
func (s *service) GetEmotions() []string             { return GetEmotions() }

func (s *service) Test(id int) error {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("音色配置不存在")
	}
	return TestConnection(cfg)
}
func (s *service) Synthesize(voiceID int, text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetByID(voiceID)
	if err != nil {
		return nil, fmt.Errorf("音色配置不存在")
	}
	return Synthesize(cfg, text)
}
func (s *service) SynthesizePreview(req *SynthesizePreviewRequest) (*SynthesizeResponse, error) {
	if req == nil || strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("试听文本不能为空")
	}
	base, err := s.repo.GetActive()
	if rawID := strings.TrimSpace(req.VoiceConfigID); rawID != "" {
		if id, parseErr := strconv.Atoi(rawID); parseErr == nil && id > 0 {
			base, err = s.repo.GetByID(id)
		}
	}
	if err != nil || base == nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	cfg := *base
	if strings.TrimSpace(req.VoiceType) != "" {
		cfg.VoiceType = strings.TrimSpace(req.VoiceType)
	}
	if req.Speed > 0 {
		cfg.Speed = req.Speed
	}
	if req.Pitch > 0 {
		cfg.Pitch = req.Pitch
	}
	if req.Volume > 0 {
		cfg.Volume = req.Volume
	}
	cfg.Emotion = strings.TrimSpace(req.Emotion)
	cfg.EmotionScale = req.EmotionScale
	cfg.SilenceDuration = req.SilenceDuration
	return Synthesize(&cfg, req.Text)
}
func (s *service) SynthesizeWithSpeaker(spaceID, speakerID, text string) (*SynthesizeResponse, error) {
	cfg, _, err := s.repo.ResolveClonedVoiceConfig(spaceID, speakerID)
	if err != nil {
		return nil, fmt.Errorf("复刻音色不存在或不属于当前用户")
	}
	apiType := strings.ToLower(strings.TrimSpace(cfg.ApiType))
	if apiType != "" && apiType != "volcengine" {
		return nil, fmt.Errorf("复刻音色试听需要使用火山引擎 TTS 配置")
	}
	cfg.ApiType = "volcengine"
	cfg.VoiceType = speakerID
	cfg.ResourceId = "seed-icl-2.0"
	return Synthesize(cfg, text)
}
func (s *service) SynthesizeWithActive(text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	return Synthesize(cfg, text)
}

func (s *service) SynthesizeForCharacter(spaceID, charID string, text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetByCharacterID(spaceID, charID)
	if err != nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	return Synthesize(cfg, text)
}

func (s *service) ListClonedVoices(spaceID string) ([]ClonedVoice, error) {
	return s.repo.ListClonedVoices(spaceID)
}

func (s *service) GetClonedVoice(spaceID, speakerID string) (*ClonedVoice, error) {
	return s.repo.GetClonedVoice(spaceID, speakerID)
}

func (s *service) GetClonedVoiceBySpeakerID(speakerID string) (*ClonedVoice, error) {
	return s.repo.GetClonedVoiceBySpeakerID(speakerID)
}

func (s *service) ResolveClonedVoiceProviderConfig(spaceID, speakerID string) (*TtsConfig, *ClonedVoice, error) {
	return s.repo.ResolveClonedVoiceConfig(spaceID, speakerID)
}

func (s *service) SaveClonedVoice(voice *ClonedVoice) error {
	return s.repo.UpsertClonedVoice(voice)
}

func (s *service) DeleteClonedVoiceMetadata(spaceID, speakerID string) error {
	return s.repo.DeleteClonedVoice(spaceID, speakerID)
}

func (s *service) ListProviders() []ProviderInfo {
	return s.repo.ListProviders()
}

func normalizeConfigUpdates(updates map[string]interface{}) map[string]interface{} {
	if len(updates) == 0 {
		return updates
	}
	aliases := map[string]string{
		"apiType":             "api_type",
		"baseUrl":             "base_url",
		"apiKey":              "api_key",
		"isActive":            "is_active",
		"resourceId":          "resource_id",
		"voiceType":           "voice_type",
		"customVoiceId":       "custom_voice_id",
		"cloneResourceId":     "clone_resource_id",
		"realtimeAppId":       "realtime_app_id",
		"realtimeAccessToken": "realtime_access_token",
		"realtimeSecretKey":   "realtime_secret_key",
	}
	allowed := map[string]bool{
		"name": true, "api_type": true, "api_key": true, "base_url": true,
		"resource_id": true, "voice_type": true, "emotion": true, "speed": true,
		"pitch": true, "volume": true, "is_active": true, "is_custom": true,
		"custom_voice_id": true, "clone_resource_id": true, "realtime_app_id": true,
		"realtime_access_token": true, "realtime_secret_key": true,
	}
	preserveOnEmpty := map[string]bool{
		"api_key": true, "realtime_access_token": true, "realtime_secret_key": true,
	}
	normalized := make(map[string]interface{}, len(updates))
	for key, value := range updates {
		column := key
		if alias, ok := aliases[key]; ok {
			column = alias
		}
		if !allowed[column] {
			continue
		}
		if preserveOnEmpty[column] {
			if text, ok := value.(string); ok && text == "" {
				continue
			}
		}
		normalized[column] = value
	}
	return normalized
}
