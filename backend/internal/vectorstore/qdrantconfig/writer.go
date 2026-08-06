// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantconfig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

func newDefaultFileReader() FileReader {
	return defaultFileReader{}
}

type defaultFileReader struct{}

func (defaultFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type Writer interface {
	Write(ctx context.Context, path string, content []byte) error
}

type atomicWriter struct {
	reader FileReader
}

func NewWriter(reader FileReader) Writer {
	if reader == nil {
		reader = newDefaultFileReader()
	}
	return &atomicWriter{reader: reader}
}

func (w *atomicWriter) Write(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return newWriteFailed(path, fmt.Errorf("parent directory does not exist: %s", dir))
		}
		return newWriteFailed(path, err)
	}

	existingContent, err := w.reader.ReadFile(path)
	if err == nil {
		if bytesEqual(existingContent, content) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return newWriteFailed(path, fmt.Errorf("read existing config: %w", err))
	}

	tmpFile, err := w.createTmpFile(dir)
	if err != nil {
		return newWriteFailed(path, err)
	}
	tmpPath := tmpFile.Name()

	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return newWriteFailed(path, fmt.Errorf("write temp: %w", err))
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return newWriteFailed(path, fmt.Errorf("sync temp: %w", err))
	}

	if err := tmpFile.Close(); err != nil {
		return newWriteFailed(path, fmt.Errorf("close temp: %w", err))
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return newWriteFailed(path, fmt.Errorf("rename temp: %w", err))
	}

	tmpPath = ""
	w.syncParent(dir)
	return nil
}

func (w *atomicWriter) createTmpFile(dir string) (*os.File, error) {
	for i := 0; i < 5; i++ {
		name := "config-" + randomHex(8) + ".tmp"
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			return f, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("failed to create unique temp file after 5 attempts")
}

func (w *atomicWriter) syncParent(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", b)
	}
	return hex.EncodeToString(b)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
