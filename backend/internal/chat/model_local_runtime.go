package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
)

type LocalModelManager interface {
	ListLocalModels(context.Context, string) ([]localmodel.ModelArtifact, error)
	ImportLocalModel(context.Context, string, string, string, io.Reader) (*localmodel.ModelArtifact, error)
	DeleteLocalModel(context.Context, string, string) error
	DetectLocalModels(context.Context, string, string) ([]ModelDetectItem, error)
	TestLocalModel(context.Context, *ModelConfig) (map[string]any, error)
}

func (s *service) ListLocalModels(ctx context.Context, owner string) ([]localmodel.ModelArtifact, error) {
	if s.modelLibrary == nil {
		return nil, errors.New("本地模型库不可用")
	}
	return s.modelLibrary.List(ctx, owner)
}

func (s *service) ImportLocalModel(ctx context.Context, owner, name, filename string, body io.Reader) (*localmodel.ModelArtifact, error) {
	if s.modelLibrary == nil {
		return nil, errors.New("本地模型库不可用")
	}
	return s.modelLibrary.Import(ctx, owner, name, filename, body)
}

func (s *service) DeleteLocalModel(ctx context.Context, owner, id string) error {
	if s.modelLibrary == nil {
		return errors.New("本地模型库不可用")
	}
	return s.modelLibrary.Delete(ctx, owner, id)
}

func (s *service) DetectLocalModels(ctx context.Context, owner, provider string) ([]ModelDetectItem, error) {
	if s.modelLibrary == nil {
		return nil, errors.New("本地模型库不可用")
	}
	rows, err := s.modelLibrary.Detect(ctx, owner, provider)
	if err != nil {
		return nil, err
	}
	result := make([]ModelDetectItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, ModelDetectItem{ID: row.ResourceURI, OwnedBy: provider})
	}
	return result, nil
}

func (s *service) TestLocalModel(ctx context.Context, cfg *ModelConfig) (map[string]any, error) {
	if cfg == nil {
		return nil, errors.New("模型配置不存在")
	}
	started := time.Now()
	available, reason := localmodel.ProviderAvailable(cfg.APIType)
	if !available {
		return map[string]any{"success": false, "latencyMs": 0, "status": "error", "message": reason}, nil
	}
	if err := s.validateModelConfiguration(ctx, cfg, false); err != nil {
		return map[string]any{"success": false, "latencyMs": time.Since(started).Milliseconds(), "status": "error", "message": err.Error()}, nil
	}
	health, output, err := localmodel.RunProbe(ctx, localmodel.CreateLocalModelParams{
		Provider:     cfg.APIType,
		ModelName:    cfg.ModelName,
		ProviderJSON: cfg.ProviderConfigJSON,
		Timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
	}, "只回复 OK")
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return map[string]any{"success": false, "latencyMs": latency, "status": "error", "message": err.Error(), "health": health}, nil
	}
	return map[string]any{"success": true, "latencyMs": latency, "status": "ok", "message": "本地模型加载与最小推理成功", "output": strings.TrimSpace(output), "health": health}, nil
}

func (s *service) providerCatalog() []ProviderInfo {
	providers := s.repo.ListProviders()
	for index := range providers {
		providers[index].RuntimeKind = "http"
		providers[index].Available = true
		providers[index].Capabilities = map[string]bool{"chat": true, "stream": true, "tools": true, "vision": providers[index].ID != "ollama"}
		providers[index].ConfigFields = []localmodel.ProviderField{
			{Key: "baseUrl", Label: "Base URL", Type: "url", Required: true, Placeholder: providers[index].DefaultBaseURL},
			{Key: "apiKey", Label: "API Key", Type: "password", Required: providers[index].ID != "ollama", Secret: true},
			{Key: "modelName", Label: "模型名称", Type: "text", Required: true, Placeholder: providers[index].DefaultModel},
		}
	}
	for _, descriptor := range localmodel.Descriptors() {
		available, reason := localmodel.ProviderAvailable(descriptor.ID)
		providers = append(providers, ProviderInfo{
			ID:                 descriptor.ID,
			Name:               descriptor.Name,
			Protocol:           descriptor.Protocol,
			DefaultProtocol:    descriptor.DefaultProtocol,
			SupportedProtocols: append([]string(nil), descriptor.SupportedProtocols...),
			DefaultModel:       descriptor.DefaultModel,
			DocsURL:            descriptor.DocsURL,
			RuntimeKind:        descriptor.RuntimeKind,
			ModelFormats:       append([]string(nil), descriptor.ModelFormats...),
			ConfigFields:       append([]localmodel.ProviderField(nil), descriptor.ConfigFields...),
			Capabilities:       cloneCapabilities(descriptor.Capabilities),
			Available:          available,
			UnavailableReason:  reason,
		})
	}
	return providers
}

func cloneCapabilities(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (s *service) validateModelConfiguration(ctx context.Context, cfg *ModelConfig, probe bool) error {
	if cfg == nil {
		return errors.New("模型配置不能为空")
	}
	provider := strings.TrimSpace(cfg.APIType)
	if !localmodel.IsEmbedded(provider) {
		return nil
	}
	if available, reason := localmodel.ProviderAvailable(provider); !available {
		return errors.New(reason)
	}
	raw, err := providerConfigObject(cfg.ProviderConfigJSON)
	if err != nil {
		return err
	}
	descriptor, _ := localmodel.Descriptor(provider)
	for _, field := range descriptor.ConfigFields {
		if !field.Required {
			continue
		}
		if value, ok := raw[field.Key]; !ok || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return fmt.Errorf("%s不能为空", field.Label)
		}
	}
	resourceKey := "resourceUri"
	if provider == "mnn" {
		resourceKey = "modelResourceUri"
	}
	resourceURI, _ := raw[resourceKey].(string)
	row, err := s.modelLibrary.FindByURI(ctx, "", resourceURI)
	if err != nil {
		return fmt.Errorf("模型文件未导入或无访问权限: %w", err)
	}
	if err := localmodel.ValidateArtifactForProvider(row, provider); err != nil {
		return err
	}
	if mmprojURI, _ := raw["mmprojUri"].(string); strings.TrimSpace(mmprojURI) != "" {
		mmproj, err := s.modelLibrary.FindByURI(ctx, "", mmprojURI)
		if err != nil {
			return fmt.Errorf("多模态投影模型未导入或无访问权限: %w", err)
		}
		if mmproj.Format != "gguf" {
			return errors.New("多模态投影模型必须是 GGUF 文件")
		}
	}
	if strings.TrimSpace(cfg.ModelName) == "" {
		cfg.ModelName = row.Name
	}
	if value, ok := raw["contextSize"]; ok {
		if number, valid := value.(float64); valid && number >= 1024 && number <= 1048576 {
			cfg.ContextWindow = int(number)
		}
	}
	if cfg.ContextWindow <= 0 {
		cfg.ContextWindow = 4096
	}
	if cfg.MaxOutputTokens <= 0 && cfg.MaxTokens > 0 {
		cfg.MaxOutputTokens = cfg.MaxTokens
	}
	if !probe {
		return nil
	}
	_, _, err = localmodel.RunProbe(ctx, localmodel.CreateLocalModelParams{
		Provider:     provider,
		ModelName:    cfg.ModelName,
		ProviderJSON: cfg.ProviderConfigJSON,
		Timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
	}, "只回复 OK")
	return err
}

func providerConfigObject(raw string) (map[string]any, error) {
	value := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return value, nil
	}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("本地模型运行参数无效: %w", err)
	}
	return value, nil
}

func projectModelConfigUpdate(current ModelConfig, updates map[string]interface{}) *ModelConfig {
	projected := current
	normalized := normalizeConfigUpdates(updates)
	for key, value := range normalized {
		stringValue := fmt.Sprint(value)
		switch key {
		case "name":
			projected.Name = stringValue
		case "api_type":
			projected.APIType = stringValue
		case "protocol":
			projected.Protocol = stringValue
		case "base_url":
			projected.BaseURL = stringValue
		case "model_name":
			projected.ModelName = stringValue
		case "provider_config_json":
			switch typed := value.(type) {
			case string:
				projected.ProviderConfigJSON = typed
			case map[string]any:
				encoded, _ := json.Marshal(typed)
				projected.ProviderConfigJSON = string(encoded)
			}
		case "timeout_seconds":
			if number, ok := value.(float64); ok {
				projected.TimeoutSeconds = int(number)
			}
		}
	}
	return &projected
}
