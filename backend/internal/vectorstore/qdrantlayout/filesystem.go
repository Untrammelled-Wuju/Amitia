// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

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

type DirectoryCreator interface {
	MkdirAll(path string, perm os.FileMode) error
}

type defaultDirectoryCreator struct{}

func (defaultDirectoryCreator) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func newDefaultDirectoryCreator() DirectoryCreator {
	return defaultDirectoryCreator{}
}

type DirectoryReader interface {
	ReadDir(path string) ([]os.DirEntry, error)
}

type defaultDirectoryReader struct{}

func (defaultDirectoryReader) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func newDefaultDirectoryReader() DirectoryReader {
	return defaultDirectoryReader{}
}

type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

type defaultFileReader struct{}

func (defaultFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func newDefaultFileReader() FileReader {
	return defaultFileReader{}
}

type FileWriter interface {
	WriteFile(path string, data []byte, perm os.FileMode) error
}

type defaultFileWriter struct{}

func (defaultFileWriter) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func newDefaultFileWriter() FileWriter {
	return defaultFileWriter{}
}

type FileRemover interface {
	Remove(path string) error
}

type defaultFileRemover struct{}

func (defaultFileRemover) Remove(path string) error {
	return os.Remove(path)
}

func newDefaultFileRemover() FileRemover {
	return defaultFileRemover{}
}
