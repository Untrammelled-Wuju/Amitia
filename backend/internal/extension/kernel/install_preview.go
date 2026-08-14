package kernel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
)

func (r *Runtime) PreviewInstall(ctx context.Context, archivePath string) (InstallPreview, error) {
	preview := InstallPreview{}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		return preview, fmt.Errorf("kernel: read archive: %w", err)
	}

	source := package_security.PackageSource{
		SourceType:  package_security.SourceLocalFile,
		LocalPath:   archivePath,
		DisplayName: filepath.Base(archivePath),
	}

	if r.container != nil && r.container.PackageSecurity != nil {
		secReport, secErr := r.container.PackageSecurity.Inspect(ctx, raw, source)
		if secErr != nil {
			return preview, fmt.Errorf("kernel: security inspect: %w", secErr)
		}
		preview.SecurityPassed = secReport.Passed
		preview.ArchiveHash = secReport.ArchiveHash
		preview.SecurityReport = secReport
		for _, issue := range secReport.BlockingIssues {
			preview.Issues = append(preview.Issues, PreviewIssue{
				Category: PreviewNotInstallable,
				Code:     "security_blocking",
				Message:  issue.Description,
				Path:     issue.Path,
			})
		}
	} else {
		hasher := package_security.NewContentHasher()
		preview.ArchiveHash = hasher.HashArchive(raw)
		preview.SecurityPassed = false
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "security_service_unavailable",
			Message:  "安全检查服务不可用，无法验证包安全性",
		})
	}

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		return preview, fmt.Errorf("kernel: open archive: %w", err)
	}

	manifest := pkg.Manifest
	preview.Manifest = manifest
	preview.ExtensionID = manifest.Extension.ID
	preview.Name = manifest.Extension.Name.Default
	preview.Version = manifest.Extension.Version
	preview.Publisher = manifest.Publisher.ID
	preview.ContentTreeHash = manifest.Integrity.ContentTreeHash

	report := manifest.Validate()
	preview.ValidationReport = report

	installable := preview.SecurityPassed && !report.HasErrors()

	if r.container != nil && r.container.PermissionDefinitions != nil {
		permIssues := validateManifestPermissions(manifest, r.container.PermissionDefinitions)
		for _, issue := range permIssues {
			preview.Issues = append(preview.Issues, issue)
			installable = false
		}
	}

	for _, e := range report.Errors {
		category := PreviewNotInstallable
		if e.Code == "unsupported_runtime" || e.Code == "unsupported_contribution" {
			category = PreviewPartialUnsupported
		}
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: category,
			Code:     e.Code,
			Message:  fmt.Sprintf("%s: %s", e.Path, e.Message),
			Path:     e.Path,
		})
	}

	supportedModuleTypes := map[string]bool{
		"builtin": true, "javascript": true, "data_only": true,
	}
	supportedRuntimeTypes := map[string]bool{
		"javascript": true, "mcp": true, "workflow": true, "static": true,
	}
	for _, mod := range manifest.Modules {
		supported := supportedModuleTypes[mod.Type]
		if mod.Runtime != nil && mod.Runtime.Type != "" {
			supported = supported && supportedRuntimeTypes[mod.Runtime.Type]
		}
		runtimeType := ""
		if mod.Runtime != nil {
			runtimeType = mod.Runtime.Type
		}
		preview.Modules = append(preview.Modules, PreviewModule{
			ID:        mod.ID,
			Name:      mod.Name.Default,
			Type:      mod.Type,
			Runtime:   runtimeType,
			Supported: supported,
		})
	}

	if r.container != nil {
		for _, dep := range manifest.Dependencies {
			missing := false
			if dep.Type == "extension" {
				if _, getErr := r.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(dep.ID)); getErr != nil {
					missing = true
				}
			}
			preview.MissingDependencies = append(preview.MissingDependencies, PreviewDependency{
				Type: dep.Type, ID: dep.ID, Version: dep.Version,
				Optional: dep.Optional, Reason: dep.Reason, Missing: missing,
			})
			if missing && !dep.Optional {
				installable = false
				preview.Issues = append(preview.Issues, PreviewIssue{
					Category: PreviewMissingDependency,
					Code:     "missing_dependency",
					Message:  fmt.Sprintf("dependency %s (%s) not installed", dep.ID, dep.Type),
				})
			}
		}
	} else {
		for _, dep := range manifest.Dependencies {
			preview.MissingDependencies = append(preview.MissingDependencies, PreviewDependency{
				Type: dep.Type, ID: dep.ID, Version: dep.Version,
				Optional: dep.Optional, Reason: dep.Reason,
			})
		}
	}

	for _, perm := range manifest.Permissions {
		preview.RequiredPermissions = append(preview.RequiredPermissions, PreviewPermission{
			Name: perm.Name, Reason: perm.Reason,
			Required: perm.Required, Scope: perm.Scope,
		})
		if perm.Required {
			preview.Issues = append(preview.Issues, PreviewIssue{
				Category: PreviewNeedsPermission,
				Code:     "permission_required",
				Message:  fmt.Sprintf("permission %s required", perm.Name),
			})
		}
	}

	for _, mod := range manifest.Modules {
		for _, c := range mod.Contributions {
			if len(c.RequiredScope) > 0 {
				preview.RequiredScopes = append(preview.RequiredScopes, PreviewScope{
					ContributionID: c.ID,
					Scopes:         c.RequiredScope,
				})
				preview.Issues = append(preview.Issues, PreviewIssue{
					Category: PreviewNeedsScope,
					Code:     "scope_required",
					Message:  fmt.Sprintf("contribution %s requires scope", c.ID),
				})
			}
		}
	}

	preview.Installable = installable
	preview.Category = classifyPreview(preview.Issues, installable)

	return preview, nil
}

func classifyPreview(issues []PreviewIssue, installable bool) PreviewCategory {
	if installable {
		return PreviewOK
	}
	priority := []PreviewCategory{
		PreviewNotInstallable,
		PreviewMissingDependency,
		PreviewPartialUnsupported,
		PreviewNeedsPermission,
		PreviewNeedsScope,
	}
	present := make(map[PreviewCategory]bool)
	for _, issue := range issues {
		present[issue.Category] = true
	}
	for _, cat := range priority {
		if present[cat] {
			return cat
		}
	}
	return PreviewNotInstallable
}
