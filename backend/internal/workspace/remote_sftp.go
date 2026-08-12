package workspace

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type sftpTransport struct {
	config RemoteMountConfig
	cred   *RemoteCredential
}

func newSFTPTransport(ctx context.Context, config RemoteMountConfig, cred *RemoteCredential) (RemoteTransport, error) {
	if config.Host == "" {
		return nil, NewRemoteError("REMOTE_CONFIG_INVALID", "host is required", nil)
	}
	if config.Port == 0 {
		config.Port = DefaultSFTPPort
	}
	if config.BasePath == "" {
		config.BasePath = "/"
	}

	return &sftpTransport{
		config: config,
		cred:   cred,
	}, nil
}

func (t *sftpTransport) Stat(ctx context.Context, remotePath string) (RemoteStatResult, error) {
	fullPath := t.toLocalPath(remotePath)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return RemoteStatResult{}, mapOSFSError(err)
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

func (t *sftpTransport) List(ctx context.Context, remotePath string, limit int) (RemoteListResult, error) {
	fullPath := t.toLocalPath(remotePath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return RemoteListResult{}, mapOSFSError(err)
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

func (t *sftpTransport) Read(ctx context.Context, remotePath string, offset int64, maxBytes int64) (ReadResult, error) {
	fullPath := t.toLocalPath(remotePath)
	file, err := os.Open(fullPath)
	if err != nil {
		return ReadResult{}, mapOSFSError(err)
	}
	defer file.Close()

	if offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return ReadResult{}, NewRemoteError("REMOTE_SEEK_FAILED", "seek failed", err)
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
		IsText:   isTextContent(content),
	}, nil
}

func (t *sftpTransport) Write(ctx context.Context, remotePath string, src io.Reader, overwrite bool) error {
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
		return mapOSFSError(err)
	}
	defer file.Close()

	if _, err := io.Copy(file, src); err != nil {
		return NewRemoteError("REMOTE_WRITE_FAILED", "write failed", err)
	}

	return nil
}

func (t *sftpTransport) Mkdir(ctx context.Context, remotePath string) error {
	fullPath := t.toLocalPath(remotePath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return mapOSFSError(err)
	}
	return nil
}

func (t *sftpTransport) Rename(ctx context.Context, oldPath string, newPath string) error {
	oldFullPath := t.toLocalPath(oldPath)
	newFullPath := t.toLocalPath(newPath)

	if err := os.MkdirAll(filepath.Dir(newFullPath), 0755); err != nil {
		return NewRemoteError("REMOTE_RENAME_FAILED", "create parent dir failed", err)
	}

	if err := os.Rename(oldFullPath, newFullPath); err != nil {
		return mapOSFSError(err)
	}
	return nil
}

func (t *sftpTransport) Move(ctx context.Context, sourcePath string, destParentPath string) error {
	sourceFullPath := t.toLocalPath(sourcePath)
	sourceInfo, err := os.Stat(sourceFullPath)
	if err != nil {
		return mapOSFSError(err)
	}
	if sourceInfo.IsDir() {
		return NewRemoteError("REMOTE_MOVE_FAILED", "directory move not supported via this path", nil)
	}
	basename := filepath.Base(sourceFullPath)
	destFullPath := filepath.Join(t.toLocalPath(destParentPath), basename)
	return t.Rename(ctx, sourcePath, destFullPath)
}

func (t *sftpTransport) Copy(ctx context.Context, sourcePath string, destParentPath string) error {
	sourceFullPath := t.toLocalPath(sourcePath)
	basename := filepath.Base(sourceFullPath)
	destFullPath := filepath.Join(t.toLocalPath(destParentPath), basename)

	if err := os.MkdirAll(filepath.Dir(destFullPath), 0755); err != nil {
		return NewRemoteError("REMOTE_COPY_FAILED", "create parent dir failed", err)
	}

	srcFile, err := os.Open(sourceFullPath)
	if err != nil {
		return mapOSFSError(err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(destFullPath)
	if err != nil {
		return mapOSFSError(err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return NewRemoteError("REMOTE_COPY_FAILED", "copy failed", err)
	}

	return nil
}

func (t *sftpTransport) Delete(ctx context.Context, remotePath string, recursive bool) error {
	fullPath := t.toLocalPath(remotePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return mapOSFSError(err)
	}

	if info.IsDir() && !recursive {
		entries, readErr := os.ReadDir(fullPath)
		if readErr != nil {
			return mapOSFSError(readErr)
		}
		if len(entries) > 0 {
			return ErrDirectoryNotEmpty
		}
	}

	if recursive {
		if err := os.RemoveAll(fullPath); err != nil {
			return mapOSFSError(err)
		}
		return nil
	}

	if err := os.Remove(fullPath); err != nil {
		return mapOSFSError(err)
	}
	return nil
}

func (t *sftpTransport) Close() error {
	return nil
}

func (t *sftpTransport) RemoteRoot() string {
	return t.config.BasePath
}

func (t *sftpTransport) toLocalPath(remotePath string) string {
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

func mapOSFSError(err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if os.IsPermission(err) {
		return NewRemoteError("REMOTE_PERMISSION_DENIED", err.Error(), err)
	}
	if strings.Contains(err.Error(), "too many links") {
		return NewRemoteError("REMOTE_SYMLINK_UNSUPPORTED", err.Error(), err)
	}
	return NewRemoteError("REMOTE_UNAVAILABLE", err.Error(), err)
}

func isTextContent(content []byte) bool {
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
