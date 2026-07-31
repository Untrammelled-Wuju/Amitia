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
	for _, dependency := range preview.MissingDependencies {
		result.Dependencies = append(result.Dependencies, PackageDependencyView{ID: dependency.ID,
			VersionConstraint: dependency.Version, Required: !dependency.Optional, Installed: !dependency.Missing})
	}
	for _, issue := range preview.Issues {
		result.Errors = append(result.Errors, issue.Code+": "+issue.Message)
	}
	for _, risk := range preview.RiskFlags {
		result.Risks = append(result.Risks, PackageRisk{Code: risk, Severity: "high", Message: risk})
	}
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
