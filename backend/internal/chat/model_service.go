// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxModelDetectResponseBytes int64 = 4 << 20

var blockedModelDetectIPs = map[string]struct{}{
	"100.100.100.200": {},
	"fd00:ec2::254":   {},
}

func validateModelDetectURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("无效的模型服务地址: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("模型服务地址只允许 http/https")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("模型服务地址缺少主机名")
	}
	if parsed.User != nil {
		return fmt.Errorf("模型服务地址不得包含内嵌凭据")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && blockedModelDetectIP(ip) {
		return fmt.Errorf("模型服务地址指向受保护的网络地址")
	}
	return nil
}

func blockedModelDetectIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	_, blocked := blockedModelDetectIPs[strings.ToLower(ip.String())]
	return blocked
}

func modelDetectOrigin(raw *url.URL) string {
	if raw == nil {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(raw.Scheme))
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw.Hostname()), "."))
	port := raw.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func modelDetectHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid model endpoint address: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, item := range ips {
			if blockedModelDetectIP(item.IP) {
				lastErr = fmt.Errorf("model endpoint resolved to a protected network address")
				continue
			}
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("model endpoint did not resolve to a usable address")
		}
		return nil, lastErr
	}
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if err := validateModelDetectURL(req.URL.String()); err != nil {
				return err
			}
			if len(via) > 0 && modelDetectOrigin(req.URL) != modelDetectOrigin(via[0].URL) {
				return fmt.Errorf("model endpoint cross-origin redirect is not allowed")
			}
			return nil
		},
	}
}

func newModelDetectRequest(rawURL string) (*http.Request, error) {
	if err := validateModelDetectURL(rawURL); err != nil {
		return nil, err
	}
	return http.NewRequest(http.MethodGet, rawURL, nil)
}

func readModelDetectBody(body io.Reader) ([]byte, error) {
	limited := io.LimitReader(body, maxModelDetectResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxModelDetectResponseBytes {
		return nil, fmt.Errorf("模型服务响应超过 %d 字节限制", maxModelDetectResponseBytes)
	}
	return data, nil
}

func (s *service) ListModels() ([]ModelConfig, error) {
	return s.repo.ListModels()
}

func (s *service) GetModel(id int) (*ModelConfig, error) {
	return s.repo.GetModelByID(id)
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
	if err := s.repo.UpdateModel(id, normalizeConfigUpdates(updates)); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}
	s.invalidateLocalModels(context.Background())
	return s.repo.GetModelByID(id)
}

func (s *service) DeleteModel(id int) error {
	err := s.repo.DeleteModel(id)
	if err == nil {
		s.invalidateLocalModels(context.Background())
	}
	return err
}

func (s *service) ActivateModel(id int) (*ModelConfig, error) {
	if err := s.repo.ActivateModel(id); err != nil {
		return nil, fmt.Errorf("激活失败: %w", err)
	}
	s.invalidateLocalModels(context.Background())
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
	req, err := newModelDetectRequest(base + "/api/tags")
	if err != nil {
		return nil, err
	}
	resp, err := modelDetectHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, err := readModelDetectBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
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
	req, err := newModelDetectRequest(base + "/models")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := modelDetectHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, err := readModelDetectBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
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
	query := url.Values{}
	query.Set("key", apiKey)
	query.Set("pageSize", "100")
	endpoint := base + "/v1beta/models?" + query.Encode()
	req, err := newModelDetectRequest(endpoint)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := modelDetectHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, err := readModelDetectBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
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
	base := strings.TrimRight(baseURL, "/")
	endpoint := base + "/v1/models?limit=1000"
	if parsed, err := url.Parse(base); err == nil && strings.HasSuffix(strings.TrimRight(strings.ToLower(parsed.Path), "/"), "/v1") {
		endpoint = base + "/models?limit=1000"
	}
	req, err := newModelDetectRequest(endpoint)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Anthropic-Version", "2023-06-01")
	resp, err := modelDetectHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	rb, err := readModelDetectBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回 %d", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rb, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	items := make([]ModelDetectItem, 0, len(result.Data))
	for _, model := range result.Data {
		if id := strings.TrimSpace(model.ID); id != "" {
			items = append(items, ModelDetectItem{ID: id, OwnedBy: "anthropic"})
		}
	}
	return items, nil
}

func (s *service) ListProviders() []ProviderInfo {
	return s.repo.ListProviders()
}

func resolveProtocol(cfg *ModelConfig) ModelProtocol {
	if cfg.Protocol != "" {
		return ModelProtocol(cfg.Protocol)
	}
	switch cfg.APIType {
	case "ollama":
		return ProtocolOllamaChat
	case "anthropic":
		return ProtocolAnthropicMessages
	case "gemini":
		return ProtocolGeminiGenerate
	case "openai":
		return ProtocolOpenAIResponses
	default:
		return ProtocolOpenAIChat
	}
}

func (s *service) GetActiveModelProtocol() (ModelProtocol, error) {
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return "", err
	}
	return resolveProtocol(cfg), nil
}

func (s *service) CreateMessageAttachment(attachment *MessageAttachment) error {
	return s.repo.CreateMessageAttachment(attachment)
}

func (s *service) GetMessageAttachments(messageID string) ([]MessageAttachment, error) {
	return s.repo.GetMessageAttachments(messageID)
}

func (s *service) GetAttachmentByID(id string) (*MessageAttachment, error) {
	return s.repo.GetAttachmentByID(id)
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
