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

type GeminiAdapter struct{}

func (a *GeminiAdapter) Protocol() ModelProtocol {
	return ProtocolGeminiGenerate
}

func (a *GeminiAdapter) Capabilities(ctx context.Context, cfg *ProviderConfig) ModelCapabilities {
	var caps ModelCapabilities
	if cfg.CapabilitiesJSON != "" {
		json.Unmarshal([]byte(cfg.CapabilitiesJSON), &caps)
	}
	return caps
}

func (a *GeminiAdapter) Generate(ctx context.Context, cfg *ProviderConfig, req ModelRequest) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"contents":         a.buildContents(req),
		"generationConfig": a.buildGenConfig(cfg),
	}
	
	if len(req.Instructions) > 0 {
		requestBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": strings.Join(req.Instructions, "\n\n")},
			},
		}
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = []map[string]interface{}{
			{"functionDeclarations": a.buildTools(req.Tools)},
		}
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, cfg.ModelName)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	q := httpReq.URL.Query()
	q.Set("key", cfg.APIKey)
	httpReq.URL.RawQuery = q.Encode()
	
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

func (a *GeminiAdapter) Stream(ctx context.Context, cfg *ProviderConfig, req ModelRequest, sink ModelEventSink) (*ModelResult, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	
	requestBody := map[string]interface{}{
		"contents":         a.buildContents(req),
		"generationConfig": a.buildGenConfig(cfg),
	}
	
	if len(req.Instructions) > 0 {
		requestBody["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": strings.Join(req.Instructions, "\n\n")},
			},
		}
	}
	
	if len(req.Tools) > 0 {
		requestBody["tools"] = []map[string]interface{}{
			{"functionDeclarations": a.buildTools(req.Tools)},
		}
	}
	
	jsonBody, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, cfg.ModelName)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	
	httpReq.Header.Set("Content-Type", "application/json")
	q := httpReq.URL.Query()
	q.Set("key", cfg.APIKey)
	httpReq.URL.RawQuery = q.Encode()
	
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

func (a *GeminiAdapter) buildContents(req ModelRequest) []map[string]interface{} {
	var contents []map[string]interface{}
	
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		
		var parts []map[string]interface{}
		for _, part := range msg.Parts {
			switch part.Type {
			case ContentTypeText:
				parts = append(parts, map[string]interface{}{
					"text": part.Text,
				})
			case ContentTypeImage:
				parts = append(parts, map[string]interface{}{
					"inlineData": map[string]interface{}{
						"mimeType": part.MIMEType,
						"data":     part.ResourceURI,
					},
				})
			}
		}
		
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": parts,
		})
	}
	
	for _, tr := range req.ToolResults {
		contents = append(contents, map[string]interface{}{
			"role": "user",
			"parts": []map[string]interface{}{
				{
					"functionResponse": map[string]interface{}{
						"name":     tr.Name,
						"response": map[string]interface{}{"result": tr.Output},
					},
				},
			},
		})
	}
	
	return contents
}

func (a *GeminiAdapter) buildGenConfig(cfg *ProviderConfig) map[string]interface{} {
	config := map[string]interface{}{
		"temperature":     cfg.Temperature,
		"maxOutputTokens": cfg.MaxOutputTokens,
	}
	
	if cfg.TopP > 0 && cfg.TopP < 1 {
		config["topP"] = cfg.TopP
	}
	
	return config
}

func (a *GeminiAdapter) buildTools(tools []ModelToolDefinition) []map[string]interface{} {
	var result []map[string]interface{}
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		})
	}
	return result
}

func (a *GeminiAdapter) parseResponse(respBytes []byte) (*ModelResult, error) {
	var result struct {
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
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	
	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("API 未返回有效回复")
	}
	
	res := &ModelResult{}
	
	for _, part := range result.Candidates[0].Content.Parts {
		if part.Text != "" {
			res.Text += part.Text
		}
		if part.FuncCall.Name != "" {
			res.ToolCalls = append(res.ToolCalls, ModelToolCall{
				Name:          part.FuncCall.Name,
				ArgumentsJSON: string(part.FuncCall.Args),
			})
		}
	}
	
	res.Usage = ModelUsage{
		InputTokens:  result.UsageMetadata.PromptTokenCount,
		OutputTokens: result.UsageMetadata.CandidatesTokenCount,
		TotalTokens:  result.UsageMetadata.TotalTokenCount,
	}
	
	return res, nil
}

func (a *GeminiAdapter) parseStream(body io.Reader, sink ModelEventSink) (*ModelResult, error) {
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
				
				var chunk struct {
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
				}
				
				if err := json.Unmarshal([]byte(content), &chunk); err != nil {
					continue
				}
				
				if len(chunk.Candidates) == 0 {
					continue
				}
				
				for _, part := range chunk.Candidates[0].Content.Parts {
					if part.Text != "" {
						result.Text += part.Text
					sink.Emit(context.Background(), ModelEvent{
						Type:      ModelEventTextDelta,
							TextDelta: part.Text,
						})
					}
					if part.FuncCall.Name != "" {
						result.ToolCalls = append(result.ToolCalls, ModelToolCall{
							Name:          part.FuncCall.Name,
							ArgumentsJSON: string(part.FuncCall.Args),
						})
					}
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
	
	sink.Emit(context.Background(), ModelEvent{
		Type: ModelEventCompleted,
	})
	
	return result, nil
}
