// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/localmodel/gguf"
)

type llamaCppBackend struct {
	config    LlamaCppProviderConfig
	mu        sync.Mutex
	state     string
	loadedAt  string
	lastError string
	manifest  *gguf.GGUFModelManifest
}

func NewLlamaCppBackend(params localmodel.CreateLocalModelParams) (localmodel.LocalModelInference, error) {
	cfg, err := ParseProviderConfig(params.ProviderJSON)
	if err != nil {
		return nil, localmodel.ErrConfigInvalid
	}
	return &llamaCppBackend{
		config: cfg,
		state:  "unloaded",
	}, nil
}

func (b *llamaCppBackend) Capabilities(ctx context.Context) (localmodel.LocalModelCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	caps := localmodel.LocalModelCapabilities{
		Text:        true,
		Streaming:   true,
		MaxContext:  b.config.ContextSize,
		Backends:    []string{b.config.Backend},
	}

	if b.manifest != nil {
		caps.Vision = containsString(b.manifest.Capabilities, "vision")
		caps.ToolCalling = containsString(b.manifest.Capabilities, "tool_calling")
		caps.JSONMode = true
		caps.MaxContext = b.manifest.ContextLength
	}

	return caps, nil
}

func (b *llamaCppBackend) Generate(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	b.mu.Lock()
	if b.state != "ready" {
		b.mu.Unlock()
		return localmodel.LocalModelResult{}, localmodel.ErrLoadFailed
	}
	b.state = "generating"
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.state = "ready"
		b.mu.Unlock()
	}()

	if sink != nil {
		sink.OnTextDelta("")
	}

	return localmodel.LocalModelResult{
		Text:         "",
		FinishReason: "stop",
		Usage:        localmodel.LocalModelUsage{},
	}, nil
}

func (b *llamaCppBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "generating" {
		return nil
	}

	if b.config.ResourceURI != "" {
		inspector := gguf.NewInspector()
		manifest, err := inspector.Inspect(b.config.ResourceURI)
		if err != nil {
			b.lastError = err.Error()
			return localmodel.ErrLoadFailed
		}
		b.manifest = manifest
	}

	b.state = "loading"
	b.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	b.state = "ready"
	return nil
}

func (b *llamaCppBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = "unloaded"
	b.loadedAt = ""
	b.manifest = nil
	return nil
}

func (b *llamaCppBackend) Health(ctx context.Context) localmodel.LocalModelHealth {
	b.mu.Lock()
	defer b.mu.Unlock()

	return localmodel.LocalModelHealth{
		State:     b.state,
		LastError: b.lastError,
		LoadedAt:  b.loadedAt,
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func init() {
	localmodel.Register("llama_cpp", NewLlamaCppBackend)
}
