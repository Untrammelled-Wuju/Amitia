package extension

import (
	"context"
	"fmt"
	"strings"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"gorm.io/gorm"
)

type legacyPackageMigrationCandidate struct {
	ExtensionID string `gorm:"column:extension_id"`
	Version     string `gorm:"column:version"`
	PackageBlob []byte `gorm:"column:package_blob"`
	UserID      string `gorm:"column:user_id"`
	ScopeType   string `gorm:"column:scope_type"`
	ScopeID     string `gorm:"column:scope_id"`
}

type LegacyMigrationReport struct {
	Total             int
	Completed         int
	PendingManual     int
	PendingExtensions []string
}

type LegacyMigrationDetector struct {
	kernel *kernelruntime.Runtime
	db     *gorm.DB
}

func NewLegacyMigrationDetector(kernel *kernelruntime.Runtime, db *gorm.DB) *LegacyMigrationDetector {
	return &LegacyMigrationDetector{kernel: kernel, db: db}
}

func (d *LegacyMigrationDetector) Detect(ctx context.Context) (LegacyMigrationReport, error) {
	var report LegacyMigrationReport
	if d == nil || d.kernel == nil || d.db == nil {
		return report, fmt.Errorf("legacy migration detector dependencies unavailable")
	}
	var candidates []legacyPackageMigrationCandidate
	err := d.db.WithContext(ctx).Raw(`
		SELECT e.extension_id, e.current_version AS version, COALESCE(v.package_blob, X'') AS package_blob,
		COALESCE(e.owner_user_id, '') AS user_id, COALESCE(e.scope_type, 'global') AS scope_type,
		COALESCE(e.scope_id, '') AS scope_id
		FROM extensions e
		LEFT JOIN extension_versions v ON v.extension_id = e.extension_id AND v.version = e.current_version
		WHERE COALESCE(e.archived_at, '') = ''
	`).Scan(&candidates).Error
	if err != nil {
		return report, err
	}
	report.Total = len(candidates)
	container := d.kernel.Container()
	if container == nil || container.InstallationRepository == nil {
		return report, fmt.Errorf("extension kernel installation repository unavailable")
	}
	for _, candidate := range candidates {
		candidate.UserID = strings.TrimSpace(candidate.UserID)
		candidate.ScopeType = strings.TrimSpace(candidate.ScopeType)
		candidate.ScopeID = strings.TrimSpace(candidate.ScopeID)
		if candidate.ScopeType == "" {
			candidate.ScopeType = "global"
		}
		_, installErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(candidate.ExtensionID))
		if installErr == nil {
			report.Completed++
			continue
		}
		report.PendingManual++
		report.PendingExtensions = append(report.PendingExtensions, candidate.ExtensionID)
	}
	return report, nil
}
