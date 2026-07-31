// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"sync"
)

const defaultEventDedupCapacity = 2048

type eventDeduplicator struct {
	mu       sync.Mutex
	seen     map[string]struct{}
	order    []string
	capacity int
}

func newEventDeduplicator(capacity int) *eventDeduplicator {
	if capacity <= 0 {
		capacity = defaultEventDedupCapacity
	}
	return &eventDeduplicator{
		seen:     make(map[string]struct{}, capacity),
		order:    make([]string, 0, capacity),
		capacity: capacity,
	}
}

func (d *eventDeduplicator) isDuplicate(eventID string) bool {
	if eventID == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[eventID]; ok {
		return true
	}
	if len(d.order) >= d.capacity {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.seen, oldest)
	}
	d.seen[eventID] = struct{}{}
	d.order = append(d.order, eventID)
	return false
}
