package chat

import (
	"context"
	"fmt"
)

type modelConfigurationService interface {
	GetModel(int) (*ModelConfig, error)
	CreateModel(*ModelConfig) (*ModelConfig, error)
	UpdateModel(int, map[string]interface{}) (*ModelConfig, error)
	DeleteModel(int) error
	ActivateModel(int) (*ModelConfig, error)
	UpdateModelRoutes([]map[string]interface{}) error
}

type contextualModelConfiguration struct {
	parent *service
	repo   Repository
}

func (h *Handler) configurationService(ctx context.Context) modelConfigurationService {
	if svc, ok := h.service.(*service); ok {
		if repo, ok := svc.repo.(*repository); ok {
			return &contextualModelConfiguration{parent: svc, repo: &repository{db: repo.db.WithContext(ctx)}}
		}
	}
	return h.service
}

func (s *contextualModelConfiguration) GetModel(id int) (*ModelConfig, error) {
	return s.repo.GetModelByID(id)
}

func (s *contextualModelConfiguration) CreateModel(cfg *ModelConfig) (*ModelConfig, error) {
	count, err := s.repo.CountModels()
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	if count == 0 {
		cfg.IsActive = 1
	}
	if err := s.parent.validateModelConfiguration(context.Background(), cfg, cfg.IsActive == 1); err != nil {
		return nil, err
	}
	if err := s.repo.CreateModel(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return cfg, nil
}

func (s *contextualModelConfiguration) UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error) {
	current, err := s.repo.GetModelByID(id)
	if err != nil {
		return nil, fmt.Errorf("模型配置不存在: %w", err)
	}
	normalized := normalizeConfigUpdates(updates)
	for _, field := range []struct{ column, name string }{
		{"base_url", "baseUrl"}, {"model_name", "modelName"}, {"api_type", "apiType"},
	} {
		if value, exists := normalized[field.column]; exists {
			if _, ok := value.(string); !ok {
				return nil, fmt.Errorf("%w: %s 必须为字符串，收到 %T", errInvalidModelConfiguration, field.name, value)
			}
		}
	}
	projected := projectModelConfigUpdate(*current, normalized)
	if err := s.parent.validateModelConfiguration(context.Background(), projected, projected.IsActive == 1 || current.IsActive == 1); err != nil {
		return nil, err
	}
	if _, ok := normalized["context_window"]; !ok && projected.ContextWindow > 0 {
		normalized["context_window"] = projected.ContextWindow
	}
	if _, ok := normalized["max_output_tokens"]; !ok && projected.MaxOutputTokens > 0 {
		normalized["max_output_tokens"] = projected.MaxOutputTokens
	}
	if err := s.repo.UpdateModel(id, normalized); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	s.parent.invalidateLocalModels(context.Background())
	return s.repo.GetModelByID(id)
}

func (s *contextualModelConfiguration) DeleteModel(id int) error {
	if err := s.repo.DeleteModel(id); err != nil {
		return err
	}
	s.parent.invalidateLocalModels(context.Background())
	return nil
}

func (s *contextualModelConfiguration) ActivateModel(id int) (*ModelConfig, error) {
	current, err := s.repo.GetModelByID(id)
	if err != nil {
		return nil, fmt.Errorf("模型配置不存在: %w", err)
	}
	if err := s.parent.validateModelConfiguration(context.Background(), current, true); err != nil {
		return nil, err
	}
	if err := s.repo.ActivateModel(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	s.parent.invalidateLocalModels(context.Background())
	return s.repo.GetModelByID(id)
}

func (s *contextualModelConfiguration) UpdateModelRoutes(routes []map[string]interface{}) error {
	return s.repo.UpdateModelRoutes(routes)
}
