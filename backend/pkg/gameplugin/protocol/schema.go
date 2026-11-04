package protocol

import "fmt"

type PluginSchema struct {
	Services     []ServiceDescriptor `json:"services,omitempty"`
	Channels     []ChannelDescriptor `json:"channels,omitempty"`
	HostFeatures []HostFeature       `json:"hostFeatures,omitempty"`
}

func (ps PluginSchema) Validate() error {
	if err := ValidateServices(ps.Services); err != nil {
		return fmt.Errorf("services validation failed: %w", err)
	}
	if err := ValidateChannels(ps.Channels); err != nil {
		return fmt.Errorf("channels validation failed: %w", err)
	}
	if err := ValidateHostFeatures(ps.HostFeatures); err != nil {
		return fmt.Errorf("hostFeatures validation failed: %w", err)
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

func (ps PluginSchema) HasHostFeature(feature HostFeature) bool {
	for _, current := range ps.HostFeatures {
		if current == feature {
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

func (ps PluginSchema) HostFeatureCount() int {
	return len(ps.HostFeatures)
}
