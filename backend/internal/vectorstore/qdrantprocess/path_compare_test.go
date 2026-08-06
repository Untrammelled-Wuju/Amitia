// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"runtime"
	"testing"
)

func TestSameExecutablePath_Unix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix test")
	}
	a := "/usr/bin/qdrant"
	b := "/usr/bin/qdrant"
	if !SameExecutablePath(a, b) {
		t.Fatal("expected same path")
	}
}

func TestSameExecutablePath_Different(t *testing.T) {
	a := "/usr/bin/qdrant"
	b := "/other/qdrant"
	if SameExecutablePath(a, b) {
		t.Fatal("expected different path")
	}
}

func TestSameExecutablePath_Cleaned(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix test")
	}
	a := "/usr/local/bin/./qdrant"
	b := "/usr/local/bin/qdrant"
	if !SameExecutablePath(a, b) {
		t.Fatal("expected same path after cleaning")
	}
}
