//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (s *PackageService) GetImportSession(ctx context.Context, sessionID, userID, scopeType, scopeID string) (PackageImportPreview, error) {
	if s.kernel == nil || s.kernel.Container() == nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if scopeType == "" {
		scopeType = string(ScopeGlobal)
	}
	record, err := s.kernel.Container().PackageRepository.GetPreview(ctx, sessionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageImportSessionExpired, "预览会话不存在", sessionID, false, err)
	}
	var preview kernelruntime.InstallPreview
	if err := json.Unmarshal([]byte(record.PreviewResultJSON), &preview); err != nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageArtifactInvalid, "预览会话数据损坏", sessionID, false, err)
	}
	return s.translateKernelPackagePreview(ctx, preview, scopeType, scopeID), nil
}

func (s *PackageService) CancelImportSession(ctx context.Context, sessionID, userID, scopeType, scopeID string) error {
	if s.kernel == nil || s.kernel.Container() == nil {
		return NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if scopeType == "" {
		scopeType = string(ScopeGlobal)
	}
	if err := s.kernel.Container().PackageRepository.CancelPreview(ctx, sessionID, userID, scopeType, scopeID); err != nil {
		return NewExtensionError(ErrPackageImportSessionConsumed, "预览会话不可取消", sessionID, false, err)
	}
	return nil
}

func (s *PackageService) PreviewImportStream(ctx context.Context, userID, scopeType, scopeID, fileName string, reader io.Reader) (result PackageImportPreview, err error) {
	defer func() {
		if err != nil {
			s.metric("package_preview_rejected_total")
			return
		}
		s.metric("package_preview_total")
	}()
	if s.kernel == nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if strings.TrimSpace(userID) == "" {
		return PackageImportPreview{}, NewExtensionError(ErrSkillPermissionDenied, "缺少导入用户", "", false, nil)
	}
	if scopeType == "" {
		scopeType = string(ScopeGlobal)
	}
	if scopeType != string(ScopeGlobal) && scopeType != string(ScopeCharacter) || scopeType == string(ScopeCharacter) && strings.TrimSpace(scopeID) == "" {
		return PackageImportPreview{}, NewExtensionError(ErrSkillPermissionDenied, "扩展安装作用域无效", scopeType, false, nil)
	}
	if scopeType == string(ScopeCharacter) {
		if err := s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: userID, CharacterID: scopeID}); err != nil {
			return PackageImportPreview{}, err
		}
	}
	preview, err := s.kernel.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{UserID: userID, ScopeType: scopeType, ScopeID: scopeID, FileName: fileName}, reader)
	if err != nil {
		return PackageImportPreview{}, NewExtensionError(ErrPackageManifestInvalid, "扩展包 Preview 失败", err.Error(), false, err)
	}
	if preview.SignatureStatus == "unknown_key" || preview.SignatureStatus == "legacy_signature" {
		s.metric("package_signer_unknown_total")
	}
	if preview.SignatureStatus != "unsigned" && preview.SignatureStatus != "valid" {
		s.metric("package_signature_invalid_total")
	}
	return s.translateKernelPackagePreview(ctx, preview, scopeType, scopeID), nil
}

func (s *PackageService) translateKernelPackagePreview(ctx context.Context, preview kernelruntime.InstallPreview, scopeType, scopeID string) PackageImportPreview {
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
	if container := s.kernel.Container(); container != nil {
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

