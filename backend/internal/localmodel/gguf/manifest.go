// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import (
	"fmt"
	"strings"
	"time"
)

type GGUFModelManifest struct {
	LocalModelID     string            `json:"localModelId"`
	ResourceURI      string            `json:"resourceUri"`
	Engine           string            `json:"engine"`
	ContentHash      string            `json:"contentHash"`
	SizeBytes        int64             `json:"sizeBytes"`
	GGUFVersion      int               `json:"ggufVersion"`
	Architecture     string            `json:"architecture"`
	DisplayName      string            `json:"displayName"`
	Description      string            `json:"description"`
	Author           string            `json:"author"`
	FileType         string            `json:"fileType"`
	Quantization     string            `json:"quantization"`
	ContextLength    int               `json:"contextLength"`
	EmbeddingLength  int               `json:"embeddingLength"`
	HeadCount        int               `json:"headCount"`
	BlockCount       int               `json:"blockCount"`
	ChatTemplate     string            `json:"chatTemplate"`
	PoolingType      string            `json:"poolingType"`
	SupportsEmbedding bool             `json:"supportsEmbedding"`
	Capabilities     []string          `json:"capabilities"`
	SplitFiles       []GGUFSplitFile   `json:"splitFiles,omitempty"`
	ImportedAt       time.Time         `json:"importedAt"`
}

type GGUFSplitFile struct {
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Hash     string `json:"hash"`
}

func BuildManifest(meta *GGUFMetadata, header *GGUFHeader, resourceURI string) (*GGUFModelManifest, error) {
	arch := meta.GetString("general.architecture")
	if arch == "" {
		return nil, fmt.Errorf("缺少 general.architecture")
	}

	manifest := &GGUFModelManifest{
		ResourceURI:      resourceURI,
		Engine:           "llama.cpp",
		GGUFVersion:      int(header.Version),
		Architecture:     arch,
		DisplayName:      meta.GetString("general.name"),
		Description:      meta.GetString("general.description"),
		Author:           meta.GetString("general.author"),
		FileType:         generalFileTypeToStr(meta, arch),
		Quantization:     meta.GetString("general.quantization_version"),
		ContextLength:    int(meta.GetUint32(arch + ".context_length")),
		EmbeddingLength:  int(meta.GetUint32(arch + ".embedding_length")),
		HeadCount:        int(meta.GetUint32(arch + ".attention.head_count")),
		BlockCount:       int(meta.GetUint32(arch + ".block_count")),
		ChatTemplate:     meta.GetString("tokenizer.chat_template"),
		SupportsEmbedding: detectSupportsEmbedding(meta, arch),
		Capabilities:     detectCapabilities(meta, arch),
	}

	if pooling := meta.GetString(arch + ".pooling_type"); pooling != "" {
		manifest.PoolingType = pooling
	}

	return manifest, nil
}

func generalFileTypeToStr(meta *GGUFMetadata, arch string) string {
	return generalFileTypeStr(uint32(meta.GetUint64(arch + ".file_type")))
}

func generalFileTypeStr(ft uint32) string {
	switch ft {
	case 0:
		return "F32"
	case 1:
		return "F16"
	case 2:
		return "Q4_0"
	case 3:
		return "Q4_1"
	case 6:
		return "Q5_0"
	case 7:
		return "Q5_1"
	case 8:
		return "Q8_0"
	case 9:
		return "Q8_1"
	case 10:
		return "Q2_K"
	case 11:
		return "Q3_K"
	case 12:
		return "Q4_K"
	case 13:
		return "Q5_K"
	case 14:
		return "Q6_K"
	case 15:
		return "Q8_K"
	case 16:
		return "IQ2_XXS"
	case 17:
		return "IQ2_XS"
	case 18:
		return "IQ3_XXS"
	case 19:
		return "IQ3_S"
	case 20:
		return "IQ4_NL"
	case 21:
		return "IQ4_XS"
	default:
		if ft >= 100 {
			return "MIX_IQ"
		}
		return "F16"
	}
}

func detectSupportsEmbedding(meta *GGUFMetadata, arch string) bool {
	embeddingLen := meta.GetUint32(arch + ".embedding_length")
	return embeddingLen > 0 && strings.Contains(strings.ToLower(meta.GetString("general.description")), "embedding")
}

func detectCapabilities(meta *GGUFMetadata, arch string) []string {
	caps := []string{"text"}
	if meta.GetUint32(arch + ".embedding_length") > 0 {
		caps = append(caps, "embedding")
	}
	if strings.Contains(meta.GetString("tokenizer.chat_template"), "tool") ||
		strings.Contains(meta.GetString("general.name"), "tool") {
		caps = append(caps, "tool_calling")
	}
	return caps
}

func ComputeEmbeddingFingerprint(manifest *GGUFModelManifest) string {
	h := fmt.Sprintf("%s|%d|%s|%s|%s",
		manifest.ContentHash,
		manifest.EmbeddingLength,
		manifest.PoolingType,
		manifest.Architecture,
		manifest.Engine,
	)
	return fmt.Sprintf("%x", h)
}
