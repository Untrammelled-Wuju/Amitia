package media

import (
	"context"
	"fmt"

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

type FFmpegBackend struct {
	inner  ffmpeg.Backend
	runner *ffmpeg.Runner
	config ffmpeg.Config
}

func NewFFmpegBackend(inner ffmpeg.Backend, runner *ffmpeg.Runner, config ffmpeg.Config) *FFmpegBackend {
	return &FFmpegBackend{
		inner:  inner,
		runner: runner,
		config: config,
	}
}

func (b *FFmpegBackend) Capabilities(ctx context.Context) (*ffmpeg.Capabilities, error) {
	return b.inner.GetCapabilities(ctx)
}

func (b *FFmpegBackend) GetMetadata(ctx context.Context, localPath string, req metadata.MetadataRequest) (*metadata.MediaMetadata, error) {
	if localPath == "" {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_METADATA_INVALID, "local path is empty")
	}

	if err := ffmpeg.SanitizeInputPath(localPath, b.config); err != nil {
		return nil, err
	}

	probeResult, err := b.inner.ProbeFull(ctx, localPath)
	if err != nil {
		if mediaErr, ok := err.(*ffmpeg.Error); ok {
			return nil, mediaErr
		}
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_METADATA_FAILED, fmt.Sprintf("probe failed: %v", err))
	}

	result, err := adaptProbeResult(req.SourceURI, probeResult, req)
	if err != nil {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_METADATA_INVALID, fmt.Sprintf("parse failed: %v", err))
	}

	return result, nil
}

func (b *FFmpegBackend) Convert(ctx context.Context, request conversion.ConvertRequest, plan *conversion.ConversionPlan, inputPath, outputPath string, opts conversion.ConvertOptions) (*conversion.ConversionResult, error) {
	env, err := b.inner.CheckVersion(ctx)
	if err != nil {
		return nil, ffmpeg.NewError(ffmpeg.FFMPEG_UNAVAILABLE, fmt.Sprintf("cannot get ffmpeg environment: %v", err))
	}
	if !env.Available || env.FFmpegPath == "" {
		return nil, ffmpeg.NewError(ffmpeg.FFMPEG_UNAVAILABLE, "ffmpeg not available")
	}

	if err := ffmpeg.SanitizeInputPath(inputPath, b.config); err != nil {
		return nil, err
	}

	converter := conversion.NewConverter(b.runner, env.FFmpegPath, env.FFprobePath, b.config, "")
	return converter.Convert(ctx, metadata.MediaMetadata{}, request, plan, inputPath, outputPath, opts)
}

func (b *FFmpegBackend) CancelAll() {
	b.inner.CancelAll()
}
