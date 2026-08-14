// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// Package desktoppet provides the DesktopPet domain services.
//
// IMPORTANT: The EventBus in this package is an EPHEMERAL, IN-PROCESS domain event stream
// for DesktopPet task progress updates. It is NOT the durable event infrastructure.
//
// Do NOT use EventBus as:
//   - A source of truth for installation/runtime/provider/task lifecycle events
//   - A durable event outbox
//   - A replacement for the kernel Durable Event system (event.Service)
//
// Critical state changes must go through the shared kernel Durable Event pipeline.
package desktoppet

import (
	"sync"
	"time"
)

type TaskEvent struct {
	TaskID    string      `json:"taskId"`
	EventType string      `json:"eventType"`
	Payload   interface{} `json:"payload"`
	EmittedAt string      `json:"emittedAt"`
}

type taskSubscription struct {
	id     string
	events chan TaskEvent
}

// EventBus is an ephemeral, in-process DesktopPet task stream.
// It is restricted to lossy, reconstructable progress updates for UI display only.
//
// This is NOT a durable event store. Events published here may be lost on restart.
// Do not use EventBus as the authoritative source of success/failure for critical operations.
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]*taskSubscription
}

// NewEventBus creates a new ephemeral task event stream.
// NOTE: The returned EventBus is for DesktopPet domain-internal use only.
// Production code must NOT use this to communicate critical state changes across domains.
func NewEventBus() *EventBus {
	return &EventBus{subscriptions: make(map[string]map[string]*taskSubscription)}
}

func (b *EventBus) Subscribe(taskID, subscriberID string) <-chan TaskEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subscriberID == "" {
		subscriberID = "default"
	}
	sub := &taskSubscription{id: subscriberID, events: make(chan TaskEvent, 32)}
	if _, ok := b.subscriptions[taskID]; !ok {
		b.subscriptions[taskID] = make(map[string]*taskSubscription)
	}
	b.subscriptions[taskID][subscriberID] = sub
	return sub.events
}

func (b *EventBus) Unsubscribe(taskID, subscriberID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subscriberID == "" {
		subscriberID = "default"
	}
	if subs, ok := b.subscriptions[taskID]; ok {
		if sub, ok := subs[subscriberID]; ok {
			close(sub.events)
			delete(subs, subscriberID)
		}
		if len(subs) == 0 {
			delete(b.subscriptions, taskID)
		}
	}
}

// PublishTaskEvent publishes an ephemeral task progress event.
// Events here are non-critical and may be dropped if subscribers are full.
// Do NOT use this for durable/critical state notifications.
func (b *EventBus) PublishTaskEvent(taskID, eventType string, payload interface{}) {
	b.mu.RLock()
	subs := b.subscriptions[taskID]
	if len(subs) == 0 {
		b.mu.RUnlock()
		return
	}
	copies := make([]*taskSubscription, 0, len(subs))
	for _, s := range subs {
		copies = append(copies, s)
	}
	b.mu.RUnlock()

	evt := TaskEvent{
		TaskID:    taskID,
		EventType: eventType,
		Payload:   payload,
		EmittedAt: time.Now().Format(desktopPetTimeFormat),
	}
	for _, s := range copies {
		select {
		case s.events <- evt:
		default:
		}
	}
}

var defaultEventBus = NewEventBus()

// DefaultEventBus returns the process-wide ephemeral DesktopPet task stream.
// WARNING: This is for domain-internal progress display only.
// It does NOT represent durable event infrastructure.
func DefaultEventBus() *EventBus { return defaultEventBus }

// PublishTaskEvent publishes to the default ephemeral task event stream.
// This is a convenience wrapper around DefaultEventBus().PublishTaskEvent.
func PublishTaskEvent(taskID, eventType string, payload interface{}) {
	defaultEventBus.PublishTaskEvent(taskID, eventType, payload)
}
