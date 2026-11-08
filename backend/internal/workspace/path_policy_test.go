package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestValidateRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"empty", "", false},
		{"simple", "foo", false},
		{"nested", "foo/bar/baz", false},
		{"traversal", "../etc/passwd", true},
		{"nested_traversal", "foo/../../etc", true},
		{"dot_dot", "..", true},
		{"backslash", "foo\\bar", true},
		{"nul", "foo\x00bar", true},
		{"control", "foo\x01bar", true},
		{"deep", "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p/q/r/s/t/u/v/w/x/y/z", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRelativePath(tt.rel)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.rel)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.rel, err)
			}
		})
	}
}

func TestResolveAndValidateRead(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "ws")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"existing", "a.txt", false},
		{"traversal", "../outside", true},
		{"root", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveAndValidateRead(root, tt.rel)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.rel)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.rel, err)
			}
		})
	}
}

func TestSymlinkProtection(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "ws")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}
	_, err := ResolveAndValidateRead(root, "link/secret.txt")
	if err == nil {
		t.Error("expected error for symlink traversal, got nil")
	}
	if !errors.Is(err, ErrSymlinkNotAllowed) {
		t.Errorf("expected ErrSymlinkNotAllowed, got %v", err)
	}
}

func TestLocalBackendListMarksSymlinkUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "ws")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Skip("symlinks not supported on this platform")
	}
	backend := NewLocalBackend(root)
	mount := WorkspaceMount{
		ID:      "test",
		Name:    "Test",
		Kind:    WorkspaceKindLocal,
		RootURI: "amitia://workspace/@test/",
	}
	entries, err := backend.List(t.Context(), mount, "", ListOptions{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	if entries[0].Readable || entries[0].Writable {
		t.Fatalf("symlink was advertised as usable: %#v", entries[0])
	}
}

func TestRegisterLocalMountAppliesReadOnlyOnExistingRoot(t *testing.T) {
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "workspace.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspace_mounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		local_root TEXT NOT NULL DEFAULT '',
		native_grant_id TEXT NOT NULL DEFAULT '',
		backend_config_json TEXT NOT NULL DEFAULT '',
		credential_ref TEXT NOT NULL DEFAULT '',
		read_only INTEGER NOT NULL DEFAULT 0,
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_used_at TEXT NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithPersistence(NewRegistry(), nil, NewMountRepository(db), nil)
	first, err := service.RegisterLocalMount(t.Context(), "workspace", root, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RegisterLocalMount(t.Context(), "workspace", root, true)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("same root produced a different mount: %s != %s", second.ID, first.ID)
	}
	if !second.ReadOnly {
		t.Fatalf("existing writable mount ignored the read-only request: %#v", second)
	}
	current, ok := service.registry.GetMount(first.ID)
	if !ok || !current.ReadOnly {
		t.Fatalf("registry did not apply read-only mode: %#v", current)
	}
}

func TestLocalBackendWriteAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewLocalBackend(tmpDir)
	mount := WorkspaceMount{ID: "test", Name: "Test", Kind: WorkspaceKindLocal, RootURI: "amitia://workspace/@test/"}
	ctx := t.Context()

	entry, err := backend.Write(ctx, mount, "hello.txt", strings.NewReader("Hello, workspace!"), WriteOptions{Atomic: true})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if entry.Name != "hello.txt" {
		t.Errorf("expected name hello.txt, got %s", entry.Name)
	}

	result, err := backend.Read(ctx, mount, "hello.txt", ReadOptions{})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(result.Content) != "Hello, workspace!" {
		t.Errorf("expected 'Hello, workspace!', got %q", string(result.Content))
	}
}

func TestLocalBackendSymlinkWriteProtection(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "ws")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "out")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Skip("symlinks not supported on this platform")
	}
	backend := NewLocalBackend(root)
	mount := WorkspaceMount{ID: "test", Name: "Test", Kind: WorkspaceKindLocal, RootURI: "amitia://workspace/@test/"}
	ctx := t.Context()

	_, err := backend.Write(ctx, mount, "out/evil.txt", strings.NewReader("evil"), WriteOptions{})
	if err == nil {
		t.Error("expected error writing through symlink, got nil")
	}
	if !errors.Is(err, ErrSymlinkNotAllowed) {
		t.Errorf("expected ErrSymlinkNotAllowed, got %v", err)
	}
}

func TestDeleteRootDenied(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewLocalBackend(tmpDir)
	mount := WorkspaceMount{ID: "test", Name: "Test", Kind: WorkspaceKindLocal, RootURI: "amitia://workspace/@test/"}
	ctx := t.Context()

	err := backend.Delete(ctx, mount, "", DeleteOptions{Recursive: true})
	if err == nil {
		t.Error("expected error deleting root, got nil")
	}
}
