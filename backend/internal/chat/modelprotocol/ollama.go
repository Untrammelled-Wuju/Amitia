// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package modelprotocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/chat"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaAdapter struct{}

func (a *OllamaAdapter) Protocol() chat.ModelProtocol {
	return chat.ProtocolOllamaChat
}

func (a *OllamaAdapter) Capabilities(ctx context.Context, cfg *chat.ModelConfig) chat.ModelCapabilities {
	var caps chat.ModelCapabilities
	if cfg.CapabilitiesJSON != "" {
		json.Unmarshal([]byte(cfg.CapabilitiesJSON), &caps)
	}
	return caps
}

func (a *OllamaAdapter) Generate(ctx context.Context, cfg *chat.ModelConfig, req chat.ModelRequest) (*chat.ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"model":    cfg.ModelName,
		"messages": a.buildMessages(req),
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": cfg.Temperature,
			"num_ctx":     cfg.ContextWindow,
		},
	}
	
	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["options"].(map[string]interface{})["top_p"] = cfg.TopP
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}
	
	if req.ResponseFormat.Type == "json" || req.ResponseFormat.Type == "json_schema" {
		requestBody["format"] = "json"
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
	}
	
	return a.parseResponse(respBytes)
}

func (a *OllamaAdapter) Stream(ctx context.Context, cfg *chat.ModelConfig, req chat.ModelRequest, sink chat.ModelEventSink) (*chat.ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"model":    cfg.ModelName,
		"messages": a.buildMessages(req),
		"stream":   true,
		"options": map[string]interface{}{
			"temperature": cfg.Temperature,
			"num_ctx":     cfg.ContextWindow,
		},
	}
	
	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["options"].(map[string]interface{})["top_p"] = cfg.TopP
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
	}
	
	return a.parseStream(resp.Body, sink)
}

func (a *OllamaAdapter) buildMessages(req chat.ModelRequest) []map[string]interface{} {
	var messages []map[string]interface{}
	
	for _, inst := range req.Instructions {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": inst,
		})
	}
	
	for _, msg := range req.Messages {
		content := a.buildContent(msg.Parts)
		messages = append(messages, map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		})
	}
	
	for _, tr := range req.ToolResults {
		messages = append(messages, map[string]interface{}{
			"role":    "tool",
			"content": tr.Output,
		})
	}
	
	return messages
}

func (a *OllamaAdapter) buildContent(parts []chat.ModelContentPart) interface{} {
	if len(parts) == 1 && parts[0].Type == chat.ContentTypeText {
		return parts[0].Text
	}
	
	content := parts[0].Text
	var images []string
	for _, part := range parts {
		if part.Type == chat.ContentTypeImage {
			images = append(images, part.ResourceURI)
		}
	}
	
	if len(images) > 0 {
		return map[string]interface{}{
			"content": content,
			"images":  images,
		}
	}
	
	return content
}

func (a *OllamaAdapter) buildTools(tools []chat.ModelToolDefinition) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return result
}

func (a *OllamaAdapter) parseResponse(respBytes []byte) (*chat.ModelResult, error) {
	var result struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		EvalCount       int `json:"eval_count"`
		PromptEvalCount int `json:"prompt_eval_count"`
	}
	
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	res := &chat.ModelResult{
		Text: result.Message.Content,
	}
	
	for _, tc := range result.Message.ToolCalls {
		res.ToolCalls = append(res.ToolCalls, chat.ModelToolCall{
			Name:          tc.Function.Name,
			ArgumentsJSON: string(tc.Function.Arguments),
		})
	}
	
	res.Usage = chat.ModelUsage{
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		TotalTokens:  result.PromptEvalCount + result.EvalCount,
	}
	
	return res, nil
}

func (a *OllamaAdapter) parseStream(body io.Reader, sink chat.ModelEventSink) (*chat.ModelResult, error) {
	result := &chat.ModelResult{
		ToolCalls: []chat.ModelToolCall{},
	}
	
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		
		var chunk struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					Function struct {
						Name      string          `json:"name"`
						Arguments json.RawMessage `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			Done    bool   `json:"done"`
			Error   string `json:"error"`
			EvalCount       int `json:"eval_count"`
			PromptEvalCount int `json:"prompt_eval_count"`
		}
		
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}
		
		if chunk.Error != "" {
			sink.Emit(context.Background(), chat.ModelEvent{
				Type: chat.ModelEventFailed,
				Error: &chat.ModelError{
					Code:     "MODEL_PROVIDER_FAILED",
					Protocol: chat.ProtocolOllamaChat,
					Message:  chunk.Error,
				},
			})
			return result, fmt.Errorf("ollama error: %s", chunk.Error)
		}
		
		if chunk.Message.Content != "" {
			result.Text += chunk.Message.Content
			sink.Emit(context.Background(), chat.ModelEvent{
				Type:      chat.ModelEventTextDelta,
				TextDelta: chunk.Message.Content,
			})
		}
		
		for _, tc := range chunk.Message.ToolCalls {
			if tc.Function.Name != "" {
				result.ToolCalls = append(result.ToolCalls, chat.ModelToolCall{
					Name:          tc.Function.Name,
					ArgumentsJSON: string(tc.Function.Arguments),
				})
			}
		}
		
		if chunk.Done {
			result.Usage = chat.ModelUsage{
				InputTokens:  chunk.PromptEvalCount,
				OutputTokens: chunk.EvalCount,
				TotalTokens:  chunk.PromptEvalCount + chunk.EvalCount,
			}
			sink.Emit(context.Background(), chat.ModelEvent{
				Type: chat.ModelEventCompleted,
			})
			return result, nil
		}
	}
	
	if err := scanner.Err(); err != nil {
		return result, err
	}
	
	return result, nil
}
