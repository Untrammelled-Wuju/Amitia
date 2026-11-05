package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (m *HostPluginPackageManager) Uninstall(ctx context.Context, request UninstallHostPluginPackageRequest) (PackageOperationRecord, error) {
	if m == nil || m.runtime == nil {
		return PackageOperationRecord{}, errors.New("plugin package manager unavailable")
	}
	extensionID := strings.TrimSpace(request.ExtensionID)
	if extensionID == "" {
		return PackageOperationRecord{}, errors.New("extensionId is required")
	}
	container := m.runtime.Container()
	if container == nil || container.DefinitionRepository == nil || container.InstallationRepository == nil {
		return PackageOperationRecord{}, errors.New("plugin package repository unavailable")
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err != nil {
		return PackageOperationRecord{}, fmt.Errorf("load installed plugin state: %w", err)
	}
	if container.DefinitionRepository == nil {
		return PackageOperationRecord{}, errors.New("plugin definition repository unavailable")
	}
	if hostPluginIsSystemManaged(ctx, container, installation) {
		return PackageOperationRecord{}, errors.New("system plugin uninstall is forbidden")
	}

	preview, err := m.runtime.PreviewPackageUninstall(ctx, extensionID, request.SpaceID, "global", "")
	if err != nil {
		return PackageOperationRecord{}, fmt.Errorf("preview plugin package uninstall: %w", err)
	}
	disabledForUninstall := false
	if !preview.Installable && preview.Enabled && len(preview.Dependents) == 0 {
		if err := m.runtime.Disable(ctx, extensionID); err != nil {
			return PackageOperationRecord{}, fmt.Errorf("disable plugin before uninstall: %w", err)
		}
		disabledForUninstall = true
		preview, err = m.runtime.PreviewPackageUninstall(ctx, extensionID, request.SpaceID, "global", "")
		if err != nil {
			enableErr := m.runtime.Enable(ctx, extensionID)
			if enableErr != nil {
				return PackageOperationRecord{}, errors.Join(
					fmt.Errorf("preview plugin package uninstall: %w", err),
					fmt.Errorf("restore plugin enabled state: %w", enableErr),
				)
			}
			return PackageOperationRecord{}, fmt.Errorf("preview plugin package uninstall: %w", err)
		}
	}
	restoreEnabled := func(cause error) error {
		if !disabledForUninstall {
			return cause
		}
		if enableErr := m.runtime.Enable(ctx, extensionID); enableErr != nil {
			return errors.Join(cause, fmt.Errorf("restore plugin enabled state: %w", enableErr))
		}
		return cause
	}
	if !preview.Installable {
		switch {
		case len(preview.Dependents) > 0:
			return PackageOperationRecord{}, restoreEnabled(
				fmt.Errorf("插件存在依赖项，无法卸载：%s", strings.Join(preview.Dependents, "、")),
			)
		case preview.Enabled:
			return PackageOperationRecord{}, restoreEnabled(errors.New("请先停用该插件后再卸载"))
		default:
			return PackageOperationRecord{}, restoreEnabled(errors.New("当前插件不可卸载"))
		}
	}

	confirmations := make(map[string]bool)
	for _, confirmation := range RequiredUninstallConfirmations(preview) {
		if value := strings.TrimSpace(confirmation); value != "" {
			confirmations[value] = true
		}
	}
	confirmation, err := m.runtime.ConfirmPackageUninstall(ctx, ConfirmPackageUninstallRequest{
		ExtensionID:   extensionID,
		SpaceID:       request.SpaceID,
		ScopeType:     "global",
		Confirmations: confirmations,
	})
	if err != nil {
		return PackageOperationRecord{}, restoreEnabled(fmt.Errorf("confirm plugin package uninstall: %w", err))
	}
	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = "host-plugin-uninstall:" + request.SpaceID + ":" + extensionID
	}
	operation, err := m.runtime.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{
		ExtensionID:       extensionID,
		SpaceID:           request.SpaceID,
		ScopeType:         "global",
		ConfirmationToken: confirmation.Token,
		IdempotencyKey:    idempotencyKey,
	})
	if err != nil {
		return PackageOperationRecord{}, restoreEnabled(fmt.Errorf("uninstall plugin package: %w", err))
	}
	return operation, nil
}
