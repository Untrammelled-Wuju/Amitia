package handshake

type RuntimeValidator interface {
	RuntimeExists(runtimeID string) (bool, error)
	ServiceBelongsToRuntime(runtimeID, serviceID, pluginID string) error
}

type DescriptorProvider interface {
	DescriptorCapabilities(pluginID string) ([]string, error)
	DescriptorChannels(pluginID string) ([]string, error)
	HasCapability(pluginID, capability string) bool
}
