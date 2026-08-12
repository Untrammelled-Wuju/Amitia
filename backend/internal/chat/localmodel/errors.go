// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package localmodel

import "errors"

var (
	ErrLocalProviderNotFound      = errors.New("local model provider not found")
	ErrModelPackageNotFound       = errors.New("model package not found")
	ErrModelPackageInvalid        = errors.New("model package invalid")
	ErrModelPackageIncomplete     = errors.New("model package incomplete")
	ErrConfigInvalid              = errors.New("model config invalid")
	ErrFileOutsidePackage         = errors.New("model file path escapes package root")
	ErrBackendUnavailable         = errors.New("backend unavailable")
	ErrModelUnsupported           = errors.New("model unsupported")
	ErrLoadFailed                 = errors.New("model load failed")
	ErrOutOfMemory                = errors.New("out of memory")
	ErrGenerateFailed             = errors.New("generation failed")
	ErrOutputInvalid              = errors.New("output invalid")
	ErrMultimodalUnsupported      = errors.New("multimodal unsupported")
	ErrToolCallUnsupported        = errors.New("tool calling unsupported")
	ErrCancelled                  = errors.New("generation cancelled")
	ErrTimeout                    = errors.New("generation timeout")
	ErrNativeBridgeUnavailable    = errors.New("native bridge unavailable")
	ErrNativeCrashed              = errors.New("native crashed")
	ErrResourceLimit              = errors.New("resource limit exceeded")
)
