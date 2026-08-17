//go:build linux && !android

package fileops

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/pkg/util"
)

const (
	maxSymlinkResolution = 32
	golangDefaultMode    = 0o600
)

type Service struct {
	paths  util.RuntimePaths
	policy Policy
}

func NewService(paths util.RuntimePaths, policy Policy) *Service {
	return &Service{
		paths:  paths,
		policy: policy,
	}
}

func (s *Service) Stat(path string) (StatResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveRead(path)
	if err != nil {
		return StatResult{}, err
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return StatResult{}, ErrFileNotFound(path)
		}
		return StatResult{}, ErrIOFailed(err.Error())
	}

	result := s.infoToStatResult(info, resolved)
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(resolved)
		if err == nil {
			result.LinkTarget = target
			result.IsSymlink = true
		}
	}

	return result, nil
}

func (s *Service) List(path string, opts ListOptions) ([]StatResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveRead(path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound(path)
		}
		return nil, ErrIOFailed(err.Error())
	}

	if !info.IsDir() {
		return nil, ErrPathDenied(path, "not a directory")
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, ErrIOFailed(err.Error())
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = s.policy.MaxListEntries
	}
	if limit > s.policy.MaxListEntries {
		limit = s.policy.MaxListEntries
	}

	results := make([]StatResult, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(resolved, name)
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		stat := s.infoToStatResult(info, fullPath)
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(fullPath)
			if err == nil {
				stat.LinkTarget = target
				stat.IsSymlink = true
				if opts.FollowSymlinks {
					if targetInfo, err := os.Stat(fullPath); err == nil {
						stat.IsDir = targetInfo.IsDir()
					}
				}
			}
		}

		results = append(results, stat)
		if len(results) >= limit {
			break
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func (s *Service) Read(path string, opts ReadOptions) (ReadResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveRead(path)
	if err != nil {
		return ReadResult{}, err
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadResult{}, ErrFileNotFound(path)
		}
		return ReadResult{}, ErrIOFailed(err.Error())
	}

	if info.IsDir() {
		return ReadResult{}, ErrPathDenied(path, "cannot read directory")
	}

	maxBytes := opts.MaxBytes
	if maxBytes <= 0 || maxBytes > s.policy.MaxReadBytes {
		maxBytes = s.policy.MaxReadBytes
	}

	f, err := os.Open(resolved)
	if err != nil {
		return ReadResult{}, ErrIOFailed(err.Error())
	}
	defer f.Close()

	if opts.Offset > 0 {
		if _, err := f.Seek(opts.Offset, io.SeekStart); err != nil {
			return ReadResult{}, ErrIOFailed(err.Error())
		}
	}

	limitReader := io.LimitReader(f, maxBytes+1)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return ReadResult{}, ErrIOFailed(err.Error())
	}

	eof := true
	if len(data) > int(maxBytes) {
		data = data[:maxBytes]
		eof = false
	}

	isBinary := !utf8.Valid(data)

	return ReadResult{
		Path:      path,
		Offset:    opts.Offset,
		BytesRead: len(data),
		Content:   data,
		EOF:       eof,
		IsBinary:  isBinary,
	}, nil
}

func (s *Service) Write(path string, data []byte, opts WriteOptions) (StatResult, error) {
	if int64(len(data)) > s.policy.MaxWriteBytes {
		return StatResult{}, ErrWriteLimitExceeded(s.policy.MaxWriteBytes)
	}

	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(path)
	if err != nil {
		return StatResult{}, err
	}

	info, err := os.Lstat(resolved)
	if err == nil {
		if !opts.Overwrite {
			return StatResult{}, ErrFileExists(path)
		}
		if info.IsDir() {
			return StatResult{}, ErrPathDenied(path, "cannot overwrite directory")
		}
	} else if !os.IsNotExist(err) {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if opts.CreateParents {
		dir := filepath.Dir(resolved)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return StatResult{}, ErrIOFailed(err.Error())
		}
	}

	mode := opts.Mode
	if mode == 0 {
		mode = golangDefaultMode
	}

	tmpFile := resolved + ".amitia.tmp"
	f, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpFile)
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if err := os.Rename(tmpFile, resolved); err != nil {
		os.Remove(tmpFile)
		return StatResult{}, ErrIOFailed(err.Error())
	}

	return s.Stat(path)
}

func (s *Service) Append(path string, data []byte) (StatResult, error) {
	if int64(len(data)) > s.policy.MaxWriteBytes {
		return StatResult{}, ErrWriteLimitExceeded(s.policy.MaxWriteBytes)
	}

	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(path)
	if err != nil {
		return StatResult{}, err
	}

	info, err := os.Lstat(resolved)
	if err == nil && info.IsDir() {
		return StatResult{}, ErrPathDenied(path, "cannot append to directory")
	}

	f, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	return s.Stat(path)
}

func (s *Service) Mkdir(path string, opts MkdirOptions) (StatResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveCreate(path)
	if err != nil {
		return StatResult{}, err
	}

	mode := opts.Mode
	if mode == 0 {
		mode = 0o700
	}

	if opts.Recursive {
		if err := os.MkdirAll(resolved, os.FileMode(mode)); err != nil {
			return StatResult{}, ErrIOFailed(err.Error())
		}
	} else {
		if err := os.Mkdir(resolved, os.FileMode(mode)); err != nil {
			if os.IsExist(err) {
				return StatResult{}, ErrFileExists(path)
			}
			return StatResult{}, ErrIOFailed(err.Error())
		}
	}

	return s.Stat(path)
}

func (s *Service) Touch(path string) (StatResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(path)
	if err != nil {
		return StatResult{}, err
	}

	info, err := os.Lstat(resolved)
	if err == nil {
		now := time.Now()
		if err := os.Chtimes(resolved, now, now); err != nil {
			return StatResult{}, ErrIOFailed(err.Error())
		}
		return s.fileStatResult(info, resolved, path), nil
	}

	if !os.IsNotExist(err) {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	dir := filepath.Dir(resolved)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return StatResult{}, ErrIOFailed(err.Error())
		}
	}

	f, err := os.OpenFile(resolved, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}
	f.Close()

	return s.Stat(path)
}

func (s *Service) Copy(source, destination string, opts CopyOptions) (StatResult, error) {
	srcResolver := NewResolver(s.paths, s.policy)
	srcResolved, err := srcResolver.ResolveRead(source)
	if err != nil {
		return StatResult{}, err
	}

	dstResolver := NewResolver(s.paths, s.policy)
	dstResolved, err := dstResolver.ResolveWrite(destination)
	if err != nil {
		return StatResult{}, err
	}

	srcInfo, err := os.Lstat(srcResolved)
	if err != nil {
		if os.IsNotExist(err) {
			return StatResult{}, ErrFileNotFound(source)
		}
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if srcInfo.IsDir() {
		if !opts.Recursive {
			return StatResult{}, ErrPathDenied(source, "source is directory, recursive copy required")
		}
		return s.copyRecursive(srcResolved, dstResolved, opts)
	}

	if err := s.copyFile(srcResolved, dstResolved); err != nil {
		return StatResult{}, err
	}

	return s.Stat(destination)
}

func (s *Service) copyRecursive(src, dst string, opts CopyOptions) (StatResult, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	type copyTask struct {
		srcPath string
		dstPath string
		depth   int
	}

	tasks := []copyTask{{srcPath: src, dstPath: dst, depth: 0}}
	fileCount := 0

	for len(tasks) > 0 {
		task := tasks[0]
		tasks = tasks[1:]

		if task.depth > s.policy.MaxCopyDepth {
			return StatResult{}, ErrRecursiveLimit("max copy depth exceeded")
		}

		entries, err := os.ReadDir(task.srcPath)
		if err != nil {
			return StatResult{}, ErrIOFailed(err.Error())
		}

		for _, entry := range entries {
			srcEntry := filepath.Join(task.srcPath, entry.Name())
			dstEntry := filepath.Join(task.dstPath, entry.Name())

			info, err := os.Lstat(srcEntry)
			if err != nil {
				continue
			}

			if info.IsDir() {
				if err := os.MkdirAll(dstEntry, info.Mode().Perm()); err != nil {
					return StatResult{}, ErrIOFailed(err.Error())
				}
				tasks = append(tasks, copyTask{srcPath: srcEntry, dstPath: dstEntry, depth: task.depth + 1})
			} else {
				fileCount++
				if fileCount > s.policy.MaxCopyFiles {
					return StatResult{}, ErrRecursiveLimit("max copy files exceeded")
				}
				if err := s.copyFile(srcEntry, dstEntry); err != nil {
					return StatResult{}, err
				}
			}
		}
	}

	return s.Stat(dst)
}

func (s *Service) copyFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return ErrIOFailed(err.Error())
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return ErrIOFailed(err.Error())
		}
		return os.Symlink(target, dst)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return ErrIOFailed(err.Error())
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return ErrIOFailed(err.Error())
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return ErrIOFailed(err.Error())
	}

	return nil
}

func (s *Service) Move(source, destination string, opts MoveOptions) (StatResult, error) {
	srcResolver := NewResolver(s.paths, s.policy)
	srcResolved, err := srcResolver.ResolveRead(source)
	if err != nil {
		return StatResult{}, err
	}

	dstResolver := NewResolver(s.paths, s.policy)
	dstResolved, err := dstResolver.ResolveWrite(destination)
	if err != nil {
		return StatResult{}, err
	}

	if !opts.Overwrite {
		if _, err := os.Lstat(dstResolved); err == nil {
			return StatResult{}, ErrFileExists(destination)
		}
	}

	if err := os.Rename(srcResolved, dstResolved); err != nil {
		if isCrossDeviceError(err) {
			if info, serr := os.Lstat(srcResolved); serr == nil && info.IsDir() {
				_, cerr := s.copyRecursive(srcResolved, dstResolved, CopyOptions{Recursive: true, Overwrite: opts.Overwrite})
				if cerr != nil {
					return StatResult{}, cerr
				}
			} else {
				if cerr := s.copyFile(srcResolved, dstResolved); cerr != nil {
					return StatResult{}, cerr
				}
			}
			if rerr := os.RemoveAll(srcResolved); rerr != nil {
				return StatResult{}, ErrIOFailed(rerr.Error())
			}
		} else {
			return StatResult{}, ErrIOFailed(err.Error())
		}
	}

	return s.Stat(destination)
}

func (s *Service) Delete(path string, opts DeleteOptions) error {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(path)
	if err != nil {
		return err
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return ErrIOFailed(err.Error())
	}

	if info.IsDir() && !opts.Recursive {
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return ErrIOFailed(err.Error())
		}
		if len(entries) > 0 {
			return ErrPathDenied(path, "directory not empty, recursive delete required")
		}
	}

	if err := os.RemoveAll(resolved); err != nil {
		return ErrIOFailed(err.Error())
	}

	return nil
}

func (s *Service) Search(root string, opts SearchOptions) ([]StatResult, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveRead(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound(root)
		}
		return nil, ErrIOFailed(err.Error())
	}

	if !info.IsDir() {
		return nil, ErrPathDenied(root, "search root must be a directory")
	}

	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = s.policy.MaxSearchDepth
	}
	if maxDepth > s.policy.MaxSearchDepth {
		maxDepth = s.policy.MaxSearchDepth
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = s.policy.MaxSearchResults
	}
	if limit > s.policy.MaxSearchResults {
		limit = s.policy.MaxSearchResults
	}

	var results []StatResult
	seen := make(map[string]bool)

	type searchTask struct {
		path  string
		depth int
	}

	tasks := []searchTask{{path: resolved, depth: 0}}

	for len(tasks) > 0 && len(results) < limit {
		task := tasks[0]
		tasks = tasks[1:]

		if task.depth > maxDepth {
			continue
		}

		entries, err := os.ReadDir(task.path)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			fullPath := filepath.Join(task.path, name)

			if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
				continue
			}

			if len(results) >= limit {
				break
			}

			if strings.Contains(name, opts.Query) {
				if !seen[fullPath] {
					seen[fullPath] = true
					info, err := os.Lstat(fullPath)
					if err == nil {
						stat := s.infoToStatResult(info, fullPath)
						results = append(results, stat)
					}
				}
			}

			isDir := entry.IsDir()
			if !isDir {
				info, err := os.Lstat(fullPath)
				if err == nil && info.Mode()&os.ModeSymlink != 0 {
					if opts.FollowSymlinks {
						if targetInfo, terr := os.Stat(fullPath); terr == nil && targetInfo.IsDir() {
							isDir = true
						}
					}
				}
			}

			if isDir {
				tasks = append(tasks, searchTask{path: fullPath, depth: task.depth + 1})
			}
		}
	}

	return results, nil
}

func (s *Service) Chmod(path string, mode uint32) (StatResult, error) {
	if !s.policy.AllowChmod {
		return StatResult{}, ErrChmodDenied(path)
	}

	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(path)
	if err != nil {
		return StatResult{}, err
	}

	if err := os.Chmod(resolved, os.FileMode(mode)); err != nil {
		if os.IsNotExist(err) {
			return StatResult{}, ErrFileNotFound(path)
		}
		return StatResult{}, ErrIOFailed(err.Error())
	}

	return s.Stat(path)
}

func (s *Service) Readlink(path string) (string, error) {
	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveRead(path)
	if err != nil {
		return "", err
	}

	target, err := os.Readlink(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrFileNotFound(path)
		}
		return "", ErrIOFailed(err.Error())
	}

	return target, nil
}

func (s *Service) Symlink(target, linkPath string) (StatResult, error) {
	if !s.policy.AllowSymlinkCreate {
		return StatResult{}, ErrSymlinkDenied(linkPath)
	}

	resolver := NewResolver(s.paths, s.policy)
	resolved, err := resolver.ResolveWrite(linkPath)
	if err != nil {
		return StatResult{}, err
	}

	if _, err := os.Lstat(resolved); err == nil {
		return StatResult{}, ErrFileExists(linkPath)
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	if err := os.Symlink(target, resolved); err != nil {
		return StatResult{}, ErrIOFailed(err.Error())
	}

	return s.Stat(linkPath)
}

func (s *Service) infoToStatResult(info os.FileInfo, fullPath string) StatResult {
	result := s.fileStatResult(info, fullPath, fullPath)
	return result
}

func (s *Service) fileStatResult(info os.FileInfo, fullPath, originalPath string) StatResult {
	if info == nil {
		var err error
		info, err = os.Lstat(fullPath)
		if err != nil {
			return StatResult{Path: originalPath}
		}
	}

	fileType := "file"
	if info.IsDir() {
		fileType = "directory"
	} else if info.Mode()&os.ModeSymlink != 0 {
		fileType = "symlink"
	}

	return StatResult{
		Path:    originalPath,
		Name:    info.Name(),
		Type:    fileType,
		Size:    info.Size(),
		Mode:    uint32(info.Mode().Perm()),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}
}

func isCrossDeviceError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cross-device link") ||
		strings.Contains(err.Error(), "invalid cross-device link")
}
