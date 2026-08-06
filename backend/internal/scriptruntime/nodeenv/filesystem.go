// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package nodeenv

import (
	"os"
	"path/filepath"
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

func newDefaultFileInspector() FileInspector {
	return defaultFileInspector{}
}
