// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

type ReleaseStager interface {
	PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (stagingPathKey string, err error)
	VerifyStagingCopy(ctx context.Context, releaseID, installationID, stagingPathKey string) error
}

type releaseStager struct {
	db          *gorm.DB
	registry    *security.PathRootRegistry
	responder   *security.SafeArtifactResponder
	stagingRoot string
}

func NewReleaseStager(db *gorm.DB, registry *security.PathRootRegistry, stagingRoot string) ReleaseStager {
	return &releaseStager{
		db:          db,
		registry:    registry,
		responder:   security.NewSafeArtifactResponder(registry),
		stagingRoot: stagingRoot,
	}
}

func (s *releaseStager) PrepareStagingCopy(ctx context.Context, releaseID, installationID string) (string, error) {
	if releaseID == "" {
		return "", fmt.Errorf("stager: releaseID is empty")
	}
	if installationID == "" {
		return "", fmt.Errorf("stager: installationID is empty")
	}

	var releaseData struct {
		ID              string `gorm:"column:id"`
		PetID           string `gorm:"column:pet_id"`
		ContentRootHash string `gorm:"column:content_root_hash"`
		ManifestJSON    string `gorm:"column:manifest_json"`
	}
	if err := s.db.WithContext(ctx).Table("desktop_pet_release_data").
		Where("id = ?", releaseID).
		Take(&releaseData).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("stager: release not found: %s", releaseID)
		}
		return "", fmt.Errorf("stager: query release: %w", err)
	}

	if releaseData.ContentRootHash == "" {
		return "", fmt.Errorf("stager: release %s has no content root hash", releaseID)
	}

	publishedDir := filepath.Join(s.stagingRoot, "..", "releases", "published", releaseData.PetID, releaseID)
	if _, err := os.Stat(publishedDir); err != nil {
		return "", fmt.Errorf("stager: published directory not found: %w", err)
	}

	stagingKey := installationID
	stagingDir := filepath.Join(s.stagingRoot, stagingKey)

	if err := os.RemoveAll(stagingDir); err != nil {
		return "", fmt.Errorf("stager: clean staging dir: %w", err)
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

	var releaseData struct {
		ID              string `gorm:"column:id"`
		PetID           string `gorm:"column:pet_id"`
		FileCount       int    `gorm:"column:file_count"`
		ContentRootHash string `gorm:"column:content_root_hash"`
		ManifestHash    string `gorm:"column:manifest_hash"`
		ManifestJSON    string `gorm:"column:manifest_json"`
	}
	if err := s.db.WithContext(ctx).Table("desktop_pet_release_data").
		Where("id = ?", releaseID).
		Take(&releaseData).Error; err != nil {
		return fmt.Errorf("stager: query release for verify: %w", err)
	}

	stagingDir := filepath.Join(s.stagingRoot, stagingPathKey)

	var actualFiles []string
	if err := filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(stagingDir, path)
		if relErr != nil {
			return relErr
		}
		actualFiles = append(actualFiles, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return fmt.Errorf("stager: walk staging dir: %w", err)
	}

	if len(actualFiles) != releaseData.FileCount {
		return fmt.Errorf("stager: file count mismatch: expected=%d actual=%d", releaseData.FileCount, len(actualFiles))
	}

	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(releaseData.ManifestJSON), &manifest); err != nil {
		return fmt.Errorf("stager: unmarshal manifest: %w", err)
	}

	actualContentRootHash, err := s.computeContentRootHash(stagingDir)
	if err != nil {
		return fmt.Errorf("stager: compute content root hash: %w", err)
	}
	if actualContentRootHash != releaseData.ContentRootHash {
		return fmt.Errorf("stager: content root hash mismatch: expected=%s actual=%s", releaseData.ContentRootHash, actualContentRootHash)
	}

	report := packageformat.NewValidator().ValidateDirectory(stagingDir, &manifest)
	if report == nil || report.Verdict == "invalid" {
		return fmt.Errorf("stager: packageformat validation failed")
	}

	return nil
}

func (s *releaseStager) computeContentRootHash(dir string) (string, error) {
	files, err := listDirFiles(dir)
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	hasher := sha256.New()
	for _, relPath := range files {
		hasher.Write([]byte(relPath))
		hasher.Write([]byte{0})

		absPath := filepath.Join(dir, filepath.FromSlash(relPath))
		f, err := os.Open(absPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func listDirFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

var _ = strings.TrimSpace
