package handshake

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ChannelAdvertiser interface {
	ValidateChannelAdvertisement(
		pluginID string,
		advertised []ChannelAdvertisement,
	) error
}

type descriptorChannelAdvertiser struct {
	provider DescriptorProvider
}

func NewChannelAdvertiser(provider DescriptorProvider) ChannelAdvertiser {
	return &descriptorChannelAdvertiser{provider: provider}
}

func (a *descriptorChannelAdvertiser) ValidateChannelAdvertisement(
	pluginID string,
	advertised []ChannelAdvertisement,
) error {
	if err := ValidateChannelAdvertisements(advertised); err != nil {
		return err
	}

	channels, err := a.provider.DescriptorChannels(pluginID)
	if err != nil {
		return NewHandshakeError(
			HandshakeErrorChannelInvalid,
			domain.ErrInternal,
			"failed to query descriptor channels",
		)
	}

	allowed := make(map[string]struct{}, len(channels))
	for _, ch := range channels {
		allowed[ch] = struct{}{}
	}

	for _, ad := range advertised {
		if _, ok := allowed[ad.ID]; !ok {
			return NewHandshakeError(
				HandshakeErrorChannelInvalid,
				domain.ErrInvalidArgument,
				"channel not declared in package descriptor: "+ad.ID,
			)
		}
	}

	return nil
}

type NoopChannelAdvertiser struct{}

func (NoopChannelAdvertiser) ValidateChannelAdvertisement(
	pluginID string,
	advertised []ChannelAdvertisement,
) error {
	return ValidateChannelAdvertisements(advertised)
}
