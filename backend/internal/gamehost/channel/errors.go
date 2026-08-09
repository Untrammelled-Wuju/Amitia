package channel

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

var (
	ErrChannelIDEmpty      = domain.NewHostError(domain.ErrInvalidArgument, "channel: id must not be empty")
	ErrOwnerEmpty          = domain.NewHostError(domain.ErrInvalidArgument, "channel: owner service id must not be empty")
	ErrKindUnknown         = domain.NewHostError(domain.ErrInvalidArgument, "channel: unknown kind")
	ErrDirectionUnknown    = domain.NewHostError(domain.ErrInvalidArgument, "channel: unknown direction")
	ErrFrequencyUnknown    = domain.NewHostError(domain.ErrInvalidArgument, "channel: unknown frequency")
	ErrChannelExists       = domain.NewHostError(domain.ErrAlreadyExists, "channel: already exists")
	ErrChannelNotFound     = domain.NewHostError(domain.ErrNotFound, "channel: not found")
	ErrPluginMismatch      = domain.NewHostError(domain.ErrInvalidArgument, "channel: plugin id mismatch")
	ErrRuntimeMismatch     = domain.NewHostError(domain.ErrInvalidArgument, "channel: runtime id mismatch")
	ErrServiceMismatch     = domain.NewHostError(domain.ErrInvalidArgument, "channel: service id mismatch")
	ErrOwnerNotFound       = domain.NewHostError(domain.ErrInvalidArgument, "channel: owner service not found")
	ErrDirectionNotAllowed = domain.NewHostError(domain.ErrInvalidArgument, "channel: direction not allowed")
	ErrChannelLimitRuntime = domain.NewHostError(domain.ErrResourceExhausted, "channel: max channels per runtime reached")
	ErrChannelLimitService = domain.NewHostError(domain.ErrResourceExhausted, "channel: max channels per service reached")
	ErrBinaryNotSupported  = domain.NewHostError(domain.ErrUnsupported, "channel: binary transport not yet supported (G33)")
	ErrMetadataTooLarge    = domain.NewHostError(domain.ErrResourceExhausted, "channel: metadata too large")
)
