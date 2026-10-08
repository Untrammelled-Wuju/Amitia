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
	if err := s.repo.CreateModel(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	return cfg, nil
}

func (s *contextualModelConfiguration) UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error) {
	if err := s.repo.UpdateModel(id, normalizeConfigUpdates(updates)); err != nil {
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
	if err := s.repo.ActivateModel(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	s.parent.invalidateLocalModels(context.Background())
	return s.repo.GetModelByID(id)
}

func (s *contextualModelConfiguration) UpdateModelRoutes(routes []map[string]interface{}) error {
	return s.repo.UpdateModelRoutes(routes)
}
