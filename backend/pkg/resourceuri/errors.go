// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package resourceuri

import "errors"

var (
	ErrInvalidScheme         = errors.New("resourceuri: invalid scheme, must be amitia")
	ErrInvalidRoot           = errors.New("resourceuri: invalid resource root")
	ErrInvalidPath           = errors.New("resourceuri: invalid path")
	ErrPathTraversal         = errors.New("resourceuri: path traversal detected")
	ErrUnsupportedResource   = errors.New("resourceuri: unsupported resource")
	ErrNonFilesystemResource = errors.New("resourceuri: non-filesystem resource cannot be resolved as file path")
	ErrResourceOutsideRoots  = errors.New("resourceuri: path is outside all configured resource roots")
	ErrRootNotConfigured     = errors.New("resourceuri: resource root is not configured")
)
