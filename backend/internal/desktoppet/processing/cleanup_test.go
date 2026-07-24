// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupTempDir(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	tmpDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", ".tmp")
	if err := os.MkdirAll(filepath.Join(tmpDir, "actions", "idle_normal"), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "actions", "idle_normal", "frame-0001.png"), []byte("test"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := c.CleanupTempDir("task-1"); err != nil {
		t.Fatalf("CleanupTempDir failed: %v", err)
	}
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("expected tmp dir removed, got err=%v", err)
	}
}

func TestCleanupTempDirNotExist(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupTempDir("task-no-tmp"); err != nil {
		t.Errorf("expected nil for non-existent tmp dir, got %v", err)
	}
}

func TestCleanupTempDirEmptyTaskID(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupTempDir(""); err == nil {
		t.Fatal("expected error for empty taskID")
	}
}

func TestCleanupProcessingVersion(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	tmpDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", ".tmp")
	versionDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", "version-1")

	if err := os.MkdirAll(filepath.Join(tmpDir, "actions", "wave"), 0755); err != nil {
		t.Fatalf("mkdir tmp failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "actions", "wave", "frame-0001.png"), []byte("test"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("mkdir version failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "package-preview.png"), []byte("final"), 0644); err != nil {
		t.Fatalf("write version file failed: %v", err)
	}

	if err := c.CleanupProcessingVersion("task-1", 1); err != nil {
		t.Fatalf("CleanupProcessingVersion failed: %v", err)
	}

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("expected tmp dir removed, got err=%v", err)
	}
	if _, err := os.Stat(versionDir); err != nil {
		t.Errorf("version dir should not be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(versionDir, "package-preview.png")); err != nil {
		t.Errorf("version file should not be removed, got err=%v", err)
	}
}

func TestCleanupProcessingVersionNoTmp(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupProcessingVersion("task-no-tmp", 1); err != nil {
		t.Errorf("expected nil for non-existent tmp, got %v", err)
	}
}

func TestCleanupProcessingVersionInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupProcessingVersion("task-v", 0); err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestCleanupFailedPackage(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	pkgDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "packages", "pkg-1")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "manifest.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	if err := c.CleanupFailedPackage("task-1", "pkg-1"); err != nil {
		t.Fatalf("CleanupFailedPackage failed: %v", err)
	}
	if _, err := os.Stat(pkgDir); !os.IsNotExist(err) {
		t.Errorf("expected pkg dir removed, got err=%v", err)
	}
}

func TestCleanupFailedPackageNotExist(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupFailedPackage("task-1", "pkg-none"); err != nil {
		t.Errorf("expected nil for non-existent pkg, got %v", err)
	}
}

func TestCleanupFailedPackageEmptyArgs(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	if err := c.CleanupFailedPackage("", "pkg-1"); err == nil {
		t.Fatal("expected error for empty taskID")
	}
	if err := c.CleanupFailedPackage("task-1", ""); err == nil {
		t.Fatal("expected error for empty packageID")
	}
}

func TestCleanupEnsureVersionDir(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	vdir, err := c.EnsureVersionDir("task-1", 2)
	if err != nil {
		t.Fatalf("EnsureVersionDir failed: %v", err)
	}
	if vdir == "" {
		t.Fatal("expected non-empty path")
	}
	if _, err := os.Stat(vdir); err != nil {
		t.Errorf("version dir not exist: %v", err)
	}
	expected := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", "version-2")
	if vdir != expected {
		t.Errorf("expected %s, got %s", expected, vdir)
	}
}

func TestCleanupEnsureVersionDirIdempotent(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	vdir1, err := c.EnsureVersionDir("task-1", 1)
	if err != nil {
		t.Fatalf("first EnsureVersionDir failed: %v", err)
	}
	vdir2, err := c.EnsureVersionDir("task-1", 1)
	if err != nil {
		t.Fatalf("second EnsureVersionDir failed: %v", err)
	}
	if vdir1 != vdir2 {
		t.Errorf("expected same path, got %s and %s", vdir1, vdir2)
	}
}

func TestCleanupEnsureVersionDirInvalidVersion(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	_, err := c.EnsureVersionDir("task-1", 0)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestCleanupEnsureActionsDir(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	adir, err := c.EnsureActionsDir("task-1", 1, "wave")
	if err != nil {
		t.Fatalf("EnsureActionsDir failed: %v", err)
	}
	if adir == "" {
		t.Fatal("expected non-empty path")
	}
	if _, err := os.Stat(adir); err != nil {
		t.Errorf("actions dir not exist: %v", err)
	}
	expected := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", "version-1", "actions", "wave")
	if adir != expected {
		t.Errorf("expected %s, got %s", expected, adir)
	}
}

func TestCleanupEnsureActionsDirEmptyActionKey(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	_, err := c.EnsureActionsDir("task-1", 1, "")
	if err == nil {
		t.Fatal("expected error for empty actionKey")
	}
}

func TestCleanupDoesNotTouchRaw(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	rawDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "generated", "raw")
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		t.Fatalf("mkdir raw failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rawDir, "frame-001.png"), []byte("raw"), 0644); err != nil {
		t.Fatalf("write raw file failed: %v", err)
	}

	tmpDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", ".tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir tmp failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "frame-001.png"), []byte("tmp"), 0644); err != nil {
		t.Fatalf("write tmp file failed: %v", err)
	}

	if err := c.CleanupProcessingVersion("task-1", 1); err != nil {
		t.Fatalf("CleanupProcessingVersion failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rawDir, "frame-001.png")); err != nil {
		t.Errorf("raw file should not be removed, got err=%v", err)
	}
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Errorf("tmp dir should be removed, got err=%v", err)
	}
}

func TestCleanupDoesNotTouchHistoryVersion(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	v1Dir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", "version-1")
	if err := os.MkdirAll(v1Dir, 0755); err != nil {
		t.Fatalf("mkdir v1 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "package-preview.png"), []byte("old"), 0644); err != nil {
		t.Fatalf("write v1 file failed: %v", err)
	}

	tmpDir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "processed", ".tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir tmp failed: %v", err)
	}

	if err := c.CleanupProcessingVersion("task-1", 2); err != nil {
		t.Fatalf("CleanupProcessingVersion failed: %v", err)
	}

	if _, err := os.Stat(v1Dir); err != nil {
		t.Errorf("v1 dir should not be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(v1Dir, "package-preview.png")); err != nil {
		t.Errorf("v1 file should not be removed, got err=%v", err)
	}
}

func TestCleanupDoesNotTouchHistoryPackages(t *testing.T) {
	dir := t.TempDir()
	c := NewCleanupManager(dir)

	pkg1Dir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "packages", "pkg-1")
	pkg2Dir := filepath.Join(dir, "desktop-pets", "generation-tasks", "task-1", "packages", "pkg-2")
	if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
		t.Fatalf("mkdir pkg1 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg1Dir, "manifest.json"), []byte("pkg1"), 0644); err != nil {
		t.Fatalf("write pkg1 file failed: %v", err)
	}
	if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
		t.Fatalf("mkdir pkg2 failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg2Dir, "manifest.json"), []byte("pkg2"), 0644); err != nil {
		t.Fatalf("write pkg2 file failed: %v", err)
	}

	if err := c.CleanupFailedPackage("task-1", "pkg-1"); err != nil {
		t.Fatalf("CleanupFailedPackage failed: %v", err)
	}

	if _, err := os.Stat(pkg1Dir); !os.IsNotExist(err) {
		t.Errorf("pkg1 dir should be removed, got err=%v", err)
	}
	if _, err := os.Stat(pkg2Dir); err != nil {
		t.Errorf("pkg2 dir should not be removed, got err=%v", err)
	}
}
