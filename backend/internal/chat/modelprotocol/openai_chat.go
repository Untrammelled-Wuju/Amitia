// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package modelprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type OpenAIChatAdapter struct{}

func (a *OpenAIChatAdapter) Protocol() ModelProtocol {
	return ProtocolOpenAIChat
}

func (a *OpenAIChatAdapter) Capabilities(ctx context.Context, cfg ProviderConfig) ModelCapabilities {
	var caps ModelCapabilities
	if cfg.CapabilitiesJSON != "" {
		json.Unmarshal([]byte(cfg.CapabilitiesJSON), &caps)
	}
	return caps
}

func (a *OpenAIChatAdapter) Generate(ctx context.Context, cfg ProviderConfig, req ModelRequest) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	messages := a.buildMessages(req)

	requestBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    messages,
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxOutputTokens,
		"stream":      false,
	}

	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["top_p"] = cfg.TopP
	}

	applyOpenAIChatControls(requestBody, cfg, req)

	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}

	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

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

func (a *OpenAIChatAdapter) Stream(ctx context.Context, cfg ProviderConfig, req ModelRequest, sink ModelEventSink) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	messages := a.buildMessages(req)

	requestBody := map[string]interface{}{
		"model":       cfg.ModelName,
		"messages":    messages,
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxOutputTokens,
		"stream":      true,
	}

	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["top_p"] = cfg.TopP
	}

	applyOpenAIChatControls(requestBody, cfg, req)

	if len(req.Tools) > 0 {
		requestBody["tools"] = a.buildTools(req.Tools)
	}

	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

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

	return a.parseStream(ctx, resp.Body, sink)
}

func applyOpenAIChatControls(requestBody map[string]interface{}, cfg ProviderConfig, req ModelRequest) {
	switch req.ResponseFormat.Type {
	case "json_schema", "json", "json_object":
		requestBody["response_format"] = map[string]string{"type": "json_object"}
	}
	if req.DisableThinking && strings.Contains(strings.ToLower(cfg.BaseURL), "deepseek.com") {
		requestBody["thinking"] = map[string]string{"type": "disabled"}
	}
	if req.ReasoningEffort != "" && !req.DisableThinking {
		requestBody["reasoning_effort"] = req.ReasoningEffort
	}
}

func (a *OpenAIChatAdapter) buildMessages(req ModelRequest) []map[string]interface{} {
	var messages []map[string]interface{}

	for _, inst := range req.Instructions {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": inst,
		})
	}

	for _, msg := range req.Messages {
		content := a.buildContent(msg.Parts)
		message := map[string]interface{}{
			"role":    msg.Role,
			"content": content,
		}
		if msg.Role == "tool" && msg.ToolCallID != "" {
			message["tool_call_id"] = msg.ToolCallID
		}
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   call.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      call.Name,
						"arguments": call.ArgumentsJSON,
					},
				})
			}
			message["tool_calls"] = toolCalls
		}
		messages = append(messages, message)
	}

	for _, tr := range req.ToolResults {
		messages = append(messages, map[string]interface{}{
			"role":         "tool",
			"tool_call_id": tr.CallID,
			"content":      tr.Output,
		})
	}

	return messages
}

func (a *OpenAIChatAdapter) buildContent(parts []ModelContentPart) interface{} {
	if len(parts) == 1 && parts[0].Type == ContentTypeText {
		return parts[0].Text
	}

	var content []map[string]interface{}
	for _, part := range parts {
		switch part.Type {
		case ContentTypeText:
			content = append(content, map[string]interface{}{
				"type": "text",
				"text": part.Text,
			})
		case ContentTypeImage:
			content = append(content, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]interface{}{
					"url":    part.ResourceURI,
					"detail": part.Detail,
				},
			})
		}
	}
	if len(content) == 0 {
		return ""
	}
	return content
}

func (a *OpenAIChatAdapter) buildTools(tools []ModelToolDefinition) []map[string]interface{} {
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

func (a *OpenAIChatAdapter) parseResponse(respBytes []byte) (*ModelResult, error) {
	var result struct {
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
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("API 未返回有效回复")
	}

	choice := result.Choices[0]
	res := &ModelResult{
		Text:         choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: ModelUsage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
			TotalTokens:  result.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		res.ToolCalls = append(res.ToolCalls, ModelToolCall{
			ID:            tc.ID,
			Name:          tc.Function.Name,
			ArgumentsJSON: tc.Function.Arguments,
		})
	}

	if choice.Message.ReasoningContent != "" {
		res.Continuation = &ModelContinuationState{
			Protocol:    string(ProtocolOpenAIChat),
			OpaqueItems: []json.RawMessage{json.RawMessage(`"` + choice.Message.ReasoningContent + `"`)},
		}
	}

	return res, nil
}

func (a *OpenAIChatAdapter) parseStream(ctx context.Context, body io.Reader, sink ModelEventSink) (*ModelResult, error) {
	result := &ModelResult{
		ToolCalls: []ModelToolCall{},
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	type toolCallState struct {
		call      ModelToolCall
		arguments strings.Builder
	}
	toolCalls := make(map[int]*toolCallState)
	finalized := false

	finalizeToolCalls := func() error {
		if finalized {
			return nil
		}
		finalized = true
		indexes := make([]int, 0, len(toolCalls))
		for index := range toolCalls {
			indexes = append(indexes, index)
		}
		sort.Ints(indexes)
		for _, index := range indexes {
			state := toolCalls[index]
			if state == nil || strings.TrimSpace(state.call.ID) == "" {
				continue
			}
			state.call.ArgumentsJSON = state.arguments.String()
			result.ToolCalls = append(result.ToolCalls, state.call)
			if err := sink.Emit(ctx, ModelEvent{Type: ModelEventToolCallDone, ToolCallID: state.call.ID, ToolName: state.call.Name}); err != nil {
				return err
			}
		}
		return nil
	}

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
				if line == "" || !strings.HasPrefix(line, "data:") {
					continue
				}

				content := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if content == "[DONE]" {
					if err := finalizeToolCalls(); err != nil {
						return result, err
					}
					if err := sink.Emit(ctx, ModelEvent{
						Type: ModelEventCompleted,
					}); err != nil {
						return result, err
					}
					if err := ctx.Err(); err != nil {
						return result, err
					}
					return result, nil
				}

				var chunk struct {
					Choices []struct {
						Delta struct {
							Content          string `json:"content"`
							ReasoningContent string `json:"reasoning_content"`
							ToolCalls        []struct {
								Index    int    `json:"index"`
								ID       string `json:"id"`
								Function struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								} `json:"function"`
							} `json:"tool_calls"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
				}

				if err := json.Unmarshal([]byte(content), &chunk); err != nil {
					continue
				}

				if len(chunk.Choices) == 0 {
					continue
				}

				choice := chunk.Choices[0]

				if choice.Delta.Content != "" {
					result.Text += choice.Delta.Content
					if err := sink.Emit(ctx, ModelEvent{Type: ModelEventTextDelta, TextDelta: choice.Delta.Content}); err != nil {
						return result, err
					}
				}
				if choice.Delta.ReasoningContent != "" {
					if err := sink.Emit(ctx, ModelEvent{Type: ModelEventReasoningSummaryDelta, TextDelta: choice.Delta.ReasoningContent}); err != nil {
						return result, err
					}
				}

				for _, tc := range choice.Delta.ToolCalls {
					id := strings.TrimSpace(tc.ID)
					state := toolCalls[tc.Index]
					isNew := id != "" && (state == nil || state.call.ID != id)
					if isNew {
						state = &toolCallState{}
						state.call.ID = id
						toolCalls[tc.Index] = state
					}
					if state == nil {
						continue
					}
					if id == "" {
						id = state.call.ID
					}
					if tc.Function.Name != "" {
						state.call.Name = tc.Function.Name
					}
					if isNew {
						if err := sink.Emit(ctx, ModelEvent{Type: ModelEventToolCallStarted, ToolCallID: id, ToolName: state.call.Name}); err != nil {
							return result, err
						}
					}
					if tc.Function.Arguments != "" {
						state.arguments.WriteString(tc.Function.Arguments)
						if err := sink.Emit(ctx, ModelEvent{Type: ModelEventToolCallArgumentsDelta, ToolCallID: id, ToolName: state.call.Name, ArgumentsDelta: tc.Function.Arguments}); err != nil {
							return result, err
						}
					}
				}

				if choice.FinishReason != "" {
					result.FinishReason = choice.FinishReason
				}
			}
		}
		if err == io.EOF {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			break
		}
		if err != nil {
			return result, err
		}
	}

	if err := finalizeToolCalls(); err != nil {
		return result, err
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}
