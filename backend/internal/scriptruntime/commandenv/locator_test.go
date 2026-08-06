// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package commandenv

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type fakeLocator struct {
	paths map[string]string
}

func (f *fakeLocator) LookPath(file string) (string, error) {
	if p, ok := f.paths[file]; ok {
		return p, nil
	}
	return "", os.ErrNotExist
}

func newFakeLocator(paths map[string]string) *fakeLocator {
	return &fakeLocator{paths: paths}
}

func TestToAbsolutePathWithAbsoluteInput(t *testing.T) {
	loc := newFakeLocator(nil)
	root := t.TempDir()
	exe := filepath.Join(root, "myapp")

	got, err := toAbsolutePath(loc, exe)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != filepath.Clean(exe) {
		t.Fatalf("expected %q, got %q", filepath.Clean(exe), got)
	}
}

func TestToAbsolutePathWithRelativeAndLookupSuccess(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "bin", "myapp")
	loc := newFakeLocator(map[string]string{"myapp": exe})

	got, err := toAbsolutePath(loc, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected, _ := filepath.Abs(exe)
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestToAbsolutePathWithRelativeAndLookupFailure(t *testing.T) {
	loc := newFakeLocator(nil)
	_, err := toAbsolutePath(loc, "nonexistent")
	if err == nil {
		t.Fatal("expected error for failed lookup")
	}
}

func TestToAbsolutePathPreservesCleanedPath(t *testing.T) {
	loc := newFakeLocator(nil)
	root := t.TempDir()
	exe := filepath.Join(root, "bin", "myapp")

	input := filepath.Join(root, "bin", ".", "myapp")
	got, err := toAbsolutePath(loc, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != exe {
		t.Fatalf("expected cleaned path %q, got %q", exe, got)
	}
}

func TestDefaultLocatorLookPathFindsSystemCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only test")
	}

	loc := newDefaultLocator()
	path, err := loc.LookPath("sh")
	if err != nil {
		t.Fatalf("expected to find sh in PATH: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

func TestDefaultLocatorLookPathReturnsErrorForMissing(t *testing.T) {
	loc := newDefaultLocator()
	_, err := loc.LookPath("totally_nonexistent_binary_xyz123")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestFakeLocatorMatchesExactKey(t *testing.T) {
	loc := newFakeLocator(map[string]string{
		"custom-tool": "/opt/bin/custom-tool",
	})

	path, err := loc.LookPath("custom-tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/opt/bin/custom-tool" {
		t.Fatalf("expected /opt/bin/custom-tool, got %q", path)
	}

	_, err = loc.LookPath("Custom-Tool")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fakeLocator should be case-sensitive")
	}
}
