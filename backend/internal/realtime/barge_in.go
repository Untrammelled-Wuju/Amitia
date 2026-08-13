// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
	"time"
)

type BargeInPolicy string

const (
	BargeInPolicyImmediate BargeInPolicy = "immediate"
	BargeInPolicyDeferred  BargeInPolicy = "deferred"
)

type BargeInController struct {
	mu             sync.Mutex
	policy         BargeInPolicy
	isPlaying      bool
	isSpeaking     bool
	playbackID     string
	postGuardUntil int64
}

func NewBargeInController(policy BargeInPolicy) *BargeInController {
	return &BargeInController{
		policy: policy,
	}
}

func (b *BargeInController) SetPolicy(policy BargeInPolicy) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.policy = policy
}

func (b *BargeInController) BeginPlayback(playbackID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isPlaying = true
	b.isSpeaking = true
	b.playbackID = playbackID
}

func (b *BargeInController) EndPlayback() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isPlaying = false
	b.isSpeaking = false
	b.postGuardUntil = time.Now().Add(100 * time.Millisecond).UnixNano()
}

func (b *BargeInController) IsPlaying() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isPlaying
}

func (b *BargeInController) ShouldBargeIn() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isSpeaking {
		return false
	}

	if b.postGuardUntil > 0 && time.Now().UnixNano() < b.postGuardUntil {
		return false
	}

	return true
}

func (b *BargeInController) HandleInterrupt() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isSpeaking {
		return "", false
	}

	if b.policy == BargeInPolicyDeferred && b.isPlaying {
		return "", false
	}

	playbackID := b.playbackID
	b.isPlaying = false
	b.isSpeaking = false

	return playbackID, true
}

func (b *BargeInController) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.isPlaying = false
	b.isSpeaking = false
	b.playbackID = ""
	b.postGuardUntil = 0
}

type PlaybackCancelEvent struct {
	PlaybackID string `json:"playbackId"`
	Reason     string `json:"reason"`
	Timestamp  int64  `json:"timestamp"`
}

func NewPlaybackCancelEvent(playbackID, reason string) PlaybackCancelEvent {
	return PlaybackCancelEvent{
		PlaybackID: playbackID,
		Reason:     reason,
		Timestamp:  time.Now().UnixMilli(),
	}
}

