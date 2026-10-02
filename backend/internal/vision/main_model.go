package vision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/chat/modelprotocol"
	"gorm.io/gorm"
)

const MainModelVisionNotice = "主模型已开启视觉模式，如需单独启用视觉模型，请先关闭文本模型的支持识图功能"

var localImageGenerator struct {
	sync.RWMutex
	generate func(context.Context, *VisionConfig, []string, string, int) (string, error)
}

func SetLocalImageGenerator(generate func(context.Context, *VisionConfig, []string, string, int) (string, error)) {
	localImageGenerator.Lock()
	defer localImageGenerator.Unlock()
	localImageGenerator.generate = generate
}

type mainModelRecord struct {
	ID               int
	Name             string
	APIType          string `gorm:"column:api_type"`
	Protocol         string
	BaseURL          string `gorm:"column:base_url"`
	APIKey           string `gorm:"column:api_key"`
	ModelName        string `gorm:"column:model_name"`
	CapabilitiesJSON string `gorm:"column:capabilities_json"`
	TimeoutSeconds   int    `gorm:"column:timeout_seconds"`
	MaxTokens        int    `gorm:"column:max_tokens"`
	MaxOutputTokens  int    `gorm:"column:max_output_tokens"`
}

func (r *repository) mainVisionModel() (*VisionConfig, error) {
	var row mainModelRecord
	err := r.db.Table("model_configs").Where("is_active = 1").Order("id").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var caps modelprotocol.ModelCapabilities
	if json.Unmarshal([]byte(row.CapabilitiesJSON), &caps) != nil || !caps.SupportsImage {
		return nil, nil
	}
	protocol := row.Protocol
	if protocol == "" {
		switch row.APIType {
		case "openai":
			protocol = string(modelprotocol.ProtocolOpenAIResponses)
		case "anthropic":
			protocol = string(modelprotocol.ProtocolAnthropicMessages)
		case "gemini":
			protocol = string(modelprotocol.ProtocolGeminiGenerate)
		case "ollama":
			protocol = string(modelprotocol.ProtocolOllamaChat)
		case "mnn", "llama_cpp":
			protocol = row.APIType
		default:
			protocol = string(modelprotocol.ProtocolOpenAIChat)
		}
	}
	maxTokens := row.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = row.MaxTokens
	}
	return &VisionConfig{ID: row.ID, Name: row.Name, ApiType: row.APIType, ApiKey: row.APIKey, ModelName: row.ModelName, BaseUrl: row.BaseURL, IsActive: 1, FromMainModel: true, Protocol: protocol, TimeoutSeconds: row.TimeoutSeconds, MaxOutputTokens: maxTokens, CapabilitiesJSON: row.CapabilitiesJSON}, nil
}

func (s *service) mainVisionModel() (*VisionConfig, error) {
	source, ok := s.repo.(interface{ mainVisionModel() (*VisionConfig, error) })
	if !ok {
		return nil, nil
	}
	return source.mainVisionModel()
}

func (s *service) requireIndependentVision() error {
	cfg, err := s.mainVisionModel()
	if err != nil {
		return err
	}
	if cfg != nil {
		return errors.New(MainModelVisionNotice)
	}
	return nil
}

func GenerateImages(ctx context.Context, cfg *VisionConfig, images []string, prompt string, maxTokens int) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("未配置可用的视觉模型")
	}
	if cfg.Protocol == "mnn" || cfg.Protocol == "llama_cpp" {
		localImageGenerator.RLock()
		generate := localImageGenerator.generate
		localImageGenerator.RUnlock()
		if generate == nil {
			return "", fmt.Errorf("本地模型视觉运行时未就绪")
		}
		return generate(ctx, cfg, images, prompt, maxTokens)
	}
	protocol := modelprotocol.ModelProtocol(cfg.Protocol)
	if protocol == "" {
		switch cfg.ApiType {
		case "volcengine":
			protocol = modelprotocol.ProtocolOpenAIResponses
		case "gemini":
			protocol = modelprotocol.ProtocolGeminiGenerate
		case "anthropic":
			protocol = modelprotocol.ProtocolAnthropicMessages
		case "ollama":
			protocol = modelprotocol.ProtocolOllamaChat
		default:
			protocol = modelprotocol.ProtocolOpenAIChat
		}
	}
	if maxTokens <= 0 {
		maxTokens = cfg.MaxOutputTokens
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
	}
	parts := make([]modelprotocol.ModelContentPart, 0, len(images)+1)
	for _, uri := range images {
		parts = append(parts, modelprotocol.ModelContentPart{Type: modelprotocol.ContentTypeImage, ResourceURI: uri})
	}
	parts = append(parts, modelprotocol.ModelContentPart{Type: modelprotocol.ContentTypeText, Text: prompt})
	result, err := modelprotocol.AdapterForProtocol(protocol).Generate(ctx, modelprotocol.ProviderConfig{ModelName: cfg.ModelName, BaseURL: cfg.BaseUrl, APIKey: cfg.ApiKey, Protocol: string(protocol), TimeoutSeconds: timeout, MaxTokens: maxTokens, MaxOutputTokens: maxTokens}, modelprotocol.ModelRequest{Model: cfg.ModelName, Messages: []modelprotocol.ModelMessage{{Role: "user", Parts: parts}}, DisableThinking: true, MaxOutputTokens: maxTokens})
	if err != nil {
		return "", err
	}
	if result == nil || result.Text == "" {
		return "", fmt.Errorf("视觉模型未返回识别内容")
	}
	return result.Text, nil
}
