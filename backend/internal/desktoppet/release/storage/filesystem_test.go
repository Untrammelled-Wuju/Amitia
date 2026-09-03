package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseStorageAcceptsDocumentedSafeIDs(t *testing.T) {
	storage := NewFileSystemStorage(t.TempDir())
	if err := storage.Validate(); err != nil {
		t.Fatalf("validate storage: %v", err)
	}
	for _, id := range []string{"release-1", "release_1", "release.1", "A123"} {
		if _, err := storage.StagingDir(id); err != nil {
			t.Fatalf("safe id %q rejected: %v", id, err)
		}
	}
}

func TestReleaseStorageRejectsUnsafeIDs(t *testing.T) {
	storage := NewFileSystemStorage(t.TempDir())
	for _, id := range []string{".", "..", "../escape", "a/b", `a\\b`, "/absolute"} {
		if _, err := storage.StagingDir(id); err == nil {
			t.Fatalf("unsafe id %q accepted", id)
		}
	}
}

func TestReleaseStorageDeleteRejectsSymlink(t *testing.T) {
	storage := NewFileSystemStorage(t.TempDir())
	if err := storage.Validate(); err != nil {
		t.Fatalf("validate storage: %v", err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	staging, err := storage.StagingDir("release-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, staging); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if err := storage.RemoveStagingDir("release-1"); err == nil {
		t.Fatal("expected symlink deletion rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "keep.txt")); err != nil {
		t.Fatalf("outside target was touched: %v", err)
	}
}

func TestReleaseStorageArchiveDoesNotOverwriteExistingDestination(t *testing.T) {
	dataDir := t.TempDir()
	storage := NewFileSystemStorage(dataDir)
	if err := storage.Validate(); err != nil {
		t.Fatalf("validate storage: %v", err)
	}
	source := filepath.Join(t.TempDir(), "source.zip")
	if err := os.WriteFile(source, []byte("archive-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := storage.StoreVerifiedArchive(source, "pet-1", "release-1", ""); err != nil {
		t.Fatalf("first archive store: %v", err)
	}
	if err := os.WriteFile(source, []byte("archive-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := storage.StoreVerifiedArchive(source, "pet-1", "release-1", ""); err == nil {
		t.Fatal("expected existing archive destination to be rejected")
	}
	finalPath, err := storage.ArchivePath("pet-1", "release-1")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "archive-v1" {
		t.Fatalf("existing archive was overwritten: %q", content)
	}
}
