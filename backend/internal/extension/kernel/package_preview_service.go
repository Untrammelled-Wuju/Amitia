package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

const packagePolicyVersion = "2026-08-26-v3"

func CurrentPackagePolicyVersion() string {
	return packagePolicyVersion
}

func computeSecurityPolicyHash() string {
	policy := package_security.DefaultArchivePolicy()
	canonical := fmt.Sprintf(`{"version":%q,"maxArchiveBytes":%d,"maxSingleEntryBytes":%d,"allowSymlink":%v,"maxDirectoryDepth":%d}`,
		packagePolicyVersion, policy.MaxArchiveBytes, policy.MaxSingleEntryBytes, policy.AllowSymlink, policy.MaxDirectoryDepth)
	h := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(h[:])
}

func packageDevelopmentModeEnabled() bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("AMITIA_EXTENSION_DEV_MODE")))
	return err == nil && enabled
}

func (r *Runtime) PreviewPackage(ctx context.Context, request PackagePreviewRequest, reader io.Reader) (InstallPreview, error) {
	if r.container == nil || r.container.PackageRepository == nil || r.container.PackageArtifactStore == nil {
		return InstallPreview{}, fmt.Errorf("kernel: package services unavailable")
	}
	if request.UserID == "" || request.ScopeType == "" {
		return InstallPreview{}, fmt.Errorf("kernel: preview owner and scope required")
	}
	artifact, err := r.container.PackageArtifactStore.PutArchive(ctx, reader, package_security.DefaultArchivePolicy().MaxArchiveBytes)
	if err != nil {
		return InstallPreview{}, err
	}
	source := package_security.PackageSource{SourceType: package_security.SourceLocalFile, LocalPath: artifact.ArchivePath, DisplayName: request.FileName}
	securityReport, err := r.container.PackageSecurity.InspectFile(ctx, artifact.ArchivePath, source)
	if err != nil {
		return InstallPreview{}, fmt.Errorf("kernel: security inspect: %w", err)
	}
	if !securityReport.Passed {
		return InstallPreview{}, fmt.Errorf("kernel: archive security rejected package")
	}
	pkg, err := amitiax.OpenArchive(artifact.ArchivePath)
	if err != nil {
		return InstallPreview{}, fmt.Errorf("kernel: manifest v2 parse: %w", err)
	}
	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		return InstallPreview{}, fmt.Errorf("kernel: integrity verification failed: %w", err)
	}
	validation := pkg.Manifest.Validate()
	preview := InstallPreview{
		SessionID: "preview-" + uuid.NewString(), ArtifactID: artifact.ArtifactID,
		ExtensionID: pkg.Manifest.Extension.ID, Name: pkg.Manifest.Extension.Name.Default,
		Version: pkg.Manifest.Extension.Version, Publisher: pkg.Manifest.Publisher.ID,
		ArchiveHash: artifact.ArchiveHash, ManifestHash: computeManifestHash(pkg),
		ContentTreeHash: pkg.Tree.TreeHash, ArtifactHash: computeArtifactHashFromPackage(pkg),
		SecurityPassed: true, SecurityReport: securityReport, Manifest: pkg.Manifest,
		ValidationReport: validation, SignatureStatus: "unsigned", TrustDecision: "unsigned",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	for _, validationError := range validation.Errors {
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: validationError.Code, Message: validationError.Message, Path: validationError.Path})
	}
	if r.container != nil && r.container.PermissionDefinitions != nil {
		permIssues := validateManifestPermissions(pkg.Manifest, r.container.PermissionDefinitions)
		for _, issue := range permIssues {
			preview.Issues = append(preview.Issues, issue)
		}
	}
	if len(pkg.V2Signature) > 0 {
		doc, parseErr := trust.ParseSignatureDocument(pkg.V2Signature)
		if parseErr != nil {
			return InstallPreview{}, fmt.Errorf("kernel: invalid v2 signature document: %w", parseErr)
		}
		preview.SignerKeyID = doc.KeyID
		verification := r.container.TrustService.Verifier().VerifyPackage(ctx, trust.PackageVerificationInput{
			Document: doc, ActualExtensionID: preview.ExtensionID, ActualVersion: preview.Version,
			ActualManifestVersion: pkg.Manifest.ManifestVersion, ActualManifestHash: preview.ManifestHash,
			ActualContentTreeHash: preview.ContentTreeHash, ActualArtifactHash: preview.ArtifactHash,
		})
		preview.SignatureStatus = string(verification.Status)
		if trust.IsBlockingSignatureStatus(verification.Status) {
			preview.TrustDecision = "rejected"
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: string(verification.Status), Message: verification.Reason})
		} else if verification.Status == trust.SignatureStatusUnknownKey {
			preview.TrustDecision = "rejected"
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_publisher_untrusted", Message: verification.Reason})
		} else {
			identity, identityErr := r.container.TrustService.Store().Get(ctx, doc.PublisherID)
			if identityErr != nil {
				preview.TrustDecision = "rejected"
				preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_publisher_untrusted", Message: "publisher is not trusted"})
			} else if identity.TrustLevel != trust.TrustLevelOfficial && identity.TrustLevel != trust.TrustLevelTrusted && identity.TrustLevel != trust.TrustLevelUserTrusted {
				preview.TrustDecision = "rejected"
				preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_publisher_untrusted", Message: "publisher trust level does not allow production installation"})
			} else {
				preview.TrustDecision = string(identity.TrustLevel)
			}
		}
	} else if pkg.Signatures != nil {
		preview.SignatureStatus = "legacy_signature"
		preview.SignerKeyID = pkg.Signatures.KeyID
		preview.TrustDecision = "rejected"
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_signature_required", Message: "Manifest v2 signature is required"})
	} else {
		if r.packageUnsignedDevAllowed(request, preview.ExtensionID) {
			preview.DevOnly = true
			preview.DeveloperSessionID = request.DeveloperSessionID
			preview.TrustDecision = string(trust.TrustLevelDevelopment)
			preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.unsigned_dev")
			preview.RiskFlags = append(preview.RiskFlags, "unsigned_dev", "dev_only")
		} else {
			preview.TrustDecision = "rejected"
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_signature_required", Message: "signed package required in production"})
		}
	}
	if reason := r.packageTrustBlockReason(ctx, pkg, artifact.ArchiveHash); reason != "" {
		preview.TrustDecision = "rejected"
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "trust_policy_rejected", Message: reason})
	}
	appendPackageHostCompatibilityIssues(pkg.Manifest, &preview)
	appendGamePluginNetworkCompatibilityIssues(pkg.Manifest, &preview)
	appendGamePluginArtifactPackageIssues(pkg, &preview)
	r.evaluatePackageCompatibilityAndDependencies(ctx, pkg, &preview)
	r.evaluatePackageUpdateRisks(ctx, &preview)
	r.evaluatePackageMigrationPreflight(ctx, pkg.Manifest, &preview)
	for _, file := range pkg.Files {
		lower := strings.ToLower(file.Path)
		if strings.Contains(lower, "/scripts/") || strings.HasPrefix(lower, "scripts/") {
			preview.RiskFlags = append(preview.RiskFlags, "scripts")
			preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.scripts")
		}
		if strings.HasPrefix(lower, "migrations/") {
			preview.RiskFlags = append(preview.RiskFlags, "config_migration")
			preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.config_migration")
		}
	}
	if len(pkg.Manifest.Permissions) > 0 {
		preview.RiskFlags = append(preview.RiskFlags, "permission_escalation")
		preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.permission_escalation")
	}
	for _, mod := range pkg.Manifest.Modules {
		for _, contribution := range mod.Contributions {
			if len(contribution.RequiredScope) > 0 {
				preview.RiskFlags = append(preview.RiskFlags, "scope_expansion")
				preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.scope_expansion")
			}
		}
	}
	preview.RequiredConfirmations = uniquePackageStrings(preview.RequiredConfirmations)
	preview.RiskFlags = uniquePackageStrings(preview.RiskFlags)
	preview.Installable = len(preview.Issues) == 0
	preview.Category = classifyPreview(preview.Issues, preview.Installable)
	artifact.ExtensionID = preview.ExtensionID
	artifact.Version = preview.Version
	artifact.ManifestHash = preview.ManifestHash
	artifact.ContentTreeHash = preview.ContentTreeHash
	artifact.ArtifactHash = preview.ArtifactHash
	artifact.SignatureStatus = preview.SignatureStatus
	artifact.SignerKeyID = preview.SignerKeyID
	artifact.PublisherID = preview.Publisher
	artifact.TrustDecision = preview.TrustDecision
	artifact.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	artifact.VerifiedAt = artifact.CreatedAt
	artifact, err = r.container.PackageArtifactStore.PlaceArchive(artifact)
	if err != nil {
		return InstallPreview{}, err
	}
	verificationJSON, _ := json.Marshal(map[string]any{"security": securityReport, "signatureStatus": preview.SignatureStatus, "trustDecision": preview.TrustDecision, "policyVersion": packagePolicyVersion, "migrationPreview": preview.MigrationPreview, "migrationPlanHash": preview.MigrationPlanHash})
	artifact.VerificationReportJSON = string(verificationJSON)
	if err := r.container.PackageRepository.PutArtifact(ctx, artifact); err != nil {
		return InstallPreview{}, err
	}
	artifact, err = r.container.PackageRepository.GetArtifactByIdentity(ctx, artifact.ExtensionID, artifact.Version, artifact.ArchiveHash)
	if err != nil {
		return InstallPreview{}, err
	}
	preview.ArtifactID = artifact.ArtifactID
	previewJSON, _ := json.Marshal(preview)
	riskJSON, _ := json.Marshal(preview.RiskFlags)
	confirmationJSON, _ := json.Marshal(preview.RequiredConfirmations)
	dependencyJSON, _ := json.Marshal(preview.MissingDependencies)
	status := "ready"
	if len(preview.RequiredConfirmations) > 0 {
		status = "awaiting_confirmation"
	}
	session := PackagePreviewSession{SessionID: preview.SessionID, UserID: request.UserID,
		ScopeType: request.ScopeType, ScopeID: request.ScopeID, ArtifactID: artifact.ArtifactID,
		ExtensionID: preview.ExtensionID, Version: preview.Version, Status: status,
		ArchiveHash: preview.ArchiveHash, ManifestHash: preview.ManifestHash,
		ContentTreeHash: preview.ContentTreeHash, RiskFlagsJSON: string(riskJSON),
		RequiredConfirmationsJSON: string(confirmationJSON), DependencyResultJSON: string(dependencyJSON),
		PreviewResultJSON: string(previewJSON), VerificationReportJSON: string(verificationJSON),
		PolicyVersion: packagePolicyVersion, SecurityPolicyHash: computeSecurityPolicyHash(),
		VerifiedAt: artifact.VerifiedAt,
		ExpiresAt:  preview.ExpiresAt.Format(time.RFC3339Nano), CreatedAt: artifact.CreatedAt}
	if err := r.container.PackageRepository.PutPreview(ctx, session); err != nil {
		return InstallPreview{}, err
	}
	return preview, nil
}

func (r *Runtime) evaluatePackageMigrationPreflight(ctx context.Context, manifest manifest_v2.Manifest, preview *InstallPreview) {
	if preview == nil || !packageManifestHasMigrations(manifest) {
		return
	}
	preview.RiskFlags = append(preview.RiskFlags, "data_migration")
	if r.container == nil || r.container.MigrationRepository == nil || r.container.InstallationRepository == nil {
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_migration_unavailable", Message: "migration preflight is unavailable"})
		return
	}
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(manifest.Extension.ID))
	if err != nil {
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_migration_source_version_required", Message: "migration package requires an installed source version"})
		return
	}
	preflight, err := NewPackageMigrationGuard(r.container.MigrationRepository).PreflightManifest(ctx, manifest, current.InstalledVersion.String())
	if err != nil {
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_migration_preflight_failed", Message: err.Error()})
		return
	}
	preview.MigrationPreview = preflight
	preview.MigrationPlanHash = preflight.PlanHash
	preview.MigrationSnapshotRequired = preflight.UserDataSnapshotRequired
	preview.MigrationManualRequired = preflight.ManualRequired
	preview.MigrationIrreversible = preflight.Irreversible
	if preflight.UserDataSnapshotRequired {
		preview.RiskFlags = append(preview.RiskFlags, "migration_snapshot_required")
	}
	if preflight.ManualRequired {
		preview.RiskFlags = append(preview.RiskFlags, "migration_manual_recovery")
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_migration_manual_recovery_required", Message: "migration requires a controlled manual recovery workflow"})
	}
	if preflight.Irreversible {
		preview.RiskFlags = append(preview.RiskFlags, "migration_irreversible")
		preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable, Code: "package_migration_irreversible", Message: "irreversible migration is not allowed in the production package flow"})
	}
}

func packageManifestHasMigrations(manifest manifest_v2.Manifest) bool {
	if len(manifest.Extension.Metadata) == 0 {
		return false
	}
	_, migrations := manifest.Extension.Metadata["migrations"]
	_, migrationValue := manifest.Extension.Metadata["migration"]
	return migrations || migrationValue
}

func (r *Runtime) packageUnsignedDevAllowed(request PackagePreviewRequest, extensionID string) bool {
	if !request.AllowUnsignedDev {
		return false
	}
	return r.validateUnsignedDeveloperSession(request.DeveloperSessionID, request.UserID, extensionID) == nil
}

func (r *Runtime) validateUnsignedDeveloperSession(sessionID, userID, extensionID string) error {
	if r.container == nil {
		return fmt.Errorf("kernel: developer session binding unavailable")
	}
	return validateDeveloperSessionBinding(r.container.DevModeSessions, r.container.DevModeRegistry, sessionID, userID, extensionID)
}

func (r *Runtime) evaluatePackageUpdateRisks(ctx context.Context, preview *InstallPreview) {
	current, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID))
	if err != nil {
		return
	}
	preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.update", PackageConfirmationSnapshotExempt)
	currentArtifact, artifactErr := r.container.PackageRepository.GetArtifact(ctx, current.PackageID)
	if artifactErr == nil {
		if current.InstalledVersion.String() == preview.Version && currentArtifact.ArchiveHash != preview.ArchiveHash {
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewNotInstallable,
				Code: "same_version_different_content", Message: "same version has different archive content"})
		}
		if currentArtifact.PublisherID != preview.Publisher || currentArtifact.SignerKeyID != preview.SignerKeyID {
			preview.RiskFlags = append(preview.RiskFlags, "signer_change")
			preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.signer_change")
		}
	}
	newVersion, newErr := domain.ParseVersion(preview.Version)
	if newErr == nil && newVersion.Compare(current.InstalledVersion) < 0 {
		preview.RiskFlags = append(preview.RiskFlags, "downgrade")
		preview.RequiredConfirmations = append(preview.RequiredConfirmations, "confirm.downgrade")
	}
}

func (r *Runtime) evaluatePackageCompatibilityAndDependencies(ctx context.Context, pkg *amitiax.Package, preview *InstallPreview) {
	platform := normalizePackagePlatform(runtime.GOOS)
	hostVersion := currentPackageHostVersion()
	for _, mod := range pkg.Manifest.Modules {
		supported := packageModuleSupported(mod, platform, hostVersion)
		runtimeType := ""
		if mod.Runtime != nil {
			runtimeType = mod.Runtime.Type
		}
		preview.Modules = append(preview.Modules, PreviewModule{ID: mod.ID, Name: mod.Name.Default, Type: mod.Type, Runtime: runtimeType, Supported: supported})
		if !supported {
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewPartialUnsupported, Code: "unsupported_module", Message: mod.ID})
		}
	}
	deps := append([]manifest_v2.Dependency(nil), pkg.Manifest.Dependencies...)
	for _, mod := range pkg.Manifest.Modules {
		deps = append(deps, mod.Dependencies...)
		for _, contribution := range mod.Contributions {
			deps = append(deps, contribution.Dependencies...)
		}
	}
	for _, dep := range deps {
		item := PreviewDependency{Type: dep.Type, ID: dep.ID, Version: dep.Version, Optional: dep.Optional, Reason: dep.Reason}
		installedVersion := ""
		switch dep.Type {
		case "extension":
			installation, err := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(dep.ID))
			if err == nil {
				installedVersion = installation.InstalledVersion.String()
			}
		case "module":
			var raw string
			if r.container.PackageRepository.DB().QueryRowContext(ctx, `SELECT definition_json FROM extension_modules WHERE module_id = ? LIMIT 1`, dep.ID).Scan(&raw) == nil {
				var definition domain.ModuleDefinition
				if json.Unmarshal([]byte(raw), &definition) == nil {
					installedVersion = definition.Version
				}
			}
		case "contribution":
			var raw string
			if r.container.PackageRepository.DB().QueryRowContext(ctx, `SELECT definition_json FROM extension_contributions WHERE contribution_id = ? LIMIT 1`, dep.ID).Scan(&raw) == nil {
				var definition domain.ContributionDefinition
				if json.Unmarshal([]byte(raw), &definition) == nil {
					installedVersion = definition.Version
				}
			}
		}
		item.Missing = installedVersion == ""
		if !item.Missing && dep.Version != "" {
			rangeValue, rangeErr := dependency.ParseRange(dep.Version)
			versionValue, versionErr := domain.ParseVersion(installedVersion)
			if rangeErr != nil || versionErr != nil || !rangeValue.Satisfies(versionValue) {
				item.Missing = true
				item.Reason = "version_constraint_unsatisfied"
			}
		}
		preview.MissingDependencies = append(preview.MissingDependencies, item)
		if item.Missing && !item.Optional {
			preview.Issues = append(preview.Issues, PreviewIssue{Category: PreviewMissingDependency, Code: "dependency_unsatisfied", Message: dep.ID})
		}
	}
}

func uniquePackageStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
