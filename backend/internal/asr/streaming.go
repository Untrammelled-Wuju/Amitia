// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package asr

import (
	"context"
	"fmt"
	"sync"
)

type ASRStreamEventType string

const (
	ASRStreamEventSessionStarted ASRStreamEventType = "session_started"
	ASRStreamEventInterim        ASRStreamEventType = "interim"
	ASRStreamEventFinal          ASRStreamEventType = "final"
	ASRStreamEventSpeechStart    ASRStreamEventType = "speech_start"
	ASRStreamEventSpeechEnd      ASRStreamEventType = "speech_end"
	ASRStreamEventError          ASRStreamEventType = "error"
	ASRStreamEventClosed         ASRStreamEventType = "closed"
)

type ASRStreamEvent struct {
	Type       ASRStreamEventType `json:"type"`
	SessionID  string             `json:"sessionId,omitempty"`
	Text       string             `json:"text,omitempty"`
	IsFinal    bool               `json:"isFinal,omitempty"`
	Confidence float64            `json:"confidence,omitempty"`
	Timestamp  int64              `json:"timestamp,omitempty"`
	Error      string             `json:"error,omitempty"`
}

type StreamingASRRequest struct {
	SessionID      string
	Language       string
	SampleRate     int
	Channels       int
	Encoding       string
	EnableInterim  bool
	EnableVAD      bool
	ConversationID string
	CharacterID    string
}

type StreamingSession interface {
	PushPCM(ctx context.Context, pcm []byte) error
	EndUtterance(ctx context.Context) error
	Events() <-chan ASRStreamEvent
	Cancel() error
	Close() error
}

type StreamingProvider interface {
	OpenStream(ctx context.Context, cfg *AsrConfig, req StreamingASRRequest) (StreamingSession, error)
	Capabilities() StreamingASRCapabilities
}

type StreamingASRCapabilities struct {
	SupportsStreaming    bool
	SupportedLanguages   []string
	SupportedSampleRates []int
	MaxDurationMS        int
	SupportsInterim      bool
	SupportsVAD          bool
	Backend              string
}

var streamingRegistry sync.Map

func RegisterStreamingProvider(name string, provider StreamingProvider) {
	streamingRegistry.Store(name, provider)
}

func GetStreamingProvider(name string) (StreamingProvider, bool) {
	val, ok := streamingRegistry.Load(name)
	if !ok {
		return nil, false
	}
	provider, ok := val.(StreamingProvider)
	return provider, ok
}

type streamSession struct {
	events   chan ASRStreamEvent
	cancel   context.CancelFunc
	done     sync.Once
	mu       sync.Mutex
	closed   bool
	provider StreamingProvider
	config   *AsrConfig
	request  StreamingASRRequest
}

func newStreamSession(provider StreamingProvider, cancel context.CancelFunc, config *AsrConfig, req StreamingASRRequest) *streamSession {
	return &streamSession{
		events:   make(chan ASRStreamEvent, 64),
		cancel:   cancel,
		provider: provider,
		config:   config,
		request:  req,
	}
}

func (s *streamSession) PushPCM(ctx context.Context, pcm []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("asr stream: session closed")
	}
	return nil
}

func (s *streamSession) EndUtterance(ctx context.Context) error {
	return nil
}

func (s *streamSession) Events() <-chan ASRStreamEvent {
	return s.events
}

func (s *streamSession) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *streamSession) Close() error {
	s.done.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.events)
	})
	return nil
}

func (s *streamSession) emit(event ASRStreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.events <- event:
	default:
	}
}

func ResolveStreamingProvider(apiType string) (StreamingProvider, bool) {
	switch apiType {
	case "openai":
		return GetStreamingProvider("openai")
	case "volcengine":
		return GetStreamingProvider("volcengine")
	case "azure":
		return GetStreamingProvider("azure")
	case "aliyun":
		return GetStreamingProvider("aliyun")
	}
	for name, p := range map[string]bool{
		"openai": true, "volcengine": true, "azure": true, "aliyun": true, "edge": true,
	} {
		if p {
			if provider, ok := GetStreamingProvider(name); ok {
				if provider.Capabilities().SupportsStreaming {
					return provider, true
				}
			}
		}
	}
	return nil, false
}
