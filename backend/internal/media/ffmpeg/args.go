package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"
)

func BuildVersionArgs() []string {
	return []string{"-version"}
}

func BuildFFprobeArgs(localPath string) []string {
	return []string{
		"-v", "error",
		"-show_streams",
		"-show_format",
		"-print_format", "json",
		localPath,
	}
}

func BuildFullFFprobeArgs(localPath string) []string {
	return []string{
		"-v", "error",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		"-print_format", "json",
		localPath,
	}
}

func BuildBaseFlags() []string {
	return []string{
		"-hide_banner",
		"-nostdin",
		"-loglevel", "error",
	}
}

func BuildProgressFlags() []string {
	return []string{
		"-progress", "pipe:1",
		"-nostats",
	}
}

type ConversionArgs struct {
	InputPath    string
	OutputPath   string
	Container    string
	VideoArgs    *VideoConversionArgs
	AudioArgs    *AudioConversionArgs
	TrimArgs     *TrimArgs
	MetadataMode string
	MetadataSet  map[string]string
	MetadataRemove []string
	StreamMaps   []string
	ExtraOutput  []string
	Overwrite    bool
}

type VideoConversionArgs struct {
	Mode             string
	Codec            string
	QualityMode      string
	Quality          int
	BitRateKbps      int
	MaxBitRateKbps   int
	Width            *int
	Height           *int
	Fit              string
	FrameRateNum     int64
	FrameRateDen     int64
	PixelFormat      string
	NormalizeOrientation bool
	PreserveHDR      bool
	StreamIndex      *int
}

type AudioConversionArgs struct {
	Mode             string
	Codec            string
	BitRateKbps      int
	SampleRate       int
	Channels         int
	ChannelLayout    string
	StreamIndex      *int
	NormalizeLoudness bool
}

type TrimArgs struct {
	StartMS   int64
	EndMS     *int64
	DurationMS *int64
	Precision string
}

func BuildConversionArgs(args ConversionArgs) []string {
	cmd := []string{}

	if args.Overwrite {
		cmd = append(cmd, "-y")
	} else {
		cmd = append(cmd, "-n")
	}

	cmd = append(cmd, BuildBaseFlags()...)

	if args.TrimArgs != nil && args.TrimArgs.Precision == "fast" {
		cmd = append(cmd, "-ss", formatDuration(float64(args.TrimArgs.StartMS)/1000))
		if args.TrimArgs.EndMS != nil {
			cmd = append(cmd, "-to", formatDuration(float64(*args.TrimArgs.EndMS)/1000))
		} else if args.TrimArgs.DurationMS != nil {
			cmd = append(cmd, "-t", formatDuration(float64(*args.TrimArgs.DurationMS)/1000))
		}
	}

	cmd = append(cmd, "-i", args.InputPath)

	if args.TrimArgs != nil && args.TrimArgs.Precision == "precise" {
		cmd = append(cmd, "-ss", formatDuration(float64(args.TrimArgs.StartMS)/1000))
		if args.TrimArgs.EndMS != nil {
			cmd = append(cmd, "-to", formatDuration(float64(*args.TrimArgs.EndMS)/1000))
		} else if args.TrimArgs.DurationMS != nil {
			cmd = append(cmd, "-t", formatDuration(float64(*args.TrimArgs.DurationMS)/1000))
		}
	}

	if len(args.StreamMaps) > 0 {
		for _, m := range args.StreamMaps {
			cmd = append(cmd, "-map", m)
		}
	} else if args.VideoArgs != nil && args.VideoArgs.StreamIndex != nil {
		cmd = append(cmd, "-map", fmt.Sprintf("0:v:%d", *args.VideoArgs.StreamIndex))
		if args.AudioArgs != nil && args.AudioArgs.StreamIndex != nil {
			cmd = append(cmd, "-map", fmt.Sprintf("0:a:%d", *args.AudioArgs.StreamIndex))
		}
	}

	if args.VideoArgs != nil {
		vFilters := []string{}

		switch args.VideoArgs.Mode {
		case "copy":
			cmd = append(cmd, "-c:v", "copy")
		case "drop":
			cmd = append(cmd, "-vn")
		case "transcode", "auto":
			codec := args.VideoArgs.Codec
			if codec == "" || codec == "copy" {
				codec = "libx264"
			}
			cmd = append(cmd, "-c:v", codec)

			if args.VideoArgs.QualityMode != "" || args.VideoArgs.Quality > 0 {
				switch args.VideoArgs.QualityMode {
				case "quality":
					cmd = append(cmd, "-crf", "18")
				case "small":
					cmd = append(cmd, "-crf", "28")
				case "balanced":
					cmd = append(cmd, "-crf", "23")
				case "bitrate":
					if args.VideoArgs.BitRateKbps > 0 {
						cmd = append(cmd, "-b:v", fmt.Sprintf("%dk", args.VideoArgs.BitRateKbps))
					}
				default:
					if args.VideoArgs.Quality > 0 {
						crf := 51 - (args.VideoArgs.Quality * 51 / 100)
						if crf < 0 {
							crf = 0
						}
						if crf > 51 {
							crf = 51
						}
						cmd = append(cmd, "-crf", strconv.Itoa(crf))
					}
				}
			}

			if args.VideoArgs.MaxBitRateKbps > 0 {
				cmd = append(cmd, "-maxrate", fmt.Sprintf("%dk", args.VideoArgs.MaxBitRateKbps))
				cmd = append(cmd, "-bufsize", fmt.Sprintf("%dk", args.VideoArgs.MaxBitRateKbps*2))
			}
		}

		if args.VideoArgs.Width != nil || args.VideoArgs.Height != nil {
			w := "-2"
			h := "-2"
			if args.VideoArgs.Width != nil {
				w = strconv.Itoa(*args.VideoArgs.Width)
			}
			if args.VideoArgs.Height != nil {
				h = strconv.Itoa(*args.VideoArgs.Height)
			}
			if args.VideoArgs.Fit == "contain" {
				vFilters = append(vFilters, fmt.Sprintf("scale=%s:%s:force_original_aspect_ratio=decrease", w, h))
			} else if args.VideoArgs.Fit == "cover" {
				vFilters = append(vFilters, fmt.Sprintf("scale=%s:%s:force_original_aspect_ratio=increase", w, h))
				vFilters = append(vFilters, fmt.Sprintf("crop=%s:%s", w, h))
			} else if args.VideoArgs.Fit == "stretch" {
				vFilters = append(vFilters, fmt.Sprintf("scale=%s:%s", w, h))
			} else {
				vFilters = append(vFilters, fmt.Sprintf("scale=%s:%s:force_original_aspect_ratio=decrease", w, h))
			}
		}

		if args.VideoArgs.FrameRateNum > 0 && args.VideoArgs.FrameRateDen > 0 {
			vFilters = append(vFilters, fmt.Sprintf("fps=%d/%d", args.VideoArgs.FrameRateNum, args.VideoArgs.FrameRateDen))
		}

		if args.VideoArgs.NormalizeOrientation {
			vFilters = append(vFilters, "transpose=1")
		}

		if args.VideoArgs.PixelFormat != "" && args.VideoArgs.PixelFormat != "auto" {
			cmd = append(cmd, "-pix_fmt", args.VideoArgs.PixelFormat)
		} else if args.VideoArgs.Mode != "copy" {
			if !args.VideoArgs.PreserveHDR {
				cmd = append(cmd, "-pix_fmt", "yuv420p")
			}
		}

		if args.VideoArgs.Mode != "copy" {
			cmd = append(cmd, "-movflags", "+faststart")
		}

		if len(vFilters) > 0 {
			cmd = append(cmd, "-vf", strings.Join(vFilters, ","))
		}
	}

	if args.AudioArgs != nil {
		switch args.AudioArgs.Mode {
		case "copy":
			cmd = append(cmd, "-c:a", "copy")
		case "drop":
			cmd = append(cmd, "-an")
		case "transcode", "auto":
			codec := args.AudioArgs.Codec
			if codec == "" || codec == "copy" {
				codec = "aac"
			}
			cmd = append(cmd, "-c:a", codec)

			if args.AudioArgs.BitRateKbps > 0 {
				cmd = append(cmd, "-b:a", fmt.Sprintf("%dk", args.AudioArgs.BitRateKbps))
			}

			if args.AudioArgs.SampleRate > 0 {
				cmd = append(cmd, "-ar", strconv.Itoa(args.AudioArgs.SampleRate))
			}

			if args.AudioArgs.Channels > 0 {
				cmd = append(cmd, "-ac", strconv.Itoa(args.AudioArgs.Channels))
			}

			if args.AudioArgs.ChannelLayout != "" {
				cmd = append(cmd, "-channel_layout", args.AudioArgs.ChannelLayout)
			}

			if args.AudioArgs.NormalizeLoudness {
				cmd = append(cmd, "-af", "loudnorm=I=-16:LRA=11:TP=-1.5")
			}
		}
	}

	switch args.MetadataMode {
	case "strip":
		cmd = append(cmd, "-map_metadata", "-1")
		cmd = append(cmd, "-map_chapters", "-1")
	case "preserve":
	case "safe":
		cmd = append(cmd, "-map_metadata", "0")
	}

	if len(args.MetadataSet) > 0 {
		for k, v := range args.MetadataSet {
			cmd = append(cmd, "-metadata", fmt.Sprintf("%s=%s", k, v))
		}
	}

	if len(args.MetadataRemove) > 0 {
	}

	cmd = append(cmd, BuildProgressFlags()...)

	if args.Container != "" {
		cmd = append(cmd, "-f", args.Container)
	}

	cmd = append(cmd, args.ExtraOutput...)

	cmd = append(cmd, args.OutputPath)

	return cmd
}

func BuildGIFArgs(inputPath, outputPath string, maxWidth, maxFPS int, startMS, durationMS int64) []string {
	cmd := []string{"-y"}
	cmd = append(cmd, BuildBaseFlags()...)

	if startMS > 0 {
		cmd = append(cmd, "-ss", formatDuration(float64(startMS)/1000))
	}
	if durationMS > 0 {
		cmd = append(cmd, "-t", formatDuration(float64(durationMS)/1000))
	}

	cmd = append(cmd, "-i", inputPath)

	fps := 10
	if maxFPS > 0 && maxFPS < fps {
		fps = maxFPS
	}

	scaleW := -1
	scaleH := -1
	if maxWidth > 0 {
		scaleW = maxWidth
		scaleH = -1
	}

	filter := fmt.Sprintf("fps=%d,scale=%d:%d:flags=lanczos", fps, scaleW, scaleH)

	cmd = append(cmd, "-vf", filter, "-loop", "0")
	cmd = append(cmd, outputPath)

	return cmd
}

func BuildThumbnailArgs(inputPath, outputPath string, atMS int64, width, height int) []string {
	cmd := []string{"-y"}
	cmd = append(cmd, BuildBaseFlags()...)

	if atMS > 0 {
		cmd = append(cmd, "-ss", formatDuration(float64(atMS)/1000))
	}

	cmd = append(cmd, "-i", inputPath)
	cmd = append(cmd, "-frames:v", "1")

	w := -1
	h := -1
	if width > 0 {
		w = width
	}
	if height > 0 {
		h = height
	}
	if w > 0 || h > 0 {
		scale := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
		cmd = append(cmd, "-vf", scale)
	}

	cmd = append(cmd, outputPath)
	return cmd
}

func formatDuration(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', 3, 64)
}

func IsNetworkProtocol(input string) bool {
	lower := strings.ToLower(input)
	for _, proto := range []string{"http://", "https://", "tcp://", "udp://", "rtsp://", "rtmp://",
		"ftp://", "sftp://", "srt://", "rist://", "gopher://"} {
		if strings.HasPrefix(lower, proto) {
			return true
		}
	}
	return false
}

func IsAllowedProtocol(input string, allowed []string) bool {
	if len(allowed) == 0 {
		return !IsNetworkProtocol(input)
	}
	lower := strings.ToLower(input)
	for _, proto := range allowed {
		if strings.HasPrefix(lower, proto+"://") || lower == proto {
			return true
		}
	}
	for _, proto := range []string{"http://", "https://", "tcp://", "udp://", "rtsp://", "rtmp://",
		"ftp://", "sftp://", "srt://", "rist://", "gopher://"} {
		if strings.HasPrefix(lower, proto) {
			for _, a := range allowed {
				if a == proto[:len(proto)-3] {
					return true
				}
			}
			return false
		}
	}
	return true
}

func SanitizeInputPath(input string, config Config) error {
	if input == "" {
		return nil
	}
	if config.AllowNetworkProtocols {
		return nil
	}
	if IsNetworkProtocol(input) {
		return NewError(FFMPEG_PROTOCOL_FORBIDDEN, "network protocol not allowed: "+input)
	}
	if !IsAllowedProtocol(input, config.AllowedProtocols) {
		return NewError(FFMPEG_PROTOCOL_FORBIDDEN, "protocol not in allowlist: "+input)
	}
	return nil
}

func ParseContainer(formatName string) string {
	if formatName == "" {
		return "unknown"
	}
	formatLower := strings.ToLower(formatName)

	if strings.Contains(formatLower, "mp4") {
		return "mp4"
	}
	if strings.Contains(formatLower, "matroska") || strings.Contains(formatLower, "mkv") {
		return "mkv"
	}
	if strings.Contains(formatLower, "webm") {
		return "webm"
	}
	if strings.Contains(formatLower, "mov") {
		return "mov"
	}
	if strings.Contains(formatLower, "avi") {
		return "avi"
	}
	if strings.Contains(formatLower, "flv") {
		return "flv"
	}
	if strings.Contains(formatLower, "ts") && !strings.Contains(formatLower, "mp4") {
		return "ts"
	}
	if strings.Contains(formatLower, "mp3") {
		return "mp3"
	}
	if strings.Contains(formatLower, "mpegts") {
		return "ts"
	}
	if strings.Contains(formatLower, "wav") {
		return "wav"
	}
	if strings.Contains(formatLower, "flac") {
		return "flac"
	}
	if strings.Contains(formatLower, "ogg") {
		return "ogg"
	}
	if strings.Contains(formatLower, "opus") {
		return "opus"
	}
	if strings.Contains(formatLower, "m4a") || strings.Contains(formatLower, "ipod") {
		return "m4a"
	}
	if strings.Contains(formatLower, "gif") {
		return "gif"
	}
	if strings.Contains(formatLower, "image2") || strings.Contains(formatLower, "png") || strings.Contains(formatLower, "jpeg") || strings.Contains(formatLower, "webp") {
		return "image"
	}
	if strings.Contains(formatLower, "hls") {
		return "hls"
	}
	if strings.Contains(formatLower, "dash") || strings.Contains(formatLower, "mpd") {
		return "dash"
	}

	return "unknown"
}
