// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *service) getActiveModel() map[string]interface{} {
	var baseURL, apiKey, modelName, apiType string
	var temperature, maxTokens float64
	err := s.db.Table("model_configs").
		Select("base_url, api_key, model_name, temperature, max_tokens, api_type").
		Where("is_active = 1").Limit(1).Row().
		Scan(&baseURL, &apiKey, &modelName, &temperature, &maxTokens, &apiType)
	if err != nil {
		return nil
	}
	return map[string]interface{}{
		"baseUrl":     baseURL,
		"apiKey":      apiKey,
		"modelName":   modelName,
		"temperature": temperature,
		"maxTokens":   int(maxTokens),
		"apiType":     apiType,
	}
}

func (s *service) callLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
	if apiType, _ := cfg["apiType"].(string); apiType == "ollama" {
		return s.callOllamaLLM(cfg, messages)
	}
	baseURL := strings.TrimRight(cfg["baseUrl"].(string), "/")
	reqBody := map[string]interface{}{
		"model":       cfg["modelName"],
		"messages":    messages,
		"temperature": cfg["temperature"],
		"max_tokens":  cfg["maxTokens"],
		"stream":      false,
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg["apiKey"].(string))
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncateStr(string(rb), 200))
	}
	var result struct {
		Choices []struct{ Message struct{ Content string } }
		Usage   struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &result)
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *service) callOllamaLLM(cfg map[string]interface{}, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg["baseUrl"].(string), "/")
	reqBody := map[string]interface{}{
		"model":    cfg["modelName"],
		"messages": messages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": cfg["temperature"],
			"num_ctx":     cfg["maxTokens"],
		},
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", baseURL+"/api/chat", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncateStr(string(rb), 200))
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
