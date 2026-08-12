//go:build linux && !android

package fileops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/pkg/util"
)

func TestResolver_ResolveRead_RelativePath(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveRead("subdir/file.txt")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/subdir/file.txt", result)
}

func TestResolver_ResolveRead_AbsolutePath(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveRead("/workspace/data/test.txt")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/data/test.txt", result)
}

func TestResolver_ResolveRead_WorkspaceURI(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveRead("amitia://workspace/project/file.txt")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/project/file.txt", result)
}

func TestResolver_ResolveRead_TempURI(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveRead("amitia://temp/cache/data.bin")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/cache/data.bin", result)
}

func TestResolver_ResolveRead_TraversalBlocked(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	_, err := resolver.ResolveRead("/workspace/../../../etc/passwd")
	assert.Error(t, err)
}

func TestResolver_ResolveWrite_OutsideWorkspace(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "",
	}
	policy := DefaultPolicy("/workspace", "")
	resolver := NewResolver(paths, policy)

	_, err := resolver.ResolveWrite("/etc/passwd")
	assert.Error(t, err)
}

func TestResolver_ResolveWrite_DeniedMutationRoot(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	_, err := resolver.ResolveWrite("/proc/version")
	assert.Error(t, err)
}

func TestResolver_ResolveWrite_ValidWorkspace(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveWrite("/workspace/output.txt")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/output.txt", result)
}

func TestResolver_ResolveCreate_EmptyPath(t *testing.T) {
	paths := util.RuntimePaths{
		WorkspaceDir: "/workspace",
		TempDir:      "/tmp",
	}
	policy := DefaultPolicy("/workspace", "/tmp")
	resolver := NewResolver(paths, policy)

	result, err := resolver.ResolveCreate("")
	require.NoError(t, err)
	assert.Equal(t, "/workspace", result)
}

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		path   string
		root   string
		expect bool
	}{
		{"/workspace", "/workspace", true},
		{"/workspace/sub", "/workspace", true},
		{"/workspace/../etc", "/workspace", false},
		{"/etc/passwd", "/workspace", false},
		{"/workspace", "/workspace/sub", false},
	}
	for _, tt := range tests {
		result := isSubPath(tt.path, tt.root)
		assert.Equal(t, tt.expect, result, "isSubPath(%q, %q)", tt.path, tt.root)
	}
}
