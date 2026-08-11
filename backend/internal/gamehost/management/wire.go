package management

import (
	"github.com/u-ai/backend/internal/gamehost"
)

func NewProductionService(container *gamehost.GameHostContainer, kernel KernelManagementReader) *GameCenterManagementService {
	if container == nil {
		return NewGameCenterManagementService(GameCenterManagementServiceOptions{
			Kernel: kernel,
		})
	}

	opts := GameCenterManagementServiceOptions{
		Kernel: kernel,
	}

	if container.PluginRegistry != nil {
		opts.Registry = NewGameHostPluginRegistry(container.PluginRegistry)
	}

	if container.HandshakeManager != nil {
		opts.Handshake = NewGameHostHandshakeManager(container.HandshakeManager)
	}

	if container.AuthorityManager != nil {
		opts.Authority = NewGameHostAuthorityManager(container.AuthorityManager)
	}

	return NewGameCenterManagementService(opts)
}
