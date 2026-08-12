package metadata

import (
	"github.com/u-ai/backend/internal/media/ffmpeg"
)

func FromFFProbeResult(resourceURI string, full *ffmpeg.FullProbeResult, req MetadataRequest) (*MediaMetadata, error) {
	if !full.Valid {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_METADATA_INVALID, "invalid probe result")
	}

	result := &MediaMetadata{
		ResourceURI:  resourceURI,
		SizeBytes:    full.SizeBytes,
		DurationMs:   full.DurationMS,
		StartTimeMS:  full.StartTimeMS,
		BitRate:      full.BitRate,
		Tags:         make(map[string]string),
		ContentHash:  "",
	}

	result.Format = MediaFormatInfo{
		FormatName:     safeFirstFormatName(full.FormatNames),
		FormatLongName: full.FormatLong,
		Container:      ffmpeg.ParseContainer(full.FormatLong),
		ProbeScore:     full.FormatScore,
	}

	maxStreams := req.EffectiveMaxStreams()
	if req.IncludeStreams && len(full.Streams) > 0 {
		streamCount := len(full.Streams)
		if streamCount > maxStreams {
			streamCount = maxStreams
			result.Warnings = append(result.Warnings, "stream count exceeded max, truncated")
		}
		for i := 0; i < streamCount; i++ {
			s := full.Streams[i]
			streamInfo := convertStreamInfo(s)
			result.Streams = append(result.Streams, streamInfo)
		}
	}

	maxChapters := req.EffectiveMaxChapters()
	if req.IncludeChapters && len(full.Chapters) > 0 {
		chapCount := len(full.Chapters)
		if chapCount > maxChapters {
			chapCount = maxChapters
			result.Warnings = append(result.Warnings, "chapter count exceeded max, truncated")
		}
		for i := 0; i < chapCount; i++ {
			c := full.Chapters[i]
			chapter := MediaChapterInfo{
				ID:     int(c.ID),
				StartMS: c.StartMS,
				EndMS:   c.EndMS,
				Title:   c.Title,
			}
			result.Chapters = append(result.Chapters, chapter)
		}
	}

	if req.IncludeTags && full.Tags != nil {
		safeTags := sanitizeTags(full.Tags, req.IncludeSensitiveTags)
		for k, v := range safeTags {
			result.Tags[k] = v
		}
	}

	if !req.IncludeTechnical {
		result.Warnings = filterTechnicalWarnings(result.Warnings)
	}

	result.MediaKind = DetermineMediaKind(result.Streams)

	return result, nil
}

func safeFirstFormatName(names []string) string {
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func convertStreamInfo(s ffmpeg.FullStreamInfo) MediaStreamInfo {
	info := MediaStreamInfo{
		Index:         s.Index,
		Type:          s.Type,
		Codec:         s.Codec,
		CodecLongName: s.CodecLongName,
		Profile:       s.Profile,
		DurationMS:    s.DurationMS,
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
			Width:          s.Video.Width,
			Height:         s.Video.Height,
			PixelFormat:    s.Video.PixelFormat,
			FrameRateNum:   s.Video.FrameRateNum,
			FrameRateDen:   s.Video.FrameRateDen,
			SAR:            s.Video.SAR,
			DAR:            s.Video.DAR,
			ColorSpace:     s.Video.ColorSpace,
			ColorTransfer:  s.Video.ColorTransfer,
			ColorPrimaries: s.Video.ColorPrimaries,
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
