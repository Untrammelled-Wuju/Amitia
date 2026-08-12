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
	"strings"
	"time"
)

type OpenAIResponsesAdapter struct{}

func (a *OpenAIResponsesAdapter) Protocol() ModelProtocol {
	return ProtocolOpenAIResponses
}

func (a *OpenAIResponsesAdapter) Capabilities(ctx context.Context, cfg ProviderConfig) ModelCapabilities {
	var caps ModelCapabilities
	if cfg.CapabilitiesJSON != "" {
		json.Unmarshal([]byte(cfg.CapabilitiesJSON), &caps)
	}
	return caps
}

func (a *OpenAIResponsesAdapter) Generate(ctx context.Context, cfg ProviderConfig, req ModelRequest) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	requestBody := map[string]interface{}{
		"model":            cfg.ModelName,
		"input":            a.buildInput(req),
		"tools":            a.buildTools(req.Tools),
		"max_output_tokens": cfg.MaxOutputTokens,
		"store":            false,
		"stream":           false,
	}

	if len(req.Instructions) > 0 {
		requestBody["instructions"] = strings.Join(req.Instructions, "\n\n")
	}

	if cfg.Temperature > 0 {
		requestBody["temperature"] = cfg.Temperature
	}

	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["top_p"] = cfg.TopP
	}

	if req.ResponseFormat.Type != "" {
		requestBody["text"] = map[string]interface{}{
			"format": map[string]interface{}{
				"type": req.ResponseFormat.Type,
			},
		}
	}

	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/responses"
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

func (a *OpenAIResponsesAdapter) Stream(ctx context.Context, cfg ProviderConfig, req ModelRequest, sink ModelEventSink) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")

	requestBody := map[string]interface{}{
		"model":            cfg.ModelName,
		"input":            a.buildInput(req),
		"tools":            a.buildTools(req.Tools),
		"max_output_tokens": cfg.MaxOutputTokens,
		"store":            false,
		"stream":           true,
	}

	if len(req.Instructions) > 0 {
		requestBody["instructions"] = strings.Join(req.Instructions, "\n\n")
	}

	if cfg.Temperature > 0 {
		requestBody["temperature"] = cfg.Temperature
	}

	if cfg.TopP > 0 && cfg.TopP < 1 {
		requestBody["top_p"] = cfg.TopP
	}

	jsonBody, _ := json.Marshal(requestBody)
	url := baseURL + "/responses"
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

	return a.parseStream(resp.Body, sink)
}

func (a *OpenAIResponsesAdapter) buildInput(req ModelRequest) []map[string]interface{} {
	var items []map[string]interface{}

	for _, msg := range req.Messages {
		item := map[string]interface{}{
			"role": msg.Role,
		}

		var content []map[string]interface{}
		for _, part := range msg.Parts {
			switch part.Type {
			case ContentTypeText:
				content = append(content, map[string]interface{}{
					"type": "input_text",
					"text": part.Text,
				})
			case ContentTypeImage:
				content = append(content, map[string]interface{}{
					"type": "input_image",
					"image_url": part.ResourceURI,
					"detail":   part.Detail,
				})
			case ContentTypeFile:
				content = append(content, map[string]interface{}{
					"type":     "input_file",
					"filename": part.Filename,
					"file_data": part.ResourceURI,
				})
			}
		}

		item["content"] = content
		items = append(items, item)
	}

	for _, tr := range req.ToolResults {
		items = append(items, map[string]interface{}{
			"type":          "function_call_output",
			"call_id":       tr.CallID,
			"output":        tr.Output,
		})
	}

	return items
}

func (a *OpenAIResponsesAdapter) buildTools(tools []ModelToolDefinition) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"type":        "function",
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
			"strict":      t.Strict,
		})
	}
	return result
}

func (a *OpenAIResponsesAdapter) parseResponse(respBytes []byte) (*ModelResult, error) {
	var result struct {
		ID     string `json:"id"`
		Output []struct {
			Type            string `json:"type"`
			Status          string `json:"status"`
			Role            string `json:"role"`
			Content         []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			CallID  string `json:"call_id"`
			Name    string `json:"name"`
			Arguments string `json:"arguments"`
			Summary string `json:"summary"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	res := &ModelResult{}

	for _, item := range result.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					res.Text += c.Text
				} else if c.Type == "refusal" {
					res.Refusal = c.Text
				}
			}
		case "function_call":
			res.ToolCalls = append(res.ToolCalls, ModelToolCall{
				ID:            item.CallID,
				Name:          item.Name,
				ArgumentsJSON: item.Arguments,
			})
		case "reasoning":
			if res.Continuation == nil {
				res.Continuation = &ModelContinuationState{
					Protocol: string(ProtocolOpenAIResponses),
				}
			}
			if item.Summary != "" {
				res.Continuation.OpaqueItems = append(res.Continuation.OpaqueItems, json.RawMessage(`"`+item.Summary+`"`))
			}
		}
	}

	res.Usage = ModelUsage{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		TotalTokens:  result.Usage.TotalTokens,
	}
	res.ProviderResponseID = result.ID

	return res, nil
}

func (a *OpenAIResponsesAdapter) parseStream(body io.Reader, sink ModelEventSink) (*ModelResult, error) {
	result := &ModelResult{
		ToolCalls: []ModelToolCall{},
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
					sink.Emit(context.Background(), ModelEvent{
						Type: ModelEventCompleted,
					})
					return result, nil
				}

				var event struct {
					Type          string `json:"type"`
					SequenceNumber int   `json:"sequence_number"`
					Delta         string `json:"delta"`
					OutputIndex   int    `json:"output_index"`
					ContentIndex  int    `json:"content_index"`
					ItemID        string `json:"item_id"`
					Name          string `json:"name"`
					Arguments     string `json:"arguments"`
					Status        string `json:"status"`
					Text          string `json:"text"`
					Summary       string `json:"summary"`
				}

				if err := json.Unmarshal([]byte(content), &event); err != nil {
					continue
				}

				switch event.Type {
				case "response.output_text.delta":
					result.Text += event.Delta
					sink.Emit(context.Background(), ModelEvent{
						Type:      ModelEventTextDelta,
						TextDelta: event.Delta,
					})
				case "response.output_text.done":
					sink.Emit(context.Background(), ModelEvent{
						Type: ModelEventTextDone,
					})
				case "response.function_call_arguments.delta":
					sink.Emit(context.Background(), ModelEvent{
						Type:           ModelEventToolCallArgumentsDelta,
						ToolCallID:     event.ItemID,
						ArgumentsDelta: event.Delta,
					})
				case "response.function_call_arguments.done":
					result.ToolCalls = append(result.ToolCalls, ModelToolCall{
						ID:            event.ItemID,
						Name:          event.Name,
						ArgumentsJSON: event.Arguments,
					})
				case "response.reasoning_summary_text.delta":
					sink.Emit(context.Background(), ModelEvent{
						Type:      ModelEventReasoningSummaryDelta,
						TextDelta: event.Delta,
					})
				case "response.completed":
					sink.Emit(context.Background(), ModelEvent{
						Type: ModelEventCompleted,
					})
					return result, nil
				case "response.failed":
					sink.Emit(context.Background(), ModelEvent{
						Type: ModelEventFailed,
						Error: &ModelError{
							Code:     "MODEL_PROVIDER_FAILED",
							Protocol: ProtocolOpenAIResponses,
							Message:  event.Status,
						},
					})
					return result, fmt.Errorf("response failed: %s", event.Status)
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
