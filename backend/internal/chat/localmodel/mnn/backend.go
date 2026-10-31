// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mnn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/runtimehost"
)

type mnnBackend struct {
	config     MNNProviderConfig
	mu         sync.Mutex
	state      string
	loadedAt   string
	lastError  string
	runtime    *mnnRuntime
	host       runtimehost.RuntimeHost
	generateMu sync.Mutex
}

func NewMNNBackend(params localmodel.CreateLocalModelParams) (localmodel.LocalModelInference, error) {
	cfg, err := ParseProviderConfig(params.ProviderJSON)
	if err != nil {
		return nil, localmodel.ErrConfigInvalid
	}
	if cfg.LocalModelID == "" {
		cfg.LocalModelID = params.ModelName
	}
	if cfg.LocalModelID == "" {
		return nil, localmodel.ErrConfigInvalid
	}
	return newMNnBackend(cfg), nil
}

func newMNnBackend(cfg MNNProviderConfig) *mnnBackend {
	b := &mnnBackend{
		config: cfg,
		state:  "unloaded",
	}
	b.runtime = newMNNRuntime(cfg)
	return b
}

func (b *mnnBackend) AttachHost(host runtimehost.RuntimeHost) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.host = host
	b.runtime.attachHost(host)
}

func (b *mnnBackend) Capabilities(ctx context.Context) (localmodel.LocalModelCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	caps := localmodel.LocalModelCapabilities{
		Text:        true,
		Vision:      b.config.Multimodal,
		ToolCalling: false,
		JSONMode:    false,
		Streaming:   true,
		MaxContext:  b.config.ContextSize,
		Backends:    []string{b.config.Backend},
	}

	if b.runtime.baseURL != "" {
		caps.Streaming = true
	}

	return caps, nil
}

func (b *mnnBackend) Generate(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
	b.generateMu.Lock()
	defer b.generateMu.Unlock()

	b.mu.Lock()
	if b.state != "ready" {
		b.mu.Unlock()
		return localmodel.LocalModelResult{}, localmodel.ErrLoadFailed
	}
	b.mu.Unlock()

	return b.runtime.chatRequest(ctx, request, sink)
}

func (b *mnnBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "loading" || b.state == "generating" {
		return nil
	}

	b.state = "loading"
	b.lastError = ""

	if err := b.runtime.validateModelArtifact(); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return fmt.Errorf("%w: %v", localmodel.ErrLoadFailed, err)
	}

	if b.host == nil {
		b.lastError = localmodel.ErrNativeBridgeUnavailable.Error()
		b.state = "failed"
		return fmt.Errorf("%w: runtime host unavailable", localmodel.ErrLoadFailed)
	}

	if err := b.runtime.startServer(ctx); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return fmt.Errorf("%w: %v", localmodel.ErrLoadFailed, err)
	}

	b.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	b.state = "ready"
	return nil
}

func (b *mnnBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.runtime.stopServer()
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

func (b *mnnBackend) shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runtime.stopServer()
	b.state = "unloaded"
}

func init() {
	if err := localmodel.RegisterWithDescriptor("mnn", NewMNNBackend, localmodel.ProviderDescriptor{
		ID:                 "mnn",
		Name:               "MNN (本地)",
		Protocol:           "mnn",
		DefaultProtocol:    "mnn",
		SupportedProtocols: []string{"mnn"},
		RuntimeKind:        "embedded",
		ModelFormats:       []string{"mnn"},
		DocsURL:            "https://github.com/alibaba/MNN",
		ConfigFields: []localmodel.ProviderField{
			{Key: "modelResourceUri", Label: "MNN 模型", Type: "modelFile", Required: true},
			{Key: "localModelId", Label: "模型标识", Type: "text", Placeholder: "留空使用配置名称"},
			{Key: "threadNum", Label: "CPU 线程", Type: "number", Default: 4},
			{Key: "contextSize", Label: "上下文长度", Type: "number", Default: 65536},
			{Key: "precision", Label: "精度", Type: "select", Options: []localmodel.ProviderFieldOption{{Label: "低", Value: "low"}, {Label: "普通", Value: "normal"}, {Label: "高", Value: "high"}}},
			{Key: "memory", Label: "内存模式", Type: "select", Options: []localmodel.ProviderFieldOption{{Label: "普通", Value: "normal"}, {Label: "低内存", Value: "low"}}},
			{Key: "useMmap", Label: "内存映射", Type: "boolean", Default: true},
			{Key: "kvCacheMmap", Label: "KV Cache 映射", Type: "boolean", Default: false},
			{Key: "multimodal", Label: "多模态", Type: "boolean", Default: false},
		},
		Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "json": true, "vision": true, "local": true},
		Availability: func() (bool, string) {
			if path := mnnServerExecutable(); path == "" {
				return false, "mnn-server 运行器未安装"
			}
			return true, ""
		},
	}); err != nil {
		panic(err)
	}
}
