package desktop_update

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type UpdateType string

const (
	UpdateTypePatch      UpdateType = "patch"
	UpdateTypeMinor      UpdateType = "minor"
	UpdateTypeMajor      UpdateType = "major"
	UpdateTypePrerelease UpdateType = "prerelease"
	UpdateTypeDowngrade  UpdateType = "downgrade"
	UpdateTypeSame       UpdateType = "same"
)

func CompareVersions(current, target string) (UpdateType, error) {
	curVer, err := ParseVersion(current)
	if err != nil {
		return "", fmt.Errorf("desktop_update: invalid current version %s: %w", current, err)
	}
	tgtVer, err := ParseVersion(target)
	if err != nil {
		return "", fmt.Errorf("desktop_update: invalid target version %s: %w", target, err)
	}

	cmp := curVer.Compare(tgtVer)
	if cmp == 0 {
		return UpdateTypeSame, nil
	}
	if cmp > 0 {
		return UpdateTypeDowngrade, nil
	}

	if tgtVer.PreRelease != "" && curVer.PreRelease == "" {
		return UpdateTypePrerelease, nil
	}
	if tgtVer.PreRelease != "" && curVer.PreRelease != "" {
		if tgtVer.Major == curVer.Major && tgtVer.Minor == curVer.Minor && tgtVer.Patch == curVer.Patch {
			return UpdateTypePrerelease, nil
		}
	}

	if tgtVer.Major > curVer.Major {
		return UpdateTypeMajor, nil
	}
	if tgtVer.Minor > curVer.Minor {
		return UpdateTypeMinor, nil
	}
	return UpdateTypePatch, nil
}

func IsDowngrade(current, target string) (bool, error) {
	curVer, err := ParseVersion(current)
	if err != nil {
		return false, fmt.Errorf("desktop_update: invalid current version %s: %w", current, err)
	}
	tgtVer, err := ParseVersion(target)
	if err != nil {
		return false, fmt.Errorf("desktop_update: invalid target version %s: %w", target, err)
	}
	return curVer.Compare(tgtVer) > 0, nil
}

func ParseVersion(s string) (domain.SemanticVersion, error) {
	return domain.ParseVersion(s)
}
