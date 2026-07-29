package desktop_update

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

type PreflightResult struct {
	Passed   bool
	Errors   []string
	Warnings []string
}

func (r *PreflightResult) AddError(msg string) {
	r.Errors = append(r.Errors, msg)
	r.Passed = false
}

func (r *PreflightResult) AddWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

type PreflightChecker struct {
	hostVersion       string
	platform          string
	arch              string
	trustedPublishers map[string]bool
	minDiskSpace      int64
	runningExtensions map[string]bool
}

func NewPreflightChecker(hostVersion string) *PreflightChecker {
	return &PreflightChecker{
		hostVersion:       hostVersion,
		platform:          runtime.GOOS,
		arch:              runtime.GOARCH,
		trustedPublishers: make(map[string]bool),
		minDiskSpace:      100 * 1024 * 1024,
		runningExtensions: make(map[string]bool),
	}
}

func (c *PreflightChecker) SetTrustedPublisher(publisherID string) {
	c.trustedPublishers[publisherID] = true
}

func (c *PreflightChecker) SetRunningExtensions(exts map[string]bool) {
	c.runningExtensions = exts
}

func (c *PreflightChecker) SetMinDiskSpace(bytes int64) {
	c.minDiskSpace = bytes
}

func (c *PreflightChecker) Check(ctx context.Context, plan *ExtensionUpdatePlan, currentVersion string) (*PreflightResult, error) {
	result := &PreflightResult{Passed: true}

	if plan == nil {
		result.AddError("plan is nil")
		return result, fmt.Errorf("%w: nil plan", ErrPreflightFailed)
	}

	meta := plan.SourceMetadata

	c.checkSignature(result, &meta)
	c.checkHash(result, &meta)
	c.checkManifest(result, &meta)
	c.checkExtensionID(result, plan, &meta)
	c.checkPublisherContinuity(result, &meta)
	c.checkVersion(result, currentVersion, &meta)
	c.checkHostCompatibility(result, &meta)
	c.checkPlatform(result, &meta)
	c.checkArch(result, &meta)
	c.checkRuntimeSupport(result, plan)
	c.checkPermissionDiff(result, plan)
	c.checkScopeDiff(result, plan)
	c.checkDependencies(result, &meta)
	c.checkDiskSpace(result, &meta)
	c.checkRunningState(result, plan)
	c.checkMigrationFeasibility(result, plan)
	c.checkRollbackFeasibility(result, plan)

	return result, nil
}

func (c *PreflightChecker) checkSignature(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.Signature == "" {
		result.AddError("package signature missing")
		return
	}
	if meta.PublisherKeyID == "" {
		result.AddError("publisher key id missing")
		return
	}
}

func (c *PreflightChecker) checkHash(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.PackageSHA256 == "" && meta.PackageSHA512 == "" {
		result.AddError("package hash missing")
		return
	}
	if meta.PackageSHA512 != "" {
		if len(meta.PackageSHA512) != 128 {
			result.AddError(fmt.Sprintf("package sha512 hash invalid length: %d", len(meta.PackageSHA512)))
			return
		}
		lower := strings.ToLower(meta.PackageSHA512)
		for _, ch := range lower {
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
				result.AddError("package sha512 hash contains invalid characters")
				return
			}
		}
		return
	}
	if len(meta.PackageSHA256) != 64 {
		result.AddError(fmt.Sprintf("package sha256 hash invalid length: %d", len(meta.PackageSHA256)))
		return
	}
	lower := strings.ToLower(meta.PackageSHA256)
	for _, ch := range lower {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			result.AddError("package sha256 hash contains invalid characters")
			return
		}
	}
}

func (c *PreflightChecker) checkManifest(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.ManifestVersion <= 0 {
		result.AddError("manifest version invalid")
		return
	}
	if meta.ManifestVersion < 2 {
		result.AddWarning(fmt.Sprintf("manifest version %d is below recommended v2", meta.ManifestVersion))
	}
}

func (c *PreflightChecker) checkExtensionID(result *PreflightResult, plan *ExtensionUpdatePlan, meta *ExtensionUpdateMetadata) {
	if meta.ExtensionID == "" {
		result.AddError("extension id missing in metadata")
		return
	}
	if plan.ExtensionID != meta.ExtensionID {
		result.AddError(fmt.Sprintf("extension id mismatch: plan=%s meta=%s", plan.ExtensionID, meta.ExtensionID))
		return
	}
}

func (c *PreflightChecker) checkPublisherContinuity(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.PublisherID == "" {
		result.AddError("publisher id missing")
		return
	}
	if len(c.trustedPublishers) > 0 {
		if !c.trustedPublishers[meta.PublisherID] {
			result.AddError(fmt.Sprintf("publisher %s not in trusted list", meta.PublisherID))
		}
	}
}

func (c *PreflightChecker) checkVersion(result *PreflightResult, currentVersion string, meta *ExtensionUpdateMetadata) {
	if currentVersion == "" {
		return
	}

	updateType, err := CompareVersions(currentVersion, meta.Version)
	if err != nil {
		result.AddError(fmt.Sprintf("version comparison failed: %v", err))
		return
	}

	if updateType == UpdateTypeDowngrade {
		result.AddError(fmt.Sprintf("downgrade not allowed: %s -> %s", currentVersion, meta.Version))
		return
	}

	if updateType == UpdateTypeSame {
		result.AddWarning(fmt.Sprintf("reinstalling same version: %s", meta.Version))
	}
}

func (c *PreflightChecker) checkHostCompatibility(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.MinimumHostVersion != "" {
		hostVer, err := ParseVersion(c.hostVersion)
		if err != nil {
			result.AddWarning(fmt.Sprintf("cannot parse host version %s: %v", c.hostVersion, err))
			return
		}
		minVer, err := ParseVersion(meta.MinimumHostVersion)
		if err != nil {
			result.AddError(fmt.Sprintf("invalid minimum host version %s: %v", meta.MinimumHostVersion, err))
			return
		}
		if hostVer.Compare(minVer) < 0 {
			result.AddError(fmt.Sprintf("host version %s below minimum required %s", c.hostVersion, meta.MinimumHostVersion))
		}
	}

	if meta.MaximumHostVersion != "" {
		hostVer, err := ParseVersion(c.hostVersion)
		if err != nil {
			result.AddWarning(fmt.Sprintf("cannot parse host version %s: %v", c.hostVersion, err))
			return
		}
		maxVer, err := ParseVersion(meta.MaximumHostVersion)
		if err != nil {
			result.AddError(fmt.Sprintf("invalid maximum host version %s: %v", meta.MaximumHostVersion, err))
			return
		}
		if hostVer.Compare(maxVer) > 0 {
			result.AddError(fmt.Sprintf("host version %s above maximum supported %s", c.hostVersion, meta.MaximumHostVersion))
		}
	}
}

func (c *PreflightChecker) checkPlatform(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if !meta.SupportsPlatform(c.platform) {
		result.AddError(fmt.Sprintf("platform %s not supported, supported: %v", c.platform, meta.SupportedPlatforms))
	}
}

func (c *PreflightChecker) checkArch(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if !meta.SupportsArch(c.arch) {
		result.AddError(fmt.Sprintf("arch %s not supported, supported: %v", c.arch, meta.SupportedArch))
	}
}

func (c *PreflightChecker) checkRuntimeSupport(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if len(plan.RuntimeDiff.RemovedRuntimes) > 0 {
		result.AddWarning(fmt.Sprintf("runtimes removed: %v", plan.RuntimeDiff.RemovedRuntimes))
	}
	if plan.RuntimeDiff.TypeUpgraded {
		result.AddWarning("runtime type upgraded, may require restart")
	}
}

func (c *PreflightChecker) checkPermissionDiff(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if len(plan.PermissionDiff.Added) > 0 {
		result.AddWarning(fmt.Sprintf("new permissions added: %v", plan.PermissionDiff.Added))
	}
	if len(plan.PermissionDiff.Changed) > 0 {
		result.AddWarning(fmt.Sprintf("permissions changed: %v", plan.PermissionDiff.Changed))
	}
}

func (c *PreflightChecker) checkScopeDiff(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if plan.ScopeDiff.Expanded {
		result.AddWarning(fmt.Sprintf("scope expanded: %v", plan.ScopeDiff.Details))
	}
}

func (c *PreflightChecker) checkDependencies(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.ManifestVersion < 2 {
		result.AddWarning("manifest v1 may have unresolved dependencies")
	}
}

func (c *PreflightChecker) checkDiskSpace(result *PreflightResult, meta *ExtensionUpdateMetadata) {
	if meta.PackageSize > c.minDiskSpace*10 {
		result.AddWarning(fmt.Sprintf("package size %d is very large", meta.PackageSize))
	}
	if meta.PackageSize <= 0 {
		result.AddError("package size must be positive")
	}
}

func (c *PreflightChecker) checkRunningState(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if c.runningExtensions[plan.ExtensionID] {
		if !plan.RuntimeDrainPlan.StopNewInvocations {
			result.AddError("extension is running but drain plan does not stop new invocations")
		}
	}
}

func (c *PreflightChecker) checkMigrationFeasibility(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if plan.MigrationPlan == nil {
		return
	}
	if plan.MigrationPlan.HasMigration && !plan.MigrationPlan.IsReversible {
		if !plan.RollbackPlan.CanRollback {
			result.AddError("irreversible migration with no rollback capability")
		} else {
			result.AddWarning("irreversible migration, rollback will use data snapshot")
		}
	}
}

func (c *PreflightChecker) checkRollbackFeasibility(result *PreflightResult, plan *ExtensionUpdatePlan) {
	if !plan.RollbackPlan.CanRollback {
		if plan.IsHighRisk() {
			result.AddError("high-risk update without rollback capability")
		} else {
			result.AddWarning("no rollback capability for this update")
		}
	}
}
