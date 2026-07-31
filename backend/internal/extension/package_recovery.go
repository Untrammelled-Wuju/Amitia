//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

func (s *PackageService) recoverPackageOperations(ctx context.Context) error {
	var operations []packageOperationRecord
	if err := s.repository.db.WithContext(ctx).Where("status NOT IN ?", []string{"succeeded", "failed", "compensated"}).Order("created_at ASC").Find(&operations).Error; err != nil {
		return err
	}
	for _, operation := range operations {
		if err := s.recoverPackageOperation(ctx, operation); err != nil {
			return err
		}
	}
	return nil
}

func (s *PackageService) recoverPackageOperation(ctx context.Context, operation packageOperationRecord) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if operation.Status == "pending" || operation.Status == "validating" || operation.Status == "testing" || operation.Status == "staging" {
		return s.failPreRegistrationOperation(ctx, operation, now)
	}
	var current extensionRecord
	currentErr := s.repository.db.WithContext(ctx).Where("extension_id = ?", operation.ExtensionID).First(&current).Error
	var target packageVersionRecord
	targetErr := s.repository.db.WithContext(ctx).Where("extension_id = ? AND version = ?", operation.ExtensionID, operation.TargetVersion).First(&target).Error
	var artifact packageArtifactRecord
	artifactErr := gorm.ErrRecordNotFound
	if targetErr == nil && target.ArtifactID != "" {
		artifactErr = s.repository.db.WithContext(ctx).Where("artifact_id = ?", target.ArtifactID).First(&artifact).Error
	}
	targetValid := targetErr == nil && artifactErr == nil && target.ArtifactID == artifact.ArtifactID && target.Checksum != "" && target.Checksum == artifact.Checksum
	if currentErr == nil && current.CurrentVersion == operation.TargetVersion && targetValid {
		if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if tx.Migrator().HasColumn("extension_versions", "artifact_status") {
				if err := tx.Model(&packageVersionRecord{}).Where("id = ?", target.ID).Updates(map[string]interface{}{"artifact_status": "active", "activation_status": "active", "failure_code": ""}).Error; err != nil {
					return err
				}
			}
			if tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
				if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", artifact.ArtifactID).Update("artifact_status", "active").Error; err != nil {
					return err
				}
			}
			return tx.Model(&packageOperationRecord{}).Where("id = ?", operation.ID).Updates(map[string]interface{}{"status": "succeeded", "error_code": "", "completed_at": now}).Error
		}); err != nil {
			return err
		}
		return s.ensureRecoveredVersionRegistered(ctx, current, target, artifact)
	}
	if currentErr == nil && current.CurrentVersion == operation.TargetVersion && operation.PreviousVersion != "" {
		var previous packageVersionRecord
		if err := s.repository.db.WithContext(ctx).Where("extension_id = ? AND version = ?", operation.ExtensionID, operation.PreviousVersion).First(&previous).Error; err == nil {
			var previousArtifact packageArtifactRecord
			if err := s.repository.db.WithContext(ctx).Where("artifact_id = ?", previous.ArtifactID).First(&previousArtifact).Error; err == nil && previous.Checksum == previousArtifact.Checksum {
				if err := s.repository.SetPackageOperationStatus(ctx, operation.ID, "compensating"); err != nil {
					return err
				}
				if err := s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
					if err := tx.Model(&extensionRecord{}).Where("extension_id = ?", operation.ExtensionID).Updates(map[string]interface{}{"current_version": previous.Version, "manifest_json": previous.ManifestJSON, "normalized_manifest_json": string(stableJSON(json.RawMessage(previous.ManifestJSON))), "updated_at": now}).Error; err != nil {
						return err
					}
					if tx.Migrator().HasColumn("extension_versions", "artifact_status") && targetErr == nil {
						if err := tx.Model(&packageVersionRecord{}).Where("id = ?", target.ID).Updates(map[string]interface{}{"artifact_status": "orphaned", "activation_status": "failed", "failure_code": ErrPackageArtifactInvalid, "archived_at": now}).Error; err != nil {
							return err
						}
						if err := tx.Model(&packageVersionRecord{}).Where("id = ?", previous.ID).Updates(map[string]interface{}{"artifact_status": "active", "activation_status": "active", "failure_code": ""}).Error; err != nil {
							return err
						}
					}
					if tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
						if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", target.ArtifactID).Update("artifact_status", "orphaned").Error; err != nil {
							return err
						}
						if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", previous.ArtifactID).Update("artifact_status", "active").Error; err != nil {
							return err
						}
					}
					return tx.Model(&packageOperationRecord{}).Where("id = ?", operation.ID).Updates(map[string]interface{}{"status": "compensated", "error_code": ErrPackageArtifactInvalid, "completed_at": now}).Error
				}); err != nil {
					return err
				}
				current.CurrentVersion = previous.Version
				current.ManifestJSON = previous.ManifestJSON
				return s.ensureRecoveredVersionRegistered(ctx, current, previous, previousArtifact)
			}
		}
	}
	if err := s.repository.SetPackageOperationStatus(ctx, operation.ID, "compensating"); err != nil {
		return err
	}
	return s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if targetErr == nil && (currentErr != nil || current.CurrentVersion != target.Version) {
			if artifactErr == nil && tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
				if err := tx.Model(&packageArtifactRecord{}).Where("artifact_id = ?", artifact.ArtifactID).Update("artifact_status", "orphaned").Error; err != nil {
					return err
				}
			}
			if tx.Migrator().HasColumn("extension_versions", "activation_status") {
				if err := tx.Model(&packageVersionRecord{}).Where("id = ?", target.ID).Updates(map[string]interface{}{"artifact_status": "orphaned", "activation_status": "failed", "failure_code": ErrPackageInstallFailed, "archived_at": now}).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&packageOperationRecord{}).Where("id = ?", operation.ID).Updates(map[string]interface{}{"status": "compensated", "error_code": ErrPackageInstallFailed, "completed_at": now}).Error
	})
}

func (s *PackageService) failPreRegistrationOperation(ctx context.Context, operation packageOperationRecord, now string) error {
	return s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var versions []packageVersionRecord
		if tx.Migrator().HasColumn("extension_versions", "operation_id") {
			if err := tx.Where("operation_id = ?", operation.ID).Find(&versions).Error; err != nil {
				return err
			}
			for _, version := range versions {
				if err := tx.Where("extension_id = ? AND extension_version = ?", version.ExtensionID, version.Version).Delete(&packageDependencyRecord{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("operation_id = ?", operation.ID).Delete(&packageVersionRecord{}).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_artifacts", "operation_id") {
			if err := tx.Where("operation_id = ?", operation.ID).Delete(&packageArtifactRecord{}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&packageOperationRecord{}).Where("id = ?", operation.ID).Updates(map[string]interface{}{"status": "failed", "error_code": ErrPackageInstallFailed, "completed_at": now}).Error
	})
}

func (s *PackageService) ensureRecoveredVersionRegistered(ctx context.Context, current extensionRecord, version packageVersionRecord, artifact packageArtifactRecord) error {
	if registered, err := s.registry.Get(ctx, current.ExtensionID); err == nil && registered.Definition.Version == version.Version {
		return nil
	} else if err == nil {
		_ = s.registry.Unregister(ctx, current.ExtensionID)
	}
	var definition SkillDefinition
	var handler SkillHandler
	var err error
	if artifact.ArtifactKind == "agent-skill" {
		var manifest Manifest
		if json.Unmarshal([]byte(artifact.ManifestJSON), &manifest) != nil {
			return NewExtensionError(ErrPackageArtifactInvalid, "恢复版本 Manifest 无效", artifact.ArtifactID, false, nil)
		}
		definition = skillDefinitionFromManifest(manifest, map[string]json.RawMessage{})
		definition.Source = SkillSourceInstructions
	} else {
		definition, handler, err = s.workflowInstaller.definitionFromArtifact(artifact.base())
		if err != nil {
			return err
		}
	}
	definition.Enabled = current.Enabled == 1
	if err := s.registry.Register(ctx, definition, handler); err != nil {
		var extErr *ExtensionError
		if errors.As(err, &extErr) && extErr.Code == ErrSkillDuplicateID {
			return nil
		}
		return err
	}
	return nil
}

func (s *PackageService) cleanupPackageRecoveryDebris(ctx context.Context) error {
	return s.repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasColumn("extension_artifacts", "artifact_status") {
			if err := tx.Exec(`DELETE FROM extension_artifacts WHERE artifact_status IN ('staged', 'orphaned') AND artifact_id NOT IN (SELECT artifact_id FROM extension_versions WHERE artifact_id <> '')`).Error; err != nil {
				return err
			}
		}
		if tx.Migrator().HasColumn("extension_versions", "activation_status") {
			if err := tx.Exec(`DELETE FROM extension_version_dependencies WHERE (extension_id, extension_version) IN (SELECT v.extension_id, v.version FROM extension_versions v LEFT JOIN extensions e ON e.extension_id = v.extension_id AND e.current_version = v.version WHERE v.activation_status = 'orphaned' AND e.extension_id IS NULL)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`DELETE FROM extension_versions WHERE activation_status = 'orphaned' AND NOT EXISTS (SELECT 1 FROM extensions e WHERE e.extension_id = extension_versions.extension_id AND e.current_version = extension_versions.version)`).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
