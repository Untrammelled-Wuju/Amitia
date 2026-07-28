package extension

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type legacyPackageMigrationCandidate struct {
	ExtensionID string `gorm:"column:extension_id"`
	Version     string `gorm:"column:version"`
	PackageBlob []byte `gorm:"column:package_blob"`
}

func (s *PackageService) MigrateLegacyPackages(ctx context.Context) error {
	if s == nil || s.repository == nil || s.repository.db == nil {
		return fmt.Errorf("legacy package migration repository unavailable")
	}
	if s.kernelProxy == nil {
		return fmt.Errorf("extension kernel migration unavailable")
	}
	var candidates []legacyPackageMigrationCandidate
	err := s.repository.db.WithContext(ctx).Raw(`
		SELECT e.extension_id, e.current_version AS version, COALESCE(v.package_blob, X'') AS package_blob
		FROM extensions e
		LEFT JOIN extension_versions v ON v.extension_id = e.extension_id AND v.version = e.current_version
		WHERE COALESCE(e.archived_at, '') = ''
	`).Scan(&candidates).Error
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
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
		} else if _, _, installErr := s.kernelProxy.InstallPackage(ctx, candidate.PackageBlob, candidate.ExtensionID+"-"+candidate.Version+".amitiax", candidate.ExtensionID); installErr != nil {
			status = "requires_manual_migration"
			reason = installErr.Error()
			migratedAt = nil
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
