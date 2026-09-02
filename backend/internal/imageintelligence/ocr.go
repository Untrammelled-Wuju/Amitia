package imageintelligence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/vision"
)

type ImageOCRRequest struct {
	Image         ImageInput `json:"image"`
	LanguageHints []string   `json:"languageHints,omitempty"`
	IncludeBoxes  bool       `json:"includeBoxes,omitempty"`
}

type NormalizedBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type OCRBlock struct {
	Text       string         `json:"text"`
	Confidence *float64       `json:"confidence,omitempty"`
	Box        *NormalizedBox `json:"box,omitempty"`
}

type ImageOCRResult struct {
	Text      string     `json:"text"`
	Blocks    []OCRBlock `json:"blocks,omitempty"`
	Languages []string   `json:"languages,omitempty"`
	Provider  string     `json:"provider"`
}

type OCRProvider struct {
	visionSvc  vision.Service
	httpClient *http.Client
}

func NewOCRProvider(visionSvc vision.Service) *OCRProvider {
	return &OCRProvider{
		visionSvc:  visionSvc,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *OCRProvider) OCR(ctx context.Context, req ImageOCRRequest, imageData []byte, summary ImageInputSummary) (*ImageOCRResult, *Error) {
	cfg, err := p.visionSvc.GetActive()
	if err != nil || cfg == nil {
		return nil, &Error{Code: ErrOCRUnavailable, Message: "no active OCR provider configured", HTTPStatus: http.StatusServiceUnavailable}
	}
	if cfg.ApiKey == "" {
		return nil, &Error{Code: ErrProviderAuth, Message: "OCR provider API key not configured", HTTPStatus: http.StatusUnauthorized}
	}

	prompt := "精确提取图片中的所有文字内容。请按照文字在图片中的位置顺序，逐行逐段提取，保持原始排版结构。不要添加任何解释或描述，只输出图片中实际存在的文字。"
	if req.IncludeBoxes {
		prompt = `精确识别图片中的文字，并返回每个可定位文本块。只输出 JSON，不要 Markdown：{"text":"完整识别文本","blocks":[{"text":"文本块","confidence":0.95,"box":{"x":0.0,"y":0.0,"width":0.0,"height":0.0}}]}。box 使用相对图片的 0..1 归一化坐标；必须给每个 blocks 项提供 box；禁止输出图片中不存在的文字。`
	}
	if len(req.LanguageHints) > 0 {
		prompt = prompt + " 图片中的文字可能包含以下语言：" + strings.Join(req.LanguageHints, "、")
	}

	dataURI := buildDataURI(summary.MIME, imageData)

	content := []map[string]interface{}{
		{"type": "input_image", "image_url": dataURI},
		{"type": "input_text", "text": prompt},
	}

	result, provErr := p.callProvider(ctx, cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, content)
	if provErr != "" {
		return nil, &Error{Code: ErrOCRFailed, Message: provErr, Provider: cfg.ApiType, HTTPStatus: http.StatusBadGateway}
	}

	if req.IncludeBoxes {
		envelope, parseErr := parsePositionedOCRResult(result)
		if parseErr != nil {
			return nil, &Error{Code: ErrOCRInvalidResponse, Message: parseErr.Error(), Provider: cfg.ApiType, HTTPStatus: http.StatusBadGateway}
		}
		return &ImageOCRResult{Text: envelope.Text, Blocks: envelope.Blocks, Provider: cfg.ApiType}, nil
	}

	result = sanitizeOCRResult(result)
	return &ImageOCRResult{Text: result, Blocks: []OCRBlock{{Text: result, Confidence: nil}}, Provider: cfg.ApiType}, nil
}

func (p *OCRProvider) callProvider(ctx context.Context, baseURL, apiKey, modelName string, content []map[string]interface{}) (string, string) {
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

func sanitizeOCRResult(text string) string {
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

type positionedOCREnvelope struct {
	Text   string     `json:"text"`
	Blocks []OCRBlock `json:"blocks"`
}

func parsePositionedOCRResult(raw string) (*positionedOCREnvelope, error) {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
	}
	var envelope positionedOCREnvelope
	if err := json.Unmarshal([]byte(cleaned), &envelope); err != nil {
		return nil, fmt.Errorf("positioned OCR must return JSON: %w", err)
	}
	valid := make([]OCRBlock, 0, len(envelope.Blocks))
	for _, block := range envelope.Blocks {
		block.Text = strings.TrimSpace(block.Text)
		if block.Text == "" || block.Box == nil {
			continue
		}
		if boxErr := validateBox(block.Box); boxErr != nil {
			return nil, fmt.Errorf("invalid OCR block box: %s", boxErr.Message)
		}
		if block.Box.Width <= 0 || block.Box.Height <= 0 || block.Box.X+block.Box.Width > 1.0001 || block.Box.Y+block.Box.Height > 1.0001 {
			return nil, fmt.Errorf("invalid OCR block box extent")
		}
		if block.Confidence != nil {
			if *block.Confidence < 0 || *block.Confidence > 1 {
				return nil, fmt.Errorf("OCR confidence out of range")
			}
		}
		valid = append(valid, block)
	}
	if len(valid) == 0 {
		return nil, fmt.Errorf("positioned OCR returned no usable blocks")
	}
	envelope.Blocks = valid
	if strings.TrimSpace(envelope.Text) == "" {
		parts := make([]string, 0, len(valid))
		for _, block := range valid {
			parts = append(parts, block.Text)
		}
		envelope.Text = strings.Join(parts, "\n")
	}
	return &envelope, nil
}

func validateBox(box *NormalizedBox) *Error {
	if box == nil {
		return nil
	}
	if box.X < 0 || box.X > 1 || box.Y < 0 || box.Y > 1 {
		return &Error{Code: ErrOCRInvalidResponse, Message: "bounding box coordinates out of range [0,1]", HTTPStatus: http.StatusBadGateway}
	}
	if box.Width < 0 || box.Width > 1 || box.Height < 0 || box.Height > 1 {
		return &Error{Code: ErrOCRInvalidResponse, Message: "bounding box dimensions out of range [0,1]", HTTPStatus: http.StatusBadGateway}
	}
	return nil
}
