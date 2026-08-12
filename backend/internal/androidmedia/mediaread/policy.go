package mediaread

import (
	"time"
)

type Policy struct {
	MaxInputBytes int64

	MaxPixels int64

	MaxWidth  int
	MaxHeight int

	MaxNormalizedBytes int64

	MaxDecodeTime time.Duration

	MaxConcurrentReads int

	NormalizeOrientation  bool
	StripSensitiveMetadata bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxInputBytes: 64 * 1024 * 1024,

		MaxPixels: 40_000_000,

		MaxWidth:  12000,
		MaxHeight: 12000,

		MaxNormalizedBytes: 32 * 1024 * 1024,

		MaxDecodeTime: 10 * time.Second,

		MaxConcurrentReads: 2,

		NormalizeOrientation:  true,
		StripSensitiveMetadata: true,
	}
}

func (p Policy) ResolveMaxWidth(requested *int) int {
	if requested != nil && *requested > 0 && *requested <= p.MaxWidth {
		return *requested
	}
	return p.MaxWidth
}

func (p Policy) ResolveMaxHeight(requested *int) int {
	if requested != nil && *requested > 0 && *requested <= p.MaxHeight {
		return *requested
	}
	return p.MaxHeight
}

func (p Policy) ResolveMaxPixels(requested *int64) int64 {
	if requested != nil && *requested > 0 && *requested <= p.MaxPixels {
		return *requested
	}
	return p.MaxPixels
}

func (p Policy) EffectiveNormalizeOrientation(requested *bool) bool {
	if requested != nil {
		return *requested
	}
	return p.NormalizeOrientation
}

func (p Policy) EffectiveStripMetadata(requested *bool) bool {
	if requested != nil {
		return *requested
	}
	return p.StripSensitiveMetadata
}
