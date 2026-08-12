package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/workspace"
)

const (
	IsolatedRootPrefix = "isolated"
	StagingDirName     = ".staging"
	DefaultStagingTTL  = 30 * time.Minute
)

type IsolatedRootResolver interface {
	ResolveRoot(mount workspace.WorkspaceMount) (string, error)
	DataRoot() string
}

type AmitiaDataRootResolver struct {
	dataRoot string
	mu       sync.Mutex
}

func NewAmitiaDataRootResolver(dataRoot string) *AmitiaDataRootResolver {
	return &AmitiaDataRootResolver{dataRoot: dataRoot}
}

func (r *AmitiaDataRootResolver) DataRoot() string {
	return r.dataRoot
}

func (r *AmitiaDataRootResolver) ResolveRoot(mount workspace.WorkspaceMount) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg, err := ParseGitMountConfig(mount.BackendConfig)
	if err != nil {
		return "", fmt.Errorf("%w: invalid isolated mount config: %v", ErrIsolatedCreateFailed, err)
	}
	return filepath.Join(r.dataRoot, "workspaces", cfg.RootKey), nil
}

func (r *AmitiaDataRootResolver) StagingRoot(opID string) string {
	return filepath.Join(r.dataRoot, "workspaces", IsolatedRootPrefix, StagingDirName, opID)
}

func (r *AmitiaDataRootResolver) IsolatedRoot(mountID string) string {
	return filepath.Join(r.dataRoot, "workspaces", IsolatedRootPrefix, mountID)
}

type IsolatedBackend struct {
	roots IsolatedRootResolver
}

func NewIsolatedBackend(roots IsolatedRootResolver) *IsolatedBackend {
	return &IsolatedBackend{roots: roots}
}

func (b *IsolatedBackend) Kind() workspace.WorkspaceKind {
	return workspace.WorkspaceKindIsolated
}

func (b *IsolatedBackend) rootForMount(mount workspace.WorkspaceMount) (string, error) {
	return b.roots.ResolveRoot(mount)
}

func (b *IsolatedBackend) hideGitInternal(rel string) bool {
	if rel == ".git" || strings.HasPrefix(rel, ".git/") || strings.HasSuffix(rel, "/.git") {
		return true
	}
	if strings.Contains(rel, "/.git/") {
		return true
	}
	return false
}

func (b *IsolatedBackend) Stat(ctx context.Context, mount workspace.WorkspaceMount, path string) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(path) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	target, err := workspace.ResolveAndValidateRead(root, path)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrFileNotFound, path)
		}
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrReadFailed, err)
	}
	return buildIsolatedEntry(mount, path, info), nil
}

func (b *IsolatedBackend) List(ctx context.Context, mount workspace.WorkspaceMount, path string, opts workspace.ListOptions) ([]workspace.WorkspaceEntry, error) {
	root, err := b.rootForMount(mount)
	if err != nil {
		return nil, err
	}
	target, err := workspace.ResolveAndValidateRead(root, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", workspace.ErrDirectoryNotFound, path)
		}
		return nil, fmt.Errorf("%w: %v", workspace.ErrListFailed, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", workspace.ErrNotDirectory, path)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", workspace.ErrListFailed, err)
	}
	limit := workspace.MaxListEntries
	if opts.Limit > 0 && opts.Limit < limit {
		limit = opts.Limit
	}
	result := make([]workspace.WorkspaceEntry, 0, len(entries))
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if len(result) >= limit {
			break
		}
		fullPath := filepath.Join(target, e.Name())
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		childPath := path
		if childPath != "" && !strings.HasSuffix(childPath, "/") {
			childPath += "/"
		}
		childPath += e.Name()
		result = append(result, buildIsolatedEntry(mount, childPath, info))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type == workspace.WorkspaceEntryTypeDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (b *IsolatedBackend) Read(ctx context.Context, mount workspace.WorkspaceMount, path string, opts workspace.ReadOptions) (workspace.ReadResult, error) {
	if b.hideGitInternal(path) {
		return workspace.ReadResult{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.ReadResult{}, err
	}
	target, err := workspace.ResolveAndValidateRead(root, path)
	if err != nil {
		return workspace.ReadResult{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return workspace.ReadResult{}, fmt.Errorf("%w: %q", workspace.ErrFileNotFound, path)
		}
		return workspace.ReadResult{}, fmt.Errorf("%w: %v", workspace.ErrReadFailed, err)
	}
	if info.IsDir() {
		return workspace.ReadResult{}, fmt.Errorf("%w: %q", workspace.ErrNotFile, path)
	}
	maxBytes := int64(workspace.MaxDirectRead)
	if opts.MaxBytes > 0 && opts.MaxBytes < maxBytes {
		maxBytes = opts.MaxBytes
	}
	if info.Size() > maxBytes {
		return workspace.ReadResult{}, fmt.Errorf("%w: file size %d exceeds max %d", workspace.ErrResourceTooLarge, info.Size(), maxBytes)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return workspace.ReadResult{}, fmt.Errorf("%w: %v", workspace.ErrReadFailed, err)
	}
	if opts.Offset > 0 {
		if opts.Offset >= int64(len(data)) {
			data = nil
		} else {
			data = data[opts.Offset:]
		}
	}
	return workspace.ReadResult{Content: data, IsText: true}, nil
}

func (b *IsolatedBackend) Write(ctx context.Context, mount workspace.WorkspaceMount, path string, src io.Reader, opts workspace.WriteOptions) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(path) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	target, err := workspace.ResolveAndValidateCreate(root, path)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if !opts.Overwrite {
		if _, err := os.Lstat(target); err == nil {
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrAlreadyExists, path)
		}
	}
	data, err := io.ReadAll(io.LimitReader(src, workspace.MaxSingleWrite+1))
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, err)
	}
	if int64(len(data)) > workspace.MaxSingleWrite {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: write size %d exceeds max %d", workspace.ErrResourceTooLarge, len(data), workspace.MaxSingleWrite)
	}
	if opts.Atomic {
		dir := filepath.Dir(target)
		tmp, err := os.CreateTemp(dir, ".workspace-*")
		if err != nil {
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, err)
		}
		tmpName := tmp.Name()
		_, writeErr := tmp.Write(data)
		closeErr := tmp.Close()
		if writeErr != nil {
			os.Remove(tmpName)
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, writeErr)
		}
		if closeErr != nil {
			os.Remove(tmpName)
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, closeErr)
		}
		if err := os.Rename(tmpName, target); err != nil {
			os.Remove(tmpName)
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, err)
		}
	} else {
		if err := os.WriteFile(target, data, 0644); err != nil {
			return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, err)
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrWriteFailed, err)
	}
	return buildIsolatedEntry(mount, path, info), nil
}

func (b *IsolatedBackend) Mkdir(ctx context.Context, mount workspace.WorkspaceMount, path string) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(path) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	target, err := workspace.ResolveAndValidateCreate(root, path)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCreateFailed, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCreateFailed, err)
	}
	return buildIsolatedEntry(mount, path, info), nil
}

func (b *IsolatedBackend) Rename(ctx context.Context, mount workspace.WorkspaceMount, path string, newName string) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(path) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	sourcePath, err := workspace.ResolveAndValidateMutation(root, path)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if err := workspace.ValidateSegment(newName); err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	parentDir := filepath.Dir(sourcePath)
	targetPath := filepath.Join(parentDir, newName)
	targetClean := filepath.Clean(targetPath)
	if err := workspace.AssertWithinRoot(root, targetClean); err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetClean); err == nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrAlreadyExists, newName)
	}
	if err := os.Rename(sourcePath, targetClean); err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrRenameFailed, err)
	}
	rel := filepath.Join(filepath.Dir(path), newName)
	info, err := os.Stat(targetClean)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrRenameFailed, err)
	}
	return buildIsolatedEntry(mount, rel, info), nil
}

func (b *IsolatedBackend) Move(ctx context.Context, mount workspace.WorkspaceMount, source string, destinationDir string) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(source) || b.hideGitInternal(destinationDir) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	srcPath, err := workspace.ResolveAndValidateMutation(root, source)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	dstDirPath, err := workspace.ResolveAndValidateRead(root, destinationDir)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	dstInfo, err := os.Stat(dstDirPath)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrMoveFailed, err)
	}
	if !dstInfo.IsDir() {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrNotDirectory, destinationDir)
	}
	baseName := filepath.Base(srcPath)
	targetPath := filepath.Join(dstDirPath, baseName)
	if err := workspace.AssertWithinRoot(root, targetPath); err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrAlreadyExists, baseName)
	}
	if err := os.Rename(srcPath, targetPath); err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrMoveFailed, err)
	}
	rel := filepath.Join(destinationDir, baseName)
	info, err := os.Stat(targetPath)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrMoveFailed, err)
	}
	return buildIsolatedEntry(mount, rel, info), nil
}

func (b *IsolatedBackend) Copy(ctx context.Context, mount workspace.WorkspaceMount, source string, destinationDir string) (workspace.WorkspaceEntry, error) {
	if b.hideGitInternal(source) || b.hideGitInternal(destinationDir) {
		return workspace.WorkspaceEntry{}, workspace.ErrInternalMetadataDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	srcPath, err := workspace.ResolveAndValidateRead(root, source)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	dstDirPath, err := workspace.ResolveAndValidateRead(root, destinationDir)
	if err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	dstInfo, osErr := os.Stat(dstDirPath)
	if osErr != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCopyFailed, osErr)
	}
	if !dstInfo.IsDir() {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrNotDirectory, destinationDir)
	}
	baseName := filepath.Base(srcPath)
	targetPath := filepath.Join(dstDirPath, baseName)
	if err := workspace.AssertWithinRoot(root, targetPath); err != nil {
		return workspace.WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %q", workspace.ErrAlreadyExists, baseName)
	}
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCopyFailed, err)
	}
	if srcInfo.IsDir() {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: directory copy not supported", workspace.ErrOperationUnsupported)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCopyFailed, err)
	}
	if int64(len(data)) > workspace.MaxSingleWrite {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: copy size %d exceeds max %d", workspace.ErrResourceTooLarge, len(data), workspace.MaxSingleWrite)
	}
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCopyFailed, err)
	}
	rel := filepath.Join(destinationDir, baseName)
	info, err := os.Stat(targetPath)
	if err != nil {
		return workspace.WorkspaceEntry{}, fmt.Errorf("%w: %v", workspace.ErrCopyFailed, err)
	}
	return buildIsolatedEntry(mount, rel, info), nil
}

func (b *IsolatedBackend) Delete(ctx context.Context, mount workspace.WorkspaceMount, path string, opts workspace.DeleteOptions) error {
	if b.hideGitInternal(path) {
		return workspace.ErrInternalMetadataDenied
	}
	if path == "" {
		return workspace.ErrRootMutationDenied
	}
	root, err := b.rootForMount(mount)
	if err != nil {
		return err
	}
	target, err := workspace.ResolveAndValidateMutation(root, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %q", workspace.ErrFileNotFound, path)
		}
		return fmt.Errorf("%w: %v", workspace.ErrDeleteFailed, err)
	}
	if info.IsDir() {
		if !opts.Recursive {
			entries, readErr := os.ReadDir(target)
			if readErr != nil {
				return fmt.Errorf("%w: %v", workspace.ErrDeleteFailed, readErr)
			}
			if len(entries) > 0 {
				return fmt.Errorf("%w: %q", workspace.ErrDirectoryNotEmpty, path)
			}
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("%w: %v", workspace.ErrDeleteFailed, err)
		}
		return nil
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("%w: %v", workspace.ErrDeleteFailed, err)
	}
	return nil
}

func buildIsolatedEntry(mount workspace.WorkspaceMount, rel string, info os.FileInfo) workspace.WorkspaceEntry {
	entryType := workspace.WorkspaceEntryTypeFile
	if info.IsDir() {
		entryType = workspace.WorkspaceEntryTypeDirectory
	}
	modTime := info.ModTime().UTC()
	var size *int64
	if !info.IsDir() {
		s := info.Size()
		size = &s
	}
	uri := mount.RootURI
	if !strings.HasSuffix(uri, "/") {
		uri += "/"
	}
	uri += rel
	return workspace.WorkspaceEntry{
		URI:        uri,
		Name:       info.Name(),
		Type:       entryType,
		SizeBytes:  size,
		ModifiedAt: &modTime,
		Readable:   true,
		Writable:   !mount.ReadOnly,
	}
}

func ParseGitMountConfig(jsonStr string) (*GitMountConfig, error) {
	if jsonStr == "" {
		return &GitMountConfig{
			Mode:    GitIsolationModeClone,
			RootKey: "default",
		}, nil
	}
	var cfg GitMountConfig
	if err := parseJSON(jsonStr, &cfg); err != nil {
		return nil, err
	}
	if cfg.Mode == "" {
		cfg.Mode = GitIsolationModeClone
	}
	if cfg.RootKey == "" {
		return nil, fmt.Errorf("rootKey required")
	}
	return &cfg, nil
}

func parseJSON(s string, v interface{}) error {
	if s == "" {
		return nil
	}
	return jsonUnmarshal([]byte(s), v)
}

func init() {
	workspace.RegisterKnownKind(workspace.WorkspaceKindIsolated)
}
