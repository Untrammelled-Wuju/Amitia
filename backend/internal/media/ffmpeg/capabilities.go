package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type CapabilityState struct {
	Supported bool `json:"supported"`

	FFmpegAvailable  bool `json:"ffmpegAvailable"`
	FFprobeAvailable bool `json:"ffprobeAvailable"`

	FFmpegVersion  string `json:"ffmpegVersion,omitempty"`
	FFprobeVersion string `json:"ffprobeVersion,omitempty"`

	RuntimeID string `json:"runtimeId,omitempty"`

	Platform     string `json:"platform,omitempty"`
	Architecture string `json:"architecture,omitempty"`

	Source string `json:"source,omitempty"`

	NetworkProtocolsAllowed bool `json:"networkProtocolsAllowed"`

	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type Capabilities struct {
	Available bool

	Probe      bool
	StreamCopy bool
	Transcode  bool

	VideoEncodeCodecs []string
	AudioEncodeCodecs []string
	SubtitleCodecs    []string

	Containers      []string
	VideoDecodeCodecs []string
	AudioDecodeCodecs []string

	SupportsScale       bool
	SupportsFPS         bool
	SupportsLoudnorm    bool
	SupportsGIF         bool
	SupportsThumbnail   bool
	SupportsHDR         bool

	HardwareAcceleration []string

	Fingerprint string

	cachedAt time.Time
}

type CapabilityIntrospector struct {
	backend    Backend
	mu         sync.Mutex
	cached     *Capabilities
	validUntil time.Duration
}

func NewCapabilityIntrospector(backend Backend) *CapabilityIntrospector {
	return &CapabilityIntrospector{
		backend:    backend,
		validUntil: 5 * time.Minute,
	}
}

func (ci *CapabilityIntrospector) Introspect(ctx context.Context) (*Capabilities, error) {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if ci.cached != nil && time.Since(ci.cached.cachedAt) < ci.validUntil {
		return ci.cached, nil
	}

	state := ci.backend.Capabilities(ctx)
	if !state.Supported || !state.FFmpegAvailable {
		cap := &Capabilities{
			Available:   false,
			Fingerprint: state.RuntimeID,
		}
		ci.cached = cap
		return cap, nil
	}

	cap := &Capabilities{
		Available:           true,
		Probe:               state.FFprobeAvailable,
		StreamCopy:          true,
		Transcode:           true,
		VideoEncodeCodecs:   []string{},
		AudioEncodeCodecs:   []string{},
		Containers:          []string{},
		SupportsScale:       true,
		SupportsFPS:         true,
		SupportsLoudnorm:    false,
		SupportsGIF:         true,
		SupportsThumbnail:   true,
		HardwareAcceleration: []string{},
		Fingerprint:         generateFingerprint(state.FFmpegVersion, state.RuntimeID),
	}

	ci.cached = cap
	return cap, nil
}

func (ci *CapabilityIntrospector) Invalidate() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.cached = nil
}

func DisabledCapabilityState(runtimeID, reason string) CapabilityState {
	return CapabilityState{
		Supported:               false,
		FFmpegAvailable:         false,
		FFprobeAvailable:        false,
		RuntimeID:               runtimeID,
		NetworkProtocolsAllowed: false,
		State:                   "disabled",
		Reason:                  reason,
	}
}

func (c CapabilityState) IsUsable() bool {
	return c.Supported && c.FFmpegAvailable
}

func generateFingerprint(version, runtimeID string) string {
	h := sha256.New()
	h.Write([]byte(version))
	h.Write([]byte("|"))
	h.Write([]byte(runtimeID))
	return hex.EncodeToString(h.Sum(nil))
}

func DetectCodecContainerCompatibility(codec, container string) bool {
	codec = strings.ToLower(codec)
	container = strings.ToLower(container)

	compatMap := map[string][]string{
		"mp4":  {"h264", "hevc", "av1", "mpeg4", "vp9", "aac", "mp3", "opus", "flac", "pcm_s16le", "mjpeg"},
		"mov":  {"h264", "hevc", "av1", "mpeg4", "prores", "aac", "mp3", "pcm_s16le"},
		"mkv":  {"h264", "hevc", "av1", "vp9", "mpeg4", "aac", "mp3", "opus", "vorbis", "flac", "pcm_s16le", "subrip", "ass", "ssa", "pgs"},
		"webm": {"vp9", "vp8", "av1", "opus", "vorbis"},
		"mp3":  {"mp3"},
		"m4a":  {"aac", "alac", "pcm_s16le"},
		"wav":  {"pcm_s16le", "pcm_f32le", "pcm_u8", "aac", "mp3"},
		"flac": {"flac"},
		"ogg":  {"opus", "vorbis", "flac"},
		"opus": {"opus"},
		"gif":  {"gif"},
	}

	if containers, ok := compatMap[container]; ok {
		for _, c := range containers {
			if c == codec || codec == "copy" {
				return true
			}
		}
	}
	return false
}
