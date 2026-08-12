//go:build linux && !android

package fileops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/u-ai/backend/pkg/util"
)

func setupTestDir(t *testing.T) (util.RuntimePaths, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	temp := filepath.Join(tmpDir, "temp")
	err := os.MkdirAll(workspace, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(temp, 0755)
	require.NoError(t, err)
	paths := util.RuntimePaths{
		WorkspaceDir: workspace,
		TempDir:      temp,
		CacheDir:     filepath.Join(tmpDir, "cache"),
	}
	return paths, func() {}
}

func TestService_AndWrite(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	result, err := svc.Write("/workspace/test.txt", []byte("hello world"), WriteOptions{Overwrite: true})
	require.NoError(t, err)
	assert.Equal(t, "test.txt", result.Name)
	assert.Equal(t, int64(11), result.Size)
	assert.Equal(t, "file", result.Type)
}

func TestService_Stat(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/stat_test.txt", []byte("content"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Stat("/workspace/stat_test.txt")
	require.NoError(t, err)
	assert.Equal(t, "stat_test.txt", result.Name)
	assert.Equal(t, int64(7), result.Size)
	assert.False(t, result.IsDir)
}

func TestService_ReadWithOffset(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	content := "0123456789"
	_, err := svc.Write("/workspace/offset_test.txt", []byte(content), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Read("/workspace/offset_test.txt", ReadOptions{Offset: 5, MaxBytes: 3})
	require.NoError(t, err)
	assert.Equal(t, "567", string(result.Content))
	assert.False(t, result.EOF)
}

func TestService_Read(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Append("/workspace/append_test.txt", []byte(" initial"))
	require.NoError(t, err)

	result, err := svc.Append("/workspace/append_test.txt", []byte(" appended"))
	require.NoError(t, err)
	assert.Equal(t, "append_test.txt", result.Name)

	readResult, err := svc.Read("/workspace/append_test.txt", ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, " initial appended", string(readResult.Content))
}

func TestService_ListDirectory(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/list.txt", []byte("a"), WriteOptions{Overwrite: true})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/list2.txt", []byte("b"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	results, err := svc.List("/workspace", ListOptions{IncludeHidden: false})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestService_MkdirRecursive(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	result, err := svc.Mkdir("/workspace/a/b/c", MkdirOptions{Recursive: true})
	require.NoError(t, err)
	assert.Equal(t, "c", result.Name)
	assert.True(t, result.IsDir)
}

func TestService_CopyFile(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/src.txt", []byte("copy me"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Copy("/workspace/src.txt", "/workspace/dst.txt", CopyOptions{})
	require.NoError(t, err)
	assert.Equal(t, "dst.txt", result.Name)

	readResult, err := svc.Read("/workspace/dst.txt", ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, "copy me", string(readResult.Content))
}

func TestService_CopyRecursive(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Mkdir("/workspace/srcdir", MkdirOptions{Recursive: true})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/srcdir/inner.txt", []byte("inner"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Copy("/workspace/srcdir", "/workspace/dstdir", CopyOptions{Recursive: true})
	require.NoError(t, err)
	assert.True(t, result.IsDir)

	statResult, err := svc.Stat("/workspace/dstdir/inner.txt")
	require.NoError(t, err)
	assert.Equal(t, "inner.txt", statResult.Name)
}

func TestService_MoveWithOverwrite(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/old.txt", []byte("content"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Move("/workspace/old.txt", "/workspace/new.txt", MoveOptions{Overwrite: true})
	require.NoError(t, err)
	assert.Equal(t, "new.txt", result.Name)

	_, err = svc.Stat("/workspace/old.txt")
	assert.Error(t, err)
}

func TestService_DeleteRecursive(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Mkdir("/workspace/to_delete", MkdirOptions{})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/to_delete/file.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	err = svc.Delete("/workspace/to_delete", DeleteOptions{Recursive: true})
	require.NoError(t, err)

	_, err = svc.Stat("/workspace/to_delete")
	assert.Error(t, err)
}

func TestService_DeleteNonRecursiveFailsOnNonEmpty(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Mkdir("/workspace/nonempty", MkdirOptions{})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/nonempty/file.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	err = svc.Delete("/workspace/nonempty", DeleteOptions{Recursive: false})
	assert.Error(t, err)
}

func TestService_SearchDirectory(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/search_a.txt", []byte("x"), WriteOptions{Overwrite: true})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/search_b.log", []byte("y"), WriteOptions{Overwrite: true})
	require.NoError(t, err)
	_, err = svc.Write("/workspace/other.txt", []byte("z"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	results, err := svc.Search("/workspace", SearchOptions{Query: "search_", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestService_TouchCreatesFile(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	result, err := svc.Touch("/workspace/touched.txt")
	require.NoError(t, err)
	assert.Equal(t, "touched.txt", result.Name)
	assert.Equal(t, int64(0), result.Size)
}

func TestService_ChmodDeniedByPolicy(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/chmod_test.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	_, err = svc.Chmod("/workspace/chmod_test.txt", 0755)
	assert.Error(t, err)
}

func TestService_ChmodAllowed(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	policy.AllowChmod = true
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/chmod_allow.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	result, err := svc.Chmod("/workspace/chmod_allow.txt", 0755)
	require.NoError(t, err)
	assert.Equal(t, "chmod_allow.txt", result.Name)
}

func TestService_SymlinkDeniedByPolicy(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/target.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	_, err = svc.Symlink("/workspace/target.txt", "/workspace/link.txt")
	assert.Error(t, err)
}

func TestService_Readlink(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	policy.AllowSymlinkCreate = true
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/link_target.txt", []byte("data"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	_, err = svc.Symlink("/workspace/link_target.txt", "/workspace/my_link")
	require.NoError(t, err)

	target, err := svc.Readlink("/workspace/my_link")
	require.NoError(t, err)
	assert.Equal(t, "/workspace/link_target.txt", target)
}

func TestService_OverwritePolicy(t *testing.T) {
	paths, cleanup := setupTestDir(t)
	defer cleanup()

	policy := DefaultPolicy(paths.WorkspaceDir, paths.TempDir)
	svc := NewService(paths, policy)

	_, err := svc.Write("/workspace/overwrite.txt", []byte("first"), WriteOptions{Overwrite: true})
	require.NoError(t, err)

	_, err = svc.Write("/workspace/overwrite.txt", []byte("second"), WriteOptions{Overwrite: false})
	assert.Error(t, err)
}
