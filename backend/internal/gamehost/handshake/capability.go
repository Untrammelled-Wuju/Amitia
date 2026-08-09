package handshake

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	MaxCapabilities = 64
	MaxNamespaces   = 256
	MaxChannels     = 32
	MaxStringLen    = 128
)

func ValidateCapabilities(caps []string) error {
	if len(caps) > MaxCapabilities {
		return NewHandshakeError(
			HandshakeErrorCapabilityMismatch,
			domain.ErrInvalidArgument,
			"too many capabilities advertised",
		)
	}
	seen := make(map[domain.Capability]struct{}, len(caps))
	for _, c := range caps {
		capab := domain.Capability(c)
		if err := domain.ValidateCapability(capab); err != nil {
			return NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrInvalidArgument,
				"invalid capability: "+string(capab),
			)
		}
		if _, ok := seen[capab]; ok {
			return NewHandshakeError(
				HandshakeErrorCapabilityMismatch,
				domain.ErrInvalidArgument,
				"duplicate capability: "+string(capab),
			)
		}
		seen[capab] = struct{}{}
	}
	return nil
}

func ValidateProtocols(protocols []string) error {
	if len(protocols) == 0 {
		return NewHandshakeError(
			HandshakeErrorProtocolMismatch,
			domain.ErrInvalidArgument,
			"supportedProtocols must not be empty",
		)
	}
	seen := make(map[string]struct{})
	for _, p := range protocols {
		if len(p) > MaxStringLen {
			return NewHandshakeError(
				HandshakeErrorProtocolMismatch,
				domain.ErrInvalidArgument,
				"protocol string too long",
			)
		}
		if _, ok := seen[p]; ok {
			return NewHandshakeError(
				HandshakeErrorProtocolMismatch,
				domain.ErrInvalidArgument,
				"duplicate protocol: "+p,
			)
		}
		seen[p] = struct{}{}
	}
	return nil
}

func ValidateChannelAdvertisements(channels []ChannelAdvertisement) error {
	if len(channels) > MaxChannels {
		return NewHandshakeError(
			HandshakeErrorChannelInvalid,
			domain.ErrInvalidArgument,
			"too many channels advertised",
		)
	}
	seen := make(map[string]struct{})
	for _, ch := range channels {
		if len(ch.ID) == 0 {
			return NewHandshakeError(
				HandshakeErrorChannelInvalid,
				domain.ErrInvalidArgument,
				"channel ID must not be empty",
			)
		}
		if len(ch.ID) > MaxStringLen {
			return NewHandshakeError(
				HandshakeErrorChannelInvalid,
				domain.ErrInvalidArgument,
				"channel ID too long",
			)
		}
		if _, ok := seen[ch.ID]; ok {
			return NewHandshakeError(
				HandshakeErrorChannelInvalid,
				domain.ErrInvalidArgument,
				"duplicate channel ID: "+ch.ID,
			)
		}
		seen[ch.ID] = struct{}{}
	}
	return nil
}
