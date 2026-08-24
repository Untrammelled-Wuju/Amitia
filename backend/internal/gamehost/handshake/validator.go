package handshake

import "github.com/u-ai/backend/internal/gamehost/domain"

type RuntimeValidator interface {
	RuntimeExists(runtimeID string) (bool, error)
	ServiceBelongsToRuntime(runtimeID, serviceID, pluginID string) error
}

type DescriptorProvider interface {
	DescriptorCapabilities(pluginID string) ([]string, error)
	DescriptorChannels(pluginID, serviceID string) ([]string, error)
	DescriptorControlSinks(pluginID string) ([]domain.ControlSinkDeclaration, error)
	HasCapability(pluginID, capability string) bool
}
