package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

type legacyDesktopPetPackage struct {
	ID               string
	SpaceID          string
	Name             string
	Version          int
	PackagePath      string
	ManifestPath     string
	GenerationTaskID string
	ProcessingTaskID string
}

type legacyDesktopPetMigrationTarget struct {
	ID               string
	SpaceID          string
	DeviceID         string
	CurrentReleaseID string
}

func migrateLegacyDesktopPetPackages(
	ctx context.Context,
	db *gorm.DB,
	port release.LegacyPackageBuildPort,
	dataDir string,
) error {
	if db == nil || port == nil {
		return errors.New("legacy desktop pet migration dependencies unavailable")
	}
	var packages []legacyDesktopPetPackage
	if err := db.WithContext(ctx).
		Table("desktop_pet_packages").
		Select("id, space_id, name, version, package_path, manifest_path, generation_task_id, processing_task_id").
		Where("status IN ?", []string{"ready", "succeeded"}).
		Order("created_at ASC").
		Find(&packages).Error; err != nil {
		return fmt.Errorf("list legacy desktop pet packages: %w", err)
	}
	for _, pkg := range packages {
		if err := migrateLegacyDesktopPetPackage(ctx, db, port, dataDir, pkg); err != nil {
			return fmt.Errorf("migrate legacy desktop pet package %s: %w", pkg.ID, err)
		}
	}
	return nil
}

func migrateLegacyDesktopPetPackage(
	ctx context.Context,
	db *gorm.DB,
	port release.LegacyPackageBuildPort,
	dataDir string,
	pkg legacyDesktopPetPackage,
) error {
	var mapping release.LegacyPackageMapping
	mappingErr := db.WithContext(ctx).
		Where("legacy_package_id = ?", pkg.ID).
		First(&mapping).Error
	if mappingErr == nil && mapping.MigrationStatus == release.LegacyMigrationStatusMigrated &&
		mapping.MigratedPetID != "" && mapping.MigratedReleaseID != "" {
		return bindLegacyDesktopPetInstallations(ctx, db, pkg.ID, mapping.MigratedPetID, mapping.MigratedReleaseID)
	}
	if mappingErr != nil && !errors.Is(mappingErr, gorm.ErrRecordNotFound) {
		return mappingErr
	}

	packageDir, err := confinedLegacyPath(dataDir, pkg.PackagePath)
	if err != nil {
		return err
	}
	manifestPath, err := confinedLegacyPath(dataDir, pkg.ManifestPath)
	if err != nil {
		return err
	}
	result, err := port.MigrateLegacyPackage(ctx, release.LegacyPackageBuildRequest{
		SpaceID:         pkg.SpaceID,
		LegacyPackageID: pkg.ID,
		PackageName:     pkg.Name,
		PackageDir:      packageDir,
		ManifestPath:    manifestPath,
		Version:         pkg.Version,
	})
	if err != nil {
		return err
	}
	if result == nil || result.Release == nil || result.PetID == "" {
		return errors.New("legacy package migration returned incomplete result")
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if mappingErr != nil {
		mapping = release.LegacyPackageMapping{
			ID:                   uuid.NewString(),
			LegacyPackageID:      pkg.ID,
			OwnerSpaceID:         pkg.SpaceID,
			CreatedAt:            now,
			UpdatedAt:            now,
			MigrationOperationId: uuid.NewString(),
		}
	}
	mapping.MigratedPetID = result.PetID
	mapping.MigratedReleaseID = result.Release.ID
	mapping.MigrationStatus = release.LegacyMigrationStatusMigrated
	mapping.SourceContentHash = result.Release.ContentRootHash
	mapping.SourceManifestHash = result.Release.ManifestHash
	mapping.ErrorMessage = ""
	mapping.UpdatedAt = now
	if mappingErr != nil {
		if err := db.WithContext(ctx).Create(&mapping).Error; err != nil {
			return fmt.Errorf("create legacy package mapping: %w", err)
		}
	} else if err := db.WithContext(ctx).Save(&mapping).Error; err != nil {
		return fmt.Errorf("update legacy package mapping: %w", err)
	}
	return bindLegacyDesktopPetInstallations(ctx, db, pkg.ID, result.PetID, result.Release.ID)
}

func bindLegacyDesktopPetInstallations(
	ctx context.Context,
	db *gorm.DB,
	legacyPackageID, petID, releaseID string,
) error {
	var installations []legacyDesktopPetMigrationTarget
	if err := db.WithContext(ctx).
		Table("desktop_pet_installations").
		Select("id, space_id, device_id, current_release_id").
		Where("package_id = ? AND (current_release_id = '' OR current_release_id IS NULL)", legacyPackageID).
		Find(&installations).Error; err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, installation := range installations {
			if err := tx.Table("desktop_pet_installations").
				Where("id = ?", installation.ID).
				Updates(map[string]any{
					"pet_id":             petID,
					"current_release_id": releaseID,
					"legacy_package_id":  legacyPackageID,
				}).Error; err != nil {
				return err
			}
			if err := tx.Table("desktop_pet_runtime_desired_states").
				Where("space_id = ? AND device_id = ? AND installation_id = ?", installation.SpaceID, installation.DeviceID, installation.ID).
				Updates(map[string]any{"pet_id": petID, "release_id": releaseID}).Error; err != nil {
				return err
			}
			if err := tx.Table("desktop_pet_device_active_installation_bindings").
				Where("space_id = ? AND device_id = ? AND installation_id = ?", installation.SpaceID, installation.DeviceID, installation.ID).
				Updates(map[string]any{"pet_id": petID, "release_id": releaseID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func confinedLegacyPath(dataDir, relativePath string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(dataDir))
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(strings.TrimSpace(relativePath))))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("legacy desktop pet package path escapes data directory")
	}
	return target, nil
}
