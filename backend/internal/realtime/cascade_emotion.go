package realtime

import (
	"context"
	"strings"
	"sync"
	"time"

	appLog "github.com/u-ai/backend/log"
)

type CascadeUserAffect struct {
	PrimaryEmotion      string   `json:"primary_emotion"`
	SecondaryEmotion    string   `json:"secondary_emotion,omitempty"`
	Intensity           float64  `json:"intensity"`
	Stress              float64  `json:"stress"`
	Need                string   `json:"need"`
	AdviceWanted        bool     `json:"advice_wanted"`
	Openness            float64  `json:"openness"`
	Severity            float64  `json:"severity"`
	PossibleConcealment bool     `json:"possible_concealment"`
	Confidence          float64  `json:"confidence"`
	Evidence            []string `json:"evidence,omitempty"`
}

func normalizeCascadeUserAffect(affect CascadeUserAffect) CascadeUserAffect {
	affect.PrimaryEmotion = strings.TrimSpace(affect.PrimaryEmotion)
	if affect.PrimaryEmotion == "" {
		affect.PrimaryEmotion = "neutral"
	}
	affect.SecondaryEmotion = strings.TrimSpace(affect.SecondaryEmotion)
	affect.Need = strings.TrimSpace(affect.Need)
	if affect.Need == "" {
		affect.Need = "none"
	}
	affect.Intensity = clampUnit(affect.Intensity)
	affect.Stress = clampUnit(affect.Stress)
	affect.Openness = clampUnit(affect.Openness)
	affect.Severity = clampUnit(affect.Severity)
	affect.Confidence = clampUnit(affect.Confidence)
	return affect
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

type CascadeUserSignals struct {
	DeviceClass           string   `json:"device_class,omitempty"`
	VolumeRMSMean         float64  `json:"volume_rms_mean"`
	SpeechRateCharsPerSec float64  `json:"speech_rate_chars_per_sec"`
	UtteranceDurationMS   int64    `json:"utterance_duration_ms"`
	ResponseLatencyMS     int64    `json:"response_latency_ms"`
	InterruptionCount     int      `json:"interruption_count"`
	Evidence              []string `json:"evidence,omitempty"`
}

type CascadeEmotionCommit struct {
	UserText      string
	DeliveredText string
	UserAffect    CascadeUserAffect
	Signals       CascadeUserSignals
}

type CascadeEmotionContext struct {
	Prompt           string
	VoiceInstruction string
}

type CascadeEmotionProvider interface {
	Load(ctx context.Context, spaceID, characterID string) (*CascadeEmotionContext, error)
	Commit(ctx context.Context, spaceID, characterID string, commit CascadeEmotionCommit) error
}

var cascadeEmotionRegistry struct {
	mu       sync.RWMutex
	provider CascadeEmotionProvider
}

func SetCascadeEmotionProvider(provider CascadeEmotionProvider) {
	cascadeEmotionRegistry.mu.Lock()
	cascadeEmotionRegistry.provider = provider
	cascadeEmotionRegistry.mu.Unlock()
}

func cascadeEmotionProviderSnapshot() CascadeEmotionProvider {
	cascadeEmotionRegistry.mu.RLock()
	defer cascadeEmotionRegistry.mu.RUnlock()
	return cascadeEmotionRegistry.provider
}

func (s *cascadeCall) loadEmotionContext(ctx context.Context) {
	provider := s.emotionProvider
	if provider == nil {
		return
	}
	loadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	emotionContext, err := provider.Load(loadCtx, s.params.SpaceID, s.params.CharacterID)
	if err != nil {
		appLog.Warn("failed to load realtime emotion context:", err.Error())
		return
	}
	s.setEmotionContext(emotionContext)
}

func (s *cascadeCall) commitEmotion(tracker *cascadePlaybackTracker, deliveredText string) {
	if s == nil || s.emotionProvider == nil || tracker == nil {
		return
	}
	affect, _ := tracker.UserAffect()
	signals, _ := tracker.UserSignals()
	userText, _ := tracker.UserText()
	if strings.TrimSpace(userText) == "" {
		return
	}
	commit := CascadeEmotionCommit{
		UserText:      userText,
		DeliveredText: deliveredText,
		UserAffect:    affect,
		Signals:       signals,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.emotionProvider.Commit(ctx, s.params.SpaceID, s.params.CharacterID, commit); err != nil {
			appLog.Warn("failed to commit realtime emotion state:", err.Error())
			return
		}
		emotionContext, err := s.emotionProvider.Load(ctx, s.params.SpaceID, s.params.CharacterID)
		if err == nil {
			s.setEmotionContext(emotionContext)
		}
	}()
}

func (s *cascadeCall) setEmotionContext(contextData *CascadeEmotionContext) {
	s.emotionMu.Lock()
	s.emotionContext = contextData
	s.emotionMu.Unlock()
}

func (s *cascadeCall) emotionPrompt() string {
	s.emotionMu.RLock()
	defer s.emotionMu.RUnlock()
	if s.emotionContext == nil {
		return ""
	}
	return strings.TrimSpace(s.emotionContext.Prompt)
}

func (s *cascadeCall) emotionVoiceInstruction() string {
	s.emotionMu.RLock()
	defer s.emotionMu.RUnlock()
	if s.emotionContext == nil {
		return ""
	}
	return strings.TrimSpace(s.emotionContext.VoiceInstruction)
}
