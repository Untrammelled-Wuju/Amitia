// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package mnn

import "encoding/json"

type MNNProviderConfig struct {
	ModelResourceURI string             `json:"modelResourceUri"`
	Backend          string             `json:"backend,omitempty"`
	ThreadNum        int                `json:"threadNum,omitempty"`
	Precision        string             `json:"precision,omitempty"`
	Memory           string             `json:"memory,omitempty"`
	UseMMap          bool               `json:"useMmap,omitempty"`
	KVCacheMMap      bool               `json:"kvCacheMmap,omitempty"`
	AttentionMode    int                `json:"attentionMode,omitempty"`
	ReuseKV          bool               `json:"reuseKv,omitempty"`
	Sampler          MNNGenerationConfig `json:"sampler,omitempty"`
	Multimodal       bool               `json:"multimodal,omitempty"`
	LocalModelID     string             `json:"localModelId,omitempty"`
}

type MNNGenerationConfig struct {
	Type        string  `json:"type,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopK        int     `json:"topK,omitempty"`
	TopP        float64 `json:"topP,omitempty"`
	MinP        float64 `json:"minP,omitempty"`
	TFS         float64 `json:"tfs,omitempty"`
	Typical     float64 `json:"typical,omitempty"`
	Penalty     float64 `json:"penalty,omitempty"`
	NGram       int     `json:"nGram,omitempty"`
}

type MNNModelManifest struct {
	Schema        int                    `json:"schema"`
	Engine        string                 `json:"engine"`
	DisplayName   string                 `json:"displayName"`
	ContentHash   string                 `json:"contentHash"`
	Capabilities  MNNModelCapabilities   `json:"capabilities"`
	EngineConfig  string                 `json:"engineConfig"`
}

type MNNModelCapabilities struct {
	Text        bool     `json:"text"`
	Vision      bool     `json:"vision"`
	ToolCalling bool     `json:"toolCalling"`
	MaxContext  int      `json:"maxContext"`
	Backends    []string `json:"backends"`
}

func ParseProviderConfig(rawJSON string) (MNNProviderConfig, error) {
	var cfg MNNProviderConfig
	if rawJSON == "" || rawJSON == "{}" {
		return cfg, nil
	}
	err := json.Unmarshal([]byte(rawJSON), &cfg)
	return cfg, err
}
