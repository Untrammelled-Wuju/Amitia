// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/agent/tool"
	"io"
	"net/http"
	"strings"
	"time"
)

type llmWithToolsFunc func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error)

func (s *service) callLLM(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	jsonBody, _ := json.Marshal(reqBody)
	url := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, truncateStr(string(respBytes), 200))
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("API 未返回有效回复")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

func (s *service) invokeProcessLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	if s.llmWithTools != nil {
		return s.llmWithTools(ctx, cfg, messages, tools)
	}
	return s.callLLMWithTools(ctx, cfg, messages, tools)
}

func (s *service) callLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqMap := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	if len(tools) > 0 {
		reqMap["tools"] = tools
	}
	reqBody, _ := json.Marshal(reqMap)
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, truncateStr(string(rb), 200))
	}
	var r struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			}
		}
		Usage struct{ TotalTokens int }
	}
	json.Unmarshal(rb, &r)
	if len(r.Choices) == 0 {
		return "", "", nil, 0, fmt.Errorf("no choices")
	}
	choice := r.Choices[0]
	var toolCalls []map[string]interface{}
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, map[string]interface{}{
			"id": tc.ID, "type": "function",
			"function": map[string]interface{}{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
		})
	}
	return choice.Message.Content, choice.Message.ReasoningContent, toolCalls, r.Usage.TotalTokens, nil
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
