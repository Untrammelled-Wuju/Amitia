// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package modelprotocol

import (
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

type AnthropicAdapter struct{}

func (a *AnthropicAdapter) Protocol() chat.ModelProtocol {
	return chat.ProtocolAnthropicMessages
}

func (a *AnthropicAdapter) Capabilities(ctx context.Context, cfg *chat.ModelConfig) chat.ModelCapabilities {
	var caps chat.ModelCapabilities
	if cfg.CapabilitiesJSON != "" {
		json.Unmarshal([]byte(cfg.CapabilitiesJSON), &caps)
	}
	return caps
}

func (a *AnthropicAdapter) Generate(ctx context.Context, cfg *chat.ModelConfig, req chat.ModelRequest) (*chat.ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    a.buildMessages(req),
		"max_tokens":  cfg.MaxOutputTokens,
		"temperature": cfg.Temperature,
	}
	
	if len(req.Instructions) > 0 {
		requestBody["system"] = strings.Join(req.Instructions, "\n\n")
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	
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

func (a *AnthropicAdapter) Stream(ctx context.Context, cfg *chat.ModelConfig, req chat.ModelRequest, sink chat.ModelEventSink) (*chat.ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    a.buildMessages(req),
		"max_tokens":  cfg.MaxOutputTokens,
		"temperature": cfg.Temperature,
		"stream":      true,
	}
	
	if len(req.Instructions) > 0 {
		requestBody["system"] = strings.Join(req.Instructions, "\n\n")
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	
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

func (a *AnthropicAdapter) buildMessages(req chat.ModelRequest) []map[string]interface{} {
	var messages []map[string]interface{}
	
	for _, msg := range req.Messages {
		content := a.buildContent(msg.Parts)
		messages = append(messages, map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		})
	}
	
	for _, tr := range req.ToolResults {
		messages = append(messages, map[string]interface{}{
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type":        "tool_result",
					"tool_use_id": tr.CallID,
					"content":     tr.Output,
					"is_error":    tr.IsError,
				},
			},
		})
	}
	
	return messages
}

func (a *AnthropicAdapter) buildContent(parts []chat.ModelContentPart) interface{} {
	if len(parts) == 1 && parts[0].Type == chat.ContentTypeText {
		return parts[0].Text
	}
	
	var content []map[string]interface{}
	for _, part := range parts {
		switch part.Type {
		case chat.ContentTypeText:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": part.Text,
			})
		case chat.ContentTypeImage:
			content = append(content, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type": "base64",
					"media_type": part.MIMEType,
					"data": part.ResourceURI,
				},
			})
		}
	}
	return content
}

func (a *AnthropicAdapter) buildTools(tools []chat.ModelToolDefinition) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.Parameters,
		})
	}
	return result
}

func (a *AnthropicAdapter) parseResponse(respBytes []byte) (*chat.ModelResult, error) {
	var result struct {
		ID      string `json:"id"`
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	res := &chat.ModelResult{
		FinishReason: result.StopReason,
	}
	
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			res.Text += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			res.ToolCalls = append(res.ToolCalls, chat.ModelToolCall{
				ID:            block.ID,
				Name:          block.Name,
				ArgumentsJSON: string(args),
			})
		}
	}
	
	res.Usage = chat.ModelUsage{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
	}
	res.ProviderResponseID = result.ID
	
	return res, nil
}

func (a *AnthropicAdapter) parseStream(body io.Reader, sink chat.ModelEventSink) (*chat.ModelResult, error) {
	result := &chat.ModelResult{
		ToolCalls: []chat.ModelToolCall{},
	}
	
	type toolCallInfo struct {
		ID   string
		Name string
		Args strings.Builder
	}
	
	toolCalls := make(map[int]*toolCallInfo)
	
	buf := make([]byte, 4096)
	var buffer strings.Builder
	
	for {
		n, err := body.Read(buf)
		if n > 0 {
			buffer.Write(buf[:n])
			data := buffer.String()
			
			lines := strings.Split(data, "\n")
			buffer.Reset()
			if len(lines) > 0 {
				buffer.WriteString(lines[len(lines)-1])
				lines = lines[:len(lines)-1]
			}
			
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				
				if strings.HasPrefix(line, "event:") {
					continue
				}
				
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				
				content := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				
				var event struct {
					Type  string `json:"type"`
					Index int    `json:"index"`
					Delta struct {
						Type         string `json:"type"`
						Text         string `json:"json"`
						PartialJSON string `json:"partial_json"`
					} `json:"delta"`
					ContentBlock struct {
						Type  string `json:"type"`
						ID    string `json:"id"`
						Name  string `json:"name"`
						Input json.RawMessage `json:"input"`
					} `json:"content_block"`
				}
				
				if err := json.Unmarshal([]byte(content), &event); err != nil {
					continue
				}
				
				switch event.Type {
				case "content_block_start":
					if event.ContentBlock.Type == "tool_use" {
						toolCalls[event.Index] = &toolCallInfo{
							ID:   event.ContentBlock.ID,
							Name: event.ContentBlock.Name,
						}
					}
				case "content_block_delta":
					switch event.Delta.Type {
					case "text_delta":
						result.Text += event.Delta.Text
						sink.Emit(context.Background(), chat.ModelEvent{
							Type:      chat.ModelEventTextDelta,
							TextDelta: event.Delta.Text,
						})
					case "input_json_delta":
						if tc, ok := toolCalls[event.Index]; ok {
							tc.Args.WriteString(event.Delta.PartialJSON)
						}
					}
				case "message_stop":
					for _, tc := range toolCalls {
						result.ToolCalls = append(result.ToolCalls, chat.ModelToolCall{
							ID:            tc.ID,
							Name:          tc.Name,
							ArgumentsJSON: tc.Args.String(),
						})
					}
					sink.Emit(context.Background(), chat.ModelEvent{
						Type: chat.ModelEventCompleted,
					})
					return result, nil
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
	}
	
	return result, nil
}
