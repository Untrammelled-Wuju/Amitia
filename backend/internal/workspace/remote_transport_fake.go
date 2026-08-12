package workspace

import (
	"context"
	"io"
	"path"
	"strings"
	"sync"
	"time"
)

type FakeRemoteTransport struct {
	mu       sync.RWMutex
	rootDir  string
	files    map[string]*fakeRemoteFile
	children map[string][]string
	closed   bool
}

type fakeRemoteFile struct {
	Name       string
	Content    []byte
	IsDir      bool
	IsSymlink  bool
	ModifiedAt time.Time
}

func NewFakeRemoteTransport() *FakeRemoteTransport {
	return &FakeRemoteTransport{
		rootDir:  "/",
		files:    make(map[string]*fakeRemoteFile),
		children: make(map[string][]string),
	}
}

func (f *FakeRemoteTransport) ensureParent(filePath string) {
	dir := path.Dir(filePath)
	if dir == filePath {
		return
	}
	if _, exists := f.files[dir]; !exists {
		f.files[dir] = &fakeRemoteFile{
			Name:  path.Base(dir),
			IsDir: true,
		}
		f.ensureParent(dir)
	}
}

func (f *FakeRemoteTransport) AddFile(filePath string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cleanPath := path.Clean(filePath)
	f.files[cleanPath] = &fakeRemoteFile{
		Name:       path.Base(cleanPath),
		Content:    content,
		IsDir:      false,
		ModifiedAt: time.Now().UTC(),
	}
	f.ensureParent(cleanPath)

	dir := path.Dir(cleanPath)
	if dir != cleanPath {
		f.children[dir] = append(f.children[dir], path.Base(cleanPath))
	}
}

func (f *FakeRemoteTransport) AddDir(dirPath string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	cleanPath := path.Clean(dirPath)
	if _, exists := f.files[cleanPath]; !exists {
		f.files[cleanPath] = &fakeRemoteFile{
			Name:  path.Base(cleanPath),
			IsDir: true,
		}
	}
	f.ensureParent(cleanPath)

	dir := path.Dir(cleanPath)
	if dir != cleanPath {
		f.children[dir] = append(f.children[dir], path.Base(cleanPath))
	}
}

func (f *FakeRemoteTransport) Stat(ctx context.Context, remotePath string) (RemoteStatResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return RemoteStatResult{}, NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	file, exists := f.files[cleanPath]
	if !exists {
		return RemoteStatResult{}, ErrFileNotFound
	}

	return RemoteStatResult{
		Name:       file.Name,
		IsDir:      file.IsDir,
		IsSymlink:  file.IsSymlink,
		SizeBytes:  int64(len(file.Content)),
		ModifiedAt: file.ModifiedAt,
		MIMEType:   InferRemoteMIMEType(file.Name),
	}, nil
}

func (f *FakeRemoteTransport) List(ctx context.Context, remotePath string, limit int) (RemoteListResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return RemoteListResult{}, NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	file, exists := f.files[cleanPath]
	if !exists {
		return RemoteListResult{}, ErrFileNotFound
	}
	if !file.IsDir {
		return RemoteListResult{}, NewRemoteError("REMOTE_NOT_A_DIRECTORY", "not a directory", nil)
	}

	children, hasChildren := f.children[cleanPath]
	if !hasChildren {
		return RemoteListResult{Entries: []RemoteStatResult{}}, nil
	}

	entries := make([]RemoteStatResult, 0, len(children))
	for i, name := range children {
		if i >= limit {
			return RemoteListResult{
				Entries: entries,
				HasMore: true,
			}, nil
		}

		childPath := path.Join(cleanPath, name)
		childFile, exists := f.files[childPath]
		if !exists {
			continue
		}

		entries = append(entries, RemoteStatResult{
			Name:       childFile.Name,
			IsDir:      childFile.IsDir,
			IsSymlink:  childFile.IsSymlink,
			SizeBytes:  int64(len(childFile.Content)),
			ModifiedAt: childFile.ModifiedAt,
			MIMEType:   InferRemoteMIMEType(childFile.Name),
		})
	}

	return RemoteListResult{Entries: entries}, nil
}

func (f *FakeRemoteTransport) Read(ctx context.Context, remotePath string, offset int64, maxBytes int64) (ReadResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.closed {
		return ReadResult{}, NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	file, exists := f.files[cleanPath]
	if !exists {
		return ReadResult{}, ErrFileNotFound
	}

	if file.IsDir {
		return ReadResult{}, NewRemoteError("REMOTE_IS_DIRECTORY", "is a directory", nil)
	}

	if offset >= int64(len(file.Content)) {
		return ReadResult{Content: []byte{}, IsText: true}, nil
	}

	end := offset + maxBytes
	if end > int64(len(file.Content)) {
		end = int64(len(file.Content))
	}

	content := file.Content[offset:end]
	return ReadResult{
		Content:  content,
		Resource: cleanPath,
		IsText:   isFakeTextContent(content),
	}, nil
}

func (f *FakeRemoteTransport) Write(ctx context.Context, remotePath string, src io.Reader, overwrite bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	if !overwrite {
		if _, exists := f.files[cleanPath]; exists {
			return ErrAlreadyExists
		}
	}

	content, err := io.ReadAll(src)
	if err != nil {
		return NewRemoteError("REMOTE_WRITE_FAILED", "read source failed", err)
	}

	f.files[cleanPath] = &fakeRemoteFile{
		Name:       path.Base(cleanPath),
		Content:    content,
		ModifiedAt: time.Now().UTC(),
	}

	dir := path.Dir(cleanPath)
	if dir != cleanPath {
		existsInDir := false
		for _, name := range f.children[dir] {
			if name == path.Base(cleanPath) {
				existsInDir = true
				break
			}
		}
		if !existsInDir {
			f.children[dir] = append(f.children[dir], path.Base(cleanPath))
		}
	}

	return nil
}

func (f *FakeRemoteTransport) Mkdir(ctx context.Context, remotePath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	f.files[cleanPath] = &fakeRemoteFile{
		Name:  path.Base(cleanPath),
		IsDir: true,
	}
	f.ensureParent(cleanPath)

	dir := path.Dir(cleanPath)
	if dir != cleanPath {
		existsInDir := false
		for _, name := range f.children[dir] {
			if name == path.Base(cleanPath) {
				existsInDir = true
				break
			}
		}
		if !existsInDir {
			f.children[dir] = append(f.children[dir], path.Base(cleanPath))
		}
	}

	return nil
}

func (f *FakeRemoteTransport) Rename(ctx context.Context, oldPath string, newPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	oldClean := path.Clean(oldPath)
	newClean := path.Clean(newPath)

	file, exists := f.files[oldClean]
	if !exists {
		return ErrFileNotFound
	}

	delete(f.files, oldClean)
	f.files[newClean] = file
	file.Name = path.Base(newClean)

	oldDir := path.Dir(oldClean)
	newDir := path.Dir(newClean)

	if oldDir == newDir {
		return nil
	}

	if children, ok := f.children[oldDir]; ok {
		newChildren := make([]string, 0, len(children))
		for _, name := range children {
			if name != path.Base(oldClean) {
				newChildren = append(newChildren, name)
			}
		}
		f.children[oldDir] = newChildren
	}

	if newDir != newClean {
		existsInDir := false
		for _, name := range f.children[newDir] {
			if name == path.Base(newClean) {
				existsInDir = true
				break
			}
		}
		if !existsInDir {
			f.children[newDir] = append(f.children[newDir], path.Base(newClean))
		}
	}

	return nil
}

func (f *FakeRemoteTransport) Move(ctx context.Context, sourcePath string, destParentPath string) error {
	sourceClean := path.Clean(sourcePath)
	basename := path.Base(sourceClean)
	destPath := path.Join(path.Clean(destParentPath), basename)
	return f.Rename(ctx, sourceClean, destPath)
}

func (f *FakeRemoteTransport) Copy(ctx context.Context, sourcePath string, destParentPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	sourceClean := path.Clean(sourcePath)
	destPath := path.Join(path.Clean(destParentPath), path.Base(sourceClean))

	srcFile, exists := f.files[sourceClean]
	if !exists {
		return ErrFileNotFound
	}

	contentCopy := make([]byte, len(srcFile.Content))
	copy(contentCopy, srcFile.Content)

	f.files[destPath] = &fakeRemoteFile{
		Name:       path.Base(destPath),
		Content:    contentCopy,
		IsDir:      srcFile.IsDir,
		ModifiedAt: srcFile.ModifiedAt,
	}

	dir := path.Clean(destParentPath)
	existsInDir := false
	for _, name := range f.children[dir] {
		if name == path.Base(destPath) {
			existsInDir = true
			break
		}
	}
	if !existsInDir {
		f.children[dir] = append(f.children[dir], path.Base(destPath))
	}

	return nil
}

func (f *FakeRemoteTransport) Delete(ctx context.Context, remotePath string, recursive bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return NewRemoteError("REMOTE_UNAVAILABLE", "transport closed", nil)
	}

	cleanPath := path.Clean(remotePath)
	file, exists := f.files[cleanPath]
	if !exists {
		return ErrFileNotFound
	}

	if file.IsDir && !recursive {
		if children, hasChildren := f.children[cleanPath]; hasChildren && len(children) > 0 {
			return ErrDirectoryNotEmpty
		}
	}

	delete(f.files, cleanPath)
	delete(f.children, cleanPath)

	dir := path.Dir(cleanPath)
	if children, ok := f.children[dir]; ok {
		newChildren := make([]string, 0, len(children))
		for _, name := range children {
			if name != path.Base(cleanPath) {
				newChildren = append(newChildren, name)
			}
		}
		f.children[dir] = newChildren
	}

	return nil
}

func (f *FakeRemoteTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *FakeRemoteTransport) RemoteRoot() string {
	return f.rootDir
}

func isFakeTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	for _, b := range content {
		if b == 0 {
			return false
		}
	}
	return true
}

var _ RemoteTransport = (*FakeRemoteTransport)(nil)

var _ = strings.Contains
