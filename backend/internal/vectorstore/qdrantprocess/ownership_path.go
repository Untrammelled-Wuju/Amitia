// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"fmt"
	"path/filepath"

	"github.com/u-ai/backend/internal/runtimehost"
)

const (
	ownershipRelativeDir = "process/qdrant"
	activeDirName        = "active"
	ownershipFileName    = "ownership.json"
)

func ResolveOwnershipRoot(host runtimehost.RuntimeHost) (string, error) {
	if host == nil {
		return "", ErrOwnershipRootUnavailable
	}
	paths := host.Paths()
	if paths.TempDir == "" {
		return "", fmt.Errorf("%w: temp dir is empty", ErrOwnershipRootUnavailable)
	}
	if !filepath.IsAbs(paths.TempDir) {
		return "", fmt.Errorf("%w: temp dir is not absolute: %s", ErrOwnershipRootUnavailable, paths.TempDir)
	}
	root := filepath.Join(paths.TempDir, ownershipRelativeDir)
	return filepath.Clean(root), nil
}

func activeDirPath(ownershipRoot string) string {
	return filepath.Join(ownershipRoot, activeDirName)
}

func ownershipFilePath(ownershipRoot string) string {
	return filepath.Join(activeDirPath(ownershipRoot), ownershipFileName)
}
