// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package script_host

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type fixedNodeResolver struct {
	env nodeenv.Environment
	err error
}

func (r *fixedNodeResolver) Resolve(_ context.Context) (nodeenv.Environment, error) {
	return r.env, r.err
}

func fixedNodeResolverOk(nodeBin string) NodeEnvironmentResolver {
	return &fixedNodeResolver{
		env: nodeenv.Environment{
			NodeBinary:                 nodeBin,
			WorkDir:                    filepath.Dir(nodeBin),
			DistributionRoot:           filepath.Dir(nodeBin),
			Source:                     nodeenv.SourceExplicit,
			Guest:                      platform.GuestPlatformNone,
			Architecture:               "amd64",
			PackageManagementAvailable: false,
		},
	}
}

func fixedNodeResolverErr(err error) NodeEnvironmentResolver {
	return &fixedNodeResolver{err: err}
}

type fixedArtifactResolver struct {
	artifact Artifact
	err      error
}

func (r *fixedArtifactResolver) Resolve(_ context.Context, _ Kind) (Artifact, error) {
	return r.artifact, r.err
}

func fixedArtifactResolverOk(entryPath string, kind Kind) ArtifactResolver {
	return &fixedArtifactResolver{
		artifact: Artifact{
			Kind:             kind,
			EntryPath:        entryPath,
			DistributionRoot: deriveDistributionRoot(entryPath),
			Source:           SourceExplicit,
		},
	}
}

func fixedArtifactResolverErr(err error) ArtifactResolver {
	return &fixedArtifactResolver{err: err}
}

type fakeFileInspector struct {
	files map[string]os.FileInfo
}

func newFakeFileInspector() *fakeFileInspector {
	return &fakeFileInspector{files: make(map[string]os.FileInfo)}
}

func (f *fakeFileInspector) addFile(path string, isDir bool) {
	f.files[path] = &fakeFileInfo{name: filepath.Base(path), isDir: isDir}
}

func (f *fakeFileInspector) Stat(path string) (os.FileInfo, error) {
	info, ok := f.files[path]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	return info, nil
}

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (fi *fakeFileInfo) Name() string       { return fi.name }
func (fi *fakeFileInfo) Size() int64        { return 0 }
func (fi *fakeFileInfo) Mode() os.FileMode  { return 0o644 }
func (fi *fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *fakeFileInfo) IsDir() bool        { return fi.isDir }
func (fi *fakeFileInfo) Sys() interface{}   { return nil }

type fakeRuntimeHost struct {
	paths util.RuntimePaths
}

func newFakeRuntimeHost(paths util.RuntimePaths) *fakeRuntimeHost {
	return &fakeRuntimeHost{paths: paths}
}

func (h *fakeRuntimeHost) Descriptor() platform.RuntimeDescriptor {
	return platform.RuntimeDescriptor{
		Host:  platform.HostPlatformWindows,
		Kind:  platform.RuntimeKindNativeProcess,
		Guest: platform.GuestPlatformUnknown,
	}
}

func (h *fakeRuntimeHost) Capabilities() *runtimehost.HostCapabilities {
	return &runtimehost.HostCapabilities{}
}

func (h *fakeRuntimeHost) Paths() util.RuntimePaths {
	return h.paths
}

func (h *fakeRuntimeHost) Processes() runtimehost.ProcessSupervisor {
	return nil
}

func (h *fakeRuntimeHost) RuntimeInstanceID() string {
	return "test-instance"
}
