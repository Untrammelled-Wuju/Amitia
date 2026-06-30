// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"strings"
	"sync"
	"time"
)

type WorkingMemory struct {
	ConversationID string
	State          *WorkingMemoryState
	LastAccess     time.Time
}

type WorkingMemoryCache struct {
	mu          sync.RWMutex
	store       map[string]*WorkingMemory
	expireAfter time.Duration
}

func NewWorkingMemoryCache(expireAfter time.Duration) *WorkingMemoryCache {
	return &WorkingMemoryCache{
		store:       make(map[string]*WorkingMemory),
		expireAfter: expireAfter,
	}
}

func (c *WorkingMemoryCache) Get(convID string) *WorkingMemory {
	c.mu.Lock()
	defer c.mu.Unlock()
	wm, ok := c.store[convID]
	if !ok {
		return nil
	}
	if time.Since(wm.LastAccess) > c.expireAfter {
		return nil
	}
	wm.LastAccess = time.Now()
	return wm
}

func (c *WorkingMemoryCache) Set(convID string, state *WorkingMemoryState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[convID] = &WorkingMemory{
		ConversationID: convID,
		State:          state,
		LastAccess:     time.Now(),
	}
	c.evict()
}

func (c *WorkingMemoryCache) UpdateSummary(convID string, reply string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	wm, ok := c.store[convID]
	if !ok {
		wm = &WorkingMemory{
			ConversationID: convID,
			State: &WorkingMemoryState{
				ConversationID: convID,
				KeyPoints:      []string{},
			},
		}
		c.store[convID] = wm
	}
	summary := reply
	if len([]rune(summary)) > 500 {
		summary = string([]rune(summary)[:500]) + "..."
	}
	wm.State.Summary = summary
	points := extractKeyPoints(reply)
	if len(points) > 0 {
		existing := map[string]bool{}
		for _, p := range wm.State.KeyPoints {
			existing[p] = true
		}
		for _, p := range points {
			if !existing[p] {
				wm.State.KeyPoints = append(wm.State.KeyPoints, p)
			}
		}
		if len(wm.State.KeyPoints) > 20 {
			wm.State.KeyPoints = wm.State.KeyPoints[len(wm.State.KeyPoints)-20:]
		}
	}
	wm.State.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	wm.LastAccess = time.Now()
	c.evict()
}

func extractKeyPoints(text string) []string {
	lines := strings.Split(text, "\n")
	var points []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) > 100 {
			continue
		}
		points = append(points, line)
	}
	if len(points) > 5 {
		points = points[:5]
	}
	return points
}

func (c *WorkingMemoryCache) evict() {
	now := time.Now()
	for id, wm := range c.store {
		if now.Sub(wm.LastAccess) > c.expireAfter {
			delete(c.store, id)
		}
	}
}
