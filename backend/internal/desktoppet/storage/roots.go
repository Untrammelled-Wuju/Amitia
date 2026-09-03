// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

type StorageRootKind string

const (
	RootReferenceAssets      StorageRootKind = "reference_assets"
	RootGenerationWork       StorageRootKind = "generation_work"
	RootGenerationArtifacts  StorageRootKind = "generation_artifacts"
	RootProcessingWork       StorageRootKind = "processing_work"
	RootProcessingRevisions  StorageRootKind = "processing_revisions"
	RootReleaseWork          StorageRootKind = "release_work"
	RootReleasePublished     StorageRootKind = "release_published"
	RootReleaseArchives      StorageRootKind = "release_archives"
	RootInstallations        StorageRootKind = "installations"
	RootInstallationRollback StorageRootKind = "installation_rollback"
	RootInstallationTrash    StorageRootKind = "installation_trash"
	RootEditingUploads       StorageRootKind = "editing_uploads"
	RootQualityReports       StorageRootKind = "quality_reports"
	RootImportQuarantine     StorageRootKind = "import_quarantine"
	RootMigrationBackup      StorageRootKind = "migration_backup"
)

type CanonicalRoot struct {
	Kind         StorageRootKind
	AbsolutePath string
}

type PathRootRegistry interface {
	Root(kind StorageRootKind) (CanonicalRoot, error)
	Register(kind StorageRootKind, absolutePath string) error
}

type defaultPathRootRegistry struct {
	mu    sync.RWMutex
	roots map[StorageRootKind]string
	base  string
}

func NewPathRootRegistry(baseDir string) PathRootRegistry {
	return &defaultPathRootRegistry{
		roots: make(map[StorageRootKind]string),
		base:  baseDir,
	}
}

func (r *defaultPathRootRegistry) Register(kind StorageRootKind, absolutePath string) error {
	if kind == "" {
		return errors.New("storage: root kind cannot be empty")
	}
	abs, err := filepath.Abs(absolutePath)
	if err != nil {
		return fmt.Errorf("storage: failed to resolve path: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roots[kind] = abs
	return nil
}

func (r *defaultPathRootRegistry) Root(kind StorageRootKind) (CanonicalRoot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if abs, ok := r.roots[kind]; ok && abs != "" {
		return CanonicalRoot{Kind: kind, AbsolutePath: abs}, nil
	}
	if r.base == "" {
		return CanonicalRoot{}, fmt.Errorf("storage: root %q not registered and no base directory", kind)
	}
	rel := string(kind)
	rel = strings.ReplaceAll(rel, "_", "/")
	joined := filepath.Join(r.base, rel)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return CanonicalRoot{}, fmt.Errorf("storage: failed to resolve root %q: %w", kind, err)
	}
	return CanonicalRoot{Kind: kind, AbsolutePath: abs}, nil
}
