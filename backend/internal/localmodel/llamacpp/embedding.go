// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/u-ai/backend/internal/localmodel/gguf"
)

type llamaCppEmbeddingBackend struct {
	config    LlamaCppEmbeddingConfig
	mu        sync.Mutex
	state     string
	loadedAt  string
	lastError string
	manifest  *gguf.GGUFModelManifest
}

type EmbeddingResult struct {
	Vectors           [][]float32
	Dimension         int
	Normalized        bool
	Pooling           string
	ModelFingerprint  string
	Truncated         []bool
	TokenCounts       []int
}

type EmbeddingCapabilities struct {
	SupportsEmbedding bool
	EmbeddingDimension int
	PoolingType        string
	SupportsTruncate   bool
}

func NewLlamaCppEmbeddingBackend(config LlamaCppEmbeddingConfig) (*llamaCppEmbeddingBackend, error) {
	return &llamaCppEmbeddingBackend{
		config: config,
		state:  "unloaded",
	}, nil
}

func (b *llamaCppEmbeddingBackend) Capabilities(ctx context.Context) (EmbeddingCapabilities, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	caps := EmbeddingCapabilities{
		SupportsEmbedding: true,
		EmbeddingDimension: 0,
		PoolingType:        b.config.Pooling,
		SupportsTruncate:   b.config.Truncate,
	}

	if b.manifest != nil {
		caps.EmbeddingDimension = b.manifest.EmbeddingLength
		caps.SupportsEmbedding = b.manifest.SupportsEmbedding
		if b.manifest.PoolingType != "" {
			caps.PoolingType = b.manifest.PoolingType
		}
	}

	return caps, nil
}

func (b *llamaCppEmbeddingBackend) Embed(ctx context.Context, inputs []string, purpose string) (*EmbeddingResult, error) {
	b.mu.Lock()
	if b.state != "ready" {
		b.mu.Unlock()
		return nil, fmt.Errorf("embedding model not loaded")
	}
	b.state = "embedding"
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		b.state = "ready"
		b.mu.Unlock()
	}()

	dim := b.getEmbeddingDimension()
	if dim <= 0 {
		dim = 1024
	}

	result := &EmbeddingResult{
		Vectors:    make([][]float32, len(inputs)),
		Dimension:  dim,
		Normalized: b.config.Normalize != "none",
		Pooling:    b.config.Pooling,
		Truncated:  make([]bool, len(inputs)),
		TokenCounts: make([]int, len(inputs)),
	}

	for i, text := range inputs {
		prefix := b.config.DocumentPrefix
		if purpose == "query" {
			prefix = b.config.QueryPrefix
		}
		fullText := prefix + text
		result.Vectors[i] = generateStubEmbedding(fullText, dim)
		result.TokenCounts[i] = len([]rune(text))
	}

	result.ModelFingerprint = b.getFingerprint()
	return result, nil
}

func (b *llamaCppEmbeddingBackend) Load(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == "ready" || b.state == "embedding" {
		return nil
	}

	if b.config.ResourceURI != "" {
		inspector := gguf.NewInspector()
		manifest, err := inspector.Inspect(b.config.ResourceURI)
		if err != nil {
			b.lastError = err.Error()
			return fmt.Errorf("load embedding model failed: %w", err)
		}
		b.manifest = manifest
	}

	b.state = "loading"
	b.loadedAt = "now"
	b.state = "ready"
	return nil
}

func (b *llamaCppEmbeddingBackend) Unload(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = "unloaded"
	b.loadedAt = ""
	b.manifest = nil
	return nil
}

func (b *llamaCppEmbeddingBackend) Health(ctx context.Context) (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.state, b.lastError
}

func (b *llamaCppEmbeddingBackend) getEmbeddingDimension() int {
	if b.manifest != nil && b.manifest.EmbeddingLength > 0 {
		return b.manifest.EmbeddingLength
	}
	return 0
}

func (b *llamaCppEmbeddingBackend) getFingerprint() string {
	if b.manifest == nil {
		return ""
	}
	return gguf.ComputeEmbeddingFingerprint(b.manifest)
}

func generateStubEmbedding(text string, dim int) []float32 {
	vector := make([]float32, dim)
	seed := uint64(0)
	for _, c := range text {
		seed = seed*31 + uint64(c)
	}
	for i := 0; i < dim; i++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		vector[i] = float32(seed>>33) / float32(1<<31)
	}
	normalizeVectorL2(vector)
	return vector
}

func normalizeVectorL2(vector []float32) {
	var sum float64
	for _, v := range vector {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if norm < 1e-9 {
		return
	}
	for i := range vector {
		vector[i] = float32(float64(vector[i]) / norm)
	}
}
