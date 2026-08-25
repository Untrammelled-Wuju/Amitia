package binary

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/channel"
)

type ChannelBinarySink struct {
	resolver *Resolver
}

func NewChannelBinarySink(resolver *Resolver) *ChannelBinarySink {
	return &ChannelBinarySink{resolver: resolver}
}

func (s *ChannelBinarySink) PublishBinary(ctx context.Context, ch channel.RuntimeChannel, msg channel.BinaryChannelMessage) (json.RawMessage, error) {
	if s == nil || s.resolver == nil {
		return nil, channel.ErrBinaryNotSupported
	}
	if err := ch.Validate(); err != nil {
		return nil, err
	}

	var ref BinaryReference
	if err := parseReference(msg.Payload, &ref); err != nil {
		return nil, ErrIDEmpty
	}

	if err := ref.Validate(); err != nil {
		return nil, err
	}

	consumer := BinaryOwner{
		PluginID:  msg.PluginID,
		RuntimeID: msg.RuntimeID,
		ServiceID: msg.ServiceID,
		ChannelID: msg.ChannelID,
	}

	resolved, err := s.resolver.Resolve(ctx, consumer, ref)
	if err != nil {
		return nil, err
	}
	if resolved.Reader != nil {
		defer resolved.Reader.Close()
	}

	// The resolver returns the registry/provider-authoritative reference. Marshal
	// that reference rather than the caller's JSON so forged mutable fields do
	// not cross the validation boundary into durable/fanout sinks.
	canonical, err := json.Marshal(resolved.Reference)
	if err != nil {
		return nil, err
	}
	return canonical, nil
}

func parseReference(data []byte, ref *BinaryReference) error {
	return json.Unmarshal(data, ref)
}
