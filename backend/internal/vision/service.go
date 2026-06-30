// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package vision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Service interface {
	List() ([]VisionConfig, error)
	GetByID(id int) (*VisionConfig, error)
	Create(req *CreateVisionConfigRequest) (*VisionConfig, error)
	Update(id int, updates map[string]interface{}) (*VisionConfig, error)
	Delete(id int) error
	Activate(id int) (*VisionConfig, error)
	GetActive() (*VisionConfig, error)
	TestConnection(id int) (map[string]interface{}, error)
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List() ([]VisionConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].HasApiKey = configs[i].ApiKey != ""
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*VisionConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("视觉模型配置不存在")
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Create(req *CreateVisionConfigRequest) (*VisionConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	if req.ModelName == "" {
		req.ModelName = "doubao-seed-2-0-lite-260428"
	}
	if req.BaseUrl == "" {
		req.BaseUrl = "https://ark.cn-beijing.volces.com/api/v3"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &VisionConfig{
		Name: req.Name, ApiKey: req.ApiKey, ModelName: req.ModelName,
		BaseUrl: req.BaseUrl, IsActive: req.IsActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*VisionConfig, error) {
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

func (s *service) Activate(id int) (*VisionConfig, error) {
	if err := s.repo.Activate(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
	}
	return cfg, nil
}

func (s *service) GetActive() (*VisionConfig, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, err
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) TestConnection(id int) (map[string]interface{}, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("视觉模型配置不存在")
	}
	if cfg.ApiKey == "" {
		return nil, fmt.Errorf("API Key未配置")
	}
	baseUrl := strings.TrimRight(cfg.BaseUrl, "/")
	reqBody := map[string]interface{}{
		"model": cfg.ModelName,
		"input": []map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{
				{"type": "input_text", "text": "你好，请简单回复连接成功"},
			}},
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseUrl+"/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	start := time.Now()
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]interface{}{"success": false, "message": err.Error(), "latency": latency}, nil
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return map[string]interface{}{"success": false, "message": fmt.Sprintf("API返回 %d: %s", resp.StatusCode, truncate(string(rb), 300)), "latency": latency}, nil
	}
	return map[string]interface{}{"success": true, "message": "连接成功", "latency": latency}, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
