// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type GGUFInspector interface {
	Inspect(resourceURI string) (*GGUFModelManifest, error)
}

type inspector struct {
}

func NewInspector() GGUFInspector {
	return &inspector{}
}

func (in *inspector) Inspect(resourceURI string) (*GGUFModelManifest, error) {
	info, err := os.Stat(resourceURI)
	if err != nil {
		return nil, fmt.Errorf("模型文件不存在: %w", err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("模型文件过大: %d bytes", info.Size())
	}
	if info.Size() < 1024 {
		return nil, fmt.Errorf("模型文件过小: %d bytes", info.Size())
	}

	f, err := os.Open(resourceURI)
	if err != nil {
		return nil, fmt.Errorf("打开模型文件失败: %w", err)
	}
	defer f.Close()

	header, err := ValidateHeader(f)
	if err != nil {
		return nil, fmt.Errorf("GGUF header 无效: %w", err)
	}

	metadata, err := ReadMetadata(f, header.MetadataCount)
	if err != nil {
		return nil, fmt.Errorf("GGUF metadata 无效: %w", err)
	}

	tensors, err := ReadTensorInfos(f, header.TensorCount)
	if err != nil {
		return nil, fmt.Errorf("GGUF tensor 无效: %w", err)
	}

	for _, t := range tensors {
		if t.NDims == 0 {
			continue
		}
		if t.Offset > uint64(info.Size()) {
			return nil, fmt.Errorf("tensor offset 溢出: %s", t.Name)
		}
	}

	manifest, err := BuildManifest(metadata, header, resourceURI)
	if err != nil {
		return nil, fmt.Errorf("构建 manifest 失败: %w", err)
	}

	splitInfo := detectSplitManifest(metadata, tensors)
	if splitInfo != nil {
		manifest.SplitFiles = in.resolveSplitFiles(resourceURI, splitInfo)
	}

	return manifest, nil
}

type splitMetadata struct {
	Count      int
	Index      int
	TotalSize  int64
	TensorCount uint64
}

func detectSplitManifest(meta *GGUFMetadata, tensors []TensorInfo) *splitMetadata {
	for _, kv := range meta.Keys {
		if kv.Key == "split_count" {
			sm := &splitMetadata{}
			switch v := kv.Value.(type) {
			case uint64:
				sm.Count = int(v)
			case uint32:
				sm.Count = int(v)
			case int64:
				sm.Count = int(v)
			case int32:
				sm.Count = int(v)
			}
			if sm.Count > 1 {
				for _, kv2 := range meta.Keys {
					if kv2.Key == "split_index" {
						switch v := kv2.Value.(type) {
						case uint64:
							sm.Index = int(v)
						case uint32:
							sm.Index = int(v)
						case int64:
							sm.Index = int(v)
						case int32:
							sm.Index = int(v)
						}
					}
					if kv2.Key == "split_tensor_count" {
						switch v := kv2.Value.(type) {
						case uint64:
							sm.TensorCount = v
						case uint32:
							sm.TensorCount = uint64(v)
						}
					}
				}
				return sm
			}
		}
	}
	return nil
}

func (in *inspector) resolveSplitFiles(primaryPath string, sm *splitMetadata) []GGUFSplitFile {
	dir := filepath.Dir(primaryPath)
	base := filepath.Base(primaryPath)
	ext := filepath.Ext(base)
	baseName := strings.TrimSuffix(base, ext)

	splitFiles := make([]GGUFSplitFile, sm.Count)
	for i := 0; i < sm.Count; i++ {
		pattern := fmt.Sprintf("%s-%05d-of-%05d%s", baseName, i+1, sm.Count, ext)
		shardPath := filepath.Join(dir, pattern)
		si, err := os.Stat(shardPath)
		size := int64(0)
		if err == nil {
			size = si.Size()
		}
		splitFiles[i] = GGUFSplitFile{
			Index:    i,
			Total:    sm.Count,
			Filename: pattern,
			Path:     shardPath,
			Size:     size,
		}
	}
	return splitFiles
}

func ValidateGGUFResource(resourceURI string) error {
	f, err := os.Open(resourceURI)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	var magicBuf [4]byte
	if _, err := io.ReadFull(f, magicBuf[:]); err != nil {
		return fmt.Errorf("读取 magic 失败: %w", err)
	}

	magic := uint32(magicBuf[0]) | uint32(magicBuf[1])<<8 | uint32(magicBuf[2])<<16 | uint32(magicBuf[3])<<24
	if magic != GGUFMagic {
		return fmt.Errorf("无效的 GGUF magic: 0x%08x", magic)
	}
	return nil
}
