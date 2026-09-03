// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type VisualSourceType string

const (
	VisualSourceCamera VisualSourceType = "camera"
	VisualSourceScreen VisualSourceType = "screen"
)

type MediaSourceState struct {
	Audio  bool `json:"audio"`
	Camera bool `json:"camera"`
	Screen bool `json:"screen"`
}

type RealtimeCallCapabilities struct {
	AudioInput        bool `json:"audioInput"`
	AudioOutput       bool `json:"audioOutput"`
	CameraInput       bool `json:"cameraInput"`
	ScreenInput       bool `json:"screenInput"`
	VisualContext     bool `json:"visualContext"`
	VisualLatestWins  bool `json:"visualLatestWins"`
	SeparateVisualWS  bool `json:"separateVisualWs"`
	DynamicSourceSwap bool `json:"dynamicSourceSwap"`
	MaxVisualFPS      int  `json:"maxVisualFps"`
	MaxVisualBytes    int  `json:"maxVisualBytes"`
}

type RealtimeCallSession struct {
	CallID         string                   `json:"callId"`
	SessionID      string                   `json:"sessionId"`
	ConversationID string                   `json:"conversationId,omitempty"`
	CharacterID    string                   `json:"characterId,omitempty"`
	UserID         string                   `json:"userId,omitempty"`
	Sources        MediaSourceState         `json:"sources"`
	Capabilities   RealtimeCallCapabilities `json:"capabilities"`
	CreatedAt      time.Time                `json:"createdAt"`
	LastActivityAt time.Time                `json:"lastActivityAt"`

	mu                     sync.RWMutex
	visualTicketHash       string
	latestVisualContext    string
	latestVisualSource     VisualSourceType
	latestVisualCapturedAt time.Time
	closed                 bool
}

func NewRealtimeCallSession(callID, sessionID, conversationID, characterID, userID, visualTicket string) *RealtimeCallSession {
	now := time.Now().UTC()
	return &RealtimeCallSession{
		CallID:         callID,
		SessionID:      sessionID,
		ConversationID: conversationID,
		CharacterID:    characterID,
		UserID:         userID,
		Sources:        MediaSourceState{Audio: true},
		Capabilities: RealtimeCallCapabilities{
			AudioInput:        true,
			AudioOutput:       true,
			CameraInput:       true,
			ScreenInput:       true,
			VisualContext:     true,
			VisualLatestWins:  true,
			SeparateVisualWS:  true,
			DynamicSourceSwap: true,
			MaxVisualFPS:      3,
			MaxVisualBytes:    maxVisualFrameBytes,
		},
		CreatedAt:        now,
		LastActivityAt:   now,
		visualTicketHash: hashVisualTicket(visualTicket),
	}
}

func (s *RealtimeCallSession) SetSources(sources MediaSourceState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sources = sources
	s.LastActivityAt = time.Now().UTC()
}

func (s *RealtimeCallSession) VerifyVisualTicket(ticket string) bool {
	if ticket == "" {
		return false
	}
	s.mu.RLock()
	expected := s.visualTicketHash
	closed := s.closed
	s.mu.RUnlock()
	if closed || expected == "" {
		return false
	}
	return hashVisualTicket(ticket) == expected
}

func (s *RealtimeCallSession) UpdateVisualContext(source VisualSourceType, capturedAt time.Time, context string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if capturedAt.Before(s.latestVisualCapturedAt) {
		return
	}
	s.latestVisualContext = context
	s.latestVisualSource = source
	s.latestVisualCapturedAt = capturedAt
	s.LastActivityAt = time.Now().UTC()
}

func (s *RealtimeCallSession) LatestVisualContext(maxAge time.Duration) (string, VisualSourceType, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latestVisualContext == "" || s.latestVisualCapturedAt.IsZero() {
		return "", "", time.Time{}, false
	}
	if maxAge > 0 && time.Since(s.latestVisualCapturedAt) > maxAge {
		return "", "", time.Time{}, false
	}
	return s.latestVisualContext, s.latestVisualSource, s.latestVisualCapturedAt, true
}

func (s *RealtimeCallSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.visualTicketHash = ""
	s.LastActivityAt = time.Now().UTC()
}

func hashVisualTicket(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(sum[:])
}
