// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat/modelprotocol"
	"io"
	"net/http"
	"strings"
	"time"
)

type llmWithToolsFunc func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error)

func protocolForApiType(apiType string) string {
	switch apiType {
	case "ollama":
		return "ollama"
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	default:
		return "openai"
	}
}

func extractSystemMessage(messages []map[string]interface{}) (string, []map[string]interface{}) {
	var systemPrompt string
	var rest []map[string]interface{}
	for _, msg := range messages {
		if role, ok := msg["role"].(string); ok && role == "system" {
			if content, ok := msg["content"].(string); ok {
				if systemPrompt == "" {
					systemPrompt = content
				} else {
					systemPrompt += "\n\n" + content
				}
			}
		} else {
			rest = append(rest, msg)
		}
	}
	return systemPrompt, rest
}

func (s *service) callLLM(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	return s.callLLMMode(ctx, cfg, messages, false)
}

func (s *service) callLLMJSON(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	return s.callLLMMode(ctx, cfg, messages, true)
}

func (s *service) callLLMMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	switch protocolForApiType(cfg.APIType) {
	case "ollama":
		return s.callOllamaMode(ctx, cfg, messages, jsonOnly)
	case "anthropic":
		return s.callAnthropicMode(ctx, cfg, messages, jsonOnly)
	case "gemini":
		return s.callGeminiMode(ctx, cfg, messages, jsonOnly)
	default:
		return s.callOpenAIMode(ctx, cfg, messages, jsonOnly)
	}
}

func (s *service) invokeProcessLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	if s.llmWithTools != nil {
		return s.llmWithTools(ctx, cfg, messages, tools)
	}
	return s.callLLMWithTools(ctx, cfg, messages, tools)
}

func (s *service) callLLMWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	switch protocolForApiType(cfg.APIType) {
	case "ollama":
		return s.callOllamaWithTools(ctx, cfg, messages, tools)
	case "anthropic":
		return s.callAnthropicWithTools(ctx, cfg, messages, tools)
	case "gemini":
		return s.callGeminiWithTools(ctx, cfg, messages, tools)
	default:
		return s.callOpenAIWithTools(ctx, cfg, messages, tools)
	}
}

func (s *service) callOpenAIMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	if jsonOnly {
		reqBody["response_format"] = map[string]string{"type": "json_object"}
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		reqBody["top_p"] = cfg.TopP
	}
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
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
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

func (s *service) callOpenAIWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqMap := map[string]interface{}{"model": cfg.ModelName, "messages": messages, "temperature": cfg.Temperature, "max_tokens": cfg.MaxTokens, "stream": false}
	if len(tools) > 0 {
		reqMap["tools"] = tools
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		reqMap["top_p"] = cfg.TopP
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
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb))
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
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", "", nil, 0, fmt.Errorf("解析响应失败: %v; 原始响应: %s", err, string(rb))
	}
	if len(r.Choices) == 0 {
		return "", "", nil, 0, fmt.Errorf("API 未返回有效回复，原始响应: %s", string(rb))
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

func (s *service) callOllamaMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{
		"model":    cfg.ModelName,
		"messages": messages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": cfg.Temperature,
			"num_ctx":     cfg.MaxTokens,
		},
	}
	if jsonOnly {
		reqBody["format"] = "json"
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
	}
	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		EvalCount       int `json:"eval_count"`
		PromptEvalCount int `json:"prompt_eval_count"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	total := result.EvalCount + result.PromptEvalCount
	return result.Message.Content, total, nil
}

func (s *service) callOllamaWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{
		"model":    cfg.ModelName,
		"messages": messages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": cfg.Temperature,
			"num_ctx":     cfg.MaxTokens,
		},
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/chat", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb))
	}
	var r struct {
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
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", "", nil, 0, fmt.Errorf("解析响应失败: %v; 原始响应: %s", err, string(rb))
	}
	var toolCalls []map[string]interface{}
	for i, tc := range r.Message.ToolCalls {
		toolCalls = append(toolCalls, map[string]interface{}{
			"id":   fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), i),
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.Function.Name,
				"arguments": string(tc.Function.Arguments),
			},
		})
	}
	total := r.EvalCount + r.PromptEvalCount
	return r.Message.Content, "", toolCalls, total, nil
}

func messagesToModelRequest(cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool, jsonOnly bool) ModelRequest {
	var instructions []string
	var msgs []ModelMessage
	for _, m := range messages {
		role, _ := m["role"].(string)
		switch role {
		case "system":
			if content, ok := m["content"].(string); ok {
				instructions = append(instructions, content)
			}
		case "user":
			parts := extractContentParts(m)
			msgs = append(msgs, ModelMessage{Role: "user", Parts: parts})
		case "assistant":
			parts := extractContentParts(m)
			if len(parts) == 0 {
				parts = []ModelContentPart{{Type: ContentTypeText, Text: ""}}
			}
			msgs = append(msgs, ModelMessage{Role: "assistant", Parts: parts})
		case "tool":
			parts := extractContentParts(m)
			msgs = append(msgs, ModelMessage{Role: "tool", Parts: parts})
		}
	}
	req := ModelRequest{
		Model:    cfg.ModelName,
		Messages: msgs,
		Stream:   false,
	}
	if cfg.MaxOutputTokens > 0 {
		req.MaxOutputTokens = cfg.MaxOutputTokens
	} else if cfg.MaxTokens > 0 {
		req.MaxOutputTokens = cfg.MaxTokens
	}
	if cfg.Temperature > 0 {
		t := cfg.Temperature
		req.Temperature = &t
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		p := cfg.TopP
		req.TopP = &p
	}
	if jsonOnly {
		req.ResponseFormat = ModelResponseFormat{Type: "json_object"}
	}
	if len(tools) > 0 {
		req.Tools = toolsToDefinitions(tools)
	}
	return req
}

func extractContentParts(m map[string]interface{}) []ModelContentPart {
	if content, ok := m["content"].(string); ok {
		if content == "" {
			return nil
		}
		return []ModelContentPart{{Type: ContentTypeText, Text: content}}
	}
	if parts, ok := m["parts"].([]ModelContentPart); ok {
		return parts
	}
	return nil
}

func toolsToDefinitions(tools []tool.Tool) []ModelToolDefinition {
	defs := make([]ModelToolDefinition, 0, len(tools))
	for _, t := range tools {
		params := map[string]any{
			"type":       t.Function.Parameters.Type,
			"properties": t.Function.Parameters.Properties,
		}
		if len(t.Function.Parameters.Required) > 0 {
			params["required"] = t.Function.Parameters.Required
		}
		defs = append(defs, ModelToolDefinition{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  params,
		})
	}
	return defs
}

func modelResultToLegacy(result *ModelResult) (string, string, []map[string]interface{}, int) {
	var toolCalls []map[string]interface{}
	for _, tc := range result.ToolCalls {
		toolCalls = append(toolCalls, map[string]interface{}{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]interface{}{
				"name":      tc.Name,
				"arguments": tc.ArgumentsJSON,
			},
		})
	}
	reasoning := ""
	return result.Text, reasoning, toolCalls, result.Usage.TotalTokens
}

func cfgToProviderConfig(cfg *ModelConfig) modelprotocol.ProviderConfig {
	return modelprotocol.ProviderConfig{
		ModelName:        cfg.ModelName,
		BaseURL:          cfg.BaseURL,
		APIKey:           cfg.APIKey,
		Temperature:      cfg.Temperature,
		TopP:             cfg.TopP,
		TimeoutSeconds:   cfg.TimeoutSeconds,
		MaxTokens:        cfg.MaxTokens,
		MaxOutputTokens:  cfg.MaxOutputTokens,
		ContextWindow:    cfg.ContextWindow,
		Protocol:         cfg.Protocol,
		CapabilitiesJSON: cfg.CapabilitiesJSON,
	}
}

func (s *service) callLLMWithAdapter(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	protocol := resolveProtocol(cfg)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	req := messagesToModelRequest(cfg, messages, nil, jsonOnly)
	pcfg := cfgToProviderConfig(cfg)
	result, err := adapter.Generate(ctx, pcfg, req)
	if err != nil {
		return "", 0, err
	}
	text, _, _, tokens := modelResultToLegacy(result)
	return text, tokens, nil
}

func (s *service) callLLMWithToolsAdapter(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	protocol := resolveProtocol(cfg)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	req := messagesToModelRequest(cfg, messages, tools, false)
	pcfg := cfgToProviderConfig(cfg)
	result, err := adapter.Generate(ctx, pcfg, req)
	if err != nil {
		return "", "", nil, 0, err
	}
	return modelResultToLegacy(result)
}

type noopEventSink struct{}

func (noopEventSink) Emit(ctx context.Context, event ModelEvent) error { return nil }

func (s *service) callLLMStreamAdapter(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool, sink ModelEventSink) (*ModelResult, error) {
	protocol := resolveProtocol(cfg)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	req := messagesToModelRequest(cfg, messages, tools, false)
	req.Stream = true
	pcfg := cfgToProviderConfig(cfg)
	if sink == nil {
		sink = noopEventSink{}
	}
	return adapter.Stream(ctx, pcfg, req, sink)
}

func (s *service) callAnthropicMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	systemPrompt, chatMessages := extractSystemMessage(messages)
	base := strings.TrimRight(cfg.BaseURL, "/")
	reqBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    chatMessages,
		"max_tokens":  cfg.MaxTokens,
		"temperature": cfg.Temperature,
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}
	if jsonOnly {
		tools := []map[string]interface{}{
			{"name": "output_json", "description": "Output the response as JSON", "input_schema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
		}
		reqBody["tools"] = tools
		reqBody["tool_choice"] = map[string]interface{}{"type": "tool", "name": "output_json"}
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	var content string
	for _, block := range result.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	return content, result.Usage.InputTokens + result.Usage.OutputTokens, nil
}

func (s *service) callAnthropicWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	systemPrompt, chatMessages := extractSystemMessage(messages)
	base := strings.TrimRight(cfg.BaseURL, "/")
	var anthropicTools []map[string]interface{}
	for _, t := range tools {
		anthropicTools = append(anthropicTools, map[string]interface{}{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": t.Function.Parameters,
		})
	}
	reqBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    chatMessages,
		"max_tokens":  cfg.MaxTokens,
		"temperature": cfg.Temperature,
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}
	if len(anthropicTools) > 0 {
		reqBody["tools"] = anthropicTools
	}
	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/v1/messages", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb))
	}
	var r struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", "", nil, 0, fmt.Errorf("解析响应失败: %v; 原始响应: %s", err, string(rb))
	}
	var content string
	var toolCalls []map[string]interface{}
	for _, block := range r.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   block.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      block.Name,
					"arguments": string(block.Input),
				},
			})
		}
	}
	return content, "", toolCalls, r.Usage.InputTokens + r.Usage.OutputTokens, nil
}

func (s *service) callGeminiMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	systemPrompt, chatMessages := extractSystemMessage(messages)
	base := strings.TrimRight(cfg.BaseURL, "/")
	var contents []map[string]interface{}
	for _, msg := range chatMessages {
		role, _ := msg["role"].(string)
		if role == "assistant" {
			role = "model"
		}
		content, _ := msg["content"].(string)
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]interface{}{{"text": content}},
		})
	}
	genConfig := map[string]interface{}{
		"temperature":     cfg.Temperature,
		"maxOutputTokens": cfg.MaxTokens,
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		genConfig["topP"] = cfg.TopP
	}
	if jsonOnly {
		genConfig["responseMimeType"] = "application/json"
	}
	reqBody := map[string]interface{}{
		"contents":         contents,
		"generationConfig": genConfig,
	}
	if systemPrompt != "" {
		reqBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": systemPrompt}},
		}
	}
	jsonBody, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", base, cfg.ModelName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("key", cfg.APIKey)
	req.URL.RawQuery = q.Encode()
	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("API 返回 %d: %s", resp.StatusCode, string(respBytes))
	}
	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w", err)
	}
	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", 0, fmt.Errorf("API 未返回有效回复")
	}
	var content string
	for _, part := range result.Candidates[0].Content.Parts {
		content += part.Text
	}
	return content, result.UsageMetadata.TotalTokenCount, nil
}

func (s *service) callGeminiWithTools(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool) (string, string, []map[string]interface{}, int, error) {
	systemPrompt, chatMessages := extractSystemMessage(messages)
	base := strings.TrimRight(cfg.BaseURL, "/")
	var contents []map[string]interface{}
	for _, msg := range chatMessages {
		role, _ := msg["role"].(string)
		if role == "assistant" {
			role = "model"
		}
		content, _ := msg["content"].(string)
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]interface{}{{"text": content}},
		})
	}
	var funcDecls []map[string]interface{}
	for _, t := range tools {
		funcDecls = append(funcDecls, map[string]interface{}{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"parameters":  t.Function.Parameters,
		})
	}
	genConfig := map[string]interface{}{
		"temperature":     cfg.Temperature,
		"maxOutputTokens": cfg.MaxTokens,
	}
	if cfg.TopP > 0 && cfg.TopP < 1 {
		genConfig["topP"] = cfg.TopP
	}
	reqBody := map[string]interface{}{
		"contents":         contents,
		"generationConfig": genConfig,
	}
	if systemPrompt != "" {
		reqBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{{"text": systemPrompt}},
		}
	}
	if len(funcDecls) > 0 {
		reqBody["tools"] = []map[string]interface{}{
			{"functionDeclarations": funcDecls},
		}
	}
	jsonBody, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", base, cfg.ModelName)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Set("key", cfg.APIKey)
	req.URL.RawQuery = q.Encode()
	resp, err := (&http.Client{Timeout: 180 * time.Second}).Do(req)
	if err != nil {
		return "", "", nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", nil, 0, fmt.Errorf("API %d: %s", resp.StatusCode, string(rb))
	}
	var r struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text     string `json:"text"`
					FuncCall struct {
						Name string          `json:"name"`
						Args json.RawMessage `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			TotalTokenCount int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(rb, &r); err != nil {
		return "", "", nil, 0, fmt.Errorf("解析响应失败: %v; 原始响应: %s", err, string(rb))
	}
	if len(r.Candidates) == 0 {
		return "", "", nil, 0, fmt.Errorf("API 未返回有效回复，原始响应: %s", string(rb))
	}
	var content string
	var toolCalls []map[string]interface{}
	for _, part := range r.Candidates[0].Content.Parts {
		if part.Text != "" {
			content += part.Text
		}
		if part.FuncCall.Name != "" {
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   fmt.Sprintf("call_%d", time.Now().UnixNano()),
				"type": "function",
				"function": map[string]interface{}{
					"name":      part.FuncCall.Name,
					"arguments": string(part.FuncCall.Args),
				},
			})
		}
	}
	return content, "", toolCalls, r.UsageMetadata.TotalTokenCount, nil
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
