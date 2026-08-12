// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package modelprotocol

import (
	"context"
	"encoding/json"
	"time"
)

type ModelCapabilities struct {
	SupportsText        bool `json:"supportsText"`
	SupportsImage       bool `json:"supportsImage"`
	SupportsFile        bool `json:"supportsFile"`
	SupportsAudio       bool `json:"supportsAudio"`
	SupportsVideo       bool `json:"supportsVideo"`
	SupportsToolUse     bool `json:"supportsToolUse"`
	SupportsStreaming   bool `json:"supportsStreaming"`
	SupportsReasoning   bool `json:"supportsReasoning"`
	SupportsStructured  bool `json:"supportsStructured"`
	MaxContextWindow    int  `json:"maxContextWindow"`
}

type ModelProtocol string

const (
	ProtocolOpenAIChat        ModelProtocol = "openai_chat"
	ProtocolOpenAIResponses   ModelProtocol = "openai_responses"
	ProtocolAnthropicMessages ModelProtocol = "anthropic_messages"
	ProtocolGeminiGenerate    ModelProtocol = "gemini_generate_content"
	ProtocolOllamaChat        ModelProtocol = "ollama_chat"
)

type ModelContentType string

const (
	ContentTypeText  ModelContentType = "text"
	ContentTypeImage ModelContentType = "image"
	ContentTypeFile  ModelContentType = "file"
	ContentTypeAudio ModelContentType = "audio"
	ContentTypeVideo ModelContentType = "video"
)

type ModelContentPart struct {
	Type        ModelContentType `json:"type"`
	Text        string           `json:"text,omitempty"`
	ResourceURI string           `json:"resourceUri,omitempty"`
	MIMEType    string           `json:"mimeType,omitempty"`
	Filename    string           `json:"filename,omitempty"`
	Detail      string           `json:"detail,omitempty"`
}

type ModelMessage struct {
	Role  string             `json:"role"`
	Parts []ModelContentPart `json:"parts"`
}

type ModelToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict"`
}

type ModelToolCall struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArgumentsJSON string `json:"argumentsJson"`
}

type ModelToolResult struct {
	CallID  string `json:"callId"`
	Name    string `json:"name"`
	Output  string `json:"output"`
	IsError bool   `json:"isError"`
}

type ModelResponseFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Schema map[string]any `json:"schema"`
	Strict bool           `json:"strict"`
}

type ModelContinuationState struct {
	Protocol           string            `json:"protocol"`
	OpaqueItems        []json.RawMessage `json:"opaqueItems"`
	ProviderResponseID string            `json:"providerResponseID"`
	ExpiresAt          *time.Time        `json:"expiresAt,omitempty"`
}

type ModelRequest struct {
	Model          string                 `json:"model"`
	Instructions   []string               `json:"instructions"`
	Messages       []ModelMessage         `json:"messages"`
	Tools          []ModelToolDefinition  `json:"tools"`
	ToolResults    []ModelToolResult      `json:"toolResults"`
	ResponseFormat ModelResponseFormat    `json:"responseFormat"`
	Temperature    *float64               `json:"temperature,omitempty"`
	TopP           *float64               `json:"topP,omitempty"`
	MaxOutputTokens int                    `json:"maxOutputTokens"`
	Stream         bool                   `json:"stream"`
	Continuation   *ModelContinuationState `json:"continuation,omitempty"`
}

type ModelUsage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	ReasoningTokens  int `json:"reasoningTokens"`
	CachedInputTokens int `json:"cachedInputTokens"`
	TotalTokens      int `json:"totalTokens"`
}

type ModelError struct {
	Code         string       `json:"code"`
	Provider     string       `json:"provider"`
	Protocol     ModelProtocol `json:"protocol"`
	HTTPStatus   int          `json:"httpStatus"`
	Message      string       `json:"message"`
	Retryable    bool         `json:"retryable"`
	AuthRelated  bool         `json:"authRelated"`
	RateLimited  bool         `json:"rateLimited"`
	Timeout      bool         `json:"timeout"`
	RequestID    string       `json:"requestId"`
}

type ModelResult struct {
	Text               string                 `json:"text"`
	Refusal            string                 `json:"refusal"`
	ToolCalls          []ModelToolCall        `json:"toolCalls"`
	Usage              ModelUsage             `json:"usage"`
	Continuation       *ModelContinuationState `json:"continuation"`
	FinishReason       string                 `json:"finishReason"`
	ProviderResponseID string                 `json:"providerResponseID"`
}

type ModelEventType string

const (
	ModelEventResponseStarted        ModelEventType = "response_started"
	ModelEventTextDelta              ModelEventType = "text_delta"
	ModelEventTextDone               ModelEventType = "text_done"
	ModelEventRefusalDelta           ModelEventType = "refusal_delta"
	ModelEventRefusalDone            ModelEventType = "refusal_done"
	ModelEventToolCallStarted        ModelEventType = "tool_call_started"
	ModelEventToolCallArgumentsDelta ModelEventType = "tool_call_arguments_delta"
	ModelEventToolCallDone           ModelEventType = "tool_call_done"
	ModelEventReasoningSummaryDelta  ModelEventType = "reasoning_summary_delta"
	ModelEventReasoningSummaryDone   ModelEventType = "reasoning_summary_done"
	ModelEventUsage                  ModelEventType = "usage"
	ModelEventCompleted              ModelEventType = "completed"
	ModelEventFailed                 ModelEventType = "failed"
	ModelEventCancelled              ModelEventType = "cancelled"
)

type ModelEvent struct {
	Type           ModelEventType `json:"type"`
	TextDelta      string         `json:"textDelta,omitempty"`
	ToolCallID     string         `json:"toolCallID,omitempty"`
	ToolName       string         `json:"toolName,omitempty"`
	ArgumentsDelta string         `json:"argumentsDelta,omitempty"`
	Usage          *ModelUsage    `json:"usage,omitempty"`
	Error          *ModelError    `json:"error,omitempty"`
}

type ModelEventSink interface {
	Emit(ctx context.Context, event ModelEvent) error
}

type ModelAdapter interface {
	Protocol() ModelProtocol
	Capabilities(ctx context.Context, cfg ProviderConfig) ModelCapabilities
	Generate(ctx context.Context, cfg ProviderConfig, req ModelRequest) (*ModelResult, error)
	Stream(ctx context.Context, cfg ProviderConfig, req ModelRequest, sink ModelEventSink) (*ModelResult, error)
}

func AdapterForProtocol(protocol ModelProtocol) ModelAdapter {
	switch protocol {
	case ProtocolOpenAIChat:
		return &OpenAIChatAdapter{}
	case ProtocolOpenAIResponses:
		return &OpenAIResponsesAdapter{}
	case ProtocolAnthropicMessages:
		return &AnthropicAdapter{}
	case ProtocolGeminiGenerate:
		return &GeminiAdapter{}
	case ProtocolOllamaChat:
		return &OllamaAdapter{}
	default:
		return &OpenAIChatAdapter{}
	}
}
