package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
)

type ChatLogsProvider interface {
	ExecuteChatLogs(context.Context, host_api.CallRequest, host_api.Method, string, json.RawMessage) (any, error)
}

type ChatLogsBridge struct {
	mu       sync.RWMutex
	provider ChatLogsProvider
}

func NewChatLogsBridge() *ChatLogsBridge {
	return &ChatLogsBridge{}
}

func (b *ChatLogsBridge) SetProvider(provider ChatLogsProvider) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.provider = provider
	b.mu.Unlock()
}

func (b *ChatLogsBridge) Execute(
	ctx context.Context,
	request host_api.CallRequest,
	method host_api.Method,
	operation string,
	input json.RawMessage,
) (any, error) {
	if b == nil {
		return nil, fmt.Errorf("conversation archive service is unavailable")
	}
	b.mu.RLock()
	provider := b.provider
	b.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("conversation archive service is unavailable")
	}
	return provider.ExecuteChatLogs(ctx, request, method, operation, input)
}
