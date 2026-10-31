// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/chat/localmodel"
	"github.com/u-ai/backend/internal/localmodel/gguf"
	"github.com/u-ai/backend/internal/runtimehost"
)

type llamaCppBackend struct {
	config     LlamaCppProviderConfig
	mu         sync.Mutex
	state      string
	loadedAt   string
	lastError  string
	manifest   atomic.Pointer[gguf.GGUFModelManifest]
	runtime    *llamaRuntime
	host       runtimehost.RuntimeHost
	generateMu sync.Mutex
}

func NewLlamaCppBackend(params localmodel.CreateLocalModelParams) (localmodel.LocalModelInference, error) {
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
	return newLlamaCppBackend(cfg), nil
}

func newLlamaCppBackend(cfg LlamaCppProviderConfig) *llamaCppBackend {
	b := &llamaCppBackend{
		config: cfg,
		state:  "unloaded",
	}
	b.runtime = newLlamaRuntime(cfg)
	return b
}

func (b *llamaCppBackend) AttachHost(host runtimehost.RuntimeHost) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.host = host
	b.runtime.attachHost(host)
}

func (b *llamaCppBackend) Capabilities(ctx context.Context) (localmodel.LocalModelCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	caps := localmodel.LocalModelCapabilities{
		Text:        true,
		Streaming:   true,
		MaxContext:  b.config.ContextSize,
		Backends:    []string{b.config.Backend},
		ToolCalling: false,
		JSONMode:    false,
	}

	if manifest := b.manifest.Load(); manifest != nil {
		caps.MaxContext = manifest.ContextLength
		for _, c := range manifest.Capabilities {
			switch strings.ToLower(c) {
			case "tool_calling":
				caps.ToolCalling = true
			case "json_mode", "json":
				caps.JSONMode = true
			case "vision", "multimodal":
				caps.Vision = true
			}
		}
	}

	return caps, nil
}

func (b *llamaCppBackend) Generate(ctx context.Context, request localmodel.LocalModelRequest, sink localmodel.LocalModelStreamSink) (localmodel.LocalModelResult, error) {
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

func (b *llamaCppBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "loading" {
		return nil
	}

	b.state = "loading"
	b.lastError = ""

	if err := b.runtime.validateModelArtifact(); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return wrapError(localmodel.ErrLoadFailed, err)
	}

	manifest, err := b.runtime.inspectModel()
	if err == nil {
		b.manifest.Store(manifest)
	}

	if b.host == nil {
		b.lastError = localmodel.ErrNativeBridgeUnavailable.Error()
		b.state = "failed"
		return wrapError(localmodel.ErrLoadFailed, fmt.Errorf("runtime host unavailable"))
	}

	if err := b.runtime.startServer(ctx); err != nil {
		b.lastError = err.Error()
		b.state = "failed"
		return wrapError(localmodel.ErrLoadFailed, err)
	}

	b.loadedAt = time.Now().Format("2006-01-02 15:04:05")
	b.state = "ready"
	return nil
}

func (b *llamaCppBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.runtime.stopServer()
	b.manifest.Store(nil)
	b.state = "unloaded"
	b.loadedAt = ""
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

func wrapError(base error, err error) error {
	if base == nil {
		return err
	}
	return fmt.Errorf("%w: %v", base, err)
}

func (b *llamaCppBackend) shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runtime.stopServer()
	b.manifest.Store(nil)
	b.state = "unloaded"
}

func init() {
	if err := localmodel.RegisterWithDescriptor("llama_cpp", NewLlamaCppBackend, localmodel.ProviderDescriptor{
		ID:                 "llama_cpp",
		Name:               "llama.cpp (本地 GGUF)",
		Protocol:           "llama_cpp",
		DefaultProtocol:    "llama_cpp",
		SupportedProtocols: []string{"llama_cpp"},
		RuntimeKind:        "embedded",
		ModelFormats:       []string{"gguf"},
		DocsURL:            "https://github.com/ggerganov/llama.cpp",
		ConfigFields: []localmodel.ProviderField{
			{Key: "resourceUri", Label: "GGUF 模型", Type: "modelFile", Required: true},
			{Key: "localModelId", Label: "模型标识", Type: "text", Placeholder: "留空使用配置名称"},
			{Key: "contextSize", Label: "上下文长度", Type: "number", Default: 65536},
			{Key: "threads", Label: "CPU 线程", Type: "number", Default: 4},
			{Key: "gpuLayers", Label: "GPU 层数", Type: "number", Default: 0},
			{Key: "batchSize", Label: "批大小", Type: "number", Default: 512},
			{Key: "mmprojUri", Label: "多模态投影模型", Type: "modelFile", Required: false},
			{Key: "temperature", Label: "温度", Type: "number", Default: 0.7},
			{Key: "topP", Label: "Top P", Type: "number", Default: 1},
			{Key: "topK", Label: "Top K", Type: "number", Default: 40},
		},
		Capabilities: map[string]bool{"chat": true, "stream": true, "tools": true, "json": true, "vision": true, "local": true},
		Availability: func() (bool, string) {
			if path := llamaServerExecutable(""); path == "" {
				return false, "llama-server 运行器未安装"
			}
			return true, ""
		},
	}); err != nil {
		panic(err)
	}
}
