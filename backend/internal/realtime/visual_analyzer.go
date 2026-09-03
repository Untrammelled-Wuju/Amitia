// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/vision"
)

type VisualAnalyzer interface {
	Analyze(ctx context.Context, frame VisualFrame) (string, error)
	Available() bool
}

type configuredVisualAnalyzer struct {
	visionSvc  vision.Service
	httpClient *http.Client
}

func NewConfiguredVisualAnalyzer(visionSvc vision.Service) VisualAnalyzer {
	if visionSvc == nil {
		return nil
	}
	return &configuredVisualAnalyzer{
		visionSvc:  visionSvc,
		httpClient: &http.Client{Timeout: 25 * time.Second},
	}
}

func (a *configuredVisualAnalyzer) Available() bool {
	if a == nil || a.visionSvc == nil {
		return false
	}
	cfg, err := a.visionSvc.GetActive()
	return err == nil && cfg != nil && strings.TrimSpace(cfg.ApiKey) != "" && strings.TrimSpace(cfg.ModelName) != ""
}

func (a *configuredVisualAnalyzer) Analyze(ctx context.Context, frame VisualFrame) (string, error) {
	if a == nil || a.visionSvc == nil {
		return "", fmt.Errorf("visual analyzer unavailable")
	}
	cfg, err := a.visionSvc.GetActive()
	if err != nil || cfg == nil {
		return "", fmt.Errorf("active visual model unavailable")
	}
	if strings.TrimSpace(cfg.ApiKey) == "" {
		return "", fmt.Errorf("active visual model has no API key")
	}

	prompt := visualPrompt(frame)
	switch strings.ToLower(strings.TrimSpace(cfg.ApiType)) {
	case "gemini":
		return a.analyzeGemini(ctx, cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, frame, prompt)
	case "volcengine":
		return a.analyzeResponsesAPI(ctx, cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, frame, prompt)
	default:
		return a.analyzeChatCompletions(ctx, cfg.BaseUrl, cfg.ApiKey, cfg.ModelName, frame, prompt)
	}
}

func visualPrompt(frame VisualFrame) string {
	if frame.Source == VisualSourceScreen {
		return "你是实时屏幕视觉状态提取器。只描述当前帧中真实可见且对后续对话有帮助的信息：当前应用/页面、主要控件、可读文字、报错、选中区域和光标附近内容。不要猜测不可见内容。输出一段紧凑的中文状态，不要主动回答用户问题。"
	}
	return "你是实时摄像头视觉状态提取器。只描述当前帧中真实可见且对后续对话有帮助的信息：人物、主要物体、动作、环境、可读文字和当前关注主体。不要猜测不可见内容。输出一段紧凑的中文状态，不要主动回答用户问题。"
}

func (a *configuredVisualAnalyzer) analyzeResponsesAPI(ctx context.Context, baseURL, apiKey, modelName string, frame VisualFrame, prompt string) (string, error) {
	dataURI := "data:" + frame.MIME + ";base64," + base64.StdEncoding.EncodeToString(frame.Data)
	requestBody := map[string]interface{}{
		"model": modelName,
		"input": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "input_image", "image_url": dataURI},
				{"type": "input_text", "text": prompt},
			},
		}},
		"max_output_tokens": 220,
	}
	payload, _ := json.Marshal(requestBody)
	endpoint := strings.TrimRight(baseURL, "/") + "/responses"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("visual provider returned %d: %s", resp.StatusCode, truncateVisualError(string(body), 800))
	}
	var decoded struct {
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode visual response: %w", err)
	}
	if text := strings.TrimSpace(decoded.OutputText); text != "" {
		return text, nil
	}
	for _, item := range decoded.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				return strings.TrimSpace(content.Text), nil
			}
		}
	}
	return "", fmt.Errorf("visual provider returned no text")
}

func (a *configuredVisualAnalyzer) analyzeChatCompletions(ctx context.Context, baseURL, apiKey, modelName string, frame VisualFrame, prompt string) (string, error) {
	dataURI := "data:" + frame.MIME + ";base64," + base64.StdEncoding.EncodeToString(frame.Data)
	requestBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": prompt},
				{"type": "image_url", "image_url": map[string]string{"url": dataURI}},
			},
		}},
		"max_tokens": 220,
	}
	payload, _ := json.Marshal(requestBody)
	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("visual provider returned %d: %s", resp.StatusCode, truncateVisualError(string(body), 800))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode visual response: %w", err)
	}
	for _, choice := range decoded.Choices {
		switch content := choice.Message.Content.(type) {
		case string:
			if text := strings.TrimSpace(content); text != "" {
				return text, nil
			}
		case []interface{}:
			for _, item := range content {
				if part, ok := item.(map[string]interface{}); ok {
					if text, ok := part["text"].(string); ok && strings.TrimSpace(text) != "" {
						return strings.TrimSpace(text), nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("visual provider returned no text")
}

func (a *configuredVisualAnalyzer) analyzeGemini(ctx context.Context, baseURL, apiKey, modelName string, frame VisualFrame, prompt string) (string, error) {
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", strings.TrimRight(baseURL, "/"), url.PathEscape(modelName), url.QueryEscape(apiKey))
	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{{
			"role": "user",
			"parts": []map[string]interface{}{
				{"text": prompt},
				{"inlineData": map[string]string{
					"mimeType": frame.MIME,
					"data":     base64.StdEncoding.EncodeToString(frame.Data),
				}},
			},
		}},
		"generationConfig": map[string]interface{}{"maxOutputTokens": 220},
	}
	payload, _ := json.Marshal(requestBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("visual provider returned %d: %s", resp.StatusCode, truncateVisualError(string(body), 800))
	}
	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode visual response: %w", err)
	}
	for _, candidate := range decoded.Candidates {
		for _, part := range candidate.Content.Parts {
			if text := strings.TrimSpace(part.Text); text != "" {
				return text, nil
			}
		}
	}
	return "", fmt.Errorf("visual provider returned no text")
}

func truncateVisualError(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
