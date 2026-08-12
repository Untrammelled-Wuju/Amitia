// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

const (
	MaxTensorCount  uint64 = 1000000
	MaxMetadataCount uint64 = 1000000
	MaxStringLen    uint64 = 1024 * 1024
	MaxArrayLen     uint64 = 1000000
	MaxFileSize     int64  = 1024 * 1024 * 1024 * 1024
)

const (
	MetadataTypeUint8   uint32 = 0
	MetadataTypeInt8    uint32 = 1
	MetadataTypeUint16  uint32 = 2
	MetadataTypeInt16   uint32 = 3
	MetadataTypeUint32  uint32 = 4
	MetadataTypeInt32   uint32 = 5
	MetadataTypeFloat32 uint32 = 6
	MetadataTypeBool    uint32 = 7
	MetadataTypeString  uint32 = 8
	MetadataTypeArray   uint32 = 9
	MetadataTypeUint64  uint32 = 10
	MetadataTypeInt64   uint32 = 11
	MetadataTypeFloat64 uint32 = 12
)
