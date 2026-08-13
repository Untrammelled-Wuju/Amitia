// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type TTSStreamEventType string

const (
	TTSStreamEventStarted   TTSStreamEventType = "started"
	TTSStreamEventAudio     TTSStreamEventType = "audio"
	TTSStreamEventEnded     TTSStreamEventType = "ended"
	TTSStreamEventError     TTSStreamEventType = "error"
	TTSStreamEventCancelled TTSStreamEventType = "cancelled"
)

type TTSStreamEvent struct {
	Type      TTSStreamEventType `json:"type"`
	Sequence  uint64             `json:"sequence,omitempty"`
	Audio     []byte             `json:"-"`
	AudioData string             `json:"audioData,omitempty"`
	Encoding  string             `json:"encoding,omitempty"`
	Error     string             `json:"error,omitempty"`
}

type TTSStream interface {
	Events() <-chan TTSStreamEvent
	Cancel() error
	Close() error
}

type StreamingTTSProvider interface {
	OpenStream(ctx context.Context, cfg *TtsConfig, text string) (TTSStream, error)
	Capabilities() StreamingTTSCapabilities
}

type StreamingTTSCapabilities struct {
	SupportsStreaming bool
	SupportedFormats  []string
	MinChunkMS        int
	MaxTextLength     int
	Backend           string
}

var ttsStreamRegistry sync.Map

func RegisterTTSStreamingProvider(name string, provider StreamingTTSProvider) {
	ttsStreamRegistry.Store(name, provider)
}

func GetTTSStreamingProvider(name string) (StreamingTTSProvider, bool) {
	val, ok := ttsStreamRegistry.Load(name)
	if !ok {
		return nil, false
	}
	provider, ok := val.(StreamingTTSProvider)
	return provider, ok
}

type fullAudioFallbackStream struct {
	events chan TTSStreamEvent
	mu     sync.Mutex
	closed bool
	done   sync.Once
}

func NewFullAudioFallbackStream(ctx context.Context, cfg *TtsConfig, text string) (TTSStream, error) {
	stream := &fullAudioFallbackStream{
		events: make(chan TTSStreamEvent, 4),
	}

	go func() {
		defer stream.Close()

		resp, err := Synthesize(cfg, text)
		if err != nil {
			stream.emit(TTSStreamEvent{Type: TTSStreamEventError, Error: err.Error()})
			return
		}

		stream.emit(TTSStreamEvent{Type: TTSStreamEventStarted, Sequence: 0})

		if resp != nil && resp.AudioURL != "" {
			audioData, err := fetchAudioByURL(resp.AudioURL)
			if err != nil {
				stream.emit(TTSStreamEvent{Type: TTSStreamEventError, Error: err.Error()})
				return
			}
			stream.emit(TTSStreamEvent{
				Type:      TTSStreamEventAudio,
				Sequence:  1,
				Audio:     audioData,
				AudioData: encodeBase64(audioData),
				Encoding:  "mp3",
			})
		}

		stream.emit(TTSStreamEvent{Type: TTSStreamEventEnded})
	}()

	return stream, nil
}

func (s *fullAudioFallbackStream) Events() <-chan TTSStreamEvent {
	return s.events
}

func (s *fullAudioFallbackStream) Cancel() error {
	return s.Close()
}

func (s *fullAudioFallbackStream) Close() error {
	s.done.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.events)
	})
	return nil
}

func (s *fullAudioFallbackStream) emit(event TTSStreamEvent) {
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

func fetchAudioByURL(url string) ([]byte, error) {
	if len(url) == 0 {
		return nil, fmt.Errorf("tts stream: empty url")
	}
	if url[0] == '/' {
		cacheDir := getCacheDir()
		filePath := filepath.Join(cacheDir, url[len("/audio/"):])
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("tts stream: read cache failed: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("tts stream: unsupported url scheme")
}

func encodeBase64(data []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	if len(data) == 0 {
		return ""
	}
	var buf []byte
	n := len(data)
	for i := 0; i < n; i += 3 {
		var b [3]byte
		remaining := n - i
		for j := 0; j < 3 && i+j < n; j++ {
			b[j] = data[i+j]
		}
		buf = append(buf, chars[(b[0]>>2)&0x3F])
		if remaining == 1 {
			buf = append(buf, chars[(b[0]<<4)&0x3F])
			buf = append(buf, '=', '=')
		} else {
			buf = append(buf, chars[((b[0]<<4)|(b[1]>>4))&0x3F])
			if remaining == 2 {
				buf = append(buf, chars[(b[1]<<2)&0x3F])
				buf = append(buf, '=')
			} else {
				buf = append(buf, chars[((b[1]<<2)|(b[2]>>6))&0x3F])
				buf = append(buf, chars[b[2]&0x3F])
			}
		}
	}
	return string(buf)
}
