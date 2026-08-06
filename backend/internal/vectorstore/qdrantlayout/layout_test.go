// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"runtime"
	"strings"
	"testing"
)

func TestLayoutValidate_Valid(t *testing.T) {
	var layout Layout
	if runtime.GOOS == "windows" {
		layout = Layout{
			DistributionRoot: "C:\\amitia\\runtime\\qdrant",
			BinaryPath:       "C:\\amitia\\runtime\\qdrant\\bin\\qdrant.exe",
			ConfigRoot:       "C:\\amitia\\config\\providers\\qdrant",
			ConfigPath:       "C:\\amitia\\config\\providers\\qdrant\\config.yaml",
			DataRoot:         "C:\\amitia\\data\\providers\\qdrant",
			StorageDir:       "C:\\amitia\\data\\providers\\qdrant\\storage",
			SnapshotsDir:     "C:\\amitia\\data\\providers\\qdrant\\snapshots",
			MigrationDir:     "C:\\amitia\\data\\providers\\qdrant\\migration",
		}
	} else {
		layout = Layout{
			DistributionRoot: "/amitia/runtime/qdrant",
			BinaryPath:       "/amitia/runtime/qdrant/bin/qdrant",
			ConfigRoot:       "/amitia/config/providers/qdrant",
			ConfigPath:       "/amitia/config/providers/qdrant/config.yaml",
			DataRoot:         "/amitia/data/providers/qdrant",
			StorageDir:       "/amitia/data/providers/qdrant/storage",
			SnapshotsDir:     "/amitia/data/providers/qdrant/snapshots",
			MigrationDir:     "/amitia/data/providers/qdrant/migration",
		}
	}

	if err := layout.Validate(); err != nil {
		t.Errorf("Expected valid layout, got error: %v", err)
	}
}

func TestLayoutValidate_EmptyFields(t *testing.T) {
	layout := Layout{}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for empty layout")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' in error message, got: %v", err)
	}
}

func TestLayoutValidate_RelativePaths(t *testing.T) {
	layout := Layout{
		DistributionRoot: "relative/path",
		BinaryPath:       "relative/path/bin/qdrant",
		ConfigRoot:       "config/providers/qdrant",
		ConfigPath:       "config/providers/qdrant/config.yaml",
		DataRoot:         "data/providers/qdrant",
		StorageDir:       "data/providers/qdrant/storage",
		SnapshotsDir:     "data/providers/qdrant/snapshots",
		MigrationDir:     "data/providers/qdrant/migration",
	}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for relative paths")
	}
}

func TestLayoutValidate_OverlapConfigInDistribution(t *testing.T) {
	var layout Layout
	if runtime.GOOS == "windows" {
		layout = Layout{
			DistributionRoot: "C:\\amitia\\runtime",
			BinaryPath:       "C:\\amitia\\runtime\\bin\\qdrant.exe",
			ConfigRoot:       "C:\\amitia\\runtime\\config",
			ConfigPath:       "C:\\amitia\\runtime\\config\\config.yaml",
			DataRoot:         "C:\\amitia\\data",
			StorageDir:       "C:\\amitia\\data\\storage",
			SnapshotsDir:     "C:\\amitia\\data\\snapshots",
			MigrationDir:     "C:\\amitia\\data\\migration",
		}
	} else {
		layout = Layout{
			DistributionRoot: "/amitia/runtime",
			BinaryPath:       "/amitia/runtime/bin/qdrant",
			ConfigRoot:       "/amitia/runtime/config",
			ConfigPath:       "/amitia/runtime/config/config.yaml",
			DataRoot:         "/amitia/data",
			StorageDir:       "/amitia/data/storage",
			SnapshotsDir:     "/amitia/data/snapshots",
			MigrationDir:     "/amitia/data/migration",
		}
	}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for config root inside distribution root")
	}
	if !IsPathOverlap(err) {
		t.Errorf("Expected ErrPathOverlap, got: %v", err)
	}
}

func TestLayoutValidate_OverlapDataInDistribution(t *testing.T) {
	var layout Layout
	if runtime.GOOS == "windows" {
		layout = Layout{
			DistributionRoot: "C:\\amitia",
			BinaryPath:       "C:\\amitia\\bin\\qdrant.exe",
			ConfigRoot:       "C:\\config",
			ConfigPath:       "C:\\config\\config.yaml",
			DataRoot:         "C:\\amitia\\data",
			StorageDir:       "C:\\amitia\\data\\storage",
			SnapshotsDir:     "C:\\amitia\\data\\snapshots",
			MigrationDir:     "C:\\amitia\\data\\migration",
		}
	} else {
		layout = Layout{
			DistributionRoot: "/amitia",
			BinaryPath:       "/amitia/bin/qdrant",
			ConfigRoot:       "/config",
			ConfigPath:       "/config/config.yaml",
			DataRoot:         "/amitia/data",
			StorageDir:       "/amitia/data/storage",
			SnapshotsDir:     "/amitia/data/snapshots",
			MigrationDir:     "/amitia/data/migration",
		}
	}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for data root inside distribution root")
	}
}

func TestLayoutValidate_StorageEqualsSnapshots(t *testing.T) {
	var layout Layout
	if runtime.GOOS == "windows" {
		layout = Layout{
			DistributionRoot: "C:\\runtime",
			BinaryPath:       "C:\\runtime\\bin\\qdrant.exe",
			ConfigRoot:       "C:\\config",
			ConfigPath:       "C:\\config\\config.yaml",
			DataRoot:         "C:\\data",
			StorageDir:       "C:\\data\\storage",
			SnapshotsDir:     "C:\\data\\storage",
			MigrationDir:     "C:\\data\\migration",
		}
	} else {
		layout = Layout{
			DistributionRoot: "/runtime",
			BinaryPath:       "/runtime/bin/qdrant",
			ConfigRoot:       "/config",
			ConfigPath:       "/config/config.yaml",
			DataRoot:         "/data",
			StorageDir:       "/data/storage",
			SnapshotsDir:     "/data/storage",
			MigrationDir:     "/data/migration",
		}
	}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for storage equals snapshots")
	}
}

func TestLayoutValidate_FilesystemRoot(t *testing.T) {
	layout := Layout{
		DistributionRoot: "/runtime",
		BinaryPath:       "/runtime/bin/qdrant",
		ConfigRoot:       "/config",
		ConfigPath:       "/config/config.yaml",
		DataRoot:         "/",
		StorageDir:       "/storage",
		SnapshotsDir:     "/snapshots",
		MigrationDir:     "/migration",
	}
	err := layout.Validate()
	if err == nil {
		t.Error("Expected error for filesystem root as data root")
	}
	if !IsUnsafeRoot(err) {
		t.Errorf("Expected ErrUnsafeRootPath, got: %v", err)
	}
}

func TestLayoutClone_ReturnsCopy(t *testing.T) {
	layout := Layout{
		DistributionRoot: "/runtime",
		BinaryPath:       "/runtime/bin/qdrant",
		ConfigRoot:       "/config",
		ConfigPath:       "/config/config.yaml",
		DataRoot:         "/data",
		StorageDir:       "/data/storage",
		SnapshotsDir:     "/data/snapshots",
		MigrationDir:     "/data/migration",
	}
	clone := layout.Clone()
	if clone.DistributionRoot != layout.DistributionRoot {
		t.Error("Clone does not match original")
	}
}

func TestContainsPath(t *testing.T) {
	tests := []struct {
		parent string
		child  string
		want   bool
	}{
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b", true},
		{"/a/b", "/a/bc", false},
		{"/a/b", "/a/c", false},
		{"/a/b", "/a", false},
		{"/data", "/database-old", false},
	}
	for _, tt := range tests {
		got := containsPath(tt.parent, tt.child)
		if got != tt.want {
			t.Errorf("containsPath(%q, %q) = %v, want %v", tt.parent, tt.child, got, tt.want)
		}
	}
}

func TestIsFilesystemRoot(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/a", false},
	}
	for _, tt := range tests {
		got := isFilesystemRoot(tt.path)
		if got != tt.want {
			t.Errorf("isFilesystemRoot(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func IsPathOverlap(err error) bool {
	return err != nil && strings.Contains(err.Error(), "overlap")
}

func IsUnsafeRoot(err error) bool {
	return err != nil && strings.Contains(err.Error(), "filesystem root")
}
