package channel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type ServiceResolver interface {
	ServiceExists(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (bool, error)
}

type Options struct {
	MaxChannelsPerRuntime int
	MaxChannelsPerService int
	MaxMetadataBytes      int
	ServiceResolver       ServiceResolver
}

type Validator struct {
	opts Options
}

func NewValidator(opts Options) *Validator {
	if opts.MaxChannelsPerRuntime <= 0 {
		opts.MaxChannelsPerRuntime = 4096
	}
	if opts.MaxChannelsPerService <= 0 {
		opts.MaxChannelsPerService = 1024
	}
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = 1 << 20
	}
	return &Validator{opts: opts}
}

func (v *Validator) ValidateRegistration(ctx context.Context, channel RuntimeChannel, currentByRuntime, currentByService int) error {
	if err := channel.Validate(); err != nil {
		return err
	}
	if channel.Metadata != nil {
		total := 0
		for k, val := range channel.Metadata {
			total += len(k) + len(val)
		}
		if total > v.opts.MaxMetadataBytes {
			return ErrMetadataTooLarge
		}
	}
	if v.opts.ServiceResolver != nil {
		exists, err := v.opts.ServiceResolver.ServiceExists(ctx, channel.RuntimeID, channel.ServiceID)
		if err != nil {
			return fmt.Errorf("channel: failed to resolve service: %w", err)
		}
		if !exists {
			return ErrOwnerNotFound
		}
	}
	if currentByRuntime >= v.opts.MaxChannelsPerRuntime {
		return ErrChannelLimitRuntime
	}
	if currentByService >= v.opts.MaxChannelsPerService {
		return ErrChannelLimitService
	}
	return nil
}

func ValidateDirection(channel RuntimeChannel, flow protocol.ChannelDirection) error {
	dir := channel.Direction
	if dir == "" {
		dir = protocol.ChannelDirectionBidirectional
	}
	switch dir {
	case protocol.ChannelDirectionBidirectional:
		return nil
	case protocol.ChannelDirectionPluginToHost:
		if flow != protocol.ChannelDirectionPluginToHost {
			return ErrDirectionNotAllowed
		}
	case protocol.ChannelDirectionHostToPlugin:
		if flow != protocol.ChannelDirectionHostToPlugin {
			return ErrDirectionNotAllowed
		}
	}
	return nil
}
