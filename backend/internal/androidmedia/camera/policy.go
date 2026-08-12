package camera

import (
	"time"
)

const (
	DefaultMaxCapturePixels = 16_000_000
	DefaultMaxEncodedBytes  = 50 * 1024 * 1024
)

type Policy struct {
	MaxCapturePixels int64
	MaxEncodedBytes  int64

	DefaultFormat  string
	DefaultQuality int

	MaxWidth  int
	MaxHeight int

	MaxCaptureTime      time.Duration
	MinInterval         time.Duration
	MaxConcurrentCaptures int

	StripSensitiveEXIF bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxCapturePixels: DefaultMaxCapturePixels,
		MaxEncodedBytes:  DefaultMaxEncodedBytes,

		DefaultFormat:  "jpeg",
		DefaultQuality: 90,

		MaxWidth:  3840,
		MaxHeight: 2160,

		MaxCaptureTime:        30 * time.Second,
		MinInterval:           500 * time.Millisecond,
		MaxConcurrentCaptures: 1,

		StripSensitiveEXIF: true,
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

func (p Policy) ResolveFormat(requested *string) string {
	if requested != nil && validFormats[*requested] {
		return *requested
	}
	return p.DefaultFormat
}

func (p Policy) ResolveQuality(requested *int) int {
	if requested != nil && *requested >= 1 && *requested <= 100 {
		return *requested
	}
	return p.DefaultQuality
}

func (p Policy) ResolveFlashMode(requested *string) string {
	if requested != nil && validFlashModes[*requested] {
		return *requested
	}
	return FlashOff
}
