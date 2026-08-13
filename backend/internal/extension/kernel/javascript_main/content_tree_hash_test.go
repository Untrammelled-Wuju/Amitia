package javascript_main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFiles(t *testing.T) {
	c := NewContentTreeHash()
	files := map[string][]byte{
		"a.js": []byte("console.log('a')"),
		"b.js": []byte("console.log('b')"),
	}
	hash1 := c.ComputeFiles(files)
	hash2 := c.ComputeFiles(files)
	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash, got %s vs %s", hash1, hash2)
	}
	if len(hash1) != 64 {
		t.Fatalf("expected sha256 hex (64 chars), got %d", len(hash1))
	}
}

func TestComputeFilesOrderIndependent(t *testing.T) {
	c := NewContentTreeHash()
	files1 := map[string][]byte{
		"a.js": []byte("aaa"),
		"b.js": []byte("bbb"),
	}
	files2 := map[string][]byte{
		"b.js": []byte("bbb"),
		"a.js": []byte("aaa"),
	}
	if c.ComputeFiles(files1) != c.ComputeFiles(files2) {
		t.Fatal("hash should be independent of map iteration order")
	}
}

func TestComputeFilesContentChange(t *testing.T) {
	c := NewContentTreeHash()
	h1 := c.ComputeFiles(map[string][]byte{"a.js": []byte("v1")})
	h2 := c.ComputeFiles(map[string][]byte{"a.js": []byte("v2")})
	if h1 == h2 {
		t.Fatal("different content should produce different hashes")
	}
}

func TestComputeDirectory(t *testing.T) {
	c := NewContentTreeHash()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.js"), []byte("export {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "util.js"), []byte("module.exports = {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := c.Compute(tmp)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(hash) != 64 {
		t.Fatalf("expected sha256 hex, got %s", hash)
	}
}

func TestComputeIgnoresHiddenFiles(t *testing.T) {
	c := NewContentTreeHash()
	clean := t.TempDir()
	if err := os.WriteFile(filepath.Join(clean, "keep.js"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := t.TempDir()
	if err := os.WriteFile(filepath.Join(dirty, "keep.js"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, ".hidden"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dirty, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, "node_modules", "pkg.js"), []byte("pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	hashClean, err := c.Compute(clean)
	if err != nil {
		t.Fatalf("compute clean: %v", err)
	}
	hashDirty, err := c.Compute(dirty)
	if err != nil {
		t.Fatalf("compute dirty: %v", err)
	}
	if hashClean != hashDirty {
		t.Fatalf("expected hidden/node_modules to be ignored: clean=%s dirty=%s", hashClean, hashDirty)
	}
}

func TestComputeDeterministic(t *testing.T) {
	c := NewContentTreeHash()
	tmp := t.TempDir()
	for _, name := range []string{"z.js", "a.js", "m.js"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h1, _ := c.Compute(tmp)
	h2, _ := c.Compute(tmp)
	if h1 != h2 {
		t.Fatalf("expected deterministic: %s vs %s", h1, h2)
	}
}

func TestShouldIgnore(t *testing.T) {
	c := NewContentTreeHash()
	tests := map[string]bool{
		"index.js":            false,
		".gitignore":          true,
		"node_modules/pkg.js": true,
		".git/config":         true,
		"src/.env":            true,
		"Thumbs.db":           true,
		"subdir/file.txt":     false,
	}
	for path, expected := range tests {
		if got := c.shouldIgnore(path); got != expected {
			t.Fatalf("shouldIgnore(%s): expected %v, got %v", path, expected, got)
		}
	}
}
