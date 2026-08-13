// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
	"time"
)

type VoiceEventType string

const (
	VoiceEventSessionStarted   VoiceEventType = "voice.session.started"
	VoiceEventSessionArmed     VoiceEventType = "voice.session.armed"
	VoiceEventWakeDetected     VoiceEventType = "voice.wake.detected"
	VoiceEventSpeechStarted    VoiceEventType = "voice.speech.started"
	VoiceEventASRInterim       VoiceEventType = "voice.asr.interim"
	VoiceEventASRFinal         VoiceEventType = "voice.asr.final"
	VoiceEventTurnProcessing   VoiceEventType = "voice.turn.processing"
	VoiceEventResponseStarted  VoiceEventType = "voice.response.started"
	VoiceEventTTSStarted       VoiceEventType = "voice.tts.started"
	VoiceEventTTSAudio         VoiceEventType = "voice.tts.audio"
	VoiceEventTTSEnded         VoiceEventType = "voice.tts.ended"
	VoiceEventBargeIn          VoiceEventType = "voice.barge_in"
	VoiceEventSessionSuspended VoiceEventType = "voice.session.suspended"
	VoiceEventSessionResumed   VoiceEventType = "voice.session.resumed"
	VoiceEventSessionEnded     VoiceEventType = "voice.session.ended"
	VoiceEventSessionError     VoiceEventType = "voice.session.error"
)

type VoiceEvent struct {
	Type          VoiceEventType         `json:"type"`
	SessionID     string                 `json:"sessionId"`
	TurnID        string                 `json:"turnId,omitempty"`
	PlaybackID    string                 `json:"playbackId,omitempty"`
	InteractionID string                 `json:"interactionId,omitempty"`
	Timestamp     int64                  `json:"timestamp"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
}

type VoiceEventBus struct {
	mu        sync.RWMutex
	listeners []func(VoiceEvent)
}

func NewVoiceEventBus() *VoiceEventBus {
	return &VoiceEventBus{
		listeners: make([]func(VoiceEvent), 0),
	}
}

func (b *VoiceEventBus) Subscribe(fn func(VoiceEvent)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, fn)
}

func (b *VoiceEventBus) Publish(event VoiceEvent) {
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, fn := range b.listeners {
		fn(event)
	}
}

func (b *VoiceEventBus) PublishSessionEvent(sessionID string, eventType VoiceEventType, payload map[string]interface{}) {
	b.Publish(VoiceEvent{
		Type:      eventType,
		SessionID: sessionID,
		Payload:   payload,
	})
}

