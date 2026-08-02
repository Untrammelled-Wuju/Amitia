//go:build legacy_migration

package extension

import (
	"context"
	"encoding/json"
	"strings"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
)

func (s *PackageService) Install(ctx context.Context, request InstallPackageRequest) (result PackageOperationResult, err error) {
	defer func() {
		if err != nil {
			s.metric("package_install_failed_total")
			return
		}
		s.metric("package_install_total")
	}()
	if s.kernel == nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageRepositoryUnavailable, "Extension Kernel 不可用", "", true, nil)
	}
	if request.ScopeType == string(ScopeCharacter) {
		if err := s.repository.ValidateCharacterScope(ctx, ExecutionScope{UserID: request.UserID, CharacterID: request.ScopeID}); err != nil {
			return PackageOperationResult{}, err
		}
	}
	confirmations := map[string]bool{}
	for key, value := range request.Confirmations {
		confirmations[key] = value
		if !strings.HasPrefix(key, "confirm.") {
			confirmations["confirm."+key] = value
		}
	}
	confirmations["confirm.unsigned"] = confirmations["confirm.unsigned"] || request.ConfirmUnsigned
	if confirmations["confirm.unsigned"] {
		s.metric("package_unsigned_confirmed_total")
	}
	confirmations["confirm.scripts"] = confirmations["confirm.scripts"] || request.ConfirmScripts
	confirmations["confirm.version_change"] = confirmations["confirm.version_change"] || request.ConfirmVersionChange
	confirmations["confirm.signer_change"] = confirmations["confirm.signer_change"] || request.ConfirmSignerChange
	confirmations["confirm.config_migration"] = confirmations["confirm.config_migration"] || request.ConfirmConfigMigration
	confirmations["confirm.permission_escalation"] = confirmations["confirm.permission_escalation"] || len(request.ConfirmedCapabilities) > 0
	kernelResult, err := s.kernel.ExecutePackageInstall(ctx, kernelruntime.PackageInstallRequest{SessionID: request.SessionID,
		UserID: request.UserID, ScopeType: request.ScopeType, ScopeID: request.ScopeID,
		Confirmations: confirmations, ExpectedExtensionID: request.ExpectedExtensionID})
	if err != nil {
		return PackageOperationResult{}, NewExtensionError(ErrPackageInstallFailed, "Extension Kernel 安装失败", err.Error(), false, err)
	}
	operation := PackageOperationInstall
	if kernelResult.Operation == "update" {
		operation = PackageOperationUpgrade
	}
	return PackageOperationResult{OperationID: kernelResult.OperationID, TraceID: kernelResult.TraceID, Operation: operation, ExtensionID: kernelResult.ExtensionID, Version: kernelResult.Version, Enabled: false, Status: "succeeded"}, nil
}

func packageSchemas(parsed parsedExtensionPackage, manifest Manifest) map[string]json.RawMessage {
	result := map[string]json.RawMessage{"input": normalizeJSON(manifest.InputSchema), "output": normalizeJSON(manifest.OutputSchema), "config": normalizeJSON(manifest.ConfigSchema), "defaults": normalizeJSON(manifest.DefaultConfig)}
	for key, value := range parsed.Schemas {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}
