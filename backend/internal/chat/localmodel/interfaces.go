// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package localmodel

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
)

type LocalModelCapabilities struct {
	Text         bool     `json:"text"`
	Vision       bool     `json:"vision"`
	AudioInput   bool     `json:"audioInput"`
	VideoInput   bool     `json:"videoInput"`
	ToolCalling  bool     `json:"toolCalling"`
	JSONMode     bool     `json:"jsonMode"`
	Reasoning    bool     `json:"reasoning"`
	Streaming    bool     `json:"streaming"`
	MaxContext   int      `json:"maxContextTokens"`
	Backends     []string `json:"backends"`
}

type LocalModelRequest struct {
	Messages    []LocalModelMessage `json:"messages"`
	Tools       []LocalModelTool    `json:"tools"`
	MaxNewTokens int                `json:"maxNewTokens"`
	Temperature float64             `json:"temperature"`
	TopP        float64             `json:"topP"`
	JSONOnly    bool                `json:"jsonOnly"`
	RequestID   string              `json:"requestId"`
}

type LocalModelMessage struct {
	Role  string              `json:"role"`
	Parts []LocalModelContent `json:"parts"`
}

type LocalModelContent struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	ResourceURI string `json:"resourceUri,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

type LocalModelTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type LocalModelResult struct {
	Text          string                `json:"text"`
	ToolCalls     []LocalModelToolCall  `json:"toolCalls"`
	Usage         LocalModelUsage       `json:"usage"`
	FinishReason  string                `json:"finishReason"`
}

type LocalModelToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LocalModelUsage struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	PrefillMS        int64   `json:"prefillMs"`
	DecodeMS         int64   `json:"decodeMs"`
	TokensPerSecond  float64 `json:"tokensPerSecond"`
}

type LocalModelStreamSink interface {
	OnTextDelta(text string) error
	OnReasoningDelta(text string) error
	OnToolCallDelta(callID string, name string, arguments string) error
	OnUsage(usage LocalModelUsage) error
}

type LocalModelHealth struct {
	State     string `json:"state"`
	LastError string `json:"lastError,omitempty"`
	LoadedAt  string `json:"loadedAt,omitempty"`
}

type LocalModelInfo struct {
	Backend      string `json:"backend"`
	Threads      int    `json:"threads"`
	Precision    string `json:"precision"`
	Memory       string `json:"memory"`
	UseMMap      bool   `json:"useMMap"`
	KVCacheMMap  bool   `json:"kvCacheMmap"`
}

type LocalModelInference interface {
	Capabilities(ctx context.Context) (LocalModelCapabilities, error)
	Generate(ctx context.Context, request LocalModelRequest, sink LocalModelStreamSink) (LocalModelResult, error)
	Load(ctx context.Context) error
	Unload(ctx context.Context) error
	Health(ctx context.Context) LocalModelHealth
}

type GenConfig struct {
	MaxNewTokens int
	Temperature  float64
	TopP         float64
}

type CreateLocalModelParams struct {
	Provider     string
	ModelName    string
	ProviderJSON string
	Timeout      time.Duration
}

type CreateInference func(params CreateLocalModelParams) (LocalModelInference, error)

var registry = make(map[string]CreateInference)

func Register(provider string, factory CreateInference) {
	registry[provider] = factory
}

func Create(params CreateLocalModelParams) (LocalModelInference, error) {
	factory, ok := registry[params.Provider]
	if !ok {
		return nil, ErrLocalProviderNotFound
	}
	backend, err := factory(params)
	if err != nil {
		return nil, err
	}
	return attachHost(backend), nil
}

func attachHost(backend LocalModelInference) LocalModelInference {
	host := GetGlobalRuntimeHost()
	if host == nil {
		return backend
	}
	type hostAttacher interface {
		AttachHost(host runtimehost.RuntimeHost)
	}
	if a, ok := backend.(hostAttacher); ok {
		a.AttachHost(host)
	}
	return backend
}
