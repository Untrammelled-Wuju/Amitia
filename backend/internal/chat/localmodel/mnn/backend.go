// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mnn

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
)

type mnnBackend struct {
	config    MNNProviderConfig
	mu        sync.Mutex
	state     string
	loadedAt  string
	lastError string
}

func NewMNNBackend(params localmodel.CreateLocalModelParams) (localmodel.LocalModelInference, error) {
	cfg, err := ParseProviderConfig(params.ProviderJSON)
	if err != nil {
		return nil, localmodel.ErrConfigInvalid
	}
	return &mnnBackend{
		config: cfg,
		state:  "unloaded",
	}, nil
}

func (b *mnnBackend) Capabilities(ctx context.Context) (localmodel.LocalModelCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return localmodel.LocalModelCapabilities{
		Text:        true,
		Vision:      b.config.Multimodal,
		ToolCalling: false,
		Streaming:   true,
		MaxContext:  4096,
		Backends:    []string{b.config.Backend},
	}, nil
}

func (b *mnnBackend) Generate(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
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

func (b *mnnBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "generating" {
		return nil
	}

	b.state = "loading"
	b.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	b.state = "ready"
	return nil
}

func (b *mnnBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = "unloaded"
	b.loadedAt = ""
	return nil
}

func (b *mnnBackend) Health(ctx context.Context) localmodel.LocalModelHealth {
	b.mu.Lock()
	defer b.mu.Unlock()

	return localmodel.LocalModelHealth{
		State:     b.state,
		LastError: b.lastError,
		LoadedAt:  b.loadedAt,
	}
}

func init() {
	localmodel.Register("mnn", NewMNNBackend)
}
