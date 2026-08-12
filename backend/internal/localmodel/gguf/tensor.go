// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

type TensorInfo struct {
	Name   string
	NDims  uint32
	Dims   [4]uint64
	Type   uint32
	Offset uint64
}

func ReadTensorInfos(r io.Reader, count uint64) ([]TensorInfo, error) {
	tensors := make([]TensorInfo, 0, count)
	for i := uint64(0); i < count; i++ {
		t, err := readTensorInfo(r)
		if err != nil {
			return nil, fmt.Errorf("读取 tensor[%d] 失败: %w", i, err)
		}
		tensors = append(tensors, *t)
	}
	return tensors, nil
}

func readTensorInfo(r io.Reader) (*TensorInfo, error) {
	nameLen, err := readUint64(r)
	if err != nil {
		return nil, fmt.Errorf("读取 tensor name length 失败: %w", err)
	}
	if nameLen > MaxStringLen {
		return nil, fmt.Errorf("tensor name 长度超出限制: %d", nameLen)
	}
	nameBytes := make([]byte, nameLen)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return nil, fmt.Errorf("读取 tensor name 失败: %w", err)
	}

	var nDims uint32
	if err := binary.Read(r, binary.LittleEndian, &nDims); err != nil {
		return nil, fmt.Errorf("读取 tensor dims 失败: %w", err)
	}
	if nDims > GGUFMaxDims {
		return nil, fmt.Errorf("tensor dims 超出限制: %d", nDims)
	}

	t := &TensorInfo{
		Name:  string(nameBytes),
		NDims: nDims,
	}
	for i := uint32(0); i < nDims; i++ {
		dim, err := readUint64(r)
		if err != nil {
			return nil, fmt.Errorf("读取 tensor dim[%d] 失败: %w", i, err)
		}
		t.Dims[i] = dim
	}

	if err := binary.Read(r, binary.LittleEndian, &t.Type); err != nil {
		return nil, fmt.Errorf("读取 tensor type 失败: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &t.Offset); err != nil {
		return nil, fmt.Errorf("读取 tensor offset 失败: %w", err)
	}
	return t, nil
}

func (t *TensorInfo) Size() uint64 {
	if t.NDims == 0 {
		return 0
	}
	size := t.Dims[0]
	for i := uint32(1); i < t.NDims; i++ {
		size *= t.Dims[i]
	}
	return size
}

func (t *TensorInfo) IsQPyramid() bool {
	return strings.HasPrefix(t.Name, "blk.") && strings.Contains(t.Name, ".attn_q.")
}
