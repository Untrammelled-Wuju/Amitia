package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/androiduiagent"
	"github.com/u-ai/backend/internal/browseragent"
	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
	"time"
)

type Service interface {
	androiduiagent.Runner
	browseragent.Runner
}
type service struct {
	db         *gorm.DB
	toolFacade *extensionkernel.ToolFacade
}

func NewService(ctx *app.AppContext, facade *extensionkernel.ToolFacade) Service {
	return &service{db: ctx.DB, toolFacade: facade}
}
func (s *service) getActiveModel() map[string]string {
	var baseURL, apiKey, modelName, apiType string
	var temp, maxTokens, topP float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens, top_p, api_type").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temp, &maxTokens, &topP, &apiType)
	if err != nil {
		return nil
	}
	_ = temp
	_ = maxTokens
	_ = topP
	return map[string]string{"baseUrl": baseURL, "apiKey": apiKey, "modelName": modelName, "apiType": apiType}
}

func (s *service) callLLM(ctx context.Context, cfg map[string]string, messages []map[string]interface{}) (string, int, error) {
	if cfg["apiType"] == "ollama" {
		return s.callOllamaLLM(ctx, cfg, messages)
	}
	baseURL := strings.TrimRight(cfg["baseUrl"], "/")
	reqBody := map[string]interface{}{
		"model": cfg["modelName"], "messages": messages,
		"temperature": 0.7, "max_tokens": 4096, "stream": false,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"])
	resp, err := (timeoutpolicy.Client(&http.Client{Timeout: 180 * time.Second})).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncate(string(rb), 200))
	}
	var r struct {
		Choices []struct{ Message struct{ Content string } }
		Usage   struct{ TotalTokens int }
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", 0, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(r.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices")
	}
	return r.Choices[0].Message.Content, r.Usage.TotalTokens, nil
}

func (s *service) callOllamaLLM(ctx context.Context, cfg map[string]string, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg["baseUrl"], "/")
	reqBody := map[string]interface{}{
		"model":    cfg["modelName"],
		"messages": messages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": 0.7,
			"num_ctx":     4096,
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/chat", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (timeoutpolicy.Client(&http.Client{Timeout: 180 * time.Second})).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncate(string(rb), 200))
	}
	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		EvalCount       int `json:"eval_count"`
		PromptEvalCount int `json:"prompt_eval_count"`
	}
	if err := json.Unmarshal(rb, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	total := result.EvalCount + result.PromptEvalCount
	return result.Message.Content, total, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
