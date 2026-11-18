package kernel

import (
	"context"
	"fmt"
	"sync"
)

type ModelInvokeToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ModelInvokeTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ModelInvokeMessage struct {
	Role       string                `json:"role"`
	Content    string                `json:"content"`
	ToolCalls  []ModelInvokeToolCall `json:"toolCalls,omitempty"`
	ToolCallID string                `json:"toolCallId,omitempty"`
}

type ModelInvokeRequest struct {
	Provider        string               `json:"provider"`
	Model           string               `json:"model"`
	System          string               `json:"system"`
	Messages        []ModelInvokeMessage `json:"messages"`
	Tools           []ModelInvokeTool    `json:"tools"`
	Temperature     *float64             `json:"temperature"`
	MaxOutputTokens int                  `json:"maxOutputTokens"`
	ReasoningEffort string               `json:"reasoningEffort"`
	SessionID       string               `json:"sessionId"`
	Purpose         string               `json:"purpose"`
}

type ModelInvokeUsage struct {
	Reported           bool `json:"reported"`
	InputTokens        int  `json:"inputTokens"`
	OutputTokens       int  `json:"outputTokens"`
	ReasoningTokens    int  `json:"reasoningTokens"`
	CachedInputTokens  int  `json:"cachedInputTokens"`
	CacheUsageReported bool `json:"cacheUsageReported"`
	TotalTokens        int  `json:"totalTokens"`
}

type ModelInvokeResult struct {
	Provider           string                `json:"provider"`
	Model              string                `json:"model"`
	Protocol           string                `json:"protocol"`
	ContextWindow      int                   `json:"contextWindow,omitempty"`
	Text               string                `json:"text"`
	Reasoning          string                `json:"reasoning"`
	Refusal            string                `json:"refusal"`
	ToolCalls          []ModelInvokeToolCall `json:"toolCalls"`
	FinishReason       string                `json:"finishReason"`
	Usage              ModelInvokeUsage      `json:"usage"`
	ProviderResponseID string                `json:"providerResponseID"`
}

type ModelGatewayPort interface {
	InvokeModelGateway(ctx context.Context, request ModelInvokeRequest) (*ModelInvokeResult, error)
}

type ModelGatewayBridge struct {
	mu   sync.RWMutex
	port ModelGatewayPort
}

func NewModelGatewayBridge(port ModelGatewayPort) *ModelGatewayBridge {
	return &ModelGatewayBridge{port: port}
}

func (b *ModelGatewayBridge) SetPort(port ModelGatewayPort) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.port = port
	b.mu.Unlock()
}

func (b *ModelGatewayBridge) Invoke(ctx context.Context, request ModelInvokeRequest) (*ModelInvokeResult, error) {
	if b == nil {
		return nil, fmt.Errorf("model gateway is unavailable")
	}
	b.mu.RLock()
	port := b.port
	b.mu.RUnlock()
	if port == nil {
		return nil, fmt.Errorf("model gateway is unavailable")
	}
	return port.InvokeModelGateway(ctx, request)
}
