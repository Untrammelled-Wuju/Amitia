package metadata

type MediaKind string

const (
	MediaKindAudio  MediaKind = "audio"
	MediaKindVideo  MediaKind = "video"
	MediaKindImage  MediaKind = "image"
	MediaKindMixed  MediaKind = "mixed"
	MediaKindUnknown MediaKind = "unknown"
)

type MetadataRequest struct {
	SourceURI string `json:"sourceUri"`

	IncludeStreams     bool `json:"includeStreams"`
	IncludeChapters    bool `json:"includeChapters"`

	IncludeTags        bool `json:"includeTags"`
	IncludeTechnical   bool `json:"includeTechnical"`

	IncludeSensitiveTags bool `json:"includeSensitiveTags"`

	MaxStreams  int `json:"maxStreams,omitempty"`
	MaxChapters int `json:"maxChapters,omitempty"`
}

type MediaMetadata struct {
	ResourceURI string `json:"resourceUri"`

	MediaKind string `json:"mediaKind"`

	Format MediaFormatInfo `json:"format"`

	DurationMs  int64 `json:"durationMs,omitempty"`
	StartTimeMs int64 `json:"startTimeMs,omitempty"`

	BitRate   int64 `json:"bitRate,omitempty"`
	SizeBytes int64 `json:"sizeBytes"`

	Streams []MediaStreamInfo `json:"streams"`

	Chapters []MediaChapterInfo `json:"chapters,omitempty"`

	Tags map[string]string `json:"tags,omitempty"`

	ContentHash string `json:"contentHash,omitempty"`

	Warnings []string `json:"warnings,omitempty"`
}

type MediaFormatInfo struct {
	FormatName     string `json:"formatName"`
	FormatLongName string `json:"formatLongName,omitempty"`

	Container string `json:"container,omitempty"`

	ProbeScore int `json:"probeScore,omitempty"`
}

type MediaStreamInfo struct {
	Index int `json:"index"`

	Type string `json:"type"`

	Codec         string `json:"codec"`
	CodecLongName string `json:"codecLongName,omitempty"`

	Profile string `json:"profile,omitempty"`

	DurationMs int64 `json:"durationMs,omitempty"`
	BitRate    int64 `json:"bitRate,omitempty"`

	Language string `json:"language,omitempty"`

	Disposition MediaDisposition `json:"disposition,omitempty"`

	Video    *MediaVideoStreamInfo    `json:"video,omitempty"`
	Audio    *MediaAudioStreamInfo    `json:"audio,omitempty"`
	Subtitle *MediaSubtitleStreamInfo `json:"subtitle,omitempty"`

	Tags map[string]string `json:"tags,omitempty"`
}

type MediaDisposition struct {
	Default         bool `json:"default"`
	Dub             bool `json:"dub"`
	Original        bool `json:"original"`
	Comment         bool `json:"comment"`
	Lyrics          bool `json:"lyrics"`
	Karaoke         bool `json:"karaoke"`
	Forced          bool `json:"forced"`
	HearingImpaired bool `json:"hearingImpaired"`
	VisualImpaired  bool `json:"visualImpaired"`
	Effects         bool `json:"effects"`
	AttachedPic     bool `json:"attachedPic"`
	TimedThumbnails bool `json:"timedThumbnails"`
}

type MediaVideoStreamInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`

	DisplayWidth  int `json:"displayWidth,omitempty"`
	DisplayHeight int `json:"displayHeight,omitempty"`

	PixelFormat string `json:"pixelFormat,omitempty"`

	FrameRateNum int64 `json:"frameRateNum,omitempty"`
	FrameRateDen int64 `json:"frameRateDen,omitempty"`

	SAR string `json:"sar,omitempty"`
	DAR string `json:"dar,omitempty"`

	ColorSpace     string `json:"colorSpace,omitempty"`
	ColorTransfer  string `json:"colorTransfer,omitempty"`
	ColorPrimaries string `json:"colorPrimaries,omitempty"`

	RotationDegrees int `json:"rotationDegrees,omitempty"`
}

type MediaAudioStreamInfo struct {
	SampleRate int `json:"sampleRate,omitempty"`

	Channels int `json:"channels,omitempty"`

	ChannelLayout string `json:"channelLayout,omitempty"`

	SampleFormat string `json:"sampleFormat,omitempty"`
}

type MediaSubtitleStreamInfo struct {
	Codec string `json:"codec"`

	Forced  bool `json:"forced"`
	Default bool `json:"default"`
}

type MediaChapterInfo struct {
	ID int `json:"id"`

	StartMs int64 `json:"startMs"`
	EndMs   int64 `json:"endMs"`

	Title string `json:"title,omitempty"`
}

func (m MetadataRequest) EffectiveMaxStreams() int {
	if m.MaxStreams <= 0 {
		return 128
	}
	return m.MaxStreams
}

func (m MetadataRequest) EffectiveMaxChapters() int {
	if m.MaxChapters <= 0 {
		return 1000
	}
	return m.MaxChapters
}

func DetermineMediaKind(streams []MediaStreamInfo) MediaKind {
	hasVideo := false
	hasAudio := false
	hasImage := false

	for _, s := range streams {
		switch s.Type {
		case "video":
			if s.Subtitle == nil {
				hasVideo = true
			}
		case "audio":
			hasAudio = true
		case "image":
			hasImage = true
		}
	}

	if hasVideo && hasAudio {
		return MediaKindMixed
	}
	if hasVideo {
		if hasImage {
			return MediaKindMixed
		}
		return MediaKindVideo
	}
	if hasAudio {
		return MediaKindAudio
	}
	if hasImage {
		return MediaKindImage
	}
	return MediaKindUnknown
}
