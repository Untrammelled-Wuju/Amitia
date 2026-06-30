// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"fmt"
	"time"
)

type Service interface {
	List() ([]TtsConfig, error)
	GetByID(id int) (*TtsConfig, error)
	Create(req *CreateTtsConfigRequest) (*TtsConfig, error)
	Update(id int, updates map[string]interface{}) (*TtsConfig, error)
	Delete(id int) error
	Activate(id int) (*TtsConfig, error)
	GetActive() (*TtsConfig, error)
	GetAvailableVoices() []VoicePreset
	GetEmotions() []string
	Test(id int) error
	Synthesize(voiceID int, text string) (*SynthesizeResponse, error)
	SynthesizeForCharacter(charID string, text string) (*SynthesizeResponse, error)
	SynthesizeWithSpeaker(speakerID string, text string) (*SynthesizeResponse, error)
	SynthesizeWithActive(text string) (*SynthesizeResponse, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List() ([]TtsConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].HasApiKey = configs[i].ApiKey != ""
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*TtsConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("音色配置不存在")
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Create(req *CreateTtsConfigRequest) (*TtsConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
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
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &TtsConfig{
		Name: req.Name, ApiKey: req.ApiKey, ResourceId: req.ResourceId,
		VoiceType: req.VoiceType, Emotion: req.Emotion,
		Speed: req.Speed, Pitch: req.Pitch, Volume: req.Volume,
		RealtimeAppId:       req.RealtimeAppId,
		RealtimeAccessToken: req.RealtimeAccessToken,
		RealtimeSecretKey:   req.RealtimeSecretKey,
		IsActive:            req.IsActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*TtsConfig, error) {
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
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
		cfg.HasApiKey = cfg.ApiKey != ""
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
func (s *service) SynthesizeWithSpeaker(speakerID string, text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	cfg.VoiceType = speakerID
	return Synthesize(cfg, text)
}
func (s *service) SynthesizeWithActive(text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	cfg.ResourceId = "seed-tts-2.0"
	return Synthesize(cfg, text)
}

func (s *service) SynthesizeForCharacter(charID string, text string) (*SynthesizeResponse, error) {
	cfg, err := s.repo.GetByCharacterID(charID)
	if err != nil {
		return nil, fmt.Errorf("没有可用的音色配置")
	}
	return Synthesize(cfg, text)
}
