// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package embedding_config

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
	List() ([]EmbeddingConfig, error)
	GetByID(id int) (*EmbeddingConfig, error)
	Create(req *CreateEmbeddingConfigRequest) (*EmbeddingConfig, error)
	Update(id int, updates map[string]interface{}) (*EmbeddingConfig, error)
	Delete(id int) error
	Activate(id int) (*EmbeddingConfig, error)
	GetActive() (*EmbeddingConfig, error)
	TestConnection(id int) (map[string]interface{}, error)
	ListProviders() []ProviderInfo
}

type service struct{ repo Repository }

func NewService(repo Repository) Service { return &service{repo: repo} }

func (s *service) List() ([]EmbeddingConfig, error) {
	configs, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range configs {
		configs[i].HasApiKey = configs[i].ApiKey != ""
	}
	return configs, nil
}

func (s *service) GetByID(id int) (*EmbeddingConfig, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("向量模型配置不存在")
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Create(req *CreateEmbeddingConfigRequest) (*EmbeddingConfig, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	if req.ApiType == "" {
		req.ApiType = "volcengine"
	}
	if req.ModelName == "" {
		req.ModelName = "doubao-embedding-vision-251215"
	}
	if req.BaseUrl == "" {
		req.BaseUrl = "https://ark.cn-beijing.volces.com/api/v3"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	cfg := &EmbeddingConfig{
		Name: req.Name, ApiType: req.ApiType, ApiKey: req.ApiKey, ModelName: req.ModelName,
		BaseUrl: req.BaseUrl, IsActive: req.IsActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(cfg); err != nil {
		return nil, fmt.Errorf("创建失败: %w", err)
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) Update(id int, updates map[string]interface{}) (*EmbeddingConfig, error) {
	if err := s.repo.Update(id, normalizeConfigUpdates(updates)); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
	}
	return cfg, nil
}

func (s *service) Delete(id int) error { return s.repo.Delete(id) }

func (s *service) Activate(id int) (*EmbeddingConfig, error) {
	if err := s.repo.Activate(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	cfg, _ := s.repo.GetByID(id)
	if cfg != nil {
		cfg.HasApiKey = cfg.ApiKey != ""
	}
	return cfg, nil
}

func (s *service) GetActive() (*EmbeddingConfig, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, err
	}
	cfg.HasApiKey = cfg.ApiKey != ""
	return cfg, nil
}

func (s *service) ListProviders() []ProviderInfo {
	return s.repo.ListProviders()
}

func (s *service) TestConnection(id int) (map[string]interface{}, error) {
	cfg, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("向量模型配置不存在")
	}
	if cfg.ApiKey == "" {
		return nil, fmt.Errorf("API Key未配置")
	}
	baseUrl := strings.TrimRight(cfg.BaseUrl, "/")
	apiType := cfg.ApiType
	if apiType == "" {
		apiType = "volcengine"
	}

	var reqBody map[string]interface{}
	var endpoint string

	if apiType == "volcengine" {
		reqBody = map[string]interface{}{
			"model": cfg.ModelName,
			"input": []map[string]interface{}{{"type": "text", "text": "连接测试"}},
		}
		endpoint = baseUrl + "/embeddings/multimodal"
	} else {
		reqBody = map[string]interface{}{
			"model": cfg.ModelName,
			"input": "test text",
		}
		endpoint = baseUrl + "/embeddings"
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(jsonBody))
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

func normalizeConfigUpdates(updates map[string]interface{}) map[string]interface{} {
	if len(updates) == 0 {
		return updates
	}
	aliases := map[string]string{
		"apiType":             "api_type",
		"baseUrl":             "base_url",
		"apiKey":              "api_key",
		"modelName":           "model_name",
		"isActive":            "is_active",
		"maxTokens":           "max_tokens",
		"contextWindow":       "context_window",
		"maxOutputTokens":     "max_output_tokens",
		"topP":                "top_p",
		"timeoutSeconds":      "timeout_seconds",
		"retryCount":          "retry_count",
		"providerConfig":      "provider_config_json",
		"providerConfigJSON":  "provider_config_json",
		"capabilitiesJson":    "capabilities_json",
		"lastTestStatus":      "last_test_status",
		"lastTestMessage":     "last_test_message",
		"lastTestAt":          "last_test_at",
		"resourceId":          "resource_id",
		"voiceType":           "voice_type",
		"customVoiceId":       "custom_voice_id",
		"cloneResourceId":     "clone_resource_id",
		"realtimeAppId":       "realtime_app_id",
		"realtimeAccessToken": "realtime_access_token",
		"realtimeSecretKey":   "realtime_secret_key",
	}
	normalized := make(map[string]interface{}, len(updates))
	for key, value := range updates {
		if column, ok := aliases[key]; ok {
			normalized[column] = value
		} else {
			normalized[key] = value
		}
	}
	return normalized
}
