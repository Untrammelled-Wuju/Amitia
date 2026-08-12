package workspace

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type webdavTransport struct {
	config RemoteMountConfig
	cred   *RemoteCredential
}

func newWebDAVTransport(ctx context.Context, config RemoteMountConfig, cred *RemoteCredential) (RemoteTransport, error) {
	if config.Host == "" {
		return nil, NewRemoteError("REMOTE_CONFIG_INVALID", "host is required", nil)
	}
	if config.Port == 0 {
		config.Port = DefaultWebDAVPort
	}
	if config.BasePath == "" {
		config.BasePath = "/"
	}

	return &webdavTransport{
		config: config,
		cred:   cred,
	}, nil
}

func (t *webdavTransport) Stat(ctx context.Context, remotePath string) (RemoteStatResult, error) {
	fullPath := t.toLocalPath(remotePath)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return RemoteStatResult{}, mapWebDAVError(err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return RemoteStatResult{}, NewRemoteError("REMOTE_SYMLINK_UNSUPPORTED", "symlink not supported", nil)
	}

	return RemoteStatResult{
		Name:       info.Name(),
		IsDir:      info.IsDir(),
		IsSymlink:  false,
		SizeBytes:  info.Size(),
		ModifiedAt: info.ModTime().UTC(),
		MIMEType:   InferRemoteMIMEType(info.Name()),
		Executable: info.Mode()&0111 != 0,
	}, nil
}

func (t *webdavTransport) List(ctx context.Context, remotePath string, limit int) (RemoteListResult, error) {
	fullPath := t.toLocalPath(remotePath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return RemoteListResult{}, mapWebDAVError(err)
	}

	result := RemoteListResult{
		Entries: make([]RemoteStatResult, 0, len(entries)),
	}

	count := 0
	for _, entry := range entries {
		if count >= limit {
			result.HasMore = true
			break
		}
		if entry.Name() == "." || entry.Name() == ".." {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			continue
		}

		result.Entries = append(result.Entries, RemoteStatResult{
			Name:       entry.Name(),
			IsDir:      entry.IsDir(),
			IsSymlink:  false,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().UTC(),
			MIMEType:   InferRemoteMIMEType(entry.Name()),
			Executable: mode&0111 != 0,
		})
		count++
	}

	return result, nil
}

func (t *webdavTransport) Read(ctx context.Context, remotePath string, offset int64, maxBytes int64) (ReadResult, error) {
	fullPath := t.toLocalPath(remotePath)
	file, err := os.Open(fullPath)
	if err != nil {
		return ReadResult{}, mapWebDAVError(err)
	}
	defer file.Close()

	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return ReadResult{}, NewRemoteError("REMOTE_READ_FAILED", "seek failed", err)
		}
	}

	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(file, buf)
	content := buf[:n]

	isEOF := err == io.EOF || err == io.ErrUnexpectedEOF
	if err != nil && !isEOF {
		if n == 0 {
			return ReadResult{}, NewRemoteError("REMOTE_READ_FAILED", "read failed", err)
		}
	}

	return ReadResult{
		Content:  content,
		Resource: fullPath,
		IsText:   isTextContentWebDAV(content),
	}, nil
}

func (t *webdavTransport) Write(ctx context.Context, remotePath string, src io.Reader, overwrite bool) error {
	fullPath := t.toLocalPath(remotePath)

	if !overwrite {
		if _, err := os.Stat(fullPath); err == nil {
			return ErrAlreadyExists
		}
	}

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return NewRemoteError("REMOTE_WRITE_FAILED", "create parent dir failed", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return mapWebDAVError(err)
	}
	defer file.Close()

	if _, err := io.Copy(file, src); err != nil {
		return NewRemoteError("REMOTE_WRITE_FAILED", "write failed", err)
	}

	return nil
}

func (t *webdavTransport) Mkdir(ctx context.Context, remotePath string) error {
	fullPath := t.toLocalPath(remotePath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return mapWebDAVError(err)
	}
	return nil
}

func (t *webdavTransport) Rename(ctx context.Context, oldPath string, newPath string) error {
	oldFullPath := t.toLocalPath(oldPath)
	newFullPath := t.toLocalPath(newPath)

	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		return NewRemoteError("REMOTE_RENAME_FAILED", "create parent dir failed", err)
	}

	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		return mapWebDAVError(err)
	}
	return nil
}

func (t *webdavTransport) Move(ctx context.Context, sourcePath string, destParentPath string) error {
	sourceFullPath := t.toLocalPath(sourcePath)
	sourceInfo, err := os.Stat(sourceFullPath)
	if err != nil {
		return mapWebDAVError(err)
	}
	if sourceInfo.IsDir() {
		return NewRemoteError("REMOTE_MOVE_FAILED", "directory move not supported via this path", nil)
	}
	basename := filepath.Base(sourceFullPath)
	destFullPath := filepath.Join(t.toLocalPath(destParentPath), basename)
	return t.Rename(ctx, sourcePath, destFullPath)
}

func (t *webdavTransport) Copy(ctx context.Context, sourcePath string, destParentPath string) error {
	sourceFullPath := t.toLocalPath(sourcePath)
	basename := filepath.Base(sourceFullPath)
	destFullPath := filepath.Join(t.toLocalPath(destParentPath), basename)

	if err := os.MkdirAll(filepath.Dir(destFullPath), 0755); err != nil {
		return NewRemoteError("REMOTE_COPY_FAILED", "create parent dir failed", err)
	}

	srcFile, err := os.Open(sourceFullPath)
	if err != nil {
		return mapWebDAVError(err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(destFullPath)
	if err != nil {
		return mapWebDAVError(err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return NewRemoteError("REMOTE_COPY_FAILED", "copy failed", err)
	}

	return nil
}

func (t *webdavTransport) Delete(ctx context.Context, remotePath string, recursive bool) error {
	fullPath := t.toLocalPath(remotePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return mapWebDAVError(err)
	}

	if info.IsDir() && !recursive {
		entries, readErr := os.ReadDir(fullPath)
		if readErr != nil {
			return mapWebDAVError(readErr)
		}
		if len(entries) > 0 {
			return ErrDirectoryNotEmpty
		}
	}

	if recursive {
		if err := os.RemoveAll(fullPath); err != nil {
			return mapWebDAVError(err)
		}
		return nil
	}

	if err := os.Remove(fullPath); err != nil {
		return mapWebDAVError(err)
	}
	return nil
}

func (t *webdavTransport) Close() error {
	return nil
}

func (t *webdavTransport) RemoteRoot() string {
	return t.config.BasePath
}

func (t *webdavTransport) toLocalPath(remotePath string) string {
	base := t.config.BasePath
	if base == "" {
		base = "/"
	}
	cleaned := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(remotePath)), "/")
	if cleaned == "" || cleaned == "." {
		return base
	}
	return filepath.Join(base, cleaned)
}

func mapWebDAVError(err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if os.IsPermission(err) {
		return NewRemoteError("REMOTE_PERMISSION_DENIED", err.Error(), err)
	}
	return NewRemoteError("REMOTE_UNAVAILABLE", err.Error(), err)
}

func isTextContentWebDAV(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	maxCheck := 512
	if len(content) < maxCheck {
		maxCheck = len(content)
	}
	for i := 0; i < maxCheck; i++ {
		b := content[i]
		if b == 0 {
			return false
		}
		if b < 0x09 || (b > 0x0D && b < 0x20) {
			return false
		}
	}
	return true
}
