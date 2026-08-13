// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type AudioPlaybackState string

const (
	AudioPlaybackStateIdle      AudioPlaybackState = "idle"
	AudioPlaybackStatePlaying   AudioPlaybackState = "playing"
	AudioPlaybackStatePaused    AudioPlaybackState = "paused"
	AudioPlaybackStateCancelled AudioPlaybackState = "cancelled"
)

type PlaybackController struct {
	mu                sync.Mutex
	state             AudioPlaybackState
	currentPlaybackID string
	generation        int64
	audioQueue        []*PlaybackChunk
	listeners         []func(PlaybackEvent)
}

type PlaybackChunk struct {
	Sequence  uint64
	AudioData []byte
	Encoding  string
	Timestamp int64
}

type PlaybackEvent struct {
	Type       string `json:"type"`
	PlaybackID string `json:"playbackId"`
	Sequence   uint64 `json:"sequence"`
	Complete   bool   `json:"complete"`
	Timestamp  int64  `json:"timestamp"`
}

func NewPlaybackController() *PlaybackController {
	return &PlaybackController{
		state:      AudioPlaybackStateIdle,
		audioQueue: make([]*PlaybackChunk, 0),
		listeners:  make([]func(PlaybackEvent), 0),
	}
}

func (p *PlaybackController) AddListener(fn func(PlaybackEvent)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listeners = append(p.listeners, fn)
}

func (p *PlaybackController) StartPlayback() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.currentPlaybackID = uuid.New().String()
	p.generation++
	p.state = AudioPlaybackStatePlaying

	event := PlaybackEvent{
		Type:       "playback_started",
		PlaybackID: p.currentPlaybackID,
		Timestamp:  time.Now().UnixMilli(),
	}
	p.emit(event)

	return p.currentPlaybackID
}

func (p *PlaybackController) EnqueueAudio(data []byte, encoding string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != AudioPlaybackStatePlaying {
		return
	}

	seq := uint64(len(p.audioQueue) + 1)
	chunk := &PlaybackChunk{
		Sequence:  seq,
		AudioData: data,
		Encoding:  encoding,
		Timestamp: time.Now().UnixMilli(),
	}
	p.audioQueue = append(p.audioQueue, chunk)

	event := PlaybackEvent{
		Type:       "playback_audio",
		PlaybackID: p.currentPlaybackID,
		Sequence:   seq,
		Timestamp:  chunk.Timestamp,
	}
	p.emit(event)
}

func (p *PlaybackController) CompletePlayback() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != AudioPlaybackStatePlaying {
		return
	}

	event := PlaybackEvent{
		Type:       "playback_ended",
		PlaybackID: p.currentPlaybackID,
		Complete:   true,
		Timestamp:  time.Now().UnixMilli(),
	}
	p.emit(event)

	p.state = AudioPlaybackStateIdle
	p.currentPlaybackID = ""
	p.audioQueue = p.audioQueue[:0]
}

func (p *PlaybackController) CancelPlayback() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != AudioPlaybackStatePlaying {
		return
	}

	event := PlaybackEvent{
		Type:       "playback_cancelled",
		PlaybackID: p.currentPlaybackID,
		Timestamp:  time.Now().UnixMilli(),
	}
	p.emit(event)

	p.state = AudioPlaybackStateCancelled
	p.audioQueue = p.audioQueue[:0]
}

func (p *PlaybackController) CurrentGeneration() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.generation
}

func (p *PlaybackController) CurrentPlaybackID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentPlaybackID
}

func (p *PlaybackController) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == AudioPlaybackStatePlaying
}

func (p *PlaybackController) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = AudioPlaybackStateIdle
	p.currentPlaybackID = ""
	p.audioQueue = p.audioQueue[:0]
}

func (p *PlaybackController) emit(event PlaybackEvent) {
	for _, fn := range p.listeners {
		fn(event)
	}
}

