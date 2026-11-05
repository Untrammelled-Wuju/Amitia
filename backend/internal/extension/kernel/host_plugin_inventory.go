package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func (m *HostPluginPackageManager) List(ctx context.Context) (HostPluginInventory, error) {
	if m == nil || m.runtime == nil {
		return HostPluginInventory{}, errors.New("plugin package manager unavailable")
	}
	container := m.runtime.Container()
	if container == nil || container.InstallationRepository == nil {
		return HostPluginInventory{}, errors.New("plugin package repository unavailable")
	}
	installations, err := container.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return HostPluginInventory{}, fmt.Errorf("list installed plugins: %w", err)
	}
	plugins := make([]HostPluginInstallation, 0, len(installations))
	for _, installation := range installations {
		plugins = append(plugins, HostPluginInstallation{
			ExtensionID:       string(installation.ExtensionID),
			Version:           installation.InstalledVersion.String(),
			InstallationState: string(installation.InstallationState),
			Enabled:           installation.EnablementState == domain.EnablementEnabled,
			SystemManaged:     hostPluginIsSystemManaged(ctx, container, installation),
		})
	}
	return HostPluginInventory{Plugins: plugins}, nil
}

func hostPluginIsSystemManaged(ctx context.Context, container *Container, installation domain.ExtensionInstallation) bool {
	if strings.HasPrefix(string(installation.ExtensionID), "com.amitia.builtin.") {
		return true
	}
	if container == nil || container.DefinitionRepository == nil {
		return false
	}
	definition, err := container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		return false
	}
	managed, _ := definition.Metadata["system.managed"].(bool)
	return managed
}
