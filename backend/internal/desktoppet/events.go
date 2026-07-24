// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
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

type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]*taskSubscription
}

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

func DefaultEventBus() *EventBus { return defaultEventBus }

func PublishTaskEvent(taskID, eventType string, payload interface{}) {
	defaultEventBus.PublishTaskEvent(taskID, eventType, payload)
}
