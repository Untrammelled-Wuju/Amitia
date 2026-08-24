package channel

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RuntimeTopologyProvider interface {
	GetServiceIDs(ctx context.Context, runtimeID domain.RuntimeInstanceID) ([]domain.ServiceID, error)
}

type ChannelMappingInput struct {
	PluginID    domain.PluginID
	RuntimeID   domain.RuntimeInstanceID
	ServiceID   domain.ServiceID
	Descriptors []protocol.ChannelDescriptor
}

type ChannelMappingResult struct {
	Channels []RuntimeChannel
	Errors   []error
}

type Mapper struct{}

func NewMapper() *Mapper {
	return &Mapper{}
}

func (m *Mapper) Map(ctx context.Context, input ChannelMappingInput) (ChannelMappingResult, error) {
	result := ChannelMappingResult{
		Channels: make([]RuntimeChannel, 0, len(input.Descriptors)),
		Errors:   make([]error, 0),
	}

	for _, desc := range input.Descriptors {
		ch, err := m.mapOne(input.PluginID, input.RuntimeID, input.ServiceID, desc)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		result.Channels = append(result.Channels, ch)
	}

	return result, nil
}

func (m *Mapper) mapOne(pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, desc protocol.ChannelDescriptor) (RuntimeChannel, error) {
	channelID := domain.ChannelID(desc.ID)
	id := NewRuntimeChannelID(runtimeID, serviceID, channelID)

	var freq *protocol.FrequencyHint
	if desc.FrequencyHint != nil {
		f := *desc.FrequencyHint
		freq = &f
	}

	var metadata map[string]json.RawMessage
	if desc.Metadata != nil {
		metadata = make(map[string]json.RawMessage, len(desc.Metadata))
		for k, v := range desc.Metadata {
			cp := make(json.RawMessage, len(v))
			copy(cp, v)
			metadata[k] = cp
		}
	}

	return RuntimeChannel{
		ID:        id,
		PluginID:  pluginID,
		RuntimeID: runtimeID,
		ServiceID: serviceID,
		ChannelID: channelID,
		Kind:      domain.ChannelKind(desc.Kind),
		SchemaID:  desc.SchemaID,
		Direction: desc.Direction,
		Frequency: freq,
		Metadata:  metadata,
	}, nil
}

func (m *Mapper) BuildForRuntime(ctx context.Context, pluginID domain.PluginID, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, descriptors []protocol.ChannelDescriptor) ([]RuntimeChannel, error) {
	input := ChannelMappingInput{
		PluginID:    pluginID,
		RuntimeID:   runtimeID,
		ServiceID:   serviceID,
		Descriptors: descriptors,
	}

	result, err := m.Map(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(result.Errors) > 0 {
		return nil, result.Errors[0]
	}
	return result.Channels, nil
}
