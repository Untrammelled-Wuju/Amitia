package extension

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

type legacyPackageMigrationCandidate struct {
	ExtensionID string `gorm:"column:extension_id"`
	Version     string `gorm:"column:version"`
	PackageBlob []byte `gorm:"column:package_blob"`
	UserID      string `gorm:"column:user_id"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
}

func (s *PackageService) MigrateLegacyPackages(ctx context.Context) error {
	if s == nil || s.repository == nil || s.repository.db == nil {
		return fmt.Errorf("legacy package migration repository unavailable")
	}
	if s.kernelProxy == nil {
		return fmt.Errorf("extension kernel migration unavailable")
	}
	if err := s.migrateLegacyPackageSigners(ctx); err != nil {
		return err
	}
	var candidates []legacyPackageMigrationCandidate
	err := s.repository.db.WithContext(ctx).Raw(`
		SELECT e.extension_id, e.current_version AS version, COALESCE(v.package_blob, X'') AS package_blob,
		COALESCE(e.owner_user_id, '') AS user_id, COALESCE(e.scope_type, 'global') AS scope_type,
		COALESCE(e.scope_id, '') AS scope_id
		FROM extensions e
		LEFT JOIN extension_versions v ON v.extension_id = e.extension_id AND v.version = e.current_version
		WHERE COALESCE(e.archived_at, '') = ''
	`).Scan(&candidates).Error
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		candidate.UserID = strings.TrimSpace(candidate.UserID)
		candidate.ScopeType = strings.TrimSpace(candidate.ScopeType)
		candidate.ScopeID = strings.TrimSpace(candidate.ScopeID)
		if candidate.ScopeType == "" {
			candidate.ScopeType = "global"
		}
		if s.kernelProxy.legacyMigrationRecorded(ctx, candidate.ExtensionID) {
			continue
		}
		status := "migrated"
		reason := ""
		migratedAt := any(time.Now().UTC())
		if s.kernelProxy.extensionExists(ctx, candidate.ExtensionID) {
			migratedAt = time.Now().UTC()
		} else if len(candidate.PackageBlob) == 0 {
			status = "requires_manual_migration"
			reason = "legacy package blob is unavailable"
			migratedAt = nil
		} else if candidate.UserID == "" {
			status = "requires_manual_migration"
			reason = "legacy package owner is unavailable"
			migratedAt = nil
		} else {
			preview, previewErr := s.kernelProxy.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{UserID: candidate.UserID,
				ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID,
				FileName: candidate.ExtensionID + "-" + candidate.Version + ".amitiax"}, bytes.NewReader(candidate.PackageBlob))
			if previewErr != nil || len(preview.RequiredConfirmations) > 0 || !preview.Installable {
				status = "requires_manual_migration"
				if previewErr != nil {
					reason = previewErr.Error()
				} else {
					reason = "legacy package requires explicit user confirmation"
				}
				migratedAt = nil
			} else if _, installErr := s.kernelProxy.InstallPreviewedPackage(ctx, kernelruntime.PackageInstallRequest{SessionID: preview.SessionID,
				UserID: candidate.UserID, ScopeType: candidate.ScopeType, ScopeID: candidate.ScopeID, ExpectedExtensionID: candidate.ExtensionID}); installErr != nil {
				status = "requires_manual_migration"
				reason = installErr.Error()
				migratedAt = nil
			}
		}
		if len(reason) > 1000 {
			reason = reason[:1000]
		}
		if err := s.kernelProxy.recordLegacyMigration(ctx, strings.TrimSpace(candidate.ExtensionID), strings.TrimSpace(candidate.Version), status, reason, migratedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PackageService) migrateLegacyPackageSigners(ctx context.Context) error {
	if !s.repository.db.Migrator().HasTable("extension_package_signers") {
		return nil
	}
	var records []packageSignerRecord
	if err := s.repository.db.WithContext(ctx).Find(&records).Error; err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, record := range records {
		if record.Fingerprint == "" {
			continue
		}
		keyID := "legacy-" + strings.TrimPrefix(strings.ReplaceAll(record.Fingerprint, ":", "-"), "sha256-")
		if len(keyID) > 96 {
			keyID = keyID[:96]
		}
		item := kernelruntime.PackagePublisherKeyRecord{KeyID: keyID, Fingerprint: record.Fingerprint,
			PublicKey: []byte{}, PublisherID: "legacy:" + record.Fingerprint, TrustSource: "legacy_fingerprint_only",
			TrustLevel: "unknown", KeyState: "unknown", CreatedAt: record.CreatedAt, UpdatedAt: now}
		if item.CreatedAt == "" {
			item.CreatedAt = now
		}
		if err := s.kernelProxy.ReadContainer().PackageTrustRepository.Put(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
