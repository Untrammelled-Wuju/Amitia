package event_bus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type EventType string

type Event struct {
	EventID     string
	Type        EventType
	Source      string
	Owner       string
	Payload     json.RawMessage
	TraceID     string
	ParentID    string
	ScopeID     string
	OccurredAt  time.Time
	PublishedAt time.Time
	Generation  int64
}

type Subscription struct {
	SubscriptionID string
	Subscriber     string
	EventType      EventType
	Owner          string
	Filter         string
	Handler        Handler
	MaxInflight    int
	RetryLimit     int
	Timeout        time.Duration
	Priority       int
	CreatedAt      time.Time
	Active         bool
}

type Handler func(ctx context.Context, event Event) error

type Delivery struct {
	DeliveryID     string
	EventID        string
	SubscriptionID string
	Status         DeliveryStatus
	Attempts       int
	LastError      string
	StartedAt      time.Time
	FinishedAt     *time.Time
	NextAttemptAt  *time.Time
}

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryDelivering DeliveryStatus = "delivering"
	DeliverySucceeded  DeliveryStatus = "succeeded"
	DeliveryFailed     DeliveryStatus = "failed"
	DeliveryDeadLetter DeliveryStatus = "dead_letter"
)

type Schema struct {
	EventType EventType
	Version   int
	Schema    json.RawMessage
	Owner     string
}

type PublishResult struct {
	EventID     string
	AcceptCount int
	RejectCount int
	Deliveries  []Delivery
}

type Bus interface {
	RegisterSchema(ctx context.Context, schema Schema) error
	Publish(ctx context.Context, event Event) (PublishResult, error)
	Subscribe(ctx context.Context, subscription Subscription) (Subscription, error)
	Unsubscribe(ctx context.Context, subscriptionID string) error
	GetDelivery(ctx context.Context, deliveryID string) (Delivery, error)
	Redrive(ctx context.Context, deliveryID string) error
}

var (
	ErrSchemaNotFound       = errors.New("event_bus: schema not found")
	ErrSchemaConflict       = errors.New("event_bus: schema conflict")
	ErrSubscriptionNotFound = errors.New("event_bus: subscription not found")
	ErrInvalidEvent         = errors.New("event_bus: invalid event")
	ErrNoSubscribers        = errors.New("event_bus: no subscribers")
	ErrDeliveryFailed       = errors.New("event_bus: delivery failed")
	ErrDeadLetter           = errors.New("event_bus: dead letter")
)

type DefaultBus struct {
	mu            sync.RWMutex
	schemas       map[EventType]Schema
	subscriptions map[string]*Subscription
	byType        map[EventType][]string
	deliveries    map[string]*Delivery
	deadLetter    map[string][]*Delivery
	muDelivery    sync.Mutex
	depth         int
	maxDepth      int
}

func NewDefaultBus() *DefaultBus {
	return &DefaultBus{
		schemas:       make(map[EventType]Schema),
		subscriptions: make(map[string]*Subscription),
		byType:        make(map[EventType][]string),
		deliveries:    make(map[string]*Delivery),
		deadLetter:    make(map[string][]*Delivery),
		maxDepth:      16,
	}
}

func (b *DefaultBus) RegisterSchema(_ context.Context, schema Schema) error {
	if schema.EventType == "" {
		return ErrInvalidEvent
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if existing, ok := b.schemas[schema.EventType]; ok && existing.Version == schema.Version {
		return fmt.Errorf("%w: %s v%d", ErrSchemaConflict, schema.EventType, schema.Version)
	}
	b.schemas[schema.EventType] = schema
	return nil
}

func (b *DefaultBus) Publish(ctx context.Context, event Event) (PublishResult, error) {
	if event.Type == "" {
		return PublishResult{}, ErrInvalidEvent
	}
	if event.EventID == "" {
		return PublishResult{}, fmt.Errorf("%w: missing event id", ErrInvalidEvent)
	}
	b.mu.RLock()
	if _, ok := b.schemas[event.Type]; !ok {
		b.mu.RUnlock()
		return PublishResult{}, fmt.Errorf("%w: %s", ErrSchemaNotFound, event.Type)
	}
	subs := b.matchingSubscriptionsLocked(event)
	b.mu.RUnlock()
	if len(subs) == 0 {
		return PublishResult{EventID: event.EventID}, nil
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	event.PublishedAt = time.Now().UTC()
	result := PublishResult{EventID: event.EventID}
	for _, sub := range subs {
		delivery := b.deliver(ctx, event, sub)
		result.Deliveries = append(result.Deliveries, delivery)
		if delivery.Status == DeliverySucceeded {
			result.AcceptCount++
		} else {
			result.RejectCount++
		}
	}
	return result, nil
}

func (b *DefaultBus) Subscribe(_ context.Context, subscription Subscription) (Subscription, error) {
	if subscription.EventType == "" {
		return Subscription{}, ErrInvalidEvent
	}
	if subscription.Handler == nil {
		return Subscription{}, fmt.Errorf("%w: handler required", ErrInvalidEvent)
	}
	if subscription.SubscriptionID == "" {
		subscription.SubscriptionID = fmt.Sprintf("sub-%s", uuid.NewString())
	}
	if subscription.MaxInflight == 0 {
		subscription.MaxInflight = 16
	}
	if subscription.RetryLimit == 0 {
		subscription.RetryLimit = 3
	}
	if subscription.Timeout == 0 {
		subscription.Timeout = 5 * time.Second
	}
	subscription.CreatedAt = time.Now().UTC()
	subscription.Active = true
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions[subscription.SubscriptionID] = &subscription
	b.byType[subscription.EventType] = append(b.byType[subscription.EventType], subscription.SubscriptionID)
	return subscription, nil
}

func (b *DefaultBus) Unsubscribe(_ context.Context, subscriptionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	sub, ok := b.subscriptions[subscriptionID]
	if !ok {
		return ErrSubscriptionNotFound
	}
	sub.Active = false
	delete(b.subscriptions, subscriptionID)
	subs := b.byType[sub.EventType]
	for i, id := range subs {
		if id == subscriptionID {
			b.byType[sub.EventType] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	return nil
}

func (b *DefaultBus) GetDelivery(_ context.Context, deliveryID string) (Delivery, error) {
	b.muDelivery.Lock()
	defer b.muDelivery.Unlock()
	d, ok := b.deliveries[deliveryID]
	if !ok {
		for _, deads := range b.deadLetter {
			for _, dead := range deads {
				if dead.DeliveryID == deliveryID {
					return *dead, nil
				}
			}
		}
		return Delivery{}, fmt.Errorf("%w: %s", ErrDeliveryFailed, deliveryID)
	}
	return *d, nil
}

func (b *DefaultBus) Redrive(ctx context.Context, deliveryID string) error {
	b.muDelivery.Lock()
	var target *Delivery
	var targetKey string
	for k, deads := range b.deadLetter {
		for i, dead := range deads {
			if dead.DeliveryID == deliveryID {
				target = dead
				targetKey = k
				b.deadLetter[targetKey] = append(b.deadLetter[targetKey][:i], b.deadLetter[targetKey][i+1:]...)
				break
			}
		}
		if target != nil {
			break
		}
	}
	b.muDelivery.Unlock()
	if target == nil {
		return fmt.Errorf("%w: %s", ErrDeadLetter, deliveryID)
	}
	b.mu.RLock()
	sub := b.subscriptions[target.SubscriptionID]
	b.mu.RUnlock()
	if sub == nil {
		return ErrSubscriptionNotFound
	}
	target.Status = DeliveryPending
	target.Attempts = 0
	target.LastError = ""
	now := time.Now().UTC()
	target.StartedAt = now
	b.deliver(ctx, Event{EventID: target.EventID}, sub)
	return nil
}

func (b *DefaultBus) deliver(ctx context.Context, event Event, sub *Subscription) Delivery {
	deliveryID := fmt.Sprintf("del-%s-%s", event.EventID, sub.SubscriptionID)
	delivery := Delivery{
		DeliveryID:     deliveryID,
		EventID:        event.EventID,
		SubscriptionID: sub.SubscriptionID,
		Status:         DeliveryDelivering,
		StartedAt:      time.Now().UTC(),
	}
	var lastErr error
	for attempt := 1; attempt <= sub.RetryLimit+1; attempt++ {
		delivery.Attempts = attempt
		callCtx, cancel := context.WithTimeout(ctx, sub.Timeout)
		err := sub.Handler(callCtx, event)
		cancel()
		if err == nil {
			delivery.Status = DeliverySucceeded
			now := time.Now().UTC()
			delivery.FinishedAt = &now
			break
		}
		lastErr = err
		if attempt > sub.RetryLimit {
			delivery.Status = DeliveryFailed
			now := time.Now().UTC()
			delivery.FinishedAt = &now
			delivery.LastError = err.Error()
		} else {
			backoff := time.Duration(attempt) * 100 * time.Millisecond
			next := time.Now().UTC().Add(backoff)
			delivery.NextAttemptAt = &next
		}
	}
	if delivery.Status == DeliveryFailed {
		delivery.Status = DeliveryDeadLetter
		b.muDelivery.Lock()
		b.deadLetter[event.EventID] = append(b.deadLetter[event.EventID], &delivery)
		b.muDelivery.Unlock()
	} else {
		b.muDelivery.Lock()
		b.deliveries[deliveryID] = &delivery
		b.muDelivery.Unlock()
	}
	_ = lastErr
	return delivery
}

func (b *DefaultBus) matchingSubscriptionsLocked(event Event) []*Subscription {
	ids := b.byType[event.Type]
	var subs []*Subscription
	for _, id := range ids {
		sub := b.subscriptions[id]
		if sub == nil || !sub.Active {
			continue
		}
		if sub.Owner != "" && sub.Owner != event.Owner {
			continue
		}
		subs = append(subs, sub)
	}
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].Priority != subs[j].Priority {
			return subs[i].Priority > subs[j].Priority
		}
		return subs[i].SubscriptionID < subs[j].SubscriptionID
	})
	return subs
}

var _ Bus = (*DefaultBus)(nil)
