// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import "path/filepath"

type Layout struct {
	DistributionRoot string
	BinaryPath       string

	ConfigRoot string
	ConfigPath string

	DataRoot     string
	StorageDir   string
	SnapshotsDir string
	MigrationDir string
}

func (l Layout) Clone() Layout {
	return l
}

func (l Layout) Validate() error {
	if l.DistributionRoot == "" {
		return newInvalidLayout("distribution root is empty")
	}
	if l.BinaryPath == "" {
		return newInvalidLayout("binary path is empty")
	}
	if l.ConfigRoot == "" {
		return newInvalidLayout("config root is empty")
	}
	if l.ConfigPath == "" {
		return newInvalidLayout("config path is empty")
	}
	if l.DataRoot == "" {
		return newInvalidLayout("data root is empty")
	}
	if l.StorageDir == "" {
		return newInvalidLayout("storage dir is empty")
	}
	if l.SnapshotsDir == "" {
		return newInvalidLayout("snapshots dir is empty")
	}
	if l.MigrationDir == "" {
		return newInvalidLayout("migration dir is empty")
	}

	if !filepath.IsAbs(l.DistributionRoot) {
		return newInvalidLayout("distribution root is not absolute")
	}
	if !filepath.IsAbs(l.BinaryPath) {
		return newInvalidLayout("binary path is not absolute")
	}
	if !filepath.IsAbs(l.ConfigRoot) {
		return newInvalidLayout("config root is not absolute")
	}
	if !filepath.IsAbs(l.ConfigPath) {
		return newInvalidLayout("config path is not absolute")
	}
	if !filepath.IsAbs(l.DataRoot) {
		return newInvalidLayout("data root is not absolute")
	}
	if !filepath.IsAbs(l.StorageDir) {
		return newInvalidLayout("storage dir is not absolute")
	}
	if !filepath.IsAbs(l.SnapshotsDir) {
		return newInvalidLayout("snapshots dir is not absolute")
	}
	if !filepath.IsAbs(l.MigrationDir) {
		return newInvalidLayout("migration dir is not absolute")
	}

	cleanDist := filepath.Clean(l.DistributionRoot)
	if cleanDist != l.DistributionRoot {
		return newInvalidLayout("distribution root is not clean")
	}
	cleanBin := filepath.Clean(l.BinaryPath)
	if cleanBin != l.BinaryPath {
		return newInvalidLayout("binary path is not clean")
	}
	cleanCfgR := filepath.Clean(l.ConfigRoot)
	if cleanCfgR != l.ConfigRoot {
		return newInvalidLayout("config root is not clean")
	}
	cleanCfgP := filepath.Clean(l.ConfigPath)
	if cleanCfgP != l.ConfigPath {
		return newInvalidLayout("config path is not clean")
	}
	cleanDataR := filepath.Clean(l.DataRoot)
	if cleanDataR != l.DataRoot {
		return newInvalidLayout("data root is not clean")
	}
	cleanStorage := filepath.Clean(l.StorageDir)
	if cleanStorage != l.StorageDir {
		return newInvalidLayout("storage dir is not clean")
	}
	cleanSnaps := filepath.Clean(l.SnapshotsDir)
	if cleanSnaps != l.SnapshotsDir {
		return newInvalidLayout("snapshots dir is not clean")
	}
	cleanMigr := filepath.Clean(l.MigrationDir)
	if cleanMigr != l.MigrationDir {
		return newInvalidLayout("migration dir is not clean")
	}

	if !containsPath(l.DistributionRoot, l.BinaryPath) {
		return newInvalidLayout("binary path must be within distribution root")
	}
	if !containsPath(l.ConfigRoot, l.ConfigPath) {
		return newInvalidLayout("config path must be within config root")
	}
	if !containsPath(l.DataRoot, l.StorageDir) {
		return newInvalidLayout("storage dir must be within data root")
	}
	if !containsPath(l.DataRoot, l.SnapshotsDir) {
		return newInvalidLayout("snapshots dir must be within data root")
	}
	if !containsPath(l.DataRoot, l.MigrationDir) {
		return newInvalidLayout("migration dir must be within data root")
	}

	if l.StorageDir == l.SnapshotsDir {
		return newPathOverlap(l.StorageDir, l.SnapshotsDir)
	}
	if l.StorageDir == l.MigrationDir {
		return newPathOverlap(l.StorageDir, l.MigrationDir)
	}
	if l.SnapshotsDir == l.MigrationDir {
		return newPathOverlap(l.SnapshotsDir, l.MigrationDir)
	}

	if l.DistributionRoot == l.ConfigRoot {
		return newPathOverlap(l.DistributionRoot, l.ConfigRoot)
	}
	if l.DistributionRoot == l.DataRoot {
		return newPathOverlap(l.DistributionRoot, l.DataRoot)
	}
	if l.ConfigRoot == l.DataRoot {
		return newPathOverlap(l.ConfigRoot, l.DataRoot)
	}

	if containsPath(l.DistributionRoot, l.ConfigRoot) || containsPath(l.ConfigRoot, l.DistributionRoot) {
		return newPathOverlap(l.DistributionRoot, l.ConfigRoot)
	}
	if containsPath(l.DistributionRoot, l.DataRoot) || containsPath(l.DataRoot, l.DistributionRoot) {
		return newPathOverlap(l.DistributionRoot, l.DataRoot)
	}
	if containsPath(l.ConfigRoot, l.DataRoot) || containsPath(l.DataRoot, l.ConfigRoot) {
		return newPathOverlap(l.ConfigRoot, l.DataRoot)
	}

	if isFilesystemRoot(l.DistributionRoot) {
		return newUnsafeRoot(l.DistributionRoot)
	}
	if isFilesystemRoot(l.ConfigRoot) {
		return newUnsafeRoot(l.ConfigRoot)
	}
	if isFilesystemRoot(l.DataRoot) {
		return newUnsafeRoot(l.DataRoot)
	}

	return nil
}
