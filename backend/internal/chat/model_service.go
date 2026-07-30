// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *service) ListModels() ([]ModelConfig, error) {
	return s.repo.ListModels()
}

func (s *service) CreateModel(cfg *ModelConfig) (*ModelConfig, error) {
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

func (s *service) UpdateModel(id int, updates map[string]interface{}) (*ModelConfig, error) {
	if err := s.repo.UpdateModel(id, updates); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	return s.repo.GetModelByID(id)
}

func (s *service) DeleteModel(id int) error {
	return s.repo.DeleteModel(id)
}

func (s *service) ActivateModel(id int) (*ModelConfig, error) {
	if err := s.repo.ActivateModel(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	return s.repo.GetModelByID(id)
}

func (s *service) GetModelRoutes() ([]map[string]interface{}, error) {
	return s.repo.GetModelRoutes()
}

func (s *service) UpdateModelRoutes(routes []map[string]interface{}) error {
	return s.repo.UpdateModelRoutes(routes)
}

func (s *service) DetectModels(baseURL, apiKey, apiType string) ([]ModelDetectItem, error) {
	switch protocolForApiType(apiType) {
	case "ollama":
		return s.detectOllamaModels(baseURL)
	case "gemini":
		return s.detectGeminiModels(baseURL, apiKey)
	case "anthropic":
		return s.detectAnthropicModels(baseURL, apiKey)
	default:
		return s.detectOpenAIModels(baseURL, apiKey)
	}
}

func (s *service) detectOllamaModels(baseURL string) ([]ModelDetectItem, error) {
	base := strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("GET", base+"/api/tags", nil)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(rb))
	}
	var r struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	items := make([]ModelDetectItem, len(r.Models))
	for i, m := range r.Models {
		items[i] = ModelDetectItem{ID: m.Name}
	}
	return items, nil
}

func (s *service) detectOpenAIModels(baseURL, apiKey string) ([]ModelDetectItem, error) {
	base := strings.TrimRight(baseURL, "/")
	req, _ := http.NewRequest("GET", base+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
	}
	var r struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		var r2 struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.Unmarshal(rb, &r2) == nil {
			items := make([]ModelDetectItem, len(r2.Models))
			for i, m := range r2.Models {
				items[i] = ModelDetectItem{ID: m.Name}
			}
			return items, nil
		}
		return nil, fmt.Errorf("解析响应失败")
	}
	items := make([]ModelDetectItem, len(r.Data))
	for i, m := range r.Data {
		items[i] = ModelDetectItem{ID: m.ID, OwnedBy: m.OwnedBy}
	}
	return items, nil
}

func (s *service) detectGeminiModels(baseURL, apiKey string) ([]ModelDetectItem, error) {
	base := strings.TrimRight(baseURL, "/")
	url := fmt.Sprintf("%s/v1beta/models?key=%s&pageSize=100", base, apiKey)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(rb))
	}
	var r struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	items := make([]ModelDetectItem, len(r.Models))
	for i, m := range r.Models {
		id := m.Name
		if strings.HasPrefix(id, "models/") {
			id = strings.TrimPrefix(id, "models/")
		}
		items[i] = ModelDetectItem{ID: id, OwnedBy: "google"}
	}
	return items, nil
}

func (s *service) detectAnthropicModels(baseURL, apiKey string) ([]ModelDetectItem, error) {
	items := []ModelDetectItem{
		{ID: "claude-sonnet-4-20250514", OwnedBy: "anthropic"},
		{ID: "claude-3-7-sonnet-20250219", OwnedBy: "anthropic"},
		{ID: "claude-3-5-haiku-20241022", OwnedBy: "anthropic"},
		{ID: "claude-3-opus-20240229", OwnedBy: "anthropic"},
	}
	return items, nil
}

func (s *service) ListProviders() []ProviderInfo {
	return s.repo.ListProviders()
}
