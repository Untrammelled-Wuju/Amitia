// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package gguf

import "errors"

var (
	ErrGGUFNotFound             = errors.New("GGUF 模型文件不存在")
	ErrGGUFInvalid              = errors.New("GGUF 模型文件无效")
	ErrGGUFMagicInvalid         = errors.New("GGUF magic 无效")
	ErrGGUFVersionUnsupported   = errors.New("GGUF 版本不支持")
	ErrGGUFMetadataInvalid      = errors.New("GGUF metadata 无效")
	ErrGGUFTensorInvalid        = errors.New("GGUF tensor 无效")
	ErrGGUFSplitIncomplete      = errors.New("GGUF split 不完整")
	ErrGGUFSplitInvalid         = errors.New("GGUF split 信息无效")
	ErrGGUFPathEscape           = errors.New("GGUF 路径逃逸")
	ErrGGUFTooLarge             = errors.New("GGUF 文件过大")
	ErrGGUFMetadataBomb         = errors.New("GGUF metadata 攻击")
	ErrGGUFTensorOffsetOverflow = errors.New("GGUF tensor offset 溢出")
	ErrGGUFStringTooLong        = errors.New("GGUF 字符串过长")
	ErrGGUFArrayTooLong         = errors.New("GGUF 数组过长")
)
