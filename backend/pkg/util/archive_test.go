package util

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzipFileWritesCompleteEntry(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "archive.zip")
	payload := bytes.Repeat([]byte("amitia-unzip-"), 131072)

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("nested/surreal.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, "target")
	if err := UnzipFile(zipPath, target); err != nil {
		t.Fatal(err)
	}
	extracted, err := os.ReadFile(filepath.Join(target, "nested", "surreal.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(extracted, payload) {
		t.Fatalf("extracted bytes mismatch: got %d want %d", len(extracted), len(payload))
	}
}
