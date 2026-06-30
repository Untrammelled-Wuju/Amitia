// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

import (
	"fmt"
	"time"
)

type Service interface {
	List() ([]AsrConfig, error)
	GetByID(id int) (*AsrConfig, error)
	Create(req *CreateAsrConfigRequest) (*AsrConfig, error)
	Update(id int, updates map[string]interface{}) (*AsrConfig, error)
	Delete(id int) error
	Activate(id int) (*AsrConfig, error)
	GetActiveApiKey() (string, error)
	TestConnection(id int) error
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List() ([]AsrConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].HasApiKey = configs[i].ApiKey != ""
		configs[i].ApiKey = ""
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*AsrConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("ASR配置不存在")
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	cfg.ApiKey = ""
	return cfg, nil
}

func (s *service) Create(req *CreateAsrConfigRequest) (*AsrConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	if req.ResourceId == "" {
		req.ResourceId = "volc.seedasr.auc"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &AsrConfig{
		Name: req.Name, ApiKey: req.ApiKey, ResourceId: req.ResourceId,
		IsActive: req.IsActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	cfg.ApiKey = ""
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*AsrConfig, error) {
	if err := s.repo.Update(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
		cfg.ApiKey = ""
	}
	return cfg, nil
}

func (s *service) Delete(id int) error { return s.repo.Delete(id) }

func (s *service) Activate(id int) (*AsrConfig, error) {
	if err := s.repo.Activate(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
		cfg.ApiKey = ""
	}
	return cfg, nil
}

func (s *service) GetActiveApiKey() (string, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return "", fmt.Errorf("没有激活的ASR配置")
	}
	return cfg.ApiKey, nil
}

func (s *service) TestConnection(id int) error {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("ASR配置不存在")
	}
	if cfg.ApiKey == "" {
		return fmt.Errorf("API Key未设置")
	}
	return nil
}
