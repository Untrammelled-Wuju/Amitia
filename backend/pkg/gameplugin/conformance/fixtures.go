package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func loadFixture(path string) ([]byte, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to get caller info")
	}
	dir := filepath.Dir(filename)
	fullPath := filepath.Join(dir, "..", "testdata", "conformance", path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixture %s: %w", path, err)
	}
	return data, nil
}

func loadFixtureString(path string) (string, error) {
	data, err := loadFixture(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
