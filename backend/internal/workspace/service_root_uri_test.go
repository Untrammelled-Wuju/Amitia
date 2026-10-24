package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveURIToMountAcceptsMountRoot(t *testing.T) {
	registry := NewRegistry()
	mount, err := registry.RegisterLocalMount(context.Background(), "root", t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(registry, nil)
	resolved, relative, err := svc.resolveURIToMount(MountURI(mount.ID))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != mount.ID || relative != "" {
		t.Fatalf("unexpected root resolution: mount=%s relative=%q", resolved.ID, relative)
	}
}

func TestPreciseEditingSupportsDirectAbsoluteFilePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "replace.txt")
	if err := os.WriteFile(file, []byte("before before"), 0644); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.RegisterBackend(WorkspaceKindLocal, NewLocalBackend("")); err != nil {
		t.Fatal(err)
	}
	svc := NewService(registry, nil)
	precise := NewDefaultPreciseEditingService(svc)
	result, err := precise.Replace(context.Background(), ReplaceRequest{WorkspaceID: "direct", FilePath: file, OldText: "before", NewText: "after", ExpectedOccurrences: 2})
	if err != nil || !result.Replaced {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	content, err := os.ReadFile(file)
	if err != nil || string(content) != "after after" {
		t.Fatalf("content=%q err=%v", content, err)
	}
}

func TestResolveURIToMountAcceptsDirectLocalPaths(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "direct.txt")
	if err := os.WriteFile(file, []byte("direct"), 0644); err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewRegistry(), nil)
	mount, relative, err := svc.resolveURIToMount(file)
	if err != nil {
		t.Fatal(err)
	}
	parent, _ := filepath.EvalSymlinks(filepath.Dir(file))
	if mount.Kind != WorkspaceKindLocal || mount.LocalRoot != parent || relative != filepath.Base(file) {
		t.Fatalf("mount=%+v relative=%q", mount, relative)
	}
}
