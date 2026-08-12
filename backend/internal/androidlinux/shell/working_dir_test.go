package shell

import (
	"testing"
)

func TestWorkingDirResolver_Resolve_Empty(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	path, err := resolver.Resolve("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/workspace" {
		t.Errorf("expected /workspace, got %s", path)
	}
}

func TestWorkingDirResolver_Resolve_AbsolutePath(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	path, err := resolver.Resolve("/workspace/subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/workspace/subdir" {
		t.Errorf("expected /workspace/subdir, got %s", path)
	}
}

func TestWorkingDirResolver_Resolve_RelativePath(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	path, err := resolver.Resolve("subdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/workspace/subdir" {
		t.Errorf("expected /workspace/subdir, got %s", path)
	}
}

func TestWorkingDirResolver_Resolve_WorkspaceURI(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	path, err := resolver.Resolve("amitia://workspace/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/workspace/project" {
		t.Errorf("expected /workspace/project, got %s", path)
	}
}

func TestWorkingDirResolver_Resolve_TempURI(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	path, err := resolver.Resolve("amitia://temp/cache")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/tmp/cache" {
		t.Errorf("expected /tmp/cache, got %s", path)
	}
}

func TestWorkingDirResolver_Resolve_RestrictedPath(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "")

	_, err := resolver.Resolve("/etc/passwd")
	if err == nil {
		t.Error("expected error for path outside workspace")
	}
}

func TestWorkingDirResolver_isUnderRoot(t *testing.T) {
	resolver := NewWorkingDirResolver("/workspace", "/tmp")

	tests := []struct {
		path   string
		root   string
		expect bool
	}{
		{"/workspace", "/workspace", true},
		{"/workspace/sub", "/workspace", true},
		{"/workspace/../etc", "/workspace", false},
		{"/etc/passwd", "/workspace", false},
	}
	for _, tt := range tests {
		result := resolver.isUnderRoot(tt.path, tt.root)
		if result != tt.expect {
			t.Errorf("isUnderRoot(%q, %q) = %v, want %v", tt.path, tt.root, result, tt.expect)
		}
	}
}
