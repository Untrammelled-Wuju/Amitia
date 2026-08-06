// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirectoryManager_EnsureAll(t *testing.T) {
	base := t.TempDir()

	distRoot := filepath.Join(base, "runtime", "qdrant")
	binPath := filepath.Join(distRoot, "bin", "qdrant")
	configRoot := filepath.Join(base, "config", "providers", "qdrant")
	dataRoot := filepath.Join(base, "data", "providers", "qdrant")

	layout := Layout{
		DistributionRoot: distRoot,
		BinaryPath:       binPath,
		ConfigRoot:       configRoot,
		ConfigPath:       filepath.Join(configRoot, "config.yaml"),
		DataRoot:         dataRoot,
		StorageDir:       filepath.Join(dataRoot, "storage"),
		SnapshotsDir:     filepath.Join(dataRoot, "snapshots"),
		MigrationDir:     filepath.Join(dataRoot, "migration"),
	}

	dm := NewDirectoryManager(nil)
	if err := dm.Ensure(context.Background(), layout); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	dirs := []string{configRoot, dataRoot, filepath.Join(dataRoot, "storage"), filepath.Join(dataRoot, "snapshots"), filepath.Join(dataRoot, "migration")}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("directory %s not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
		if runtime.GOOS != "windows" {
			if info.Mode().Perm() != 0750 {
				t.Errorf("%s: expected mode 0750, got %o", dir, info.Mode().Perm())
			}
		}
	}
}

func TestDirectoryManager_Idempotent(t *testing.T) {
	base := t.TempDir()

	layout := buildTestLayout(base)

	dm := NewDirectoryManager(nil)
	if err := dm.Ensure(context.Background(), layout); err != nil {
		t.Fatalf("First Ensure: %v", err)
	}
	if err := dm.Ensure(context.Background(), layout); err != nil {
		t.Fatalf("Second Ensure: %v", err)
	}
}

func TestDirectoryManager_FileConflict(t *testing.T) {
	base := t.TempDir()

	layout := buildTestLayout(base)

	conflictPath := filepath.Join(base, "config", "providers", "qdrant")
	if err := os.MkdirAll(filepath.Dir(conflictPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conflictPath, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}

	dm := NewDirectoryManager(nil)
	err := dm.Ensure(context.Background(), layout)
	if err == nil {
		t.Error("Expected error when path exists as file")
	}
}

func buildTestLayout(base string) Layout {
	distRoot := filepath.Join(base, "runtime", "qdrant")
	binPath := filepath.Join(distRoot, "bin", "qdrant")
	configRoot := filepath.Join(base, "config", "providers", "qdrant")
	dataRoot := filepath.Join(base, "data", "providers", "qdrant")

	return Layout{
		DistributionRoot: distRoot,
		BinaryPath:       binPath,
		ConfigRoot:       configRoot,
		ConfigPath:       filepath.Join(configRoot, "config.yaml"),
		DataRoot:         dataRoot,
		StorageDir:       filepath.Join(dataRoot, "storage"),
		SnapshotsDir:     filepath.Join(dataRoot, "snapshots"),
		MigrationDir:     filepath.Join(dataRoot, "migration"),
	}
}
