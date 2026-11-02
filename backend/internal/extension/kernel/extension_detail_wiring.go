package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/extension_detail"
)

type packageVersionDetailReader struct {
	repository *PackageRepository
}

func (r packageVersionDetailReader) ListVersionDetails(ctx context.Context, extensionID string) ([]extension_detail.VersionDetail, error) {
	if r.repository == nil {
		return []extension_detail.VersionDetail{}, nil
	}
	records, err := r.repository.ListPackageVersions(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("kernel: list package version details %s: %w", extensionID, err)
	}
	details := make([]extension_detail.VersionDetail, 0, len(records))
	for _, record := range records {
		var releasedAt *time.Time
		if record.InstalledAt != "" {
			if parsed, parseErr := time.Parse(time.RFC3339Nano, record.InstalledAt); parseErr == nil {
				releasedAt = &parsed
			}
		}
		details = append(details, extension_detail.VersionDetail{
			Version:    record.Version,
			ReleasedAt: releasedAt,
			Current:    record.IsActive || record.VersionState == string(PackageVersionStateCurrent),
		})
	}
	return details, nil
}

func (r packageVersionDetailReader) HasRollbackVersion(ctx context.Context, extensionID, currentVersion string) (bool, error) {
	if r.repository == nil {
		return false, nil
	}
	records, err := r.repository.ListPackageVersions(ctx, extensionID)
	if err != nil {
		return false, fmt.Errorf("kernel: list rollback versions %s: %w", extensionID, err)
	}
	for _, record := range records {
		if record.Version == currentVersion {
			continue
		}
		if record.IsActive || record.VersionState == string(PackageVersionStateCurrent) {
			continue
		}
		if record.VersionState == string(PackageVersionStateRemoved) ||
			record.VersionState == string(PackageVersionStateBlocked) ||
			record.VersionState == string(PackageVersionStateCorrupted) {
			continue
		}
		return true, nil
	}
	return false, nil
}
