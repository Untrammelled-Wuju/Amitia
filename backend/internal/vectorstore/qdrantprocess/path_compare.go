// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"path/filepath"
	"runtime"
	"strings"
)

func SameExecutablePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return sameExecutablePathWindows(a, b)
	}
	return sameExecutablePathUnix(a, b)
}

func sameExecutablePathUnix(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func sameExecutablePathWindows(a, b string) bool {
	a = normalizeWindowsPath(a)
	b = normalizeWindowsPath(b)
	return strings.EqualFold(a, b)
}

func normalizeWindowsPath(p string) string {
	p = filepath.Clean(p)
	p = strings.TrimPrefix(p, `\\?\`)
	p = strings.TrimPrefix(p, `\\.\`)
	return p
}
