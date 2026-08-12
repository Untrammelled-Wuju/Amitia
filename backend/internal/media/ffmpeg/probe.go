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
	Size       string            `json:"size"`
	BitRate    string            `json:"bit_rate"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type probeStream struct {
	Index          int               `json:"index"`
	CodecName      string            `json:"codec_name,omitempty"`
	CodecLongName  string            `json:"codec_long_name,omitempty"`
	CodecType      string            `json:"codec_type,omitempty"`
	Profile        string            `json:"profile,omitempty"`
	Width          int               `json:"width,omitempty"`
	Height         int               `json:"height,omitempty"`
	PixFmt         string            `json:"pix_fmt,omitempty"`
	Level          int               `json:"level,omitempty"`
	FieldOrder     string            `json:"field_order,omitempty"`
	FrameRate      string            `json:"r_frame_rate,omitempty"`
	AvgFrameRate   string            `json:"avg_frame_rate,omitempty"`
	SampleRate     string            `json:"sample_rate,omitempty"`
	Channels       int               `json:"channels,omitempty"`
	ChannelLayout  string            `json:"channel_layout,omitempty"`
	SampleFmt      string            `json:"sample_fmt,omitempty"`
	Duration       string            `json:"duration,omitempty"`
	BitRate        string            `json:"bit_rate,omitempty"`
	TimeBase       string            `json:"time_base,omitempty"`
	SAR            string            `json:"sample_aspect_ratio,omitempty"`
	DAR            string            `json:"display_aspect_ratio,omitempty"`
	ColorSpace     string            `json:"color_space,omitempty"`
	ColorTransfer  string            `json:"color_transfer,omitempty"`
	ColorPrimaries string            `json:"color_primaries,omitempty"`
	ColorRange     string            `json:"color_range,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Disposition    map[string]int    `json:"disposition,omitempty"`
	SideData       []probeSideData   `json:"side_data_list,omitempty"`
}

type probeSideData struct {
	SideDataType string `json:"side_data_type,omitempty"`
	Rotation     int    `json:"rotation,omitempty"`
}

type probeChapter struct {
	ID    int64             `json:"id"`
	Start string            `json:"start_time"`
	End   string            `json:"end_time"`
	Tags  map[string]string `json:"tags,omitempty"`
}

type probeOutput struct {
	Format   *probeFormat    `json:"format,omitempty"`
	Streams  []probeStream   `json:"streams,omitempty"`
	Chapters []probeChapter  `json:"chapters,omitempty"`
}

type FullProbeResult struct {
	Valid bool

	FormatNames []string
	FormatLong  string

	DurationMS int64
	StartTimeMS int64
	BitRate    int64
	SizeBytes  int64

	Streams  []FullStreamInfo
	Chapters []FullChapterInfo

	Tags map[string]string

	FormatScore int

	Warnings []string
}

type FullStreamInfo struct {
	Index          int
	Type           string
	Codec          string
	CodecLongName  string
	Profile        string
	DurationMS     int64
	BitRate        int64
	Language       string
	Disposition    StreamDisposition
	Tags           map[string]string
	Video          *VideoStreamInfo
	Audio          *AudioStreamInfo
	Subtitle       *SubtitleStreamInfo
}

type StreamDisposition struct {
	Default         bool `json:"default"`
	Dub             bool `json:"dub"`
	Original        bool `json:"original"`
	Comment         bool `json:"comment"`
	Lyrics          bool `json:"lyrics"`
	Karaoke         bool `json:"karaoke"`
	Forced          bool `json:"forced"`
	HearingImpaired bool `json:"hearing_impaired"`
	VisualImpaired  bool `json:"visual_impaired"`
	Effects         bool `json:"effects"`
	AttachedPic     bool `json:"attached_pic"`
	TimedThumbnails bool `json:"timed_thumbnails"`
}

type VideoStreamInfo struct {
	Width        int
	Height       int
	PixelFormat  string
	FrameRateNum int64
	FrameRateDen int64
	SAR          string
	DAR          string
	ColorSpace   string
	ColorTransfer string
	ColorPrimaries string
	Rotation     int
	Level        int
	FieldOrder   string
}

type AudioStreamInfo struct {
	SampleRate    int
	Channels      int
	ChannelLayout string
	SampleFormat  string
}

type SubtitleStreamInfo struct {
	Codec   string
	Forced  bool
	Default bool
}

type FullChapterInfo struct {
	ID    int64
	StartMS int64
	EndMS   int64
	Title   string
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

func ParseFullProbeOutput(data []byte) (*FullProbeResult, error) {
	if len(data) == 0 {
		return nil, NewError(FFMPEG_INVALID_PROBE_OUTPUT, "empty probe output")
	}

	var out probeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, NewError(FFMPEG_INVALID_PROBE_OUTPUT, fmt.Sprintf("invalid probe JSON: %v", err))
	}

	result := &FullProbeResult{
		Valid: true,
	}

	if out.Format != nil {
		result.FormatNames = parseFormatNames(out.Format.FormatName)
		result.FormatLong = out.Format.FormatName
		result.FormatScore = out.Format.NBStreams
		if out.Format.Duration != "" {
			if d, err := strconv.ParseFloat(out.Format.Duration, 64); err == nil {
				result.DurationMS = int64(d * 1000)
			}
		}
		if out.Format.BitRate != "" {
			if br, err := strconv.ParseInt(out.Format.BitRate, 10, 64); err == nil {
				result.BitRate = br
			}
		}
		if out.Format.Size != "" {
			if sz, err := strconv.ParseInt(out.Format.Size, 10, 64); err == nil {
				result.SizeBytes = sz
			}
		}
		if out.Format.Tags != nil {
			result.Tags = make(map[string]string)
			for k, v := range out.Format.Tags {
				result.Tags[k] = v
			}
		}
	}

	for _, s := range out.Streams {
		info := FullStreamInfo{
			Index:         s.Index,
			Type:          s.CodecType,
			Codec:         s.CodecName,
			CodecLongName: s.CodecLongName,
			Profile:       s.Profile,
			Tags:          s.Tags,
		}

		if s.Duration != "" {
			if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
				info.DurationMS = int64(d * 1000)
			}
		}
		if s.BitRate != "" {
			if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
				info.BitRate = br
			}
		}
		if s.Tags != nil {
			if lang, ok := s.Tags["language"]; ok {
				info.Language = lang
			}
		}

		info.Disposition = parseDisposition(s.Disposition)

		switch s.CodecType {
		case "video":
			v := &VideoStreamInfo{
				Width:          s.Width,
				Height:         s.Height,
				PixelFormat:    s.PixFmt,
				SAR:            s.SAR,
				DAR:            s.DAR,
				ColorSpace:     s.ColorSpace,
				ColorTransfer:  s.ColorTransfer,
				ColorPrimaries: s.ColorPrimaries,
				Level:          s.Level,
				FieldOrder:     s.FieldOrder,
			}
			v.FrameRateNum, v.FrameRateDen = parseFrameRate(s.FrameRate)
			for _, sd := range s.SideData {
				if sd.SideDataType == "Display Matrix" || strings.Contains(sd.SideDataType, "Rotation") {
					v.Rotation = sd.Rotation
				}
			}
			if v.Rotation == 0 && s.Tags != nil {
				if rot, ok := s.Tags["rotate"]; ok {
					if r, err := strconv.Atoi(rot); err == nil {
						v.Rotation = r
					}
				}
			}
			info.Video = v
		case "audio":
			a := &AudioStreamInfo{
				Channels:      s.Channels,
				ChannelLayout: s.ChannelLayout,
				SampleFormat:  s.SampleFmt,
			}
			if s.SampleRate != "" {
				if sr, err := strconv.Atoi(s.SampleRate); err == nil {
					a.SampleRate = sr
				}
			}
			info.Audio = a
		case "subtitle":
			info.Subtitle = &SubtitleStreamInfo{
				Codec:   s.CodecName,
				Forced:  info.Disposition.Forced,
				Default: info.Disposition.Default,
			}
		}

		result.Streams = append(result.Streams, info)
	}

	for _, c := range out.Chapters {
		ch := FullChapterInfo{ID: c.ID}
		if c.Start != "" {
			if s, err := strconv.ParseFloat(c.Start, 64); err == nil {
				ch.StartMS = int64(s * 1000)
			}
		}
		if c.End != "" {
			if e, err := strconv.ParseFloat(c.End, 64); err == nil {
				ch.EndMS = int64(e * 1000)
			}
		}
		if c.Tags != nil {
			if title, ok := c.Tags["title"]; ok {
				ch.Title = title
			}
		}
		result.Chapters = append(result.Chapters, ch)
	}

	return result, nil
}

func parseDisposition(d map[string]int) StreamDisposition {
	if d == nil {
		return StreamDisposition{}
	}
	return StreamDisposition{
		Default:         d["default"] != 0,
		Dub:             d["dub"] != 0,
		Original:        d["original"] != 0,
		Comment:         d["comment"] != 0,
		Lyrics:          d["lyrics"] != 0,
		Karaoke:         d["karaoke"] != 0,
		Forced:          d["forced"] != 0,
		HearingImpaired: d["hearing_impaired"] != 0,
		VisualImpaired:  d["visual_impaired"] != 0,
		Effects:         d["effects"] != 0,
		AttachedPic:     d["attached_pic"] != 0,
		TimedThumbnails: d["timed_thumbnails"] != 0,
	}
}

func parseFrameRate(rate string) (num, den int64) {
	if rate == "" || rate == "0/0" {
		return 0, 0
	}
	parts := strings.Split(rate, "/")
	if len(parts) != 2 {
		return 0, 0
	}
	num, _ = strconv.ParseInt(parts[0], 10, 64)
	den, _ = strconv.ParseInt(parts[1], 10, 64)
	return
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

func ProbeFull(ctx context.Context, runner *Runner, ffprobePath, localPath string, config Config) (*FullProbeResult, error) {
	if localPath == "" {
		return nil, NewError(FFMPEG_BINARY_INVALID, "probe input path is empty")
	}

	if err := SanitizeInputPath(localPath, config); err != nil {
		return nil, err
	}

	args := BuildFullFFprobeArgs(localPath)
	result, err := runner.RunProcess(ctx, ffprobePath, args)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, NewError(FFMPEG_PROCESS_FAILED, fmt.Sprintf("ffprobe exit code: %d", result.ExitCode))
	}

	return ParseFullProbeOutput(result.Stdout)
}
