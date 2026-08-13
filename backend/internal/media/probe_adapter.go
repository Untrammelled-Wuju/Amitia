package media

import (
	"fmt"

	"github.com/u-ai/backend/internal/media/ffmpeg"
	"github.com/u-ai/backend/internal/media/metadata"
)

func adaptProbeResult(resourceURI string, full *ffmpeg.FullProbeResult, req metadata.MetadataRequest) (*metadata.MediaMetadata, error) {
	if !full.Valid {
		return nil, ffmpeg.NewError(ffmpeg.MEDIA_METADATA_INVALID, "invalid probe result")
	}

	result := &metadata.MediaMetadata{
		ResourceURI: resourceURI,
		SizeBytes:   full.SizeBytes,
		DurationMs:  full.DurationMS,
		StartTimeMs: full.StartTimeMS,
		BitRate:     full.BitRate,
		Tags:        make(map[string]string),
	}

	result.Format = metadata.MediaFormatInfo{
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
			streamInfo := adaptStream(s)
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
		chapter := metadata.MediaChapterInfo{
			ID:     int(c.ID),
			StartMs: c.StartMS,
			EndMs:   c.EndMS,
				Title:   c.Title,
			}
			result.Chapters = append(result.Chapters, chapter)
		}
	}

	if req.IncludeTags && full.Tags != nil {
		for k, v := range full.Tags {
			result.Tags[k] = v
		}
	}

	result.MediaKind = string(metadata.DetermineMediaKind(result.Streams))

	return result, nil
}

func adaptStream(s ffmpeg.FullStreamInfo) metadata.MediaStreamInfo {
	info := metadata.MediaStreamInfo{
		Index:         s.Index,
		Type:          s.Type,
		Codec:         s.Codec,
		CodecLongName: s.CodecLongName,
		Profile:       s.Profile,
		DurationMs:    s.DurationMS,
		BitRate:       s.BitRate,
		Language:      s.Language,
	}

	info.Disposition = metadata.MediaDisposition{
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
		info.Video = &metadata.MediaVideoStreamInfo{
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
		info.Audio = &metadata.MediaAudioStreamInfo{
			SampleRate:    s.Audio.SampleRate,
			Channels:      s.Audio.Channels,
			ChannelLayout: s.Audio.ChannelLayout,
			SampleFormat:  s.Audio.SampleFormat,
		}
	}

	if s.Subtitle != nil {
		info.Subtitle = &metadata.MediaSubtitleStreamInfo{
			Codec:   s.Subtitle.Codec,
			Forced:  s.Subtitle.Forced,
			Default: s.Subtitle.Default,
		}
	}

	return info
}

func safeFirstFormatName(names []string) string {
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

func formatDuration(seconds float64) string {
	return fmt.Sprintf("%.3f", seconds)
}
