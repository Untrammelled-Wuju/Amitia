package extension

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"path"
	"sort"
	"strings"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func (s *PackageService) Export(ctx context.Context, request ExportPackageRequest) (ExportedPackage, error) {
	if err := s.validatePackageScope(ctx, request.UserID, request.ScopeType, request.ScopeID); err != nil {
		return ExportedPackage{}, err
	}
	versionName := request.Version
	if s.readModel != nil && (request.Format == "" || request.Format == "amitiax") {
		if exported, ok, err := s.readModel.TryExport(ctx, request.ExtensionID, versionName, request.UserID, request.ScopeType, request.ScopeID); err != nil {
			return ExportedPackage{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", request.ExtensionID, true, err)
		} else if ok {
			s.metric("extension_package_export_total")
			return exported, nil
		}
	}
	return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "扩展不存在于 Kernel，且不处于人工迁移状态", request.ExtensionID, false, nil)
}

func (s *PackageService) GetExport(ctx context.Context, userID, extensionID, exportID string) (ExportedPackage, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := s.kernelProxy.ReadContainer()
	ticket, err := container.PackageRepository.GetExport(ctx, exportID, userID, extensionID)
	if err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "导出凭据不存在", exportID, false, err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, ticket.ExpiresAt)
	if err != nil || time.Now().UTC().After(expiresAt) {
		return ExportedPackage{}, NewExtensionError(ErrPackageExportNotAllowed, "导出凭据已过期", exportID, false, err)
	}
	artifact, err := container.PackageRepository.GetArtifact(ctx, ticket.ArtifactID)
	if err != nil {
		return ExportedPackage{}, err
	}
	if _, err := s.kernelProxy.kernel.VerifyStoredPackage(ctx, artifact); err != nil {
		return ExportedPackage{}, NewExtensionError(ErrPackageArtifactInvalid, "导出前制品复验失败", artifact.ArtifactID, false, err)
	}
	return ExportedPackage{ExportID: exportID, FileName: ticket.FileName, MIME: ticket.MIMEType,
		Size: artifact.SizeBytes, Hash: artifact.ArchiveHash, Version: artifact.Version,
		Format: "amitiax", SecretScan: "passed", SignatureStatus: artifact.SignatureStatus,
		ExpiresAt: expiresAt, LocalPath: artifact.ArchivePath}, nil
}

func (s *PackageService) ListVersions(ctx context.Context, extensionID, userID, scopeType, scopeID string) ([]PackageVersionView, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return nil, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryListVersions(ctx, extensionID); err != nil {
			return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不处于人工迁移状态", extensionID, false, nil)
}

func (s *PackageService) CompareVersions(ctx context.Context, extensionID, userID, scopeType, scopeID, fromVersion, toVersion string) (PackageVersionDiff, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageVersionDiff{}, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryCompareVersions(ctx, extensionID, fromVersion, toVersion); err != nil {
			return PackageVersionDiff{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	return PackageVersionDiff{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不处于人工迁移状态", extensionID, false, nil)
}

func packageVersionDiffRecords(extensionID string, from, to packageVersionRecord, fromArtifact, toArtifact *packageArtifactRecord) PackageVersionDiff {
	diff := PackageVersionDiff{ExtensionID: extensionID, FromVersion: from.Version, ToVersion: to.Version, Manifest: jsonObjectDiff(from.ManifestJSON, to.ManifestJSON), Schemas: map[string]interface{}{}, Workflow: map[string]interface{}{}, Instructions: map[string]interface{}{}, Capabilities: map[string][]string{}, Signature: map[string]string{"from": from.SignatureStatus, "to": to.SignatureStatus}, Scripts: map[string][]string{"added": {}, "removed": {}}, Dependencies: map[string][]string{"added": {}, "removed": {}}, Trust: map[string]string{"from": from.SignatureStatus, "to": to.SignatureStatus}, Risks: []PackageRisk{}}
	var fromCapabilities, toCapabilities []string
	_ = json.Unmarshal([]byte(from.CapabilitiesJSON), &fromCapabilities)
	_ = json.Unmarshal([]byte(to.CapabilitiesJSON), &toCapabilities)
	diff.Capabilities["added"] = stringSetDifference(toCapabilities, fromCapabilities)
	diff.Capabilities["removed"] = stringSetDifference(fromCapabilities, toCapabilities)
	if len(diff.Capabilities["added"]) > 0 {
		diff.Risks = append(diff.Risks, PackageRisk{Code: "CAPABILITY_ADDED", Severity: "high", Message: strings.Join(diff.Capabilities["added"], ", ")})
	}
	if fromArtifact != nil && toArtifact != nil {
		diff.Schemas = jsonObjectDiff(fromArtifact.SchemasJSON, toArtifact.SchemasJSON)
		if fromArtifact.ArtifactKind == "agent-skill" || toArtifact.ArtifactKind == "agent-skill" {
			diff.Instructions = agentArtifactDiff(*fromArtifact, *toArtifact)
		} else {
			diff.Workflow = jsonObjectDiff(fromArtifact.WorkflowJSON, toArtifact.WorkflowJSON)
		}
	}
	if from.SignerFingerprint != to.SignerFingerprint {
		diff.Signature["fingerprintFrom"] = from.SignerFingerprint
		diff.Signature["fingerprintTo"] = to.SignerFingerprint
		diff.Risks = append(diff.Risks, PackageRisk{Code: "SIGNER_CHANGED", Severity: "high", Message: "签名者发生变化"})
	}
	return diff
}

func jsonObjectDiff(fromRaw, toRaw string) map[string]interface{} {
	var from, to interface{}
	if json.Unmarshal([]byte(fromRaw), &from) != nil || json.Unmarshal([]byte(toRaw), &to) != nil {
		return map[string]interface{}{"changed": fromRaw != toRaw}
	}
	fromJSON, _ := json.Marshal(from)
	toJSON, _ := json.Marshal(to)
	return map[string]interface{}{"changed": string(fromJSON) != string(toJSON), "from": from, "to": to}
}

func agentArtifactDiff(from, to packageArtifactRecord) map[string]interface{} {
	fromFiles, fromErr := decodeAgentSkillArtifact(from.ContentBlob, DefaultAgentSkillLimits())
	toFiles, toErr := decodeAgentSkillArtifact(to.ContentBlob, DefaultAgentSkillLimits())
	if fromErr != nil || toErr != nil {
		return map[string]interface{}{"changed": true, "error": ErrPackageArtifactInvalid}
	}
	fromNames := make([]string, 0, len(fromFiles))
	toNames := make([]string, 0, len(toFiles))
	for name := range fromFiles {
		fromNames = append(fromNames, name)
	}
	for name := range toFiles {
		toNames = append(toNames, name)
	}
	sort.Strings(fromNames)
	sort.Strings(toNames)
	return map[string]interface{}{"skillChanged": string(fromFiles["SKILL.md"]) != string(toFiles["SKILL.md"]), "added": stringSetDifference(toNames, fromNames), "removed": stringSetDifference(fromNames, toNames)}
}

func stringSetDifference(left, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	result := []string{}
	for _, value := range left {
		if !rightSet[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func (s *PackageService) Rollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if s.kernelProxy == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	result, err := s.kernelProxy.kernel.ExecutePackageRollback(ctx, extensionID, version, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRollbackFailed, "Extension Kernel 回滚失败", err.Error(), false, err)
	}
	return PackageOperationResult{OperationID: result.OperationID, TraceID: result.TraceID, Operation: PackageOperationRollback, ExtensionID: extensionID, Version: version, Enabled: false, Status: "succeeded"}, nil
}

func (s *PackageService) Dependencies(ctx context.Context, extensionID, userID, scopeType, scopeID string) (map[string]interface{}, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return nil, err
	}
	if s.readModel != nil {
		if result, ok, err := s.readModel.TryDependencies(ctx, extensionID); err != nil {
			return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
		} else if ok {
			return result, nil
		}
	}
	return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展不存在于 Kernel，且不处于人工迁移状态", extensionID, false, nil)
}

func (s *PackageService) PreviewUninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageUninstallPreview, error) {
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageUninstallPreview{}, err
	}
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return PackageUninstallPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	preview, err := s.kernelProxy.kernel.PreviewPackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageUninstallPreview{}, NewExtensionError(ErrPackageRepositoryUnavailable, "扩展内核读取失败", extensionID, true, err)
	}
	result := PackageUninstallPreview{ExtensionID: preview.ExtensionID, CurrentVersion: preview.CurrentVersion,
		Enabled: preview.Enabled, Dependents: []PackageDependencyView{}, ArtifactArchived: true,
		Cleanup:   []string{"Kernel 定义", "Module", "Contribution", "Installed Tree"},
		Preserved: []string{"Artifact", "Operation", "Rollback Point"}, ReadSource: "kernel"}
	for _, dependent := range preview.Dependents {
		result.Dependents = append(result.Dependents, PackageDependencyView{ID: dependent, Required: true, Installed: true})
	}
	return result, nil
}

func (s *PackageService) Uninstall(ctx context.Context, extensionID, userID, scopeType, scopeID string) (PackageOperationResult, error) {
	if s.kernelProxy == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return PackageOperationResult{}, err
	}
	op, err := s.kernelProxy.kernel.ExecutePackageUninstall(ctx, extensionID, userID, scopeType, scopeID)
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageUninstallFailed, "Extension Kernel 卸载失败", err.Error(), false, err)
	}
	return PackageOperationResult{OperationID: op.OperationID, TraceID: op.TraceID, Operation: PackageOperationUninstall, ExtensionID: extensionID, Status: "succeeded"}, nil
}

func (s *PackageService) validatePackageScope(ctx context.Context, userID, scopeType, scopeID string) error {
	if scopeType == string(ScopeCharacter) {
		return s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: userID, CharacterID: scopeID})
	}
	if scopeType != "" && scopeType != string(ScopeGlobal) {
		return NewExtensionError(ErrSkillPermissionDenied, "扩展作用域无效", scopeType, false, nil)
	}
	return nil
}

func safePackageFileName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var builder strings.Builder
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			builder.WriteRune(char)
		} else {
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), ".-")
}

func (s *PackageService) ListOperations(ctx context.Context, userID string, limit int) ([]PackageOperationView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	records, err := s.kernelProxy.ReadContainer().PackageRepository.ListOperations(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]PackageOperationView, 0, len(records))
	for _, record := range records {
		result = append(result, kernelPackageOperationView(record, nil))
	}
	return result, nil
}

func (s *PackageService) GetOperation(ctx context.Context, userID, id string) (PackageOperationView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return PackageOperationView{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	record, steps, err := s.kernelProxy.ReadContainer().PackageRepository.GetOperation(ctx, userID, id)
	if err != nil {
		return PackageOperationView{}, err
	}
	return kernelPackageOperationView(record, steps), nil
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

func (s *PackageService) ListSigners(ctx context.Context) ([]PackageSignerView, error) {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return nil, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	records, err := s.kernelProxy.ReadContainer().PackageTrustRepository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PackageSignerView, 0, len(records))
	for _, record := range records {
		result = append(result, PackageSignerView{Fingerprint: record.Fingerprint, KeyID: record.KeyID,
			PublisherID: record.PublisherID, Algorithm: "ed25519", DisplayName: record.PublisherID,
			Trusted:     record.TrustLevel == "user_trusted" && record.KeyState == "active",
			TrustSource: record.TrustSource, TrustLevel: record.TrustLevel, KeyState: record.KeyState,
			TrustedAt: record.TrustedAt, RevokedAt: record.RevokedAt})
	}
	return result, nil
}

func (s *PackageService) TrustSigner(ctx context.Context, fingerprint string) error {
	return s.setKernelSignerTrust(ctx, fingerprint, true)
}

func (s *PackageService) RegisterSigner(ctx context.Context, fingerprint, publisherID, keyID string, publicKey []byte) error {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	now := time.Now().UTC()
	key := trust.PublisherKey{KeyID: keyID, PublisherID: publisherID, PublicKey: publicKey,
		Algorithm: trust.AlgorithmEd25519, State: trust.KeyStateActive, CreatedAt: now}
	if err := key.Validate(); err != nil || key.Fingerprint() != fingerprint {
		return NewExtensionError(ErrPackageSignatureInvalid, "发布者密钥无效或指纹不匹配", fingerprint, false, err)
	}
	record := kernelruntime.PackagePublisherKeyRecord{KeyID: keyID, Fingerprint: fingerprint,
		PublicKey: publicKey, PublisherID: publisherID, TrustSource: string(trust.TrustSourceUserDecision),
		TrustLevel: string(trust.TrustLevelUnknown), KeyState: string(trust.KeyStateActive),
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano)}
	container := s.kernelProxy.ReadContainer()
	newValue, err := json.Marshal(record)
	if err != nil {
		return err
	}
	coordinator := kernelruntime.NewPackageTrustMutationCoordinator(container)
	_, err = coordinator.Execute(ctx, trust.PolicyMutation{Kind: trust.PolicyMutationPublisherTrust,
		Actor: "authenticated_user", Reason: "register signer", PublisherID: publisherID, KeyID: keyID,
		NewValue: newValue, Restrictive: true})
	return err
}

func (s *PackageService) UntrustSigner(ctx context.Context, fingerprint string) error {
	return s.setKernelSignerTrust(ctx, fingerprint, false)
}

func (s *PackageService) setKernelSignerTrust(ctx context.Context, fingerprint string, trusted bool) error {
	if s.kernelProxy == nil || s.kernelProxy.ReadContainer() == nil {
		return NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	container := s.kernelProxy.ReadContainer()
	record, err := container.PackageTrustRepository.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return NewExtensionError(ErrPackageSignatureInvalid, "签名密钥不可用", fingerprint, false, err)
	}
	if len(record.PublicKey) != ed25519.PublicKeySize || record.TrustSource == "legacy_fingerprint_only" {
		return NewExtensionError(ErrPackageSignatureInvalid, "签名密钥不可用", fingerprint, false, nil)
	}
	before := record
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if trusted {
		record.TrustLevel = string(trust.TrustLevelUserTrusted)
		record.KeyState = string(trust.KeyStateActive)
		record.TrustedAt = now
		record.RevokedAt = ""
		record.RevocationReason = ""
	} else {
		record.TrustLevel = string(trust.TrustLevelRevoked)
		record.KeyState = string(trust.KeyStateRevoked)
		record.RevokedAt = now
		record.RevocationReason = "user decision"
	}
	record.UpdatedAt = now
	oldValue, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newValue, err := json.Marshal(record)
	if err != nil {
		return err
	}
	reason := "user revoked signer trust"
	if trusted {
		reason = "user trusted signer"
	}
	coordinator := kernelruntime.NewPackageTrustMutationCoordinator(container)
	_, err = coordinator.Execute(ctx, trust.PolicyMutation{Kind: trust.PolicyMutationPublisherTrust,
		Actor: "authenticated_user", Reason: reason, PublisherID: record.PublisherID, KeyID: record.KeyID,
		OldValue: oldValue, NewValue: newValue, Restrictive: !trusted})
	return err
}

func (s *PackageService) PreviewRollback(ctx context.Context, extensionID, version, userID, scopeType, scopeID string) (kernelruntime.PackageRollbackPreviewResult, error) {
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return kernelruntime.PackageRollbackPreviewResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if err := s.validatePackageScope(ctx, userID, scopeType, scopeID); err != nil {
		return kernelruntime.PackageRollbackPreviewResult{}, err
	}
	return s.kernelProxy.kernel.PreviewPackageRollback(ctx, extensionID, version, userID, scopeType, scopeID)
}

func (s *PackageService) VerifyOperationFinalGate(ctx context.Context, operationID string) (kernelruntime.PackageFinalGateResult, error) {
	if s.kernelProxy == nil || s.kernelProxy.kernel == nil {
		return kernelruntime.PackageFinalGateResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	return s.kernelProxy.kernel.VerifyPackageFinalGate(ctx, operationID)
}
