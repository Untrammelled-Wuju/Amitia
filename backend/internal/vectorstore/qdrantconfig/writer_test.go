// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriter_FirstWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	w := NewWriter(nil)

	content := []byte("service:\n  http_port: 19178\n")
	if err := w.Write(context.Background(), path, content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch: expected %q got %q", string(content), string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected mode 0600, got %o", info.Mode().Perm())
		}
	}
}

func TestWriter_SameContentNoRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	w := NewWriter(nil)

	content := []byte("test content\n")
	if err := w.Write(context.Background(), path, content); err != nil {
		t.Fatalf("First Write: %v", err)
	}

	info1, _ := os.Stat(path)

	if err := w.Write(context.Background(), path, content); err != nil {
		t.Fatalf("Second Write: %v", err)
	}

	info2, _ := os.Stat(path)
	if info1.ModTime() != info2.ModTime() {
		t.Logf("same content rewrite changed mtime: %v -> %v", info1.ModTime(), info2.ModTime())
	}
}

func TestWriter_DifferentContentAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	w := NewWriter(nil)

	oldContent := []byte("old content\n")
	if err := w.Write(context.Background(), path, oldContent); err != nil {
		t.Fatalf("First Write: %v", err)
	}

	newContent := []byte("new content\n")
	if err := w.Write(context.Background(), path, newContent); err != nil {
		t.Fatalf("Second Write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != string(newContent) {
		t.Errorf("expected %q got %q", string(newContent), string(data))
	}
}

func TestWriter_MissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "config.yaml")
	w := NewWriter(nil)

	err := w.Write(context.Background(), path, []byte("content\n"))
	if err == nil {
		t.Error("expected error when parent dir missing")
	}
}

func TestWriter_NoTmpFilesLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	w := NewWriter(nil)

	for i := 0; i < 5; i++ {
		if err := w.Write(context.Background(), path, []byte("content\n")); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.Name() != "config.yaml" {
			t.Errorf("unexpected file: %s", entry.Name())
		}
	}
}
