// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantprocess

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fakeFileSystem struct {
	mu        sync.Mutex
	files     map[string][]byte
	dirs      map[string]bool
	stats     map[string]os.FileInfo
	readErr   error
	writeErr  error
	mkdirErr  error
	renameErr error
	removeErr error
	openErr   error
}

func newFakeFileSystem() *fakeFileSystem {
	return &fakeFileSystem{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
		stats: make(map[string]os.FileInfo),
	}
}

func (f *fakeFileSystem) Mkdir(path string, perm os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirs[path] = true
	f.files[path] = nil
	return nil
}

func (f *fakeFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirs[path] = true
	f.files[path] = nil
	return nil
}

func (f *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

func (f *fakeFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]byte, len(data))
	copy(out, data)
	f.files[path] = out
	return nil
}

func (f *fakeFileSystem) Rename(oldPath, newPath string) error {
	if f.renameErr != nil {
		return f.renameErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.files[oldPath]
	if !ok {
		return os.ErrNotExist
	}
	f.files[newPath] = data
	delete(f.files, oldPath)
	return nil
}

func (f *fakeFileSystem) Remove(path string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.dirs[path]; ok && len(f.files) > 0 {
		return os.ErrInvalid
	}
	delete(f.files, path)
	delete(f.dirs, path)
	return nil
}

func (f *fakeFileSystem) Stat(path string) (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.files[path]; ok {
		return &fakeFileInfo{name: filepath.Base(path), isDir: f.dirs[path]}, nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeFileSystem) Open(path string) (ReadDirCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.dirs[path]; !ok {
		return nil, os.ErrNotExist
	}
	entries := make([]os.DirEntry, 0)
	for p := range f.files {
		if filepath.Dir(p) == path {
			entries = append(entries, &fakeFileInfo{name: filepath.Base(p), isDir: false})
		}
	}
	return &fakeReadDirCloser{entries: entries}, nil
}

type ReadDirCloser interface {
	ReadDir(int) ([]os.DirEntry, error)
	Close() error
}

type fakeReadDirCloser struct {
	entries []os.DirEntry
	idx     int
}

func (f *fakeReadDirCloser) ReadDir(n int) ([]os.DirEntry, error) {
	if f.idx >= len(f.entries) {
		return nil, os.ErrInvalid
	}
	out := f.entries[f.idx:]
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	f.idx += len(out)
	return out, nil
}

func (f *fakeReadDirCloser) Close() error { return nil }

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string               { return f.name }
func (f fakeFileInfo) IsDir() bool                { return f.isDir }
func (f fakeFileInfo) Type() os.FileMode          { return 0 }
func (f fakeFileInfo) Info() (os.FileInfo, error) { return nil, nil }
func (f fakeFileInfo) ModTime() time.Time         { return time.Time{} }
func (f fakeFileInfo) Mode() os.FileMode          { return 0 }
func (f fakeFileInfo) Size() int64                { return 0 }
func (f fakeFileInfo) Sys() interface{}           { return nil }
