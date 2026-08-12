//go:build linux && !android

package archive

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/androidlinux/fileops"
)

type mockFileService struct {
	stats  map[string]fileops.StatResult
	files  map[string][]byte
	dirs   map[string][]fileops.StatResult
	writes map[string][]byte
}

func newMockFileService() *mockFileService {
	return &mockFileService{
		stats:  make(map[string]fileops.StatResult),
		files:  make(map[string][]byte),
		dirs:   make(map[string][]fileops.StatResult),
		writes: make(map[string][]byte),
	}
}

func (m *mockFileService) Stat(path string) (fileops.StatResult, error) {
	if s, ok := m.stats[path]; ok {
		return s, nil
	}
	return fileops.StatResult{}, fileops.ErrFileNotFound(path)
}

func (m *mockFileService) List(path string, opts fileops.ListOptions) ([]fileops.StatResult, error) {
	if entries, ok := m.dirs[path]; ok {
		return entries, nil
	}
	return nil, fileops.ErrFileNotFound(path)
}

func (m *mockFileService) Read(path string, opts fileops.ReadOptions) (fileops.ReadResult, error) {
	if data, ok := m.files[path]; ok {
		return fileops.ReadResult{
			Path:      path,
			BytesRead: len(data),
			Content:   data,
			EOF:       true,
		}, nil
	}
	return fileops.ReadResult{}, fileops.ErrFileNotFound(path)
}

func (m *mockFileService) Write(path string, data []byte, opts fileops.WriteOptions) (fileops.StatResult, error) {
	if !opts.Overwrite {
		if _, exists := m.writes[path]; exists {
			return fileops.StatResult{}, fileops.ErrPathDenied(path, "file exists")
		}
	}
	m.writes[path] = data
	m.stats[path] = fileops.StatResult{
		Path:  path,
		Name:  filepath.Base(path),
		Type:  "file",
		Size:  int64(len(data)),
		IsDir: false,
	}
	return m.stats[path], nil
}

func (m *mockFileService) Append(path string, data []byte) (fileops.StatResult, error) {
	existing := m.writes[path]
	m.writes[path] = append(existing, data...)
	m.stats[path] = fileops.StatResult{
		Path:  path,
		Name:  filepath.Base(path),
		Type:  "file",
		Size:  int64(len(m.writes[path])),
		IsDir: false,
	}
	return m.stats[path], nil
}

func (m *mockFileService) Mkdir(path string, opts fileops.MkdirOptions) (fileops.StatResult, error) {
	m.stats[path] = fileops.StatResult{
		Path:  path,
		Name:  filepath.Base(path),
		Type:  "directory",
		IsDir: true,
	}
	return m.stats[path], nil
}

func (m *mockFileService) Touch(path string) (fileops.StatResult, error) {
	m.files[path] = []byte{}
	m.stats[path] = fileops.StatResult{
		Path:  path,
		Name:  filepath.Base(path),
		Type:  "file",
		IsDir: false,
	}
	return m.stats[path], nil
}

func (m *mockFileService) Copy(source, destination string, opts fileops.CopyOptions) (fileops.StatResult, error) {
	data, ok := m.files[source]
	if !ok {
		return fileops.StatResult{}, fileops.ErrFileNotFound(source)
	}
	m.files[destination] = data
	m.stats[destination] = fileops.StatResult{
		Path:  destination,
		Name:  filepath.Base(destination),
		Type:  "file",
		Size:  int64(len(data)),
		IsDir: false,
	}
	return m.stats[destination], nil
}

func (m *mockFileService) Move(source, destination string, opts fileops.MoveOptions) (fileops.StatResult, error) {
	data, ok := m.files[source]
	if !ok {
		return fileops.StatResult{}, fileops.ErrFileNotFound(source)
	}
	delete(m.files, source)
	delete(m.stats, source)
	m.files[destination] = data
	m.stats[destination] = fileops.StatResult{
		Path:  destination,
		Name:  filepath.Base(destination),
		Type:  "file",
		Size:  int64(len(data)),
		IsDir: false,
	}
	return m.stats[destination], nil
}

func (m *mockFileService) Delete(path string, opts fileops.DeleteOptions) error {
	delete(m.files, path)
	delete(m.stats, path)
	return nil
}

func (m *mockFileService) Search(root string, opts fileops.SearchOptions) ([]fileops.StatResult, error) {
	return nil, nil
}

func (m *mockFileService) Chmod(path string, mode uint32) (fileops.StatResult, error) {
	if s, ok := m.stats[path]; ok {
		s.Mode = mode
		m.stats[path] = s
		return s, nil
	}
	return fileops.StatResult{}, fileops.ErrFileNotFound(path)
}

func (m *mockFileService) Readlink(path string) (string, error) {
	return "", fileops.ErrFileNotFound(path)
}

func (m *mockFileService) Symlink(target, linkPath string) (fileops.StatResult, error) {
	m.stats[linkPath] = fileops.StatResult{
		Path:       linkPath,
		Name:       filepath.Base(linkPath),
		Type:       "symlink",
		IsSymlink:  true,
		LinkTarget: target,
	}
	return m.stats[linkPath], nil
}

func TestServiceDetect(t *testing.T) {
	dir := t.TempDir()

	zipFile := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(zipFile, []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"), 0644); err != nil {
		t.Fatal(err)
	}

	tarGzFile := filepath.Join(dir, "test.tar.gz")
	if err := os.WriteFile(tarGzFile, []byte{0x1f, 0x8b, 0x08, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}

	mock := newMockFileService()
	mock.stats[zipFile] = fileops.StatResult{
		Path:  zipFile,
		Name:  "test.zip",
		Type:  "file",
		IsDir: false,
	}
	mock.stats[tarGzFile] = fileops.StatResult{
		Path:  tarGzFile,
		Name:  "test.tar.gz",
		Type:  "file",
		IsDir: false,
	}

	svc := NewService(mock, DefaultPolicy())
	ctx := context.Background()

	t.Run("detect ZIP", func(t *testing.T) {
		result, err := svc.Detect(ctx, DetectRequest{Path: zipFile})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Format != FormatZIP {
			t.Errorf("Format = %v, want %v", result.Format, FormatZIP)
		}
		if !result.Archive {
			t.Error("Archive = false, want true")
		}
	})

	t.Run("detect TAR.GZ", func(t *testing.T) {
		result, err := svc.Detect(ctx, DetectRequest{Path: tarGzFile})
		if err != nil {
			t.Fatalf("Detect() error = %v", err)
		}
		if result.Format != FormatTARGZ {
			t.Errorf("Format = %v, want %v", result.Format, FormatTARGZ)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := svc.Detect(ctx, DetectRequest{Path: ""})
		if err == nil {
			t.Error("Detect() expected error for empty path")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := svc.Detect(ctx, DetectRequest{Path: filepath.Join(dir, "nonexistent.zip")})
		if err == nil {
			t.Error("Detect() expected error for non-existent file")
		}
	})
}

func TestServiceListEmptyZip(t *testing.T) {
	dir := t.TempDir()

	zipFile := filepath.Join(dir, "empty.zip")
	zipData := []byte{
		0x50, 0x4b, 0x05, 0x06, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if err := os.WriteFile(zipFile, zipData, 0644); err != nil {
		t.Fatal(err)
	}

	mock := newMockFileService()
	mock.stats[zipFile] = fileops.StatResult{
		Path:  zipFile,
		Name:  "empty.zip",
		Type:  "file",
		IsDir: false,
		Size:  int64(len(zipData)),
	}

	svc := NewService(mock, DefaultPolicy())
	ctx := context.Background()

	entries, total, err := svc.List(ctx, ListRequest{Path: zipFile})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestServiceCreateAndExtractZip(t *testing.T) {
	dir := t.TempDir()

	sourceFile := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(sourceFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	zipFile := filepath.Join(dir, "output.zip")
	extractDir := filepath.Join(dir, "extracted")

	mock := newMockFileService()
	mock.stats[sourceFile] = fileops.StatResult{
		Path:  sourceFile,
		Name:  "source.txt",
		Type:  "file",
		IsDir: false,
		Size:  11,
	}
	mock.files[sourceFile] = []byte("hello world")

	svc := NewService(mock, DefaultPolicy())
	ctx := context.Background()

	t.Run("create ZIP", func(t *testing.T) {
		count, bytes, err := svc.Create(ctx, CreateRequest{
			Sources:   []string{sourceFile},
			Target:    zipFile,
			Format:    FormatZIP,
			Overwrite: true,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if bytes != 11 {
			t.Errorf("bytes = %d, want 11", bytes)
		}
	})

	t.Run("extract ZIP", func(t *testing.T) {
		mock.stats[zipFile] = fileops.StatResult{
			Path:  zipFile,
			Name:  "output.zip",
			Type:  "file",
			IsDir: false,
		}

		count, bytes, err := svc.Extract(ctx, ExtractRequest{
			Path:      zipFile,
			Target:    extractDir,
			Overwrite: true,
		})
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}
		if count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
		if bytes != 11 {
			t.Errorf("bytes = %d, want 11", bytes)
		}
	})
}

func TestServiceVerifyZip(t *testing.T) {
	dir := t.TempDir()

	zipFile := filepath.Join(dir, "test.zip")
	zipData := []byte{
		0x50, 0x4b, 0x03, 0x04, 0x14, 0x00, 0x00, 0x00,
		0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	if err := os.WriteFile(zipFile, zipData, 0644); err != nil {
		t.Fatal(err)
	}

	mock := newMockFileService()
	mock.stats[zipFile] = fileops.StatResult{
		Path:  zipFile,
		Name:  "test.zip",
		Type:  "file",
		IsDir: false,
		Size:  int64(len(zipData)),
	}

	svc := NewService(mock, DefaultPolicy())
	ctx := context.Background()

	t.Run("verify ZIP", func(t *testing.T) {
		result, err := svc.Verify(ctx, DetectRequest{Path: zipFile})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if result == nil {
			t.Fatal("Verify() returned nil result")
		}
		if result.Format != FormatZIP {
			t.Errorf("Format = %v, want %v", result.Format, FormatZIP)
		}
	})
}
