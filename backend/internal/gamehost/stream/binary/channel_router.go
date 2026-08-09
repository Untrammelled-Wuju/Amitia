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

func (s *ChannelBinarySink) PublishBinary(ctx context.Context, ch channel.RuntimeChannel, msg channel.BinaryChannelMessage) error {
	if err := ch.Validate(); err != nil {
		return err
	}

	var ref BinaryReference
	if err := parseReference(msg.Payload, &ref); err != nil {
		return ErrIDEmpty
	}

	if err := ref.Validate(); err != nil {
		return err
	}

	consumer := BinaryOwner{
		PluginID:  msg.PluginID,
		RuntimeID: msg.RuntimeID,
		ServiceID: msg.ServiceID,
		ChannelID: msg.ChannelID,
	}

	resolved, err := s.resolver.Resolve(ctx, consumer, ref)
	if err != nil {
		return err
	}
	if resolved.Reader != nil {
		defer resolved.Reader.Close()
	}

	return nil
}

func parseReference(data []byte, ref *BinaryReference) error {
	return json.Unmarshal(data, ref)
}
