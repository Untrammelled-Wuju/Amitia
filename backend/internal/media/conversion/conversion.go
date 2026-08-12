package conversion

type MediaRational struct {
	Num int64 `json:"num"`
	Den int64 `json:"den"`
}

type ConvertRequest struct {
	SourceURI string `json:"sourceUri"`

	TargetURI string `json:"targetUri,omitempty"`

	Output MediaOutputSpec `json:"output"`

	Video *VideoConversionSpec `json:"video,omitempty"`

	Audio *AudioConversionSpec `json:"audio,omitempty"`

	Trim *MediaTrimSpec `json:"trim,omitempty"`

	Metadata MediaMetadataPolicy `json:"metadata"`

	Overwrite bool `json:"overwrite"`

	Limits *MediaConversionLimits `json:"limits,omitempty"`
}

type MediaOutputSpec struct {
	Container string `json:"container"`

	Preset string `json:"preset,omitempty"`

	Extension string `json:"extension,omitempty"`
}

type VideoConversionSpec struct {
	Mode string `json:"mode"`

	Codec string `json:"codec,omitempty"`

	Quality *VideoQualitySpec `json:"quality,omitempty"`

	Width  *int `json:"width,omitempty"`
	Height *int `json:"height,omitempty"`

	Fit string `json:"fit,omitempty"`

	FrameRate *MediaRational `json:"frameRate,omitempty"`

	PixelFormat string `json:"pixelFormat,omitempty"`

	NormalizeOrientation bool `json:"normalizeOrientation"`

	PreserveHDR bool `json:"preserveHdr"`

	StreamIndex *int `json:"streamIndex,omitempty"`
}

type VideoQualitySpec struct {
	Mode string `json:"mode"`

	Quality int `json:"quality,omitempty"`

	BitRateKbps int `json:"bitRateKbps,omitempty"`

	MaxBitRateKbps int `json:"maxBitRateKbps,omitempty"`
}

type AudioConversionSpec struct {
	Mode string `json:"mode"`

	Codec string `json:"codec,omitempty"`

	BitRateKbps int `json:"bitRateKbps,omitempty"`

	SampleRate int `json:"sampleRate,omitempty"`

	Channels int `json:"channels,omitempty"`

	ChannelLayout string `json:"channelLayout,omitempty"`

	StreamIndex *int `json:"streamIndex,omitempty"`

	NormalizeLoudness bool `json:"normalizeLoudness"`
}

type MediaTrimSpec struct {
	StartMs int64 `json:"startMs"`

	EndMs *int64 `json:"endMs,omitempty"`

	DurationMs *int64 `json:"durationMs,omitempty"`

	Precision string `json:"precision"`
}

type MediaMetadataPolicy struct {
	Mode string `json:"mode"`

	PreserveChapters bool `json:"preserveChapters"`

	PreserveLanguage bool `json:"preserveLanguage"`

	PreserveDisposition bool `json:"preserveDisposition"`

	Set map[string]string `json:"set,omitempty"`

	Remove []string `json:"remove,omitempty"`
}

type MediaConversionLimits struct {
	MaxOutputBytes int64 `json:"maxOutputBytes,omitempty"`

	MaxDurationMs int64 `json:"maxDurationMs,omitempty"`

	MaxWidth int `json:"maxWidth,omitempty"`

	MaxHeight int `json:"maxHeight,omitempty"`

	MaxFPS int `json:"maxFPS,omitempty"`
}

type ConversionResult struct {
	ResourceURI string `json:"resourceUri"`

	MediaKind string `json:"mediaKind"`

	Container string `json:"container"`

	VideoCodec string `json:"videoCodec,omitempty"`

	AudioCodec string `json:"audioCodec,omitempty"`

	DurationMs int64 `json:"durationMs"`

	SizeBytes int64 `json:"sizeBytes"`

	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	ContentHash string `json:"contentHash"`

	Transcoded bool `json:"transcoded"`

	UsedAcceleration string `json:"usedAcceleration,omitempty"`

	MetadataMode string `json:"metadataMode"`

	Warnings []string `json:"warnings,omitempty"`
}

type MediaProgress struct {
	Phase string `json:"phase"`

	ProcessedMs int64 `json:"processedMs"`

	TotalMs int64 `json:"totalMs"`

	Percent float64 `json:"percent"`

	OutputBytes int64 `json:"outputBytes"`

	Speed float64 `json:"speed"`
}

type ConversionPreview struct {
	RequiresTranscode    bool   `json:"requiresTranscode"`
	EstimatedOutputBytes int64  `json:"estimatedOutputBytes"`
	OutputContainer      string `json:"outputContainer"`
	VideoCodec           string `json:"videoCodec"`
	AudioCodec           string `json:"audioCodec"`
	Resolution           string `json:"resolution"`
	Warnings             []string `json:"warnings"`
}

func (p *ConversionPlan) VideoMode() string {
	if p.VideoPlan.Mode != "" {
		return p.VideoPlan.Mode
	}
	return ModeCopy
}

func (p *ConversionPlan) AudioMode() string {
	if p.AudioPlan.Mode != "" {
		return p.AudioPlan.Mode
	}
	return ModeCopy
}

type ConversionPlan struct {
	Backend string

	RequiresTranscode bool

	VideoPlan VideoPlan
	AudioPlan AudioPlan

	TrimRequired bool
	TrimPrecise  bool

	EstimatedOutputBytes int64

	Warnings []string
}

type VideoPlan struct {
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
	StreamIndex      *int
}

type AudioPlan struct {
	Mode             string
	Codec            string
	BitRateKbps      int
	SampleRate       int
	Channels         int
	ChannelLayout    string
	StreamIndex      *int
	NormalizeLoudness bool
}

type ThumbnailPreset struct {
	AtMs int64

	Width int
	Height int

	Format string
}

const (
	ModeCopy     = "copy"
	ModeTranscode = "transcode"
	ModeDrop     = "drop"
	ModeAuto     = "auto"
)

const (
	FitContain  = "contain"
	FitCover    = "cover"
	FitStretch  = "stretch"
)

const (
	PrecisionFast    = "fast"
	PrecisionPrecise = "precise"
)

const (
	MetadataModeStrip    = "strip"
	MetadataModeSafe     = "safe"
	MetadataModePreserve = "preserve"
)

const (
	PresetOriginalQuality = "original_quality"
	PresetBalanced        = "balanced"
	PresetSmallFile       = "small_file"
	PresetAudioOnly       = "audio_only"
	PresetThumbnail       = "thumbnail"
	PresetGIF             = "gif"
)

const (
	QualityModeBalanced = "balanced"
	QualityModeQuality  = "quality"
	QualityModeSmall    = "small"
	QualityModeBitrate  = "bitrate"
)

const (
	PhaseProbing     = "probing"
	PhasePlanning    = "planning"
	PhaseConverting  = "converting"
	PhaseVerifying   = "verifying"
	PhaseCommitting  = "committing"
)
