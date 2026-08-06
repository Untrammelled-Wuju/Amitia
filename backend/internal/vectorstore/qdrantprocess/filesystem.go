// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"io"
	"os"
	"path/filepath"
)

type FileSystem interface {
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Rename(oldPath, newPath string) error
	Remove(path string) error
	Stat(path string) (os.FileInfo, error)
	Open(path string) (io.ReadCloser, error)
}

type realFileSystem struct{}

func NewFileSystem() FileSystem { return realFileSystem{} }

func (realFileSystem) Mkdir(path string, perm os.FileMode) error {
	return os.Mkdir(path, perm)
}

func (realFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (realFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (realFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (realFileSystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (realFileSystem) Remove(path string) error {
	return os.Remove(path)
}

func (realFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (realFileSystem) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func atomicWriteFile(fs FileSystem, path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := fs.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
