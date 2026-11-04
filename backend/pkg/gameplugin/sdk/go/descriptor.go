package sdk

import (
	"encoding/json"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type DescriptorBuilder struct {
	descriptor Descriptor
}

func NewDescriptor(id string, name string, version string) *DescriptorBuilder {
	return &DescriptorBuilder{
		descriptor: Descriptor{
			ID:              id,
			Name:            name,
			Version:         version,
			ProtocolVersion: protocol.ProtocolVersion,
			Services:        make([]protocol.ServiceDescriptor, 0),
			Channels:        make([]protocol.ChannelDescriptor, 0),
			HostFeatures:    make([]protocol.HostFeature, 0),
			Metadata:        make(map[string]json.RawMessage),
		},
	}
}

func (b *DescriptorBuilder) WithService(svc protocol.ServiceDescriptor) *DescriptorBuilder {
	b.descriptor.Services = append(b.descriptor.Services, svc)
	return b
}

func (b *DescriptorBuilder) WithChannel(ch protocol.ChannelDescriptor) *DescriptorBuilder {
	b.descriptor.Channels = append(b.descriptor.Channels, ch)
	return b
}

func (b *DescriptorBuilder) WithHostFeature(feature protocol.HostFeature) *DescriptorBuilder {
	b.descriptor.HostFeatures = append(b.descriptor.HostFeatures, feature)
	return b
}

func (b *DescriptorBuilder) WithMetadata(key string, value json.RawMessage) *DescriptorBuilder {
	b.descriptor.Metadata[key] = value
	return b
}

func (b *DescriptorBuilder) Build() (Descriptor, error) {
	if err := b.descriptor.Validate(); err != nil {
		return Descriptor{}, err
	}
	return b.descriptor, nil
}
