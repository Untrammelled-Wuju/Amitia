// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"os"
	"path/filepath"
	"strings"
)

type FileInspector interface {
	Stat(path string) (os.FileInfo, error)
	Abs(path string) (string, error)
}

type defaultFileInspector struct{}

func (defaultFileInspector) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (defaultFileInspector) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func isJSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".mjs", ".cjs":
		return true
	}
	return false
}
