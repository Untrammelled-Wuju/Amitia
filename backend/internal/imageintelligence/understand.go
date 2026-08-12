package imageintelligence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/vision"
)

type ImageUnderstandRequest struct {
	Image  ImageInput       `json:"image"`
Prompt string            `json:"prompt,omitempty"`
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
	httpClient *http.Client
}

func NewUnderstandProvider(visionSvc vision.Service) *UnderstandProvider {
	return &UnderstandProvider{
		visionSvc: visionSvc,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *UnderstandProvider) Understand(ctx context.Context, req ImageUnderstandRequest, imageData []byte, summary ImageInputSummary) (*ImageUnderstandResult, *Error) {
	cfg, err := p.visionSvc.GetActive()
	if err != nil || cfg == nil {
		return nil, &Error{Code: ErrUnAvailable, Message: "no active vision provider configured", HTTPStatus: http.StatusServiceUnavailable}
	}
	if cfg.ApiKey == "" {
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

	content := []map[string]interface{}{
		{"type": "input_image", "image_url": dataURI},
		{"type": "input_text", "text": prompt},
	}

	result, provErr := p.callProvider(ctx, cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
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

func (p *UnderstandProvider) callProvider(ctx context.Context, baseURL, apiKey, modelName string, content []map[string]interface{}) (string, string) {
	reqBody := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{{
			"role":    "user",
			"content": content,
		}},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(baseURL, "/")+"/responses", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Sprintf("provider returned %d: %s", resp.StatusCode, string(body))
	}

	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	var result map[string]interface{}
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return string(rawBody), ""
	}

	output, ok := result["output"].([]interface{})
	if !ok {
		return string(rawBody), ""
	}

	var texts []string
	for _, item := range output {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if m["type"] == "message" {
			contentArr, ok := m["content"].([]interface{})
			if !ok {
				continue
			}
			for _, c := range contentArr {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				if cm["type"] == "output_text" {
					texts = append(texts, fmt.Sprint(cm["text"]))
				}
			}
		}
	}

	return strings.Join(texts, ""), ""
}

func encodeBase64Std(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
