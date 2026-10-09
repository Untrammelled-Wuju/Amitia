package kernel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostCommandWorkspaceDirectoryBoundaries(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "subdir")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "subdir", sub} {
		got, err := resolveHostCommandDirectory(root, value)
		if err != nil {
			t.Fatalf("resolve %q: %v", value, err)
		}
		if got != root && got != sub {
			t.Fatalf("unexpected working directory: %q", got)
		}
	}
	if _, err := resolveHostCommandDirectory(root, ".."); err == nil {
		t.Fatal("parent directory traversal must be rejected")
	}
	if _, err := resolveHostCommandDirectory(root, filepath.Dir(root)); err == nil {
		t.Fatal("absolute directory outside the workspace must be rejected")
	}
	if _, err := resolveHostCommandDirectory(root, "not-created"); err == nil {
		t.Fatal("nonexistent working directory must be rejected")
	}
}

func TestHostCommandOutputIsBounded(t *testing.T) {
	output := &hostCommandOutput{limit: 5}
	if n, err := output.Write([]byte("abcdefgh")); n != 8 || err != nil {
		t.Fatalf("write returned %d, %v", n, err)
	}
	content, truncated := output.Snapshot()
	if content != "abcde" || !truncated {
		t.Fatalf("output = %q, truncated = %v", content, truncated)
	}
}
