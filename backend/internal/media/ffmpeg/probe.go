package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type StreamSummary struct {
	CodecName string `json:"codec_name,omitempty"`
	CodecType string `json:"codec_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

type ProbeResult struct {
	Valid bool

	FormatNames []string

	DurationMS int64

	Streams []StreamSummary
}

type probeFormat struct {
	Filename   string            `json:"filename"`
	NBStreams  int               `json:"nb_streams"`
	FormatName string            `json:"format_name"`
	Duration   string            `json:"duration"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type probeStream struct {
	CodecName string `json:"codec_name,omitempty"`
	CodecType string `json:"codec_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

type probeOutput struct {
	Format  *probeFormat   `json:"format,omitempty"`
	Streams []probeStream  `json:"streams,omitempty"`
}

func ParseProbeOutput(data []byte) (*ProbeResult, error) {
	if len(data) == 0 {
		return nil, NewError(FFMPEG_INVALID_PROBE_OUTPUT, "empty probe output")
	}

	var out probeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, NewError(FFMPEG_INVALID_PROBE_OUTPUT, fmt.Sprintf("invalid probe JSON: %v", err))
	}

	result := &ProbeResult{
		Valid: true,
	}

	if out.Format != nil {
		result.FormatNames = parseFormatNames(out.Format.FormatName)
		if out.Format.Duration != "" {
			if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
				result.DurationMS = int64(d * 1000)
			}
		}
	}

	for _, s := range out.Streams {
		result.Streams = append(result.Streams, StreamSummary{
			CodecName: s.CodecName,
			CodecType: s.CodecType,
			Width:     s.Width,
			Height:    s.Height,
		})
	}

	return result, nil
}

func ParseVersionOutput(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	text := string(data)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ffmpeg version") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				version := parts[2]
				if idx := strings.IndexFunc(version, func(r rune) bool {
					return (r < '0' || r > '9') && r != '.'
				}); idx > 0 {
					return version[:idx]
				}
				return version
			}
		}
	}
	return ""
}

func parseFormatNames(format string) []string {
	if format == "" {
		return nil
	}
	var names []string
	for _, n := range strings.Split(format, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

func Probe(ctx context.Context, runner *Runner, ffprobePath, localPath string, config Config) (*ProbeResult, error) {
	if localPath == "" {
		return nil, NewError(FFMPEG_BINARY_INVALID, "probe input path is empty")
	}

	if err := SanitizeInputPath(localPath, config); err != nil {
		return nil, err
	}

	args := BuildFFprobeArgs(localPath)
	result, err := runner.RunProcess(ctx, ffprobePath, args)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, NewError(FFMPEG_PROCESS_FAILED, fmt.Sprintf("ffprobe exit code: %d", result.ExitCode))
	}

	return ParseProbeOutput(result.Stdout)
}
