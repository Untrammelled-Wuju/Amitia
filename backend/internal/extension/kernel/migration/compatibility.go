package migration

import (
	"fmt"
	"strconv"
	"strings"
)

type SemverVersion struct {
	Major int
	Minor int
	Patch int
}

type VersionCompatibilityChecker struct{}

func NewVersionCompatibilityChecker() *VersionCompatibilityChecker {
	return &VersionCompatibilityChecker{}
}

func (c *VersionCompatibilityChecker) MatchVersionRange(versionRange, version string) bool {
	return strings.TrimSpace(versionRange) == strings.TrimSpace(version)
}

func (c *VersionCompatibilityChecker) ParseSemver(version string) (*SemverVersion, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, fmt.Errorf("migration: empty version")
	}
	if version[0] == 'v' || version[0] == 'V' {
		version = version[1:]
	}
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("migration: invalid semver: %s", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("migration: invalid major version: %s", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("migration: invalid minor version: %s", parts[1])
	}
	patch := 0
	if len(parts) >= 3 {
		patchStr := parts[2]
		if idx := strings.IndexAny(patchStr, "-+"); idx >= 0 {
			patchStr = patchStr[:idx]
		}
		patchStr = strings.TrimSpace(patchStr)
		if patchStr != "" {
			p, err := strconv.Atoi(patchStr)
			if err != nil {
				return nil, fmt.Errorf("migration: invalid patch version: %s", patchStr)
			}
			patch = p
		}
	}
	return &SemverVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

func (c *VersionCompatibilityChecker) IsCompatibleRange(fromRange, toVersion string) bool {
	return c.MatchVersionRange(fromRange, toVersion)
}
