// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package llamacpp

import "encoding/json"

type LlamaCppProviderConfig struct {
	LocalModelID    string `json:"localModelId"`
	Backend         string `json:"backend,omitempty"`
	Threads         int    `json:"threads,omitempty"`
	ThreadsBatch    int    `json:"threadsBatch,omitempty"`
	GPULayers       int    `json:"gpuLayers,omitempty"`
	ContextSize     int    `json:"contextSize,omitempty"`
	BatchSize       int    `json:"batchSize,omitempty"`
	UBatchSize      int    `json:"uBatchSize,omitempty"`
	FlashAttention  bool   `json:"flashAttention,omitempty"`
	MMap            bool   `json:"mmap,omitempty"`
	MLock           bool   `json:"mlock,omitempty"`
	KVCacheTypeK    string `json:"kvCacheTypeK,omitempty"`
	KVCacheTypeV    string `json:"kvCacheTypeV,omitempty"`
	Seed            *int64 `json:"seed,omitempty"`
	ResourceURI     string `json:"resourceUri,omitempty"`
	MMProjURI       string `json:"mmprojUri,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
	MinP            float64 `json:"minP,omitempty"`
}

type LlamaCppEmbeddingConfig struct {
	LocalModelID     string `json:"localModelId"`
	Backend          string `json:"backend,omitempty"`
	Threads          int    `json:"threads,omitempty"`
	GPULayers        int    `json:"gpuLayers,omitempty"`
	ContextSize      int    `json:"contextSize,omitempty"`
	BatchSize        int    `json:"batchSize,omitempty"`
	UBatchSize       int    `json:"uBatchSize,omitempty"`
	Pooling          string `json:"pooling,omitempty"`
	Normalize        string `json:"normalize,omitempty"`
	QueryPrefix      string `json:"queryPrefix,omitempty"`
	DocumentPrefix   string `json:"documentPrefix,omitempty"`
	Truncate         bool   `json:"truncate,omitempty"`
	ResourceURI      string `json:"resourceUri,omitempty"`
}

func ParseProviderConfig(rawJSON string) (LlamaCppProviderConfig, error) {
	var cfg LlamaCppProviderConfig
	if rawJSON == "" || rawJSON == "{}" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(rawJSON), &cfg)
	return cfg, err
}

func ParseEmbeddingConfig(rawJSON string) (LlamaCppEmbeddingConfig, error) {
	var cfg LlamaCppEmbeddingConfig
	if rawJSON == "" || rawJSON == "{}" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(rawJSON), &cfg)
	return cfg, err
}

func DefaultEmbeddingConfig() LlamaCppEmbeddingConfig {
	return LlamaCppEmbeddingConfig{
		Backend:     "cpu",
		Threads:     4,
		GPULayers:   0,
		ContextSize: 4096,
		BatchSize:   512,
		UBatchSize:  512,
		Pooling:     "mean",
		Normalize:   "l2",
		Truncate:    true,
	}
}
