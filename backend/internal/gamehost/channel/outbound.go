package channel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const MethodChannelDeliver = "channel.deliver"

type RuntimeGenerationReader interface {
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
}

type OutboundMessage struct {
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
	ChannelID domain.ChannelID
	Payload   json.RawMessage
	Metadata  map[string]json.RawMessage
}

type channelDeliveryPayload struct {
	ChannelID string                     `json:"channelId"`
	Payload   json.RawMessage            `json:"payload"`
	Metadata  map[string]json.RawMessage `json:"metadata,omitempty"`
}

// OutboundPublisher is the canonical host -> plugin channel transport. It
// validates the channel declaration and current runtime generation before the
// delivery is written to the negotiated service connection.
type OutboundControlPlane interface {
	Send(ctx context.Context, peer ipc.Peer, envelope protocol.Envelope) error
}

type OutboundPublisher struct {
	registry    Registry
	control     OutboundControlPlane
	generations RuntimeGenerationReader
}

func NewOutboundPublisher(registry Registry, control OutboundControlPlane, generations RuntimeGenerationReader) (*OutboundPublisher, error) {
	if registry == nil || control == nil || generations == nil {
		return nil, fmt.Errorf("channel: registry, control plane and generation reader are required")
	}
	return &OutboundPublisher{registry: registry, control: control, generations: generations}, nil
}

func (p *OutboundPublisher) Publish(ctx context.Context, message OutboundMessage) error {
	if p == nil {
		return fmt.Errorf("channel: outbound publisher is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if message.RuntimeID == "" || message.ServiceID == "" || message.ChannelID == "" {
		return domain.NewHostError(domain.ErrInvalidArgument, "channel: runtimeId, serviceId and channelId are required")
	}

	declared, err := p.registry.Resolve(ctx, message.RuntimeID, message.ServiceID, message.ChannelID)
	if err != nil {
		return err
	}
	if err := ValidateDirection(declared, protocol.ChannelDirectionHostToPlugin); err != nil {
		return err
	}

	generation, err := p.generations.GetCurrentGeneration(message.RuntimeID)
	if err != nil {
		return domain.NewHostErrorWithCause(domain.ErrRuntimeUnavailable, "channel: runtime generation unavailable", err)
	}
	if generation <= 0 {
		return domain.NewHostError(domain.ErrInvalidState, "channel: runtime generation must be positive")
	}

	payload := channelDeliveryPayload{
		ChannelID: string(message.ChannelID),
		Payload:   append(json.RawMessage(nil), message.Payload...),
		Metadata:  cloneRawMetadata(message.Metadata),
	}
	envelope, err := protocol.NewNotificationWithRoute(
		"channel-deliver-"+uuid.NewString(),
		MethodChannelDeliver,
		payload,
		string(message.RuntimeID),
		string(declared.PluginID),
		string(message.ServiceID),
	)
	if err != nil {
		return fmt.Errorf("channel: build outbound delivery: %w", err)
	}
	envelope.Generation = uint64(generation)

	peer := ipc.Peer{
		PluginID:   declared.PluginID,
		RuntimeID:  message.RuntimeID,
		ServiceID:  message.ServiceID,
		Generation: generation,
	}
	if err := p.control.Send(ctx, peer, envelope); err != nil {
		return fmt.Errorf("channel: outbound delivery failed: %w", err)
	}
	return nil
}

func cloneRawMetadata(source map[string]json.RawMessage) map[string]json.RawMessage {
	if source == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		result[key] = append(json.RawMessage(nil), value...)
	}
	return result
}
