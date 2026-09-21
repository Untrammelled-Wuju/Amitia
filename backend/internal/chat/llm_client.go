// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/chat/modelprotocol"
	"io"
	"net/http"
	"strings"
	"time"
)

type LocalModelInfer = localmodel.LocalModelInference

type llmWithToolsFunc func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error)

func protocolForApiType(apiType string) string {
	switch apiType {
	case "ollama":
		return "ollama"
	case "anthropic":
		return "anthropic"
	case "gemini":
		return "gemini"
	case "mnn":
		return "mnn"
	case "llama_cpp":
		return "llama_cpp"
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

func (s *service) callLLMWithoutThinking(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}) (string, int, error) {
	return s.callLLMWithAdapterMode(ctx, cfg, messages, false, true)
}

func (s *service) callLLMMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	switch protocolForApiType(cfg.APIType) {
	case "mnn":
		return s.callMNNMode(ctx, cfg, messages, jsonOnly)
	case "llama_cpp":
		return s.callLlamaCppMode(ctx, cfg, messages, jsonOnly)
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

type localModelEventSink struct {
	ctx  context.Context
	sink ModelEventSink
}

func (s localModelEventSink) OnTextDelta(text string) error {
	return s.sink.Emit(s.ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: text})
}

func (s localModelEventSink) OnReasoningDelta(text string) error {
	return s.sink.Emit(s.ctx, ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: text})
}

func (s localModelEventSink) OnToolCallDelta(callID string, name string, arguments string) error {
	if err := s.sink.Emit(s.ctx, ModelEvent{Type: ModelEventToolCallStarted, ToolCallID: callID, ToolName: name}); err != nil {
		return err
	}
	if arguments == "" {
		return nil
	}
	return s.sink.Emit(s.ctx, ModelEvent{Type: ModelEventToolCallArgumentsDelta, ToolCallID: callID, ToolName: name, ArgumentsDelta: arguments})
}

func (s localModelEventSink) OnUsage(usage localmodel.LocalModelUsage) error {
	return s.sink.Emit(s.ctx, ModelEvent{Type: ModelEventUsage, Usage: &ModelUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}})
}

func (s *service) invokeProcessLLMWithToolsStream(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool, sink ModelEventSink) (string, string, []map[string]interface{}, int, error) {
	if sink == nil {
		sink = noopEventSink{}
	}
	if s.llmWithTools != nil {
		text, reasoning, calls, tokens, err := s.llmWithTools(ctx, cfg, messages, tools)
		if reasoning != "" {
			if emitErr := sink.Emit(ctx, ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: reasoning}); emitErr != nil {
				return "", "", nil, 0, emitErr
			}
			_ = sink.Emit(ctx, ModelEvent{Type: ModelEventReasoningSummaryDone})
		}
		if text != "" {
			if emitErr := sink.Emit(ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: text}); emitErr != nil {
				return "", "", nil, 0, emitErr
			}
			_ = sink.Emit(ctx, ModelEvent{Type: ModelEventTextDone})
		}
		if err == nil {
			_ = sink.Emit(ctx, ModelEvent{Type: ModelEventCompleted})
		}
		return text, reasoning, calls, tokens, err
	}
	protocol := protocolForApiType(cfg.APIType)
	if protocol == "mnn" || protocol == "llama_cpp" {
		backend, err := s.getLocalModelBackend(ctx, cfg)
		if err != nil {
			return "", "", nil, 0, err
		}
		req := messagesToModelRequest(cfg, messages, tools, false)
		localReq := toLocalModelRequest(req, messages)
		localReq.Tools = toolsToLocalModelTools(tools)
		result, err := backend.Generate(ctx, localReq, localModelEventSink{ctx: ctx, sink: sink})
		if err != nil {
			return "", "", nil, 0, err
		}
		calls := make([]map[string]interface{}, 0, len(result.ToolCalls))
		for _, tc := range result.ToolCalls {
			calls = append(calls, map[string]interface{}{"id": tc.ID, "type": "function", "function": map[string]interface{}{"name": tc.Name, "arguments": tc.Arguments}})
		}
		_ = sink.Emit(ctx, ModelEvent{Type: ModelEventCompleted})
		return result.Text, "", calls, result.Usage.TotalTokens, nil
	}
	result, err := s.callLLMStreamAdapter(ctx, cfg, messages, tools, false, false, sink)
	if err != nil {
		return "", "", nil, 0, err
	}
	text, reasoning, calls, tokens := modelResultToLegacy(result)
	return text, reasoning, calls, tokens, nil
}

func (s *service) callMNNMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	backend, err := s.getLocalModelBackend(ctx, cfg)
	if err != nil {
		return "", 0, err
	}

	req := messagesToModelRequest(cfg, messages, nil, jsonOnly)
	localReq := toLocalModelRequest(req, messages)

	result, err := backend.Generate(ctx, localReq, nil)
	if err != nil {
		return "", 0, err
	}
	return result.Text, result.Usage.TotalTokens, nil
}

func (s *service) callLlamaCppMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool) (string, int, error) {
	backend, err := s.getLocalModelBackend(ctx, cfg)
	if err != nil {
		return "", 0, err
	}

	req := messagesToModelRequest(cfg, messages, nil, jsonOnly)
	localReq := toLocalModelRequest(req, messages)

	result, err := backend.Generate(ctx, localReq, nil)
	if err != nil {
		return "", 0, err
	}
	return result.Text, result.Usage.TotalTokens, nil
}

func (s *service) getLocalModelBackend(ctx context.Context, cfg *ModelConfig) (LocalModelInfer, error) {
	key := localModelRuntimeKey(cfg)

	s.localModelMu.Lock()

	if backend := s.localModels[key]; backend != nil {
		s.localModelMu.Unlock()

		if err := ensureLocalModelReady(ctx, backend); err != nil {
			return nil, err
		}

		return backend, nil
	}

	backend, err := localmodel.Create(localmodel.CreateLocalModelParams{
		Provider:     cfg.APIType,
		ModelName:    cfg.ModelName,
		ProviderJSON: cfg.ProviderConfigJSON,
		Timeout:      time.Duration(cfg.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		s.localModelMu.Unlock()
		return nil, err
	}

	if err := backend.Load(ctx); err != nil {
		s.localModelMu.Unlock()
		_ = backend.Unload(context.Background())
		return nil, err
	}

	health := backend.Health(ctx)
	if health.State != "ready" {
		s.localModelMu.Unlock()
		_ = backend.Unload(context.Background())
		return nil, fmt.Errorf("local model not ready: %s", health.State)
	}

	s.localModels[key] = backend
	s.localModelMu.Unlock()

	return backend, nil
}

func localModelRuntimeKey(cfg *ModelConfig) string {
	h := sha256.New()
	h.Write([]byte(cfg.APIType))
	h.Write([]byte(cfg.ModelName))
	h.Write([]byte(cfg.ProviderConfigJSON))
	return hex.EncodeToString(h.Sum(nil))
}

func ensureLocalModelReady(ctx context.Context, backend LocalModelInfer) error {
	health := backend.Health(ctx)
	switch health.State {
	case "ready":
		return nil
	case "loading", "generating", "embedding":
		return localmodel.ErrModelNotYetReady
	case "stopping":
		return localmodel.ErrModelUnavailable
	case "failed", "unloaded":
		return backend.Load(ctx)
	default:
		return backend.Load(ctx)
	}
}

func toLocalModelRequest(req ModelRequest, messages []map[string]interface{}) localmodel.LocalModelRequest {
	localReq := localmodel.LocalModelRequest{
		MaxNewTokens: req.MaxOutputTokens,
		Temperature:  cfgTemperature(req),
		TopP:         cfgTopP(req),
		JSONOnly:     req.ResponseFormat.Type == "json_object" || req.ResponseFormat.Type == "json",
	}
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		localReq.Messages = append(localReq.Messages, localmodel.LocalModelMessage{
			Role:  role,
			Parts: []localmodel.LocalModelContent{{Type: "text", Text: content}},
		})
	}
	return localReq
}

func cfgTemperature(req ModelRequest) float64 {
	if req.Temperature != nil {
		return *req.Temperature
	}
	return 0.7
}

func cfgTopP(req ModelRequest) float64 {
	if req.TopP != nil {
		return *req.TopP
	}
	return 1.0
}

func toolsToLocalModelTools(tools []tool.Tool) []localmodel.LocalModelTool {
	result := make([]localmodel.LocalModelTool, 0, len(tools))
	for _, t := range tools {
		params := map[string]any{
			"type":       t.Function.Parameters.Type,
			"properties": t.Function.Parameters.Properties,
		}
		if len(t.Function.Parameters.Required) > 0 {
			params["required"] = t.Function.Parameters.Required
		}
		result = append(result, localmodel.LocalModelTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  params,
		})
	}
	return result
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

func openAICompatibleTools(tools []tool.Tool) []tool.Tool {
	result := make([]tool.Tool, 0, len(tools))
	for _, candidate := range tools {
		name := candidate.Function.Name
		if name == "" || len(name) > 64 {
			continue
		}
		valid := true
		for _, char := range name {
			if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' && char != '-' {
				valid = false
				break
			}
		}
		if valid {
			result = append(result, candidate)
		}
	}
	return result
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
		Model:           cfg.ModelName,
		Messages:        msgs,
		Stream:          false,
		ReasoningEffort: cfg.ReasoningEffort,
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
	return s.callLLMWithAdapterMode(ctx, cfg, messages, jsonOnly, false)
}

func (s *service) callLLMWithAdapterMode(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, jsonOnly bool, disableThinking bool) (string, int, error) {
	protocol := resolveProtocol(cfg)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	req := messagesToModelRequest(cfg, messages, nil, jsonOnly)
	req.DisableThinking = disableThinking
	pcfg := cfgToProviderConfig(cfg)
	result, err := adapter.Generate(ctx, pcfg, req)
	if err != nil {
		return "", 0, err
	}
	text, _, _, tokens := modelResultToLegacy(result)
	return text, tokens, nil
}

type noopEventSink struct{}

func (noopEventSink) Emit(ctx context.Context, event ModelEvent) error { return nil }

func reasoningBudget(effort string) int {
	switch effort {
	case "low":
		return 1024
	case "medium":
		return 4096
	case "high":
		return 8192
	case "xhigh":
		return 16384
	default:
		return 0
	}
}

func applyLegacyAnthropicThinking(
	requestBody map[string]interface{},
	effort string,
	maxTokens int,
) {
	budget := reasoningBudget(effort)
	if budget <= 0 {
		return
	}
	if maxTokens <= budget {
		requestBody["max_tokens"] = budget + 2048
	}
	requestBody["thinking"] = map[string]interface{}{
		"type":          "enabled",
		"budget_tokens": budget,
	}
}

func (s *service) callLLMStreamAdapter(ctx context.Context, cfg *ModelConfig, messages []map[string]interface{}, tools []tool.Tool, jsonOnly bool, disableThinking bool, sink ModelEventSink) (*ModelResult, error) {
	protocol := resolveProtocol(cfg)
	adapter := modelprotocol.AdapterForProtocol(protocol)
	req := messagesToModelRequest(cfg, messages, tools, jsonOnly)
	req.Stream = true
	req.DisableThinking = disableThinking
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
	if budget := reasoningBudget(cfg.ReasoningEffort); budget > 0 {
		genConfig["thinkingConfig"] = map[string]interface{}{
			"thinkingBudget": budget,
		}
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

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
