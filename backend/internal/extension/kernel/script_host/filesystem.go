// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import "os"

type FileInspector interface {
	Stat(path string) (os.FileInfo, error)
}

type defaultFileInspector struct{}

func newDefaultFileInspector() *defaultFileInspector {
	return &defaultFileInspector{}
}

func (d *defaultFileInspector) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
