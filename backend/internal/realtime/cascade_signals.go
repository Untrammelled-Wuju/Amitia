package realtime

import (
	"fmt"
	"math"
	"time"
	"unicode/utf8"
)

func (s *cascadeCall) beginUserSignal(now time.Time) {
	s.signalMu.Lock()
	s.speechStartedAt = now
	s.utteranceDuration = 0
	s.energySum = 0
	s.energySamples = 0
	s.signalMu.Unlock()
}

func (s *cascadeCall) observeAudioEnergy(energy float64) {
	if energy < 0 {
		return
	}
	s.signalMu.Lock()
	s.energySum += energy
	s.energySamples++
	s.signalMu.Unlock()
}

func (s *cascadeCall) endUserSignal(now time.Time) {
	s.signalMu.Lock()
	if !s.speechStartedAt.IsZero() && now.After(s.speechStartedAt) {
		s.utteranceDuration = now.Sub(s.speechStartedAt)
	}
	s.signalMu.Unlock()
}

func (s *cascadeCall) completeUserUtterance(text string) {
	s.signalMu.Lock()
	s.lastUtteranceChars = utf8.RuneCountInString(text)
	s.signalMu.Unlock()
}

func (s *cascadeCall) markAssistantPlaybackEnded(now time.Time) {
	s.signalMu.Lock()
	s.lastAssistantEndedAt = now
	s.signalMu.Unlock()
}

func (s *cascadeCall) signalSnapshot() CascadeUserSignals {
	s.signalMu.Lock()
	defer s.signalMu.Unlock()
	signals := CascadeUserSignals{}
	if s.energySamples > 0 {
		signals.VolumeRMSMean = math.Sqrt(s.energySum / float64(s.energySamples))
	}
	signals.UtteranceDurationMS = s.utteranceDuration.Milliseconds()
	if s.lastUtteranceChars > 0 && signals.UtteranceDurationMS > 0 {
		signals.SpeechRateCharsPerSec = float64(s.lastUtteranceChars) / (float64(signals.UtteranceDurationMS) / 1000)
	}
	if !s.speechStartedAt.IsZero() && !s.lastAssistantEndedAt.IsZero() && s.speechStartedAt.After(s.lastAssistantEndedAt) {
		signals.ResponseLatencyMS = s.speechStartedAt.Sub(s.lastAssistantEndedAt).Milliseconds()
	}
	signals.InterruptionCount = int(s.interruptCount.Load())
	if signals.ResponseLatencyMS >= 1200 {
		signals.Evidence = append(signals.Evidence, "response_latency_high")
	}
	if signals.UtteranceDurationMS > 0 {
		signals.Evidence = append(signals.Evidence, fmt.Sprintf("utterance_duration_ms=%d", signals.UtteranceDurationMS))
	}
	return signals
}
