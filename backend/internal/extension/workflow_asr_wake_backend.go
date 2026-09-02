package extension

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/realtime"
)

const (
	workflowASRWakeBackend       = "asr_phrase"
	workflowASRWakeSampleRate    = 16000
	workflowASRWakeBytesPerFrame = 2
	workflowASRWakePreRollMS     = 250
	workflowASRWakeEndSilenceMS  = 550
	workflowASRWakeMinSpeechMS   = 180
	workflowASRWakeMaxSpeechMS   = 4500
	workflowASRWakeVADEnergy     = 0.0025
)

type workflowASRPhraseWake struct {
	mu sync.Mutex

	cfg       realtime.WakeDetectorConfig
	asrConfig *asr.AsrConfig
	loaded    bool

	preRoll               []byte
	utterance             []byte
	speechMS              int
	silenceMS             int
	inSpeech              bool
	recognizing           bool
	recognizingGeneration uint64
	generation            uint64
	pending               []realtime.WakeDetectionResult
	cooldownTill          int64

	ctx    context.Context
	cancel context.CancelFunc
}

func newWorkflowASRPhraseWake() realtime.WakeDetector {
	return &workflowASRPhraseWake{}
}

func (w *workflowASRPhraseWake) Capabilities() realtime.WakeCapabilities {
	return realtime.WakeCapabilities{
		Supported:        true,
		LocalOnly:        false,
		MaxPhrases:       16,
		SupportsCustom:   true,
		RequiresModel:    false,
		SupportedLocales: []string{"zh-CN", "en-US", "ja-JP", "ko-KR"},
		Backend:          workflowASRWakeBackend,
	}
}

func (w *workflowASRPhraseWake) Load(_ context.Context, cfg realtime.WakeDetectorConfig) error {
	if !cfg.Enabled {
		return errors.New("wake: config disabled")
	}
	if len(cfg.Phrases) == 0 {
		return errors.New("wake: no phrases configured")
	}
	active, err := asr.ActiveRuntimeConfig()
	if err != nil {
		return fmt.Errorf("wake: active ASR unavailable: %w", err)
	}
	if !asr.SupportsSegmentPCM(active) {
		return fmt.Errorf("wake: ASR provider %q cannot recognize private PCM segments; use an OpenAI-compatible or Azure ASR config", active.ApiType)
	}
	if strings.TrimSpace(active.ApiKey) == "" {
		return errors.New("wake: active ASR credential is empty")
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.generation++
	w.recognizing = false
	w.recognizingGeneration = 0
	w.cfg = cfg
	w.asrConfig = active
	w.loaded = true
	w.resetAudioLocked()
	w.pending = nil
	w.cooldownTill = 0
	return nil
}

func (w *workflowASRPhraseWake) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	if w.loaded {
		w.ctx, w.cancel = context.WithCancel(context.Background())
	} else {
		w.ctx = nil
		w.cancel = nil
	}
	w.generation++
	w.recognizing = false
	w.recognizingGeneration = 0
	w.resetAudioLocked()
	w.pending = nil
	w.cooldownTill = 0
}

func (w *workflowASRPhraseWake) Process(frame *realtime.VoiceAudioFrame) (realtime.WakeDetectionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.loaded || !w.cfg.Enabled {
		return realtime.WakeDetectionResult{}, nil
	}
	if len(w.pending) > 0 {
		result := w.pending[0]
		w.pending = w.pending[1:]
		return result, nil
	}
	if frame == nil || len(frame.PCM) < workflowASRWakeBytesPerFrame {
		return realtime.WakeDetectionResult{}, nil
	}
	if w.recognizing {
		return realtime.WakeDetectionResult{}, nil
	}

	now := time.Now().UnixNano()
	if w.cooldownTill > now {
		w.resetAudioLocked()
		return realtime.WakeDetectionResult{}, nil
	}

	frameMS := pcmDurationMS(frame.PCM, workflowASRWakeSampleRate)
	if frameMS <= 0 {
		return realtime.WakeDetectionResult{}, nil
	}
	energy := pcmRMSEnergy(frame.PCM)
	isSpeech := energy >= workflowASRWakeVADEnergy

	if !w.inSpeech {
		if !isSpeech {
			w.appendPreRollLocked(frame.PCM)
			return realtime.WakeDetectionResult{}, nil
		}
		w.inSpeech = true
		w.utterance = append(w.utterance[:0], w.preRoll...)
		w.utterance = append(w.utterance, frame.PCM...)
		w.speechMS = frameMS
		w.silenceMS = 0
		return realtime.WakeDetectionResult{}, nil
	}

	w.utterance = append(w.utterance, frame.PCM...)
	if isSpeech {
		w.speechMS += frameMS
		w.silenceMS = 0
	} else {
		w.silenceMS += frameMS
	}

	utteranceMS := pcmDurationMS(w.utterance, workflowASRWakeSampleRate)
	shouldRecognize := (w.speechMS >= workflowASRWakeMinSpeechMS && w.silenceMS >= workflowASRWakeEndSilenceMS) || utteranceMS >= workflowASRWakeMaxSpeechMS
	if !shouldRecognize {
		return realtime.WakeDetectionResult{}, nil
	}

	pcm := append([]byte(nil), w.utterance...)
	cfg := w.cfg
	asrCfg := *w.asrConfig
	ctx := w.ctx
	generation := w.generation
	w.recognizing = true
	w.recognizingGeneration = generation
	w.resetAudioLocked()
	go w.recognize(ctx, &asrCfg, cfg, pcm, now, generation)
	return realtime.WakeDetectionResult{}, nil
}

func (w *workflowASRPhraseWake) recognize(parent context.Context, cfg *asr.AsrConfig, wakeCfg realtime.WakeDetectorConfig, pcm []byte, detectedAtNS int64, generation uint64) {
	defer clear(pcm)
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()

	language := ""
	if len(wakeCfg.Phrases) > 0 {
		language = strings.TrimSpace(wakeCfg.Phrases[0].Locale)
	}
	text, err := asr.RecognizePCM(ctx, cfg, pcm, language)

	w.mu.Lock()
	defer w.mu.Unlock()
	if generation != w.generation {
		if w.recognizingGeneration == generation {
			w.recognizing = false
			w.recognizingGeneration = 0
		}
		return
	}
	if w.recognizingGeneration == generation {
		w.recognizing = false
		w.recognizingGeneration = 0
	}
	if !w.loaded || err != nil || strings.TrimSpace(text) == "" {
		return
	}
	phraseID, confidence, matched := matchWorkflowWakePhrase(text, wakeCfg.Phrases)
	if !matched {
		return
	}
	threshold := wakeCfg.Threshold
	if threshold > 0 && threshold <= 1 && confidence < threshold {
		return
	}
	if detectedAtNS <= 0 {
		detectedAtNS = time.Now().UnixNano()
	}
	w.pending = append(w.pending, realtime.WakeDetectionResult{
		Detected:     true,
		PhraseID:     phraseID,
		Confidence:   confidence,
		DetectedAtNS: detectedAtNS,
	})
	cooldown := wakeCfg.CooldownMS
	if cooldown <= 0 {
		cooldown = 2000
	}
	w.cooldownTill = time.Now().Add(time.Duration(cooldown) * time.Millisecond).UnixNano()
}

func (w *workflowASRPhraseWake) Unload() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.cancel = nil
	w.ctx = nil
	w.generation++
	w.recognizing = false
	w.recognizingGeneration = 0
	w.loaded = false
	w.asrConfig = nil
	w.pending = nil
	w.resetAudioLocked()
	return nil
}

func (w *workflowASRPhraseWake) resetAudioLocked() {
	if len(w.utterance) > 0 {
		clear(w.utterance)
	}
	if len(w.preRoll) > 0 {
		clear(w.preRoll)
	}
	w.utterance = w.utterance[:0]
	w.preRoll = w.preRoll[:0]
	w.speechMS = 0
	w.silenceMS = 0
	w.inSpeech = false
}

func (w *workflowASRPhraseWake) appendPreRollLocked(pcm []byte) {
	w.preRoll = append(w.preRoll, pcm...)
	maxBytes := workflowASRWakeSampleRate * workflowASRWakeBytesPerFrame * workflowASRWakePreRollMS / 1000
	if len(w.preRoll) > maxBytes {
		copy(w.preRoll, w.preRoll[len(w.preRoll)-maxBytes:])
		w.preRoll = w.preRoll[:maxBytes]
	}
}

func pcmDurationMS(pcm []byte, sampleRate int) int {
	if sampleRate <= 0 {
		return 0
	}
	return (len(pcm) / workflowASRWakeBytesPerFrame) * 1000 / sampleRate
}

func pcmRMSEnergy(pcm []byte) float64 {
	if len(pcm) < 2 {
		return 0
	}
	var sum float64
	var samples int
	for i := 0; i+1 < len(pcm); i += 2 {
		sample := int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8)
		normalized := float64(sample) / 32768.0
		sum += normalized * normalized
		samples++
	}
	if samples == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(samples))
}

func matchWorkflowWakePhrase(transcript string, phrases []realtime.WakePhrase) (string, float64, bool) {
	normalizedTranscript := normalizeWorkflowWakeText(transcript)
	if normalizedTranscript == "" {
		return "", 0, false
	}
	for _, phrase := range phrases {
		normalizedPhrase := normalizeWorkflowWakeText(phrase.DisplayText)
		if normalizedPhrase == "" {
			continue
		}
		if normalizedTranscript == normalizedPhrase {
			return phrase.ID, 1, true
		}
		// Contains matching tolerates a normal ASR response such as “你好 Amitia。”
		// while refusing one-character wake phrases that would be too permissive.
		if len([]rune(normalizedPhrase)) >= 3 && strings.Contains(normalizedTranscript, normalizedPhrase) {
			return phrase.ID, 0.9, true
		}
	}
	return "", 0, false
}

func normalizeWorkflowWakeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func init() {
	realtime.RegisterWakeBackend(workflowASRWakeBackend, func(string) (realtime.WakeDetector, error) {
		return newWorkflowASRPhraseWake(), nil
	})
}
