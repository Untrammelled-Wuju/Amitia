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

type ReadCompatibility struct {
	Compatible        bool          `json:"compatible"`
	WriteStrategy     WriteStrategy `json:"write_strategy"`
	OldReadNew        bool          `json:"old_read_new"`
	NewReadOld        bool          `json:"new_read_old"`
	OldWriteNew       bool          `json:"old_write_new"`
	NewWriteOld       bool          `json:"new_write_old"`
	RequiresMigration bool          `json:"requires_migration"`
	Issues            []string      `json:"issues"`
}

type VersionCompatibilityChecker struct{}

func NewVersionCompatibilityChecker() *VersionCompatibilityChecker {
	return &VersionCompatibilityChecker{}
}

func (c *VersionCompatibilityChecker) CheckReadCompatibility(writeStrategy WriteStrategy) (*ReadCompatibility, error) {
	rc := &ReadCompatibility{
		WriteStrategy: writeStrategy,
		Issues:        []string{},
	}
	switch writeStrategy {
	case WriteStrategyOldOnly:
		rc.Compatible = true
		rc.OldReadNew = false
		rc.NewReadOld = true
		rc.OldWriteNew = true
		rc.NewWriteOld = false
		rc.RequiresMigration = true
	case WriteStrategyNewOnly:
		rc.Compatible = true
		rc.OldReadNew = true
		rc.NewReadOld = false
		rc.OldWriteNew = false
		rc.NewWriteOld = true
		rc.RequiresMigration = true
	case WriteStrategyDualWriteValidated:
		rc.Compatible = true
		rc.OldReadNew = true
		rc.NewReadOld = true
		rc.OldWriteNew = true
		rc.NewWriteOld = true
		rc.RequiresMigration = false
	case WriteStrategyStagedWrite:
		rc.Compatible = true
		rc.OldReadNew = true
		rc.NewReadOld = true
		rc.OldWriteNew = true
		rc.NewWriteOld = false
		rc.RequiresMigration = true
	case WriteStrategyReadCompatibleShared:
		rc.Compatible = true
		rc.OldReadNew = true
		rc.NewReadOld = true
		rc.OldWriteNew = false
		rc.NewWriteOld = false
		rc.RequiresMigration = false
	default:
		return nil, fmt.Errorf("migration: unknown write strategy: %s", writeStrategy)
	}
	return rc, nil
}

func (c *VersionCompatibilityChecker) MatchVersionRange(versionRange, version string) bool {
	versionRange = strings.TrimSpace(versionRange)
	version = strings.TrimSpace(version)
	if versionRange == "" || versionRange == "*" {
		return true
	}
	if versionRange == version {
		return true
	}
	if strings.HasPrefix(versionRange, ">=") {
		return c.compareVersionStrings(version, strings.TrimSpace(versionRange[2:])) >= 0
	}
	if strings.HasPrefix(versionRange, "<=") {
		return c.compareVersionStrings(version, strings.TrimSpace(versionRange[2:])) <= 0
	}
	if strings.HasPrefix(versionRange, ">") {
		return c.compareVersionStrings(version, strings.TrimSpace(versionRange[1:])) > 0
	}
	if strings.HasPrefix(versionRange, "<") {
		return c.compareVersionStrings(version, strings.TrimSpace(versionRange[1:])) < 0
	}
	if strings.HasPrefix(versionRange, "[") || strings.HasPrefix(versionRange, "(") {
		return c.matchInterval(versionRange, version)
	}
	return false
}

func (c *VersionCompatibilityChecker) matchInterval(expr, version string) bool {
	if len(expr) < 4 {
		return false
	}
	var lowerInclusive bool
	if expr[0] == '[' {
		lowerInclusive = true
	} else if expr[0] == '(' {
		lowerInclusive = false
	} else {
		return false
	}
	last := expr[len(expr)-1]
	var upperInclusive bool
	if last == ']' {
		upperInclusive = true
	} else if last == ')' {
		upperInclusive = false
	} else {
		return false
	}
	inner := expr[1 : len(expr)-1]
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return false
	}
	lower := strings.TrimSpace(parts[0])
	upper := strings.TrimSpace(parts[1])
	if lower == "" || upper == "" {
		return false
	}
	cmpLower := c.compareVersionStrings(version, lower)
	if lowerInclusive {
		if cmpLower < 0 {
			return false
		}
	} else {
		if cmpLower <= 0 {
			return false
		}
	}
	cmpUpper := c.compareVersionStrings(version, upper)
	if upperInclusive {
		if cmpUpper > 0 {
			return false
		}
	} else {
		if cmpUpper >= 0 {
			return false
		}
	}
	return true
}

func (c *VersionCompatibilityChecker) compareVersionStrings(a, b string) int {
	va, err1 := c.ParseSemver(a)
	vb, err2 := c.ParseSemver(b)
	if err1 != nil || err2 != nil {
		if a == b {
			return 0
		}
		if a < b {
			return -1
		}
		return 1
	}
	return c.CompareVersions(va, vb)
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

func (c *VersionCompatibilityChecker) CompareVersions(v1, v2 *SemverVersion) int {
	if v1.Major != v2.Major {
		if v1.Major < v2.Major {
			return -1
		}
		return 1
	}
	if v1.Minor != v2.Minor {
		if v1.Minor < v2.Minor {
			return -1
		}
		return 1
	}
	if v1.Patch != v2.Patch {
		if v1.Patch < v2.Patch {
			return -1
		}
		return 1
	}
	return 0
}

func (c *VersionCompatibilityChecker) IsCompatibleRange(fromRange, toVersion string) bool {
	return c.MatchVersionRange(fromRange, toVersion)
}
