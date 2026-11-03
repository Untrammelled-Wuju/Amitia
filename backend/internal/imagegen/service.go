// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package imagegen

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/imageprovider"
)

type Service interface {
	List() ([]ImageGenConfig, error)
	GetByID(id int) (*ImageGenConfig, error)
	GetRuntimeByID(id int) (*ImageGenConfig, error)
	Create(req *CreateImageGenConfigRequest) (*ImageGenConfig, error)
	Update(id int, updates map[string]interface{}) (*ImageGenConfig, error)
	Delete(id int) error
	Activate(id int) (*ImageGenConfig, error)
	GetActive() (*ImageGenConfig, error)
	TestConnection(ctx context.Context, id int) (map[string]interface{}, error)
	ListProviders() []ProviderInfo
}

type service struct {
	repo     Repository
	registry *imageprovider.Registry
}

func NewService(repo Repository, registry *imageprovider.Registry) Service {
	return &service{repo: repo, registry: registry}
}

func redactConfigForResponse(cfg *ImageGenConfig) {
	if cfg == nil {
		return
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	cfg.ApiKey = ""
}
func (s *service) List() ([]ImageGenConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		redactConfigForResponse(&configs[i])
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*ImageGenConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("生图模型配置不存在: %w", err)
	}
	redactConfigForResponse(cfg)
	return cfg, nil
}

func (s *service) GetRuntimeByID(id int) (*ImageGenConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("生图模型配置不存在: %w", err)
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Create(req *CreateImageGenConfigRequest) (*ImageGenConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	if req.ApiType == "" {
		req.ApiType = "seedream"
	}
	if req.ModelName == "" {
		req.ModelName = "doubao-seedream-5-0"
	}
	if req.BaseUrl == "" {
		req.BaseUrl = "https://ark.cn-beijing.volces.com/api/v3"
	}
	if err := s.requireProvider(req.ApiType); err != nil {
		return nil, err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &ImageGenConfig{
		Name: req.Name, ApiType: req.ApiType, ApiKey: req.ApiKey, ModelName: req.ModelName,
		BaseUrl: req.BaseUrl, IsActive: req.IsActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	redactConfigForResponse(cfg)
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*ImageGenConfig, error) {
	normalized := normalizeConfigUpdates(updates)
	if apiType, ok := normalized["api_type"].(string); ok {
		if err := s.requireProvider(apiType); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(id, normalized); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		redactConfigForResponse(cfg)
	}
	return cfg, nil
}

func (s *service) Delete(id int) error { return s.repo.Delete(id) }

func (s *service) Activate(id int) (*ImageGenConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("生图模型配置不存在: %w", err)
	}
	if err := s.requireProvider(cfg.ApiType); err != nil {
		return nil, err
	}
	if err := s.repo.Activate(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	cfg, err = s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		redactConfigForResponse(cfg)
	}
	return cfg, nil
}

func (s *service) requireProvider(apiType string) error {
	name := imageprovider.NormalizeProviderName(apiType)
	if name == "" {
		return fmt.Errorf("生图 Provider 不能为空")
	}
	if s.registry == nil {
		return fmt.Errorf("生图 Provider Registry 不可用")
	}
	if _, ok := s.registry.Resolve(name); !ok {
		return fmt.Errorf("生图 Provider %s 未注册", name)
	}
	return nil
}

func (s *service) GetActive() (*ImageGenConfig, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, err
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) TestConnection(ctx context.Context, id int) (map[string]interface{}, error) {
	cfg, err := s.GetRuntimeByID(id)
	if err != nil {
		return nil, err
	}
	if s.registry == nil {
		return nil, fmt.Errorf("生图 Provider Registry 不可用")
	}
	providerName, provider, ok := s.registry.Select(cfg.ApiType, cfg.ApiKey)
	if !ok {
		return nil, fmt.Errorf("生图 Provider %s 未注册", cfg.ApiType)
	}
	if cfg.ApiKey == "" && providerName != "cloud_core" {
		return nil, fmt.Errorf("API Key未配置")
	}
	providerConfig := imageprovider.ImageModelConfig{
		ConfigID:  cfg.ID,
		Name:      cfg.Name,
		ApiType:   cfg.ApiType,
		ApiKey:    cfg.ApiKey,
		ModelName: cfg.ModelName,
		BaseUrl:   cfg.BaseUrl,
	}
	if providerName == "cloud_core" {
		providerConfig.ConfigID = 0
	}
	start := time.Now()
	if probe, supported := provider.(imageprovider.ConnectionProbe); supported {
		submission, probeErr := probe.Probe(ctx, providerConfig)
		err = probeErr
		if probeErr == nil && (submission == nil || submission.Result == nil || len(submission.Result.Images) == 0) {
			err = fmt.Errorf("生图 Provider %s 测试未返回图片", cfg.ApiType)
		}
	} else {
		err = provider.ValidateConfig(ctx, providerConfig)
	}
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error(), "latency": latency}, nil
	}
	return map[string]interface{}{"success": true, "message": "连接成功", "provider": cfg.ApiType, "latency": latency}, nil
}

func (s *service) ListProviders() []ProviderInfo {
	if s.registry == nil {
		return nil
	}
	descriptors := s.registry.Descriptors()
	sort.SliceStable(descriptors, func(i, j int) bool {
		order := map[string]int{"seedream": 0, "openai": 1, "stability": 2, "tongyi": 3, "cogview": 4, "siliconflow": 5}
		left, leftOK := order[descriptors[i].Name]
		right, rightOK := order[descriptors[j].Name]
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && left != right {
			return left < right
		}
		return descriptors[i].Name < descriptors[j].Name
	})
	providers := make([]ProviderInfo, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.DeviceOnly {
			continue
		}
		providers = append(providers, ProviderInfo{
			ID:             descriptor.Name,
			Name:           descriptor.DisplayName,
			DefaultBaseURL: descriptor.DefaultBaseURL,
			DefaultModel:   descriptor.DefaultModel,
		})
	}
	return providers
}

func normalizeConfigUpdates(updates map[string]interface{}) map[string]interface{} {
	if len(updates) == 0 {
		return updates
	}
	aliases := map[string]string{
		"apiType":   "api_type",
		"baseUrl":   "base_url",
		"apiKey":    "api_key",
		"modelName": "model_name",
		"isActive":  "is_active",
	}
	allowed := map[string]bool{"name": true, "api_type": true, "api_key": true, "model_name": true, "base_url": true, "is_active": true, "enabled": true}
	normalized := make(map[string]interface{}, len(updates))
	for key, value := range updates {
		column := key
		if alias, ok := aliases[key]; ok {
			column = alias
		}
		if !allowed[column] {
			continue
		}
		if column == "api_key" {
			if text, ok := value.(string); ok && text == "" {
				continue
			}
		}
		normalized[column] = value
	}
	return normalized
}
