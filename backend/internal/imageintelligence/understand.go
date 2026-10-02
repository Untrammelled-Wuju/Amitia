package imageintelligence

import (
	"context"
	"encoding/base64"
	"net/http"

	"github.com/u-ai/backend/internal/vision"
)

type ImageUnderstandRequest struct {
	Image  ImageInput       `json:"image"`
	Prompt string           `json:"prompt,omitempty"`
	Detail ImageDetailLevel `json:"detail,omitempty"`
}

type ImageUnderstandResult struct {
	Text     string            `json:"text"`
	Provider string            `json:"provider"`
	Model    string            `json:"model,omitempty"`
	Input    ImageInputSummary `json:"input"`
	Usage    *UsageSummary     `json:"usage,omitempty"`
}

type UsageSummary struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}

type UnderstandProvider struct {
	visionSvc vision.Service
}

func NewUnderstandProvider(visionSvc vision.Service) *UnderstandProvider {
	return &UnderstandProvider{
		visionSvc: visionSvc,
	}
}

func (p *UnderstandProvider) Understand(ctx context.Context, req ImageUnderstandRequest, imageData []byte, summary ImageInputSummary) (*ImageUnderstandResult, *Error) {
	cfg, err := p.visionSvc.GetActive()
	if err != nil || cfg == nil {
		return nil, &Error{Code: ErrUnAvailable, Message: "no active vision provider configured", HTTPStatus: http.StatusServiceUnavailable}
	}
	if cfg.ApiKey == "" && !cfg.IsLocal() {
		return nil, &Error{Code: ErrProviderAuth, Message: "vision provider API key not configured", HTTPStatus: http.StatusUnauthorized}
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = "请详细描述这张图片的内容，包括场景、物体、人物、文字、表情、氛围等所有可见信息，严禁描述不存在于图片中的信息"
	}
	if len(prompt) > 16384 {
		prompt = prompt[:16384]
	}

	dataURI := buildDataURI(summary.MIME, imageData)

	result, generateErr := vision.GenerateImages(ctx, cfg, []string{dataURI}, prompt, 0)
	provErr := ""
	if generateErr != nil {
		provErr = generateErr.Error()
	}

	if provErr != "" {
		return nil, mapImageErrorToDomain(provErr, false)
	}

	return &ImageUnderstandResult{
		Text:     result,
		Provider: cfg.ApiType,
		Model:    cfg.ModelName,
		Input:    summary,
	}, nil
}

func encodeBase64Std(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
