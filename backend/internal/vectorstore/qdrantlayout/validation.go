// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"path/filepath"
)

func containsPath(parent, child string) bool {
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	if len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' {
		return false
	}
	return true
}

func isFilesystemRoot(path string) bool {
	clean := filepath.Clean(path)
	if clean == "/" {
		return true
	}
	if len(clean) == 3 && clean[1] == ':' && (clean[2] == '\\' || clean[2] == '/') {
		return true
	}
	if len(clean) == 2 && clean[1] == ':' {
		return true
	}
	return filepath.Dir(clean) == clean
}
