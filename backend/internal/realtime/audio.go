package realtime

import (
	"fmt"
	"sync"
	"time"
)

const (
	CanonicalSampleRate = 16000
	CanonicalChannels   = 1
	CanonicalEncoding   = "pcm_s16le"
)

type AudioEncoding string

const (
	AudioEncodingPCM16LE AudioEncoding = "pcm_s16le"
	AudioEncodingPCM24LE AudioEncoding = "pcm_s24le"
	AudioEncodingPCM32LE AudioEncoding = "pcm_s32le"
	AudioEncodingOpus    AudioEncoding = "opus"
	AudioEncodingAAC     AudioEncoding = "aac"
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

func (f *VoiceAudioFrame) DurationMS() float64 {
	if f.SampleRate <= 0 || f.Channels <= 0 {
		return 0
	}
	bytesPerSample := 2
	if f.Encoding == "pcm_s24le" {
		bytesPerSample = 3
	} else if f.Encoding == "pcm_s32le" {
		bytesPerSample = 4
	}
	samples := len(f.PCM) / (bytesPerSample * f.Channels)
	return float64(samples) / float64(f.SampleRate) * 1000.0
}

func (f *VoiceAudioFrame) IsEmpty() bool {
	return len(f.PCM) == 0
}

func (f *VoiceAudioFrame) Clone() *VoiceAudioFrame {
	clone := &VoiceAudioFrame{
		SessionID:   f.SessionID,
		Sequence:    f.Sequence,
		TimestampNS: f.TimestampNS,
		SampleRate:  f.SampleRate,
		Channels:    f.Channels,
		Encoding:    f.Encoding,
	}
	if len(f.PCM) > 0 {
		clone.PCM = make([]byte, len(f.PCM))
		copy(clone.PCM, f.PCM)
	}
	return clone
}

type AudioRingBuffer struct {
	mu         sync.Mutex
	buffer     []byte
	capacity   int
	readPos    int
	writePos   int
	filled     int
	sampleRate int
	channels   int
}

func NewAudioRingBuffer(durationMS int, sampleRate int, channels int) *AudioRingBuffer {
	bytesPerSample := 2
	capacity := sampleRate * channels * bytesPerSample * durationMS / 1000
	return &AudioRingBuffer{
		buffer:     make([]byte, capacity),
		capacity:   capacity,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

func (rb *AudioRingBuffer) Write(pcm []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i := 0; i < len(pcm); i++ {
		rb.buffer[rb.writePos] = pcm[i]
		rb.writePos = (rb.writePos + 1) % rb.capacity
		if rb.filled < rb.capacity {
			rb.filled++
		} else {
			rb.readPos = (rb.readPos + 1) % rb.capacity
		}
	}
}

func (rb *AudioRingBuffer) ReadAll() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.filled == 0 {
		return nil
	}
	result := make([]byte, rb.filled)
	for i := 0; i < rb.filled; i++ {
		result[i] = rb.buffer[(rb.readPos+i)%rb.capacity]
	}
	return result
}

func (rb *AudioRingBuffer) DurationMS() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.sampleRate <= 0 || rb.channels <= 0 {
		return 0
	}
	bytesPerFrame := 2 * rb.channels
	frames := rb.filled / bytesPerFrame
	return frames * 1000 / rb.sampleRate
}

type VADCapabilities struct {
	SupportsStreaming    bool
	SupportedSampleRates []int
	MinFrameMS           int
	MaxFrameMS           int
	ProvidesProbability  bool
	Backend              string
}

type VADResult struct {
	Speech        bool
	Probability   float64
	SpeechStarted bool
	SpeechEnded   bool
	TimestampNS   int64
}

type EndpointConfig struct {
	PreRollMS      int
	MinSpeechMS    int
	EndSilenceMS   int
	MaxUtteranceMS int
}

func DefaultEndpointConfig() EndpointConfig {
	return EndpointConfig{
		PreRollMS:      300,
		MinSpeechMS:    200,
		EndSilenceMS:   650,
		MaxUtteranceMS: 60000,
	}
}

type EndpointResult struct {
	TurnStart   bool
	TurnEnd     bool
	TimestampNS int64
	DurationMS  int
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

type VoiceCapabilities struct {
	StreamingASR       bool
	StreamingTTS       bool
	FullDuplexRealtime bool
	VAD                bool
	WakeWord           bool
	SystemHotword      bool
	BackgroundCapture  bool
	BargeIn            bool
	EchoCancellation   bool
}

type ContinuousVoiceSessionMode string

const (
	ContinuousVoiceSessionModePushToTalk       ContinuousVoiceSessionMode = "push_to_talk"
	ContinuousVoiceSessionModeOpenMic          ContinuousVoiceSessionMode = "open_mic"
	ContinuousVoiceSessionModeWakeArmed        ContinuousVoiceSessionMode = "wake_armed"
	ContinuousVoiceSessionModeProviderRealtime ContinuousVoiceSessionMode = "provider_realtime"
)

type ContinuousVoiceSessionStatus string

const (
	ContinuousVoiceSessionStatusIdle         ContinuousVoiceSessionStatus = "idle"
	ContinuousVoiceSessionStatusStarting     ContinuousVoiceSessionStatus = "starting"
	ContinuousVoiceSessionStatusArmed        ContinuousVoiceSessionStatus = "armed"
	ContinuousVoiceSessionStatusListening    ContinuousVoiceSessionStatus = "listening"
	ContinuousVoiceSessionStatusTranscribing ContinuousVoiceSessionStatus = "transcribing"
	ContinuousVoiceSessionStatusProcessing   ContinuousVoiceSessionStatus = "processing"
	ContinuousVoiceSessionStatusSpeaking     ContinuousVoiceSessionStatus = "speaking"
	ContinuousVoiceSessionStatusSuspended    ContinuousVoiceSessionStatus = "suspended"
	ContinuousVoiceSessionStatusStopping     ContinuousVoiceSessionStatus = "stopping"
	ContinuousVoiceSessionStatusEnded        ContinuousVoiceSessionStatus = "ended"
	ContinuousVoiceSessionStatusFailed       ContinuousVoiceSessionStatus = "failed"
)

type AudioRoute string

const (
	AudioRouteBuiltIn   AudioRoute = "built_in"
	AudioRouteSpeaker   AudioRoute = "speaker"
	AudioRouteWired     AudioRoute = "wired"
	AudioRouteBluetooth AudioRoute = "bluetooth"
	AudioRouteUSB       AudioRoute = "usb"
	AudioRouteExternal  AudioRoute = "external"
	AudioRouteUnknown   AudioRoute = "unknown"
)

type AudioSessionCategory string

const (
	AudioSessionCategoryPlayAndRecord AudioSessionCategory = "playAndRecord"
	AudioSessionCategoryRecord        AudioSessionCategory = "record"
	AudioSessionCategoryPlayback      AudioSessionCategory = "playback"
)

type Platform string

const (
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformDesktop Platform = "desktop"
	PlatformWeb     Platform = "web"
)

func NewVoiceAudioFrame(sessionID string, sequence uint64, pcm []byte) *VoiceAudioFrame {
	return &VoiceAudioFrame{
		SessionID:   sessionID,
		Sequence:    sequence,
		TimestampNS: time.Now().UnixNano(),
		SampleRate:  CanonicalSampleRate,
		Channels:    CanonicalChannels,
		Encoding:    CanonicalEncoding,
		PCM:         pcm,
	}
}

func (s ContinuousVoiceSessionStatus) IsActive() bool {
	switch s {
	case ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusTranscribing,
		ContinuousVoiceSessionStatusProcessing, ContinuousVoiceSessionStatusSpeaking:
		return true
	}
	return false
}

func (s ContinuousVoiceSessionStatus) IsTerminal() bool {
	return s == ContinuousVoiceSessionStatusEnded || s == ContinuousVoiceSessionStatusFailed
}

func (s ContinuousVoiceSessionStatus) CanTransitionTo(target ContinuousVoiceSessionStatus) bool {
	transitions := map[ContinuousVoiceSessionStatus][]ContinuousVoiceSessionStatus{
		ContinuousVoiceSessionStatusIdle:         {ContinuousVoiceSessionStatusStarting},
		ContinuousVoiceSessionStatusStarting:     {ContinuousVoiceSessionStatusArmed, ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusTranscribing, ContinuousVoiceSessionStatusFailed, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusArmed:        {ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusSuspended, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusListening:    {ContinuousVoiceSessionStatusTranscribing, ContinuousVoiceSessionStatusSuspended, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusTranscribing: {ContinuousVoiceSessionStatusProcessing, ContinuousVoiceSessionStatusSuspended, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusProcessing:   {ContinuousVoiceSessionStatusSpeaking, ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusSuspended, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusSpeaking:     {ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusSuspended, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusSuspended:    {ContinuousVoiceSessionStatusListening, ContinuousVoiceSessionStatusStopping},
		ContinuousVoiceSessionStatusStopping:     {ContinuousVoiceSessionStatusEnded, ContinuousVoiceSessionStatusFailed},
	}
	valid, ok := transitions[s]
	if !ok {
		return false
	}
	for _, v := range valid {
		if v == target {
			return true
		}
	}
	return false
}

type VoiceSessionRequest struct {
	SessionID      string
	ConversationID string
	CharacterID    string
	Mode           ContinuousVoiceSessionMode
	Platform       Platform
	UserID         string
	ProfileID      string
}

type VoiceExecutionPlan struct {
	Path               string
	UseLocalVAD        bool
	UseStreamingASR    bool
	UseSegmentASR      bool
	UseStreamingTTS    bool
	UseFullTTS         bool
	RealtimeProviderID string
	RequiresNetwork    bool
	LocalOnly          bool
}

func (p *VoiceExecutionPlan) Validate() error {
	if p.Path == "" {
		return fmt.Errorf("voice execution plan: path is empty")
	}
	if p.UseStreamingASR && p.UseSegmentASR {
		return fmt.Errorf("voice execution plan: cannot use both streaming and segment ASR")
	}
	if p.UseStreamingTTS && p.UseFullTTS {
		return fmt.Errorf("voice execution plan: cannot use both streaming and full TTS")
	}
	return nil
}
