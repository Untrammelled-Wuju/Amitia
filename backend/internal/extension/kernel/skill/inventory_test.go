package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInventory_Empty(t *testing.T) {
	dir := t.TempDir()
	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(inv.Items))
	}
	if inv.ScriptsPresent {
		t.Fatal("expected scripts not present")
	}
}

func TestInventory_ScriptsPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "extract.py"), []byte("#!/usr/bin/env python3\nprint('hello')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\ndescription: test\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inv.ScriptsPresent {
		t.Fatal("expected scripts present")
	}
	found := false
	for _, item := range inv.Items {
		if item.Path == "scripts/extract.py" && item.Kind == ResourceKindScript {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected scripts/extract.py in items: %+v", inv.Items)
	}
}

func TestInventory_References(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "REF.md"), []byte("# Reference\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, item := range inv.Items {
		if item.Path == "references/REF.md" && item.Kind == ResourceKindReference {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected references/REF.md: %+v", inv.Items)
	}
}

func TestInventory_Assets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "logo.png"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, item := range inv.Items {
		if item.Path == "assets/logo.png" && item.Kind == ResourceKindAsset {
			found = true
			if item.SizeBytes != 100 {
				t.Fatalf("expected size 100, got %d", item.SizeBytes)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected assets/logo.png: %+v", inv.Items)
	}
}

func TestInventory_NoSymlinkFollow(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, "secret.txt"), filepath.Join(dir, "scripts", "link.txt")); err != nil {
		t.Skip("cannot create symlink (Windows privilege):", err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range inv.Items {
		if item.Path == "scripts/link.txt" {
			t.Fatal("symlink should not appear in inventory")
		}
	}
}

func TestInventory_NoContentRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	largeContent := make([]byte, 1024)
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), largeContent, 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range inv.Items {
		if item.SizeBytes != 1024 {
			t.Fatalf("expected size 1024, got %d", item.SizeBytes)
		}
	}
}

func TestInventory_NoDotFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range inv.Items {
		if item.Path == ".DS_Store" {
			t.Fatal("dot files should be skipped")
		}
	}
}

func TestInventory_SKILLMDSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: test\ndescription: test\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.md"), []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range inv.Items {
		if item.Path == "SKILL.md" {
			t.Fatal("SKILL.md should be skipped in inventory")
		}
	}
}

func TestInventory_Disabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := ParsePolicy{CollectResourceIndex: false}
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inv.Items) != 0 {
		t.Fatal("expected no items when CollectResourceIndex is disabled")
	}
}

func TestValidateResourcePath_Valid(t *testing.T) {
	tests := []string{
		"scripts/run.sh",
		"references/ref.md",
		"assets/img.png",
		"a/b/c.txt",
	}
	for _, tc := range tests {
		if err := ValidateResourcePath("/skills/test", tc); err != nil {
			t.Fatalf("expected valid path %s, got error: %v", tc, err)
		}
	}
}

func TestValidateResourcePath_RejectAbsolute(t *testing.T) {
	if err := ValidateResourcePath("/skills/test", "/etc/passwd"); err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestValidateResourcePath_RejectTraversal(t *testing.T) {
	tests := []string{
		"../etc/passwd",
		"foo/../../etc/passwd",
		"..",
		"foo/../bar/../../../baz",
	}
	for _, tc := range tests {
		if err := ValidateResourcePath("/skills/test", tc); err == nil {
			t.Fatalf("expected error for traversal path %s", tc)
		}
	}
}

func TestValidateResourcePath_Empty(t *testing.T) {
	if err := ValidateResourcePath("/skills/test", ""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestInventory_Counts(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "a.py"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "b.py"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "ref.md"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}

	policy := DefaultParsePolicy
	inv, err := CollectResourceInventory(dir, policy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.CountByType[ResourceKindScript] != 2 {
		t.Fatalf("expected 2 scripts, got %d", inv.CountByType[ResourceKindScript])
	}
	if inv.CountByType[ResourceKindReference] != 1 {
		t.Fatalf("expected 1 reference, got %d", inv.CountByType[ResourceKindReference])
	}
}
