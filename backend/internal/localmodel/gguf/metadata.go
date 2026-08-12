// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import (
	"encoding/binary"
	"fmt"
	"io"
)

type KeyValue struct {
	Key   string
	Type  uint32
	Value interface{}
}

type GGUFMetadata struct {
	Keys []KeyValue
}

func (m *GGUFMetadata) GetString(key string) string {
	for _, kv := range m.Keys {
		if kv.Key == key {
			if s, ok := kv.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func (m *GGUFMetadata) GetUint64(key string) uint64 {
	for _, kv := range m.Keys {
		if kv.Key == key {
			switch v := kv.Value.(type) {
			case uint64:
				return v
			case uint32:
				return uint64(v)
			case int64:
				return uint64(v)
			case int32:
				return uint64(v)
			}
		}
	}
	return 0
}

func (m *GGUFMetadata) GetUint32(key string) uint32 {
	for _, kv := range m.Keys {
		if kv.Key == key {
			switch v := kv.Value.(type) {
			case uint32:
				return v
			case uint64:
				return uint32(v)
			case int32:
				return uint32(v)
			case int64:
				return uint32(v)
			}
		}
	}
	return 0
}

func ReadMetadata(r io.Reader, count uint64) (*GGUFMetadata, error) {
	meta := &GGUFMetadata{Keys: make([]KeyValue, 0, count)}
	for i := uint64(0); i < count; i++ {
		kv, err := readKeyValue(r)
		if err != nil {
			return nil, fmt.Errorf("读取 metadata[%d] 失败: %w", i, err)
		}
		meta.Keys = append(meta.Keys, *kv)
	}
	return meta, nil
}

func readKeyValue(r io.Reader) (*KeyValue, error) {
	keyLen, err := readUint64(r)
	if err != nil {
		return nil, fmt.Errorf("读取 key length 失败: %w", err)
	}
	if keyLen > MaxStringLen {
		return nil, fmt.Errorf("key 长度超出限制: %d", keyLen)
	}
	keyBytes := make([]byte, keyLen)
	if _, err := io.ReadFull(r, keyBytes); err != nil {
		return nil, fmt.Errorf("读取 key 内容失败: %w", err)
	}

	var metaType uint32
	if err := binary.Read(r, binary.LittleEndian, &metaType); err != nil {
		return nil, fmt.Errorf("读取 type 失败: %w", err)
	}

	value, err := readMetadataValue(r, metaType)
	if err != nil {
		return nil, fmt.Errorf("读取 value 失败: %w", err)
	}

	return &KeyValue{
		Key:   string(keyBytes),
		Type:  metaType,
		Value: value,
	}, nil
}

func readMetadataValue(r io.Reader, metaType uint32) (interface{}, error) {
	switch metaType {
	case MetadataTypeUint8:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	case MetadataTypeInt8:
		var v int8
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case MetadataTypeUint16:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	case MetadataTypeInt16:
		var v int16
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case MetadataTypeUint32:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case MetadataTypeInt32:
		var v int32
		err := binary.Read(r, binary.LittleEndian, &v)
		return int64(v), err
	case MetadataTypeFloat32:
		var v float32
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case MetadataTypeBool:
		var v uint8
		err := binary.Read(r, binary.LittleEndian, &v)
		return v != 0, err
	case MetadataTypeString:
		return readMetadataString(r)
	case MetadataTypeUint64:
		return readUint64(r)
	case MetadataTypeInt64:
		var v int64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case MetadataTypeFloat64:
		var v float64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case MetadataTypeArray:
		return readMetadataArray(r)
	default:
		return nil, fmt.Errorf("未知 metadata type: %d", metaType)
	}
}

func readMetadataString(r io.Reader) (string, error) {
	length, err := readUint64(r)
	if err != nil {
		return "", err
	}
	if length > MaxStringLen {
		return "", fmt.Errorf("字符串长度超出限制: %d", length)
	}
	bytes := make([]byte, length)
	if _, err := io.ReadFull(r, bytes); err != nil {
		return "", err
	}
	return string(bytes), nil
}

func readMetadataArray(r io.Reader) ([]interface{}, error) {
	var length uint64
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length > MaxArrayLen {
		return nil, fmt.Errorf("array 长度超出限制: %d", length)
	}
	var metaType uint32
	if err := binary.Read(r, binary.LittleEndian, &metaType); err != nil {
		return nil, err
	}
	result := make([]interface{}, 0, length)
	for i := uint64(0); i < length; i++ {
		v, err := readMetadataValue(r, metaType)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

func readUint64(r io.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}
