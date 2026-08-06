// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sidecar

import (
	"os"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func testPaths() util.RuntimePaths {
	root := "/tmp/test-runtime"
	return util.RuntimePaths{
		Root:         root,
		ConfigDir:    root + "/config/config.json",
		DataDir:      root + "/data",
		LogDir:       root + "/logs",
		WorkspaceDir: root + "/workspace",
		CacheDir:     root + "/cache",
		TempDir:      root + "/tmp",
	}
}

type fakeHost struct {
	descr platform.RuntimeDescriptor
	paths util.RuntimePaths
}

func (h *fakeHost) Descriptor() platform.RuntimeDescriptor      { return h.descr }
func (h *fakeHost) Capabilities() *runtimehost.HostCapabilities { return nil }
func (h *fakeHost) Paths() util.RuntimePaths                    { return h.paths }
func (h *fakeHost) Processes() runtimehost.ProcessSupervisor    { return nil }
func (h *fakeHost) RuntimeInstanceID() string                   { return "test" }

type stubFileInfo struct {
	mode os.FileMode
	dir  bool
}

func (fi stubFileInfo) Name() string       { return "stub" }
func (fi stubFileInfo) Size() int64        { return 0 }
func (fi stubFileInfo) Mode() os.FileMode  { return fi.mode }
func (fi stubFileInfo) ModTime() time.Time { return time.Time{} }
func (fi stubFileInfo) IsDir() bool        { return fi.dir }
func (fi stubFileInfo) Sys() interface{}   { return nil }

type stubInspector struct {
	files map[string]bool
	absFn func(string) (string, error)
}

func (s *stubInspector) Stat(path string) (os.FileInfo, error) {
	if s.files[path] {
		return stubFileInfo{mode: 0755}, nil
	}
	return nil, os.ErrNotExist
}

func (s *stubInspector) Abs(path string) (string, error) {
	if s.absFn != nil {
		return s.absFn(path)
	}
	return path, nil
}
