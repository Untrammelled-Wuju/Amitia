package screenframe

import (
	"time"
)

type ScreenFramePolicy struct {
	Enabled bool

	MaxSessions int

	DefaultFPS float64
	MaxFPS     float64

	MaxWidth  int
	MaxHeight int
	MaxPixels int64

	MaxSessionDuration time.Duration
	IdleTimeout        time.Duration

	MaxLatestFrameBytes int64

	MaxEncodedFrameBytes int64

	MaxEncodeConcurrency int

	DefaultLatestWait time.Duration
	MaxLatestWait     time.Duration
}

func DefaultScreenFramePolicy() ScreenFramePolicy {
	return ScreenFramePolicy{
		Enabled: true,

		MaxSessions: 1,

		DefaultFPS: 2,
		MaxFPS:     10,

		MaxWidth:  1280,
		MaxHeight: 1280,
		MaxPixels: 16_000_000,

		MaxSessionDuration: 5 * time.Minute,
		IdleTimeout:        30 * time.Second,

		MaxLatestFrameBytes: 1280 * 1280 * 4,

		MaxEncodedFrameBytes: 50 * 1024 * 1024,

		MaxEncodeConcurrency: 1,

		DefaultLatestWait: 1 * time.Second,
		MaxLatestWait:     5 * time.Second,
	}
}

func (p ScreenFramePolicy) FrameInterval() time.Duration {
	if p.DefaultFPS <= 0 {
		return 500 * time.Millisecond
	}
	return time.Duration(float64(time.Second) / p.DefaultFPS)
}

func (p ScreenFramePolicy) MaxFrameAge() time.Duration {
	interval := p.FrameInterval()
	if interval <= 0 {
		return 2 * time.Second
	}
	maxAge := 2 * interval
	if maxAge > 2*time.Second {
		maxAge = 2 * time.Second
	}
	return maxAge
}

type ScreenshotFormat string

const (
	FormatPNG  ScreenshotFormat = "png"
	FormatJPEG ScreenshotFormat = "jpeg"
	FormatWebP ScreenshotFormat = "webp"
)

var formatMimeMap = map[ScreenshotFormat]string{
	FormatPNG:  "image/png",
	FormatJPEG: "image/jpeg",
	FormatWebP: "image/webp",
}

var formatExtMap = map[ScreenshotFormat]string{
	FormatPNG:  ".png",
	FormatJPEG: ".jpg",
	FormatWebP: ".webp",
}

func (f ScreenshotFormat) MIME() string {
	if mime, ok := formatMimeMap[f]; ok {
		return mime
	}
	return "application/octet-stream"
}

func (f ScreenshotFormat) Ext() string {
	if ext, ok := formatExtMap[f]; ok {
		return ext
	}
	return ".bin"
}

func (f ScreenshotFormat) IsValid() bool {
	switch f {
	case FormatPNG, FormatJPEG, FormatWebP:
		return true
	}
	return false
}
