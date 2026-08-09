package registry

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RegistrySnapshot struct {
	Plugins []domain.PluginDescriptor
}
