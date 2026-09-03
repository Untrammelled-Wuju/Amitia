package workspace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalBackend struct {
	defaultRoot string
}

func NewLocalBackend(defaultRoot string) *LocalBackend {
	return &LocalBackend{defaultRoot: defaultRoot}
}

func (b *LocalBackend) Kind() WorkspaceKind {
	return WorkspaceKindLocal
}

func (b *LocalBackend) rootForMount(mount WorkspaceMount) string {
	if root := strings.TrimSpace(mount.LocalRoot); root != "" {
		return root
	}
	return b.defaultRoot
}

func (b *LocalBackend) Stat(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateRead(root, path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrFileNotFound, path)
		}
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return b.buildEntry(mount, path, info), nil
}

func (b *LocalBackend) List(ctx context.Context, mount WorkspaceMount, path string, opts ListOptions) ([]WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateRead(root, path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrDirectoryNotFound, path)
		}
		return nil, fmt.Errorf("%w: %v", ErrListFailed, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %q", ErrNotDirectory, path)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrListFailed, err)
	}
	limit := MaxListEntries
	if opts.Limit > 0 && opts.Limit < limit {
		limit = opts.Limit
	}
	result := make([]WorkspaceEntry, 0, len(entries))
	for _, e := range entries {
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
		result = append(result, b.buildEntry(mount, childPath, info))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type == WorkspaceEntryTypeDirectory
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (b *LocalBackend) Read(ctx context.Context, mount WorkspaceMount, path string, opts ReadOptions) (ReadResult, error) {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateRead(root, path)
	if err != nil {
		return ReadResult{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadResult{}, fmt.Errorf("%w: %q", ErrFileNotFound, path)
		}
		return ReadResult{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	if info.IsDir() {
		return ReadResult{}, fmt.Errorf("%w: %q", ErrNotFile, path)
	}
	maxBytes := int64(MaxDirectRead)
	if opts.MaxBytes > 0 && opts.MaxBytes < maxBytes {
		maxBytes = opts.MaxBytes
	}
	// A default read remains bounded by MaxDirectRead. Explicit ranged reads
	// (Offset and/or MaxBytes supplied) are allowed against larger files and
	// return at most maxBytes, which is required by CopyTo and line-range tools.
	explicitRange := opts.Offset > 0 || opts.MaxBytes > 0
	if !explicitRange && info.Size() > maxBytes {
		return ReadResult{}, fmt.Errorf("%w: file size %d exceeds max %d", ErrResourceTooLarge, info.Size(), maxBytes)
	}

	file, err := os.Open(target)
	if err != nil {
		return ReadResult{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer file.Close()

	if opts.Offset > 0 {
		if _, err := file.Seek(opts.Offset, io.SeekStart); err != nil {
			return ReadResult{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		return ReadResult{}, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	return ReadResult{
		Content: data,
		IsText:  true,
	}, nil
}

func (b *LocalBackend) Write(ctx context.Context, mount WorkspaceMount, path string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateCreate(root, path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if !opts.Overwrite {
		if _, err := os.Lstat(target); err == nil {
			return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrAlreadyExists, path)
		}
	}
	data, err := io.ReadAll(io.LimitReader(src, MaxSingleWrite+1))
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}
	if int64(len(data)) > MaxSingleWrite {
		return WorkspaceEntry{}, fmt.Errorf("%w: write size %d exceeds max %d", ErrResourceTooLarge, len(data), MaxSingleWrite)
	}
	if opts.Atomic {
		dir := filepath.Dir(target)
		tmp, err := os.CreateTemp(dir, ".workspace-*")
		if err != nil {
			return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
		}
		tmpName := tmp.Name()
		_, writeErr := tmp.Write(data)
		closeErr := tmp.Close()
		if writeErr != nil {
			os.Remove(tmpName)
			return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, writeErr)
		}
		if closeErr != nil {
			os.Remove(tmpName)
			return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, closeErr)
		}
		if err := os.Rename(tmpName, target); err != nil {
			os.Remove(tmpName)
			return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
		}
	} else {
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}
	return b.buildEntry(mount, path, info), nil
}

func (b *LocalBackend) Mkdir(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateCreate(root, path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCreateFailed, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCreateFailed, err)
	}
	return b.buildEntry(mount, path, info), nil
}

func (b *LocalBackend) Rename(ctx context.Context, mount WorkspaceMount, path string, newName string) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	sourcePath, err := ResolveAndValidateMutation(root, path)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	if err := validateSegment(newName); err != nil {
		return WorkspaceEntry{}, err
	}
	parentDir := filepath.Dir(sourcePath)
	targetPath := filepath.Join(parentDir, newName)
	targetClean := filepath.Clean(targetPath)
	if err := assertWithinRoot(root, targetClean); err != nil {
		return WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetClean); err == nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrAlreadyExists, newName)
	}
	if err := os.Rename(sourcePath, targetClean); err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrRenameFailed, err)
	}
	rel := filepath.Join(filepath.Dir(path), newName)
	info, err := os.Stat(targetClean)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrRenameFailed, err)
	}
	return b.buildEntry(mount, rel, info), nil
}

func (b *LocalBackend) Move(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	srcPath, err := ResolveAndValidateMutation(root, source)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstDirPath, err := ResolveAndValidateRead(root, destinationDir)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstInfo, err := os.Stat(dstDirPath)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrMoveFailed, err)
	}
	if !dstInfo.IsDir() {
		return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrNotDirectory, destinationDir)
	}
	baseName := filepath.Base(srcPath)
	targetPath := filepath.Join(dstDirPath, baseName)
	if err := assertWithinRoot(root, targetPath); err != nil {
		return WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrAlreadyExists, baseName)
	}
	if err := os.Rename(srcPath, targetPath); err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrMoveFailed, err)
	}
	rel := filepath.Join(destinationDir, baseName)
	info, err := os.Stat(targetPath)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrMoveFailed, err)
	}
	return b.buildEntry(mount, rel, info), nil
}

func (b *LocalBackend) Copy(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	root := b.rootForMount(mount)
	srcPath, err := ResolveAndValidateRead(root, source)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstDirPath, err := ResolveAndValidateRead(root, destinationDir)
	if err != nil {
		return WorkspaceEntry{}, err
	}
	dstInfo, osErr := os.Stat(dstDirPath)
	if osErr != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCopyFailed, osErr)
	}
	if !dstInfo.IsDir() {
		return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrNotDirectory, destinationDir)
	}
	baseName := filepath.Base(srcPath)
	targetPath := filepath.Join(dstDirPath, baseName)
	if err := assertWithinRoot(root, targetPath); err != nil {
		return WorkspaceEntry{}, err
	}
	if _, err := os.Lstat(targetPath); err == nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %q", ErrAlreadyExists, baseName)
	}
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCopyFailed, err)
	}
	if srcInfo.IsDir() {
		return WorkspaceEntry{}, fmt.Errorf("%w: directory copy not supported in B55", ErrOperationUnsupported)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCopyFailed, err)
	}
	if int64(len(data)) > MaxSingleWrite {
		return WorkspaceEntry{}, fmt.Errorf("%w: copy size %d exceeds max %d", ErrResourceTooLarge, len(data), MaxSingleWrite)
	}
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCopyFailed, err)
	}
	rel := filepath.Join(destinationDir, baseName)
	info, err := os.Stat(targetPath)
	if err != nil {
		return WorkspaceEntry{}, fmt.Errorf("%w: %v", ErrCopyFailed, err)
	}
	return b.buildEntry(mount, rel, info), nil
}

func (b *LocalBackend) Delete(ctx context.Context, mount WorkspaceMount, path string, opts DeleteOptions) error {
	root := b.rootForMount(mount)
	target, err := ResolveAndValidateMutation(root, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %q", ErrFileNotFound, path)
		}
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	if info.IsDir() {
		if !opts.Recursive {
			entries, readErr := os.ReadDir(target)
			if readErr != nil {
				return fmt.Errorf("%w: %v", ErrDeleteFailed, readErr)
			}
			if len(entries) > 0 {
				return fmt.Errorf("%w: %q", ErrDirectoryNotEmpty, path)
			}
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
		}
		return nil
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	return nil
}

func (b *LocalBackend) buildEntry(mount WorkspaceMount, rel string, info os.FileInfo) WorkspaceEntry {
	entryType := WorkspaceEntryTypeFile
	if info.IsDir() {
		entryType = WorkspaceEntryTypeDirectory
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
	return WorkspaceEntry{
		URI:        uri,
		Name:       info.Name(),
		Type:       entryType,
		SizeBytes:  size,
		ModifiedAt: &modTime,
		Readable:   true,
		Writable:   !mount.ReadOnly,
	}
}
