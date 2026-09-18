package extension

import (
	"os"
	"path/filepath"
	"testing"
)

type TestFixture struct {
	BaseDir string
}

func NewTestFixture(t *testing.T) *TestFixture {
	t.Helper()
	dir := t.TempDir()
	return &TestFixture{BaseDir: dir}
}

func (f *TestFixture) Path(elem ...string) string {
	return filepath.Join(append([]string{f.BaseDir}, elem...)...)
}

func (f *TestFixture) WriteFile(path string, content []byte) error {
	fullPath := filepath.Join(f.BaseDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, content, 0644)
}

func (f *TestFixture) WriteFiles(files map[string][]byte) error {
	for path, content := range files {
		if err := f.WriteFile(path, content); err != nil {
			return err
		}
	}
	return nil
}
