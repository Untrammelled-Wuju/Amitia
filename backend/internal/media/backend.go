package media

import (
	"context"

	"github.com/u-ai/backend/internal/media/conversion"
	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/media/metadata"
)

type Backend interface {
	Capabilities(ctx context.Context) (*ffmpeg.Capabilities, error)

	GetMetadata(ctx context.Context, localPath string, req metadata.MetadataRequest) (*metadata.MediaMetadata, error)

	Convert(ctx context.Context, request conversion.ConvertRequest, plan *conversion.ConversionPlan, inputPath, outputPath string, opts conversion.ConvertOptions) (*conversion.ConversionResult, error)

	CancelAll()
}

type MediaBackendCapabilities struct {
	Available bool

	FFmpegVersion  string `json:"ffmpegVersion,omitempty"`
	FFprobeVersion string `json:"ffprobeVersion,omitempty"`

	Containers  []string `json:"containers,omitempty"`
	VideoCodecs []string `json:"videoCodecs,omitempty"`
	AudioCodecs []string `json:"audioCodecs,omitempty"`

	SupportsScale    bool     `json:"supportsScale"`
	SupportsFPS      bool     `json:"supportsFps"`
	SupportsLoudnorm bool     `json:"supportsLoudnorm"`
	SupportsGIF      bool     `json:"supportsGif"`

	HardwareAcceleration []string `json:"hardwareAcceleration,omitempty"`

	Fingerprint string `json:"fingerprint,omitempty"`
}
