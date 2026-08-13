package conversion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/media/metadata"
)

type Converter struct {
	backend    *ffmpeg.Runner
	ffmpegPath string
	ffprobePath string
	config     ffmpeg.Config
	tempDir    string
	mu         sync.Mutex
}

func NewConverter(backend *ffmpeg.Runner, ffmpegPath, ffprobePath string, config ffmpeg.Config, tempDir string) *Converter {
	return &Converter{
		backend:     backend,
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		config:      config,
		tempDir:     tempDir,
	}
}

type ConvertOptions struct {
	OnProgress func(MediaProgress)
}

func (c *Converter) Convert(ctx context.Context, source metadata.MediaMetadata, request ConvertRequest, plan *ConversionPlan, inputPath, outputPath string, opts ConvertOptions) (*ConversionResult, error) {
	conversionArgs := c.buildFFmpegArgs(source, request, plan, inputPath, outputPath)

	progress := &progressTracker{
		totalMs: source.DurationMs,
		onUpdate: opts.OnProgress,
	}

	stderrLines := []string{}
	result, err := c.backend.RunProcessWithOptions(ctx, c.ffmpegPath, conversionArgs, ffmpeg.RunOptions{
		OnStdoutLine: progress.parseProgressLine,
		OnStderrLine: func(line string) {
			if len(stderrLines) < 1000 {
				stderrLines = append(stderrLines, sanitizeStderrLine(line))
			}
		},
	})

	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_CONVERSION_FAILED,
			fmt.Sprintf("ffmpeg conversion failed (exit code %d): %s", result.ExitCode, safeLastLines(stderrLines, 5)))
	}

	verifyResult, err := c.verifyOutput(outputPath, request, plan)
	if err != nil {
		_ = os.Remove(outputPath)
		return nil, err
	}

	return verifyResult, nil
}

func (c *Converter) buildFFmpegArgs(source metadata.MediaMetadata, request ConvertRequest, plan *ConversionPlan, inputPath, outputPath string) []string {
	trimArgs := (*ffmpeg.TrimArgs)(nil)
	if request.Trim != nil {
		t := &ffmpeg.TrimArgs{
			StartMS:    request.Trim.StartMs,
			EndMS:      request.Trim.EndMs,
			DurationMS: request.Trim.DurationMs,
			Precision:  request.Trim.Precision,
		}
		trimArgs = t
	}

	videoArgs := (*ffmpeg.VideoConversionArgs)(nil)
	if request.Video != nil {
		v := &ffmpeg.VideoConversionArgs{
			Mode:        resolveVideoMode(plan.VideoMode()),
			Codec:       plan.VideoPlan.Codec,
			QualityMode: plan.VideoPlan.QualityMode,
			Quality:     plan.VideoPlan.Quality,
			BitRateKbps: plan.VideoPlan.BitRateKbps,
			MaxBitRateKbps: plan.VideoPlan.MaxBitRateKbps,
			Width:       plan.VideoPlan.Width,
			Height:      plan.VideoPlan.Height,
			Fit:         plan.VideoPlan.Fit,
			FrameRateNum: plan.VideoPlan.FrameRateNum,
			FrameRateDen: plan.VideoPlan.FrameRateDen,
			PixelFormat: plan.VideoPlan.PixelFormat,
			NormalizeOrientation: plan.VideoPlan.NormalizeOrientation,
			StreamIndex: plan.VideoPlan.StreamIndex,
		}
		videoArgs = v
	}

	audioArgs := (*ffmpeg.AudioConversionArgs)(nil)
	if request.Audio != nil {
		a := &ffmpeg.AudioConversionArgs{
			Mode:             resolveAudioMode(plan.AudioMode()),
			Codec:            plan.AudioPlan.Codec,
			BitRateKbps:      plan.AudioPlan.BitRateKbps,
			SampleRate:       plan.AudioPlan.SampleRate,
			Channels:         plan.AudioPlan.Channels,
			ChannelLayout:    plan.AudioPlan.ChannelLayout,
			StreamIndex:      plan.AudioPlan.StreamIndex,
			NormalizeLoudness: plan.AudioPlan.NormalizeLoudness,
		}
		audioArgs = a
	}

	metadataMode := request.Metadata.Mode
	if metadataMode == "" {
		metadataMode = MetadataModeSafe
	}

	return ffmpeg.BuildConversionArgs(ffmpeg.ConversionArgs{
		InputPath:    inputPath,
		OutputPath:   outputPath,
		Container:    request.Output.Container,
		VideoArgs:    videoArgs,
		AudioArgs:    audioArgs,
		TrimArgs:     trimArgs,
		MetadataMode: metadataMode,
		MetadataSet:  request.Metadata.Set,
		Overwrite:    true,
	})
}

func (c *Converter) verifyOutput(outputPath string, request ConvertRequest, plan *ConversionPlan) (*ConversionResult, error) {
	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_OUTPUT_VERIFY_FAILED, "output file not found after conversion")
	}

	if info.Size() == 0 {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_OUTPUT_VERIFY_FAILED, "output file is empty")
	}

	hash, err := computeFileHash(outputPath)
	if err != nil {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_OUTPUT_VERIFY_FAILED, "failed to hash output: "+err.Error())
	}

	fullProbe, err := ffmpeg.ProbeFull(context.Background(), c.backend, c.ffprobePath, outputPath, c.config)
	if err != nil {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_OUTPUT_VERIFY_FAILED, "failed to probe output: "+err.Error())
	}

	result := &ConversionResult{
		MediaKind:     string(metadata.DetermineMediaKind(convertStreams(fullProbe.Streams))),
		Container:     ffmpeg.ParseContainer(fullProbe.FormatLong),
		DurationMs:    fullProbe.DurationMS,
		SizeBytes:      info.Size(),
		ContentHash:   hash,
		Transcoded:    plan.RequiresTranscode,
		MetadataMode:  request.Metadata.Mode,
	}

	for _, s := range fullProbe.Streams {
		if s.Type == "video" && s.Video != nil {
			result.VideoCodec = s.Codec
			result.Width = s.Video.Width
			result.Height = s.Video.Height
		} else if s.Type == "audio" {
			result.AudioCodec = s.Codec
		}
	}

	return result, nil
}

func (c *Converter) createStagingPath(request ConvertRequest) (string, error) {
	ext := request.Output.Extension
	if ext == "" {
		ext = request.Output.Container
	}

	suffix := strconv.FormatInt(int64(os.Getpid()), 10)
	staging := filepath.Join(c.tempDir, fmt.Sprintf("media_convert_%s.%s", suffix, ext))

	if err := os.MkdirAll(filepath.Dir(staging), 0o755); err != nil {
		return "", ffmpeg.NewError(ffmpeg.MEDIA_RESOURCE_MATERIALIZE_FAILED, "failed to create staging directory")
	}

	return staging, nil
}

type progressTracker struct {
	totalMs  int64
	onUpdate func(MediaProgress)
	mu       sync.Mutex
}

func (p *progressTracker) parseProgressLine(line string) {
	if p.onUpdate == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !strings.HasPrefix(line, "out_time_ms=") {
		return
	}

	timeStr := strings.TrimPrefix(line, "out_time_ms=")
	timeStr = strings.TrimSpace(timeStr)

	processedMs, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil || processedMs < 0 {
		return
	}

	processedMs /= 1000

	progress := MediaProgress{
		Phase:       PhaseConverting,
		ProcessedMs: processedMs,
		TotalMs:     p.totalMs,
	}

	if p.totalMs > 0 {
		progress.Percent = float64(processedMs) / float64(p.totalMs) * 100
		if progress.Percent > 100 {
			progress.Percent = 100
		}
	}

	p.onUpdate(progress)
}

func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func sanitizeStderrLine(line string) string {
	if len(line) > 512 {
		line = line[:512]
	}
	line = strings.ReplaceAll(line, "\x00", "")
	if idx := strings.Index(line, "/"); idx > 0 {
		line = line[idx:]
	}
	return line
}

func safeLastLines(lines []string, n int) string {
	if len(lines) == 0 {
		return "no stderr output"
	}
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	return strings.Join(lines[start:], "; ")
}

func convertStreams(streams []ffmpeg.FullStreamInfo) []metadata.MediaStreamInfo {
	result := make([]metadata.MediaStreamInfo, 0, len(streams))
	for _, s := range streams {
		info := metadata.MediaStreamInfo{
			Index:    s.Index,
			Type:     s.Type,
			Codec:    s.Codec,
			Language: s.Language,
		}
		if s.Video != nil {
			info.Video = &metadata.MediaVideoStreamInfo{
				Width:  s.Video.Width,
				Height: s.Video.Height,
			}
		}
		if s.Audio != nil {
			info.Audio = &metadata.MediaAudioStreamInfo{
				SampleRate: s.Audio.SampleRate,
				Channels:   s.Audio.Channels,
			}
		}
		result = append(result, info)
	}
	return result
}

func resolveVideoMode(mode string) string {
	switch mode {
	case ModeCopy, ModeTranscode, ModeDrop, ModeAuto:
		return mode
	default:
		return ModeTranscode
	}
}

func resolveAudioMode(mode string) string {
	switch mode {
	case ModeCopy, ModeTranscode, ModeDrop, ModeAuto:
		return mode
	default:
		return ModeTranscode
	}
}
