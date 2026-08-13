package metadata

import "errors"

var ErrInvalidProbeResult = errors.New("invalid probe result")

type rawStream struct {
	Index         int
	Type          string
	Codec         string
	CodecLongName string
	Profile       string
	DurationMs    int64
	BitRate       int64
	Language      string
	Tags          map[string]string
	Disposition   StreamDisposition
	Video         *rawVideoStream
	Audio         *rawAudioStream
	Subtitle      *rawSubtitleStream
}

type StreamDisposition struct {
	Default         bool
	Dub             bool
	Original        bool
	Comment         bool
	Lyrics          bool
	Karaoke         bool
	Forced          bool
	HearingImpaired bool
	VisualImpaired  bool
	Effects         bool
	AttachedPic     bool
	TimedThumbnails bool
}

type rawVideoStream struct {
	Width          int
	Height         int
	PixelFormat    string
	FrameRateNum   int64
	FrameRateDen   int64
	SAR            string
	DAR            string
	ColorSpace     string
	ColorTransfer  string
	ColorPrimaries string
	Rotation       int
	Level          int
	FieldOrder     string
}

type rawAudioStream struct {
	SampleRate    int
	Channels      int
	ChannelLayout string
	SampleFormat  string
}

type rawSubtitleStream struct {
	Codec   string
	Forced  bool
	Default bool
}

type rawChapter struct {
	ID      int64
	StartMs int64
	EndMs   int64
	Title   string
}

type rawFormat struct {
	FormatName     string
	FormatLongName string
	Container      string
	ProbeScore     int
}

type rawProbeData struct {
	Valid       bool
	Format      rawFormat
	DurationMs  int64
	StartTimeMs int64
	BitRate     int64
	SizeBytes   int64
	Streams     []rawStream
	Chapters    []rawChapter
	Tags        map[string]string
	Warnings    []string
}

func FromRawProbeResult(resourceURI string, data rawProbeData, req MetadataRequest) (*MediaMetadata, error) {
	if !data.Valid {
		return nil, ErrInvalidProbeResult
	}

	result := &MediaMetadata{
		ResourceURI: resourceURI,
		SizeBytes:   data.SizeBytes,
		DurationMs:  data.DurationMs,
		StartTimeMs: data.StartTimeMs,
		BitRate:     data.BitRate,
		Tags:        make(map[string]string),
	}

	result.Format = MediaFormatInfo{
		FormatName:     data.Format.FormatName,
		FormatLongName: data.Format.FormatLongName,
		Container:      data.Format.Container,
		ProbeScore:     data.Format.ProbeScore,
	}

	maxStreams := req.EffectiveMaxStreams()
	if req.IncludeStreams && len(data.Streams) > 0 {
		streamCount := len(data.Streams)
		if streamCount > maxStreams {
			streamCount = maxStreams
			result.Warnings = append(result.Warnings, "stream count exceeded max, truncated")
		}
		for i := 0; i < streamCount; i++ {
			s := data.Streams[i]
			streamInfo := convertRawStream(s)
			result.Streams = append(result.Streams, streamInfo)
		}
	}

	maxChapters := req.EffectiveMaxChapters()
	if req.IncludeChapters && len(data.Chapters) > 0 {
		chapCount := len(data.Chapters)
		if chapCount > maxChapters {
			chapCount = maxChapters
			result.Warnings = append(result.Warnings, "chapter count exceeded max, truncated")
		}
		for i := 0; i < chapCount; i++ {
			c := data.Chapters[i]
			chapter := MediaChapterInfo{
				ID:     int(c.ID),
				StartMs: c.StartMs,
				EndMs:   c.EndMs,
				Title:   c.Title,
			}
			result.Chapters = append(result.Chapters, chapter)
		}
	}

	if req.IncludeTags && data.Tags != nil {
		safeTags := sanitizeTags(data.Tags, req.IncludeSensitiveTags)
		for k, v := range safeTags {
			result.Tags[k] = v
		}
	}

	if !req.IncludeTechnical {
		result.Warnings = filterTechnicalWarnings(result.Warnings)
	}

	result.MediaKind = string(DetermineMediaKind(result.Streams))

	return result, nil
}

func convertRawStream(s rawStream) MediaStreamInfo {
	info := MediaStreamInfo{
		Index:         s.Index,
		Type:          s.Type,
		Codec:         s.Codec,
		CodecLongName: s.CodecLongName,
		Profile:       s.Profile,
		DurationMs:    s.DurationMs,
		BitRate:       s.BitRate,
		Language:      s.Language,
	}

	info.Disposition = MediaDisposition{
		Default:         s.Disposition.Default,
		Dub:             s.Disposition.Dub,
		Original:        s.Disposition.Original,
		Comment:         s.Disposition.Comment,
		Lyrics:          s.Disposition.Lyrics,
		Karaoke:         s.Disposition.Karaoke,
		Forced:          s.Disposition.Forced,
		HearingImpaired: s.Disposition.HearingImpaired,
		VisualImpaired:  s.Disposition.VisualImpaired,
		Effects:         s.Disposition.Effects,
		AttachedPic:     s.Disposition.AttachedPic,
		TimedThumbnails: s.Disposition.TimedThumbnails,
	}

	if s.Tags != nil {
		info.Tags = make(map[string]string)
		for k, v := range s.Tags {
			info.Tags[k] = v
		}
	}

	if s.Video != nil {
		info.Video = &MediaVideoStreamInfo{
			Width:           s.Video.Width,
			Height:          s.Video.Height,
			PixelFormat:     s.Video.PixelFormat,
			FrameRateNum:    s.Video.FrameRateNum,
			FrameRateDen:    s.Video.FrameRateDen,
			SAR:             s.Video.SAR,
			DAR:             s.Video.DAR,
			ColorSpace:      s.Video.ColorSpace,
			ColorTransfer:   s.Video.ColorTransfer,
			ColorPrimaries:  s.Video.ColorPrimaries,
			RotationDegrees: s.Video.Rotation,
		}

		if s.Video.Width > 0 && s.Video.Rotation != 0 && (s.Video.Rotation == 90 || s.Video.Rotation == 270) {
			info.Video.DisplayWidth = s.Video.Height
			info.Video.DisplayHeight = s.Video.Width
		} else {
			info.Video.DisplayWidth = s.Video.Width
			info.Video.DisplayHeight = s.Video.Height
		}
	}

	if s.Audio != nil {
		info.Audio = &MediaAudioStreamInfo{
			SampleRate:    s.Audio.SampleRate,
			Channels:      s.Audio.Channels,
			ChannelLayout: s.Audio.ChannelLayout,
			SampleFormat:  s.Audio.SampleFormat,
		}
	}

	if s.Subtitle != nil {
		info.Subtitle = &MediaSubtitleStreamInfo{
			Codec:   s.Subtitle.Codec,
			Forced:  s.Subtitle.Forced,
			Default: s.Subtitle.Default,
		}
	}

	return info
}

var sensitiveTagKeys = []string{
	"location", "gps", "com.apple.quicktime.location.ISO6709",
	"com.android.capture.fps", "make", "model", "creation_time",
	"handler_name", "encoder", "vendor_id",
}

func sanitizeTags(tags map[string]string, includeSensitive bool) map[string]string {
	if tags == nil {
		return nil
	}

	result := make(map[string]string)
	for k, v := range tags {
		if isSensitiveTag(k) && !includeSensitive {
			continue
		}
		if len(v) > 4096 {
			v = v[:4096]
		}
		result[k] = v
	}
	return result
}

func isSensitiveTag(key string) bool {
	lower := toLower(key)
	for _, s := range sensitiveTagKeys {
		if lower == s || contains(lower, s) {
			return true
		}
	}
	return false
}

func filterTechnicalWarnings(warnings []string) []string {
	var result []string
	for _, w := range warnings {
		if !contains(w, "probe") && !contains(w, "format") {
			result = append(result, w)
		}
	}
	return result
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}
