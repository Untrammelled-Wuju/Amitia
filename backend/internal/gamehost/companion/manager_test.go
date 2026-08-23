package companion

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTargetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveTarget(root, "../mods/evil.jar"); err == nil {
		t.Fatal("resolveTarget() accepted traversal")
	}
}

func TestExtractZipAtomicRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZipAtomic(archive, filepath.Join(root, "target")); err == nil {
		t.Fatal("extractZipAtomic() accepted zip-slip entry")
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("zip-slip wrote outside target: err=%v", err)
	}
}

func TestCopyTreeAtomicRejectsSymlinkSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyTreeAtomic(source, filepath.Join(root, "target")); err == nil {
		t.Fatal("copyTreeAtomic() accepted symlink")
	}
}
