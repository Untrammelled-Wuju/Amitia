package extension

import (
	"context"
	"encoding/json"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func kernelReadPackageOperation(ctx context.Context, runtime *Runtime, userID, operationID string) (PackageOperationView, error) {
	if runtime == nil || runtime.Kernel == nil {
		return PackageOperationView{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := runtime.Kernel.Container()
	if container == nil || container.PackageRepository == nil {
		return PackageOperationView{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	record, steps, err := container.PackageRepository.GetOperation(ctx, userID, operationID)
	if err != nil {
		return PackageOperationView{}, err
	}
	return kernelPackageOperationView(record, steps), nil
}

func kernelReadImportSession(ctx context.Context, runtime *Runtime, sessionID, userID, scopeType, scopeID string) (PackageImportPreview, error) {
	if runtime == nil || runtime.Kernel == nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := runtime.Kernel.Container()
	if container == nil || container.PackageRepository == nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if scopeType == "" {
		scopeType = string(ScopeGlobal)
	}
	record, err := container.PackageRepository.GetPreview(ctx, sessionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageImportSessionExpired, "预览会话不存在", sessionID, false, err)
	}
	var preview kernelruntime.InstallPreview
	if err := json.Unmarshal([]byte(record.PreviewResultJSON), &preview); err != nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageArtifactInvalid, "预览会话数据损坏", sessionID, false, err)
	}
	return translateKernelPackagePreviewDirect(ctx, container, preview, scopeType, scopeID), nil
}

func translateKernelPackagePreviewDirect(ctx context.Context, container *kernelruntime.Container, preview kernelruntime.InstallPreview, scopeType, scopeID string) PackageImportPreview {
	result := PackageImportPreview{SessionID: preview.SessionID, Format: PackageFormatAmitiax,
		ID: preview.ExtensionID, Name: preview.Name, Version: preview.Version,
		Description: preview.Manifest.Extension.Description.Default, License: preview.Manifest.Extension.License,
		Source: "local-amitiax", ScopeType: scopeType, ScopeID: scopeID, PackageHash: preview.ArchiveHash,
		Checksum:   PackageChecksumView{Valid: true, PackageHash: preview.ArchiveHash},
		Signature:  PackageSignatureView{Status: kernelSignatureStatus(preview.SignatureStatus), Fingerprint: preview.SignerKeyID},
		Compatible: preview.Installable, Compatibility: string(preview.Category),
		Capabilities: []string{}, HighRisk: append([]string(nil), preview.RiskFlags...),
		CapabilityConfirmations: append([]string(nil), preview.RequiredConfirmations...),
		Dependencies:            []PackageDependencyView{}, Files: []PackageFileView{}, Risks: []PackageRisk{},
		Warnings: []string{}, Errors: []string{}, AvailableActions: []string{}, ExpiresAt: preview.ExpiresAt,
		Conflict: PackageConflictNew}
	for _, permission := range preview.RequiredPermissions {
		if permission.Name != "" {
			result.Capabilities = append(result.Capabilities, permission.Name)
		}
	}
	for _, dependency := range preview.MissingDependencies {
		result.Dependencies = append(result.Dependencies, PackageDependencyView{ID: dependency.ID,
			VersionConstraint: dependency.Version, Required: !dependency.Optional, Installed: !dependency.Missing})
	}
	for _, warning := range preview.ValidationReport.Warnings {
		message := warning.Message
		if warning.Code != "" {
			message = warning.Code + ": " + message
		}
		result.Warnings = append(result.Warnings, message)
	}
	for _, issue := range preview.Issues {
		result.Errors = append(result.Errors, issue.Code+": "+issue.Message)
	}
	for _, risk := range preview.RiskFlags {
		result.Risks = append(result.Risks, PackageRisk{Code: risk, Severity: "high", Message: risk})
	}
	result.ManagementTarget, result.ContributionKinds = packagePreviewManagementTarget(preview)
	if preview.Installable {
		result.AvailableActions = []string{"install"}
	}
	if container != nil {
		if current, currentErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID)); currentErr == nil {
			result.CurrentVersion = current.InstalledVersion.String()
			result.Conflict = PackageConflictUpgrade
			if result.CurrentVersion == preview.Version {
				result.Conflict = PackageConflictSame
			}
		}
	}
	return result
}

func packagePreviewManagementTarget(preview kernelruntime.InstallPreview) (string, []string) {
	kinds := make([]domain.ContributionKind, 0)
	kindNames := make([]string, 0)
	seen := make(map[string]struct{})
	for _, module := range preview.Manifest.Modules {
		for _, contribution := range module.Contributions {
			kind := string(contribution.Kind)
			if kind == "" {
				continue
			}
			kinds = append(kinds, domain.ContributionKind(kind))
			if _, exists := seen[kind]; !exists {
				seen[kind] = struct{}{}
				kindNames = append(kindNames, kind)
			}
		}
	}
	if len(kinds) == 0 {
		return "", kindNames
	}
	extensionDomain, err := domain.ResolveDomainFromKinds(kinds)
	if err != nil {
		return "", kindNames
	}
	target, err := domain.ManagementTargetForDomain(extensionDomain)
	if err != nil {
		return "", kindNames
	}
	return string(target), kindNames
}

func kernelPackageOperationView(record kernelruntime.PackageOperationRecord, steps []kernelruntime.PackageOperationStep) PackageOperationView {
	view := PackageOperationView{ID: record.OperationID, Operation: PackageOperation(record.OperationType),
		ExtensionID: record.ExtensionID, TargetVersion: record.TargetVersion, Source: "kernel",
		ScopeType: record.ScopeType, ScopeID: record.ScopeID, Status: record.Status,
		ErrorCode: record.ErrorCode, TraceID: record.TraceID, CreatedAt: record.StartedAt,
		CompletedAt: record.CompletedAt, CurrentStep: record.CurrentStep}
	for _, step := range steps {
		view.Steps = append(view.Steps, PackageOperationStepView{Name: step.StepName, Order: step.StepOrder,
			Status: step.Status, AttemptCount: step.AttemptCount, ResultJSON: step.ResultJSON,
			ErrorCode: step.ErrorCode, StartedAt: step.StartedAt, CompletedAt: step.CompletedAt})
	}
	return view
}
