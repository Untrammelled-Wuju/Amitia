// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"context"
	"fmt"
	"sync"
)

type localEmbeddingProvider interface {
	EmbedSingle(text string, purpose string) ([]float32, error)
	EmbedBatch(texts []string, purpose string) ([][]float32, error)
}

var (
	localEmbeddingFactories = make(map[string]func(configJSON string) (localEmbeddingProvider, error))
	localEmbeddingRegMu      sync.RWMutex
)

func RegisterLocalEmbeddingFactory(providerType string, factory func(configJSON string) (localEmbeddingProvider, error)) {
	localEmbeddingRegMu.Lock()
	defer localEmbeddingRegMu.Unlock()
	localEmbeddingFactories[providerType] = factory
}

func GetLocalEmbeddingFactory(providerType string) (func(configJSON string) (localEmbeddingProvider, error), bool) {
	localEmbeddingRegMu.RLock()
	defer localEmbeddingRegMu.RUnlock()
	f, ok := localEmbeddingFactories[providerType]
	return f, ok
}

type llamaCppLocalEmbedding struct {
	config LlamaCppEmbeddingConfig
}

func newLlamaCppLocalEmbedding(configJSON string) (localEmbeddingProvider, error) {
	cfg := DefaultEmbeddingConfig()
	if configJSON != "" {
		parsed, err := ParseEmbeddingConfig(configJSON)
		if err != nil {
			return nil, err
		}
		cfg = parsed
	}
	return &llamaCppLocalEmbedding{config: cfg}, nil
}

func (p *llamaCppLocalEmbedding) EmbedSingle(text string, purpose string) ([]float32, error) {
	ctx := context.Background()
	backend, err := NewLlamaCppEmbeddingBackend(p.config)
	if err != nil {
		return nil, err
	}
	if err := backend.Load(ctx); err != nil {
		return nil, err
	}
	defer backend.Unload(ctx)
	result, err := backend.Embed(ctx, []string{text}, purpose)
	if err != nil {
		return nil, err
	}
	if len(result.Vectors) == 0 {
		return nil, fmt.Errorf("no embedding generated")
	}
	return result.Vectors[0], nil
}

func (p *llamaCppLocalEmbedding) EmbedBatch(texts []string, purpose string) ([][]float32, error) {
	ctx := context.Background()
	backend, err := NewLlamaCppEmbeddingBackend(p.config)
	if err != nil {
		return nil, err
	}
	if err := backend.Load(ctx); err != nil {
		return nil, err
	}
	defer backend.Unload(ctx)
	result, err := backend.Embed(ctx, texts, purpose)
	if err != nil {
		return nil, err
	}
	return result.Vectors, nil
}

func init() {
	RegisterLocalEmbeddingFactory("llama_cpp", newLlamaCppLocalEmbedding)
}
