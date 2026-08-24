package binary

import (
	"github.com/u-ai/backend/internal/gamehost/domain"
)

var (
	ErrIDEmpty             = domain.NewHostError(domain.ErrInvalidArgument, "binary: id must not be empty")
	ErrIDFormat            = domain.NewHostError(domain.ErrInvalidArgument, "binary: id format invalid")
	ErrSizeNegative        = domain.NewHostError(domain.ErrInvalidArgument, "binary: size must not be negative")
	ErrSizeTooLarge        = domain.NewHostError(domain.ErrResourceExhausted, "binary: size exceeds maximum allowed")
	ErrKindUnknown         = domain.NewHostError(domain.ErrInvalidArgument, "binary: unknown storage kind")
	ErrLifetimeUnknown     = domain.NewHostError(domain.ErrInvalidArgument, "binary: unknown lifetime")
	ErrMediaTypeTooLong    = domain.NewHostError(domain.ErrInvalidArgument, "binary: media type too long")
	ErrChecksumInvalid     = domain.NewHostError(domain.ErrInvalidArgument, "binary: checksum invalid")
	ErrMetadataTooLarge    = domain.NewHostError(domain.ErrResourceExhausted, "binary: metadata too large")
	ErrOwnerRequired       = domain.NewHostError(domain.ErrInvalidArgument, "binary: owner required")
	ErrChannelIDEmpty      = domain.NewHostError(domain.ErrInvalidArgument, "binary: channel id must not be empty")
	ErrObjectNotFound      = domain.NewHostError(domain.ErrNotFound, "binary: object not found")
	ErrObjectReleased      = domain.NewHostError(domain.ErrNotFound, "binary: object already released")
	ErrObjectNotReady      = domain.NewHostError(domain.ErrInvalidState, "binary: object not ready")
	ErrOwnerMismatch       = domain.NewHostError(domain.ErrNotFound, "binary: owner mismatch")
	ErrSizeMismatch        = domain.NewHostError(domain.ErrInvalidArgument, "binary: size mismatch")
	ErrKindMismatch        = domain.NewHostError(domain.ErrInvalidArgument, "binary: kind mismatch")
	ErrLifetimeMismatch    = domain.NewHostError(domain.ErrInvalidArgument, "binary: lifetime mismatch")
	ErrActiveObjectLimit   = domain.NewHostError(domain.ErrResourceExhausted, "binary: active object limit reached")
	ErrActiveBytesLimit    = domain.NewHostError(domain.ErrResourceExhausted, "binary: active byte limit reached")
	ErrUnsupportedPlatform = domain.NewHostError(domain.ErrUnsupported, "binary: shared memory not supported on this platform")
	ErrPathEscapesRoot     = domain.NewHostError(domain.ErrInvalidArgument, "binary: path escapes root")
	ErrFailedPrecondition  = domain.NewHostError(domain.ErrInvalidState, "binary: failed precondition")
)
