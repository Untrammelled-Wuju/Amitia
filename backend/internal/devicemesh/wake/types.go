package wake

import (
	"context"
	"sync"
)

const (
	CanonicalSampleRate = 16000
	CanonicalChannels   = 1
	CanonicalEncoding   = "pcm_s16le"
)

type VoiceAudioFrame struct {
	SessionID   string
	Sequence    uint64
	TimestampNS int64
	SampleRate  int
	Channels    int
	Encoding    string
	PCM         []byte
}

type WakePhrase struct {
	ID          string
	DisplayText string
	Locale      string
}

type WakeDetectorConfig struct {
	Enabled          bool
	Backend          string
	ModelResourceURI string
	Phrases          []WakePhrase
	Threshold        float64
	CooldownMS       int64
}

type WakeCapabilities struct {
	Supported        bool
	LocalOnly        bool
	MaxPhrases       int
	SupportsCustom   bool
	RequiresModel    bool
	SupportedLocales []string
	Backend          string
}

type WakeDetectionResult struct {
	Detected     bool
	PhraseID     string
	Confidence   float64
	DetectedAtNS int64
}

type WakeDetector interface {
	Capabilities() WakeCapabilities
	Load(context.Context, WakeDetectorConfig) error
	Reset()
	Process(*VoiceAudioFrame) (WakeDetectionResult, error)
	Unload() error
}

var detectorRegistry sync.Map

func RegisterWakeBackend(name string, factory func(string) (WakeDetector, error)) {
	detectorRegistry.Store(name, factory)
}

func GetWakeBackend(name string) (func(string) (WakeDetector, error), bool) {
	value, ok := detectorRegistry.Load(name)
	if !ok {
		return nil, false
	}
	factory, ok := value.(func(string) (WakeDetector, error))
	return factory, ok
}
