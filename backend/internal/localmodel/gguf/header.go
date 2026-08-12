// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	GGUFMagic    uint32 = 0x46554747
	GGUFVersion  uint32 = 3
	GGUFMaxDims  uint32 = 4
)

type GGUFHeader struct {
	Magic      uint32
	Version    uint32
	TensorCount uint64
	MetadataCount uint64
}

func ValidateHeader(r io.Reader) (*GGUFHeader, error) {
	var h GGUFHeader
	if err := binary.Read(r, binary.LittleEndian, &h.Magic); err != nil {
		return nil, fmt.Errorf("读取 magic 失败: %w", err)
	}
	if h.Magic != GGUFMagic {
		return nil, fmt.Errorf("无效的 GGUF magic: 0x%08x", h.Magic)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return nil, fmt.Errorf("读取 version 失败: %w", err)
	}
	if h.Version > GGUFVersion {
		return nil, fmt.Errorf("不支持的 GGUF version: %d", h.Version)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.TensorCount); err != nil {
		return nil, fmt.Errorf("读取 tensor count 失败: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.MetadataCount); err != nil {
		return nil, fmt.Errorf("读取 metadata count 失败: %w", err)
	}
	if h.TensorCount > MaxTensorCount {
		return nil, fmt.Errorf("tensor count 超出限制: %d", h.TensorCount)
	}
	if h.MetadataCount > MaxMetadataCount {
		return nil, fmt.Errorf("metadata count 超出限制: %d", h.MetadataCount)
	}
	return &h, nil
}
