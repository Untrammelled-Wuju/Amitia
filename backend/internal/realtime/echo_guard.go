// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package realtime

import (
	"sync"
	"time"
)

type EchoGuard struct {
	mu                   sync.Mutex
	playbackActive       bool
	wakeSuppressionUntil int64
	aecEnabled           bool
}

func NewEchoGuard() *EchoGuard {
	return &EchoGuard{}
}

func (g *EchoGuard) SetAECEnabled(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.aecEnabled = enabled
}

func (g *EchoGuard) BeginPlayback() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.playbackActive = true
}

func (g *EchoGuard) EndPlayback() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.playbackActive = false
	g.wakeSuppressionUntil = time.Now().Add(150 * time.Millisecond).UnixNano()
}

func (g *EchoGuard) IsPlaybackActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.playbackActive
}

func (g *EchoGuard) ShouldSuppressWake() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.playbackActive {
		return true
	}

	if g.wakeSuppressionUntil > 0 {
		now := time.Now().UnixNano()
		if now < g.wakeSuppressionUntil {
			return true
		}
	}

	return false
}

func (g *EchoGuard) ShouldSuppressVAD() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.playbackActive && !g.aecEnabled {
		return true
	}

	return false
}

func (g *EchoGuard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.playbackActive = false
	g.wakeSuppressionUntil = 0
}

