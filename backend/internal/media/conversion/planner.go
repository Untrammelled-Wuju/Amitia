package conversion

import (
	"fmt"

	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/media/metadata"
)

type Planner struct {
	capabilities *ffmpeg.Capabilities
}

func NewPlanner(capabilities *ffmpeg.Capabilities) *Planner {
	return &Planner{capabilities: capabilities}
}

func (p *Planner) Plan(source *metadata.MediaMetadata, request ConvertRequest) (*ConversionPlan, error) {
	plan := &ConversionPlan{
		Backend:   "ffmpeg",
		Warnings:  []string{},
	}

	if request.Video != nil {
		videoPlan, err := p.planVideo(source, request)
		if err != nil {
			return nil, err
		}
		plan.VideoPlan = videoPlan
		if videoPlan.Mode == ModeTranscode {
			plan.RequiresTranscode = true
		}
	} else {
		plan.VideoPlan = VideoPlan{Mode: ModeCopy}
	}

	if request.Audio != nil {
		audioPlan, err := p.planAudio(source, request)
		if err != nil {
			return nil, err
		}
		plan.AudioPlan = audioPlan
		if audioPlan.Mode == ModeTranscode {
			plan.RequiresTranscode = true
		}
	} else {
		plan.AudioPlan = AudioPlan{Mode: ModeCopy}
	}

	if request.Trim != nil {
		plan.TrimRequired = true
		if request.Trim.Precision == PrecisionPrecise {
			plan.TrimPrecise = true
			plan.RequiresTranscode = true
		}
	}

	if request.Video != nil && request.Video.Mode == ModeCopy && plan.RequiresTranscode {
		if !plan.TrimPrecise && request.Trim == nil {
			return nil, ffmpeg.NewError(ffmpeg.MEDIA_VIDEO_TRANSCODE_REQUIRED,
				"video Mode=copy conflicts with required transcoding")
		}
	}

	if err := p.validateContainerCompatibility(request); err != nil {
		return nil, err
	}

	if request.Limits != nil {
		if err := p.validateLimits(source, request); err != nil {
			return nil, err
		}
	}

	plan.EstimatedOutputBytes = p.estimateOutputSize(source, plan)

	return plan, nil
}

func (p *Planner) planVideo(source *metadata.MediaMetadata, request ConvertRequest) (VideoPlan, error) {
	v := request.Video
	plan := VideoPlan{
		Mode:        v.Mode,
		Codec:       v.Codec,
		Width:       v.Width,
		Height:      v.Height,
		Fit:         v.Fit,
		PixelFormat: v.PixelFormat,
		StreamIndex: v.StreamIndex,
	}

	if v.Quality != nil {
		plan.QualityMode = v.Quality.Mode
		plan.Quality = v.Quality.Quality
		plan.BitRateKbps = v.Quality.BitRateKbps
		plan.MaxBitRateKbps = v.Quality.MaxBitRateKbps
	}

	if v.FrameRate != nil && v.FrameRate.Num > 0 {
		plan.FrameRateNum = v.FrameRate.Num
		plan.FrameRateDen = v.FrameRate.Den
	}

	plan.NormalizeOrientation = v.NormalizeOrientation

	sourceVideoStream := findVideoStream(source, v.StreamIndex)
	if plan.Mode == ModeAuto {
		if sourceVideoStream == nil {
			plan.Mode = ModeDrop
		} else if p.canCopyVideo(sourceVideoStream, request) {
			plan.Mode = ModeCopy
		} else {
			plan.Mode = ModeTranscode
		}
	}

	if plan.Mode == ModeTranscode && plan.Codec == "" {
		plan.Codec = "libx264"
	}

	if plan.Mode != ModeCopy && plan.Mode != ModeDrop {
		if plan.Width != nil || plan.Height != nil {
			plan.Fit = normalizeFit(plan.Fit)
		}
	}

	return plan, nil
}

func (p *Planner) planAudio(source *metadata.MediaMetadata, request ConvertRequest) (AudioPlan, error) {
	a := request.Audio
	plan := AudioPlan{
		Mode:             a.Mode,
		Codec:            a.Codec,
		BitRateKbps:      a.BitRateKbps,
		SampleRate:       a.SampleRate,
		Channels:         a.Channels,
		ChannelLayout:    a.ChannelLayout,
		StreamIndex:      a.StreamIndex,
		NormalizeLoudness: a.NormalizeLoudness,
	}

	sourceAudioStream := findAudioStream(source, a.StreamIndex)
	if plan.Mode == ModeAuto {
		if sourceAudioStream == nil {
			plan.Mode = ModeDrop
		} else if p.canCopyAudio(sourceAudioStream, request) {
			plan.Mode = ModeCopy
		} else {
			plan.Mode = ModeTranscode
		}
	}

	if plan.Mode == ModeTranscode && plan.Codec == "" {
		plan.Codec = "aac"
	}

	return plan, nil
}

func (p *Planner) canCopyVideo(stream *metadata.MediaStreamInfo, request ConvertRequest) bool {
	if request.Output.Container == "" {
		return false
	}
	codec := stream.Codec
	return ffmpeg.DetectCodecContainerCompatibility(codec, request.Output.Container)
}

func (p *Planner) canCopyAudio(stream *metadata.MediaStreamInfo, request ConvertRequest) bool {
	if request.Output.Container == "" {
		return false
	}
	codec := stream.Codec
	return ffmpeg.DetectCodecContainerCompatibility(codec, request.Output.Container)
}

func (p *Planner) validateContainerCompatibility(request ConvertRequest) error {
	container := request.Output.Container
	if container == "" {
		return ffmpeg.NewError(ffmpeg.MEDIA_CONTAINER_UNSUPPORTED, "output container not specified")
	}

	supportedContainers := map[string]bool{
		"mp4": true, "mov": true, "mkv": true, "webm": true,
		"mp3": true, "m4a": true, "wav": true, "flac": true,
		"ogg": true, "opus": true, "gif": true,
	}

	if !supportedContainers[container] {
		return ffmpeg.NewError(ffmpeg.MEDIA_CONTAINER_UNSUPPORTED, "unsupported container: "+container)
	}

	return nil
}

func (p *Planner) validateLimits(source *metadata.MediaMetadata, request ConvertRequest) error {
	limits := request.Limits

	if limits.MaxWidth > 0 && request.Video != nil && request.Video.Width != nil {
		if *request.Video.Width > limits.MaxWidth {
			return ffmpeg.NewError(ffmpeg.MEDIA_RESOLUTION_TOO_LARGE,
				fmt.Sprintf("width %d exceeds limit %d", *request.Video.Width, limits.MaxWidth))
		}
	}

	if limits.MaxHeight > 0 && request.Video != nil && request.Video.Height != nil {
		if *request.Video.Height > limits.MaxHeight {
			return ffmpeg.NewError(ffmpeg.MEDIA_RESOLUTION_TOO_LARGE,
				fmt.Sprintf("height %d exceeds limit %d", *request.Video.Height, limits.MaxHeight))
		}
	}

	if limits.MaxFPS > 0 && request.Video != nil && request.Video.FrameRate != nil {
		fps := float64(request.Video.FrameRate.Num) / float64(request.Video.FrameRate.Den)
		if fps > float64(limits.MaxFPS) {
			return ffmpeg.NewError(ffmpeg.MEDIA_FRAME_RATE_INVALID,
				fmt.Sprintf("fps %.2f exceeds limit %d", fps, limits.MaxFPS))
		}
	}

	if limits.MaxDurationMs > 0 && source.DurationMs > limits.MaxDurationMs {
		return ffmpeg.NewError(ffmpeg.MEDIA_TRIM_INVALID,
			fmt.Sprintf("source duration %dms exceeds limit %dms", source.DurationMs, limits.MaxDurationMs))
	}

	return nil
}

func (p *Planner) estimateOutputSize(source *metadata.MediaMetadata, plan *ConversionPlan) int64 {
	if !plan.RequiresTranscode && source.SizeBytes > 0 {
		return source.SizeBytes
	}
	return source.SizeBytes / 2
}

func findVideoStream(metadata *metadata.MediaMetadata, index *int) *metadata.MediaStreamInfo {
	if index != nil {
		for i := range metadata.Streams {
			if metadata.Streams[i].Index == *index && metadata.Streams[i].Type == "video" {
				return &metadata.Streams[i]
			}
		}
		return nil
	}
	for i := range metadata.Streams {
		if metadata.Streams[i].Type == "video" {
			return &metadata.Streams[i]
		}
	}
	return nil
}

func findAudioStream(metadata *metadata.MediaMetadata, index *int) *metadata.MediaStreamInfo {
	if index != nil {
		for i := range metadata.Streams {
			if metadata.Streams[i].Index == *index && metadata.Streams[i].Type == "audio" {
				return &metadata.Streams[i]
			}
		}
		return nil
	}
	for i := range metadata.Streams {
		if metadata.Streams[i].Type == "audio" {
			return &metadata.Streams[i]
		}
	}
	return nil
}

func normalizeFit(fit string) string {
	switch fit {
	case FitContain, FitCover, FitStretch:
		return fit
	default:
		return FitContain
	}
}
