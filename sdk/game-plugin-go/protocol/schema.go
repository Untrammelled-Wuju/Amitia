package protocol

import "fmt"

type PluginSchema struct {
	Services     []ServiceDescriptor `json:"services,omitempty"`
	Channels     []ChannelDescriptor `json:"channels,omitempty"`
	Capabilities []Capability        `json:"capabilities,omitempty"`
}

func (ps PluginSchema) Validate() error {
	if err := ValidateServices(ps.Services); err != nil {
		return fmt.Errorf("services validation failed: %w", err)
	}
	if err := ValidateChannels(ps.Channels); err != nil {
		return fmt.Errorf("channels validation failed: %w", err)
	}
	if err := ValidateCapabilities(ps.Capabilities); err != nil {
		return fmt.Errorf("capabilities validation failed: %w", err)
	}
	return nil
}

func (ps PluginSchema) FindService(id ServiceID) (*ServiceDescriptor, bool) {
	for i := range ps.Services {
		if ps.Services[i].ID == id {
			return &ps.Services[i], true
		}
	}
	return nil, false
}

func (ps PluginSchema) FindChannel(id ChannelID) (*ChannelDescriptor, bool) {
	for i := range ps.Channels {
		if ps.Channels[i].ID == id {
			return &ps.Channels[i], true
		}
	}
	return nil, false
}

func (ps PluginSchema) HasCapability(cap Capability) bool {
	for _, c := range ps.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (ps PluginSchema) ServiceCount() int {
	return len(ps.Services)
}

func (ps PluginSchema) ChannelCount() int {
	return len(ps.Channels)
}

func (ps PluginSchema) CapabilityCount() int {
	return len(ps.Capabilities)
}
