// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

type ReleaseStager interface {
	PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (stagingPathKey string, err error)
	VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error
}

type releaseStager struct {
	db        *gorm.DB
	registry  *security.PathRootRegistry
	responder *security.SafeArtifactResponder
}

func NewReleaseStager(db *gorm.DB, registry *security.PathRootRegistry) ReleaseStager {
	return &releaseStager{
		db:        db,
		registry:  registry,
		responder: security.NewSafeArtifactResponder(registry),
	}
}

func (s *releaseStager) PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (string, error) {
	if releaseID == "" {
		return "", fmt.Errorf("stager: releaseID is empty")
	}
	if installationID == "" {
		return "", fmt.Errorf("stager: installationID is empty")
	}
	if s.db == nil || s.registry == nil || s.responder == nil {
		return "", fmt.Errorf("stager: dependencies are not initialized")
	}

	var releaseData struct {
		ID              string `gorm:"column:id"`
		StorageKey      string `gorm:"column:storage_key"`
		ContentRootHash string `gorm:"column:content_root_hash"`
	}
	if err := s.db.WithContext(ctx).Table("desktop_pet_package_releases").
		Where("id = ?", releaseID).
		Take(&releaseData).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("stager: release not found: %s", releaseID)
		}
		return "", fmt.Errorf("stager: query release: %w", err)
	}
	if releaseData.ContentRootHash == "" || strings.TrimSpace(releaseData.StorageKey) == "" {
		return "", fmt.Errorf("stager: release %s has incomplete published metadata", releaseID)
	}

	publishedDir, err := s.registry.Resolve(security.RootReleasePublished, releaseData.StorageKey)
	if err != nil {
		return "", fmt.Errorf("stager: resolve published directory: %w", err)
	}
	info, err := os.Lstat(publishedDir)
	if err != nil {
		return "", fmt.Errorf("stager: published directory not found: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("stager: published path is not a safe directory")
	}

	installRoot, err := s.registry.Root(security.RootInstallations)
	if err != nil {
		return "", fmt.Errorf("stager: installation root unavailable: %w", err)
	}
	stagingParent := filepath.Join(installRoot, ".staging")
	if err := os.MkdirAll(stagingParent, 0o700); err != nil {
		return "", fmt.Errorf("stager: create staging parent: %w", err)
	}

	stagingKey := filepath.ToSlash(filepath.Join(".staging", installationID))
	stagingDir, err := s.registry.Resolve(security.RootInstallations, stagingKey)
	if err != nil {
		return "", fmt.Errorf("stager: resolve installation staging path: %w", err)
	}
	if _, err := os.Lstat(stagingDir); err == nil {
		if err := s.responder.SafeDelete(
			security.RootInstallations,
			stagingKey,
			security.DeleteExpectation{EntityType: "installation_staging", EntityID: installationID},
		); err != nil {
			return "", fmt.Errorf("stager: safely clean existing staging dir: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stager: inspect existing staging dir: %w", err)
	}

	if err := s.responder.SafeCopyTree(publishedDir, stagingDir); err != nil {
		return "", fmt.Errorf("stager: copy tree: %w", err)
	}
	return stagingKey, nil
}

func (s *releaseStager) VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error {
	if releaseID == "" || installationID == "" || stagingPathKey == "" {
		return fmt.Errorf("stager: invalid verify parameters")
	}
	if s.db == nil || s.registry == nil {
		return fmt.Errorf("stager: dependencies are not initialized")
	}

	var releaseData struct {
		ID              string `gorm:"column:id"`
		FileCount       int    `gorm:"column:file_count"`
		ContentRootHash string `gorm:"column:content_root_hash"`
		ManifestHash    string `gorm:"column:manifest_hash"`
		ManifestJSON    string `gorm:"column:manifest_json"`
	}
	if err := s.db.WithContext(ctx).Table("desktop_pet_package_releases").
		Where("id = ?", releaseID).
		Take(&releaseData).Error; err != nil {
		return fmt.Errorf("stager: query release for verify: %w", err)
	}

	stagingDir, err := s.registry.Resolve(security.RootInstallations, stagingPathKey)
	if err != nil {
		return fmt.Errorf("stager: resolve staging directory: %w", err)
	}
	info, err := os.Lstat(stagingDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("stager: staging directory is missing or unsafe")
	}

	var releaseFiles []release.ReleaseFileData
	if err := s.db.WithContext(ctx).Table("desktop_pet_release_files").
		Where("release_id = ?", releaseID).
		Order("path ASC").
		Find(&releaseFiles).Error; err != nil {
		return fmt.Errorf("stager: query release files: %w", err)
	}
	if len(releaseFiles) != releaseData.FileCount {
		return fmt.Errorf("stager: file count mismatch: expected=%d actual=%d", releaseData.FileCount, len(releaseFiles))
	}
	for _, item := range releaseFiles {
		fullPath, err := packageformat.SecureJoinUnderRoot(stagingDir, item.Path)
		if err != nil {
			return fmt.Errorf("stager: unsafe release file path %s: %w", item.Path, err)
		}
		fileInfo, err := os.Lstat(fullPath)
		if err != nil {
			return fmt.Errorf("stager: missing release file %s: %w", item.Path, err)
		}
		if !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("stager: release file is not regular: %s", item.Path)
		}
		hash, size, err := release.HashFile(fullPath)
		if err != nil {
			return fmt.Errorf("stager: hash release file %s: %w", item.Path, err)
		}
		if !strings.EqualFold(hash, item.SHA256) || size != item.Bytes {
			return fmt.Errorf("stager: release file mismatch %s", item.Path)
		}
	}

	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(releaseData.ManifestJSON), &manifest); err != nil {
		return fmt.Errorf("stager: unmarshal manifest: %w", err)
	}
	canonicalManifestHash, err := packageformat.CanonicalManifestHash(&manifest)
	if err != nil {
		return fmt.Errorf("stager: canonical manifest hash: %w", err)
	}
	if !strings.EqualFold(canonicalManifestHash, releaseData.ManifestHash) ||
		!strings.EqualFold(manifest.Integrity.ManifestHash, releaseData.ManifestHash) {
		return fmt.Errorf("stager: manifest hash mismatch")
	}
	if !strings.EqualFold(manifest.Integrity.ContentRootHash, releaseData.ContentRootHash) {
		return fmt.Errorf("stager: content root hash mismatch")
	}

	report := packageformat.NewValidator().ValidateDirectory(stagingDir, &manifest)
	if report == nil || report.Verdict == "invalid" {
		return fmt.Errorf("stager: packageformat validation failed")
	}
	return nil
}
