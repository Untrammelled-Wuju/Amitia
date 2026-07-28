package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type UISubscription struct {
	ID            string
	SessionID     string
	DataSourceID  string
	Callback      func(json.RawMessage)
	RatePerMinute int
	LastUpdate    time.Time
	Active        bool
	cancel        context.CancelFunc
}

type UISubscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[string]*UISubscription
	rateLimiter   map[string][]time.Time
}

func NewUISubscriptionManager() *UISubscriptionManager {
	return &UISubscriptionManager{
		subscriptions: make(map[string]*UISubscription),
		rateLimiter:   make(map[string][]time.Time),
	}
}

func (m *UISubscriptionManager) Subscribe(sessionID, sourceID string, callback func(json.RawMessage), ratePerMinute int) (*UISubscription, error) {
	if ratePerMinute <= 0 {
		ratePerMinute = 10
	}
	if ratePerMinute > 60 {
		ratePerMinute = 60
	}

	sub := &UISubscription{
		ID:            fmt.Sprintf("sub-%s", uuid.NewString()),
		SessionID:     sessionID,
		DataSourceID:  sourceID,
		Callback:      callback,
		RatePerMinute: ratePerMinute,
		LastUpdate:    time.Now().UTC(),
		Active:        true,
	}

	m.mu.Lock()
	m.subscriptions[sub.ID] = sub
	rateKey := sessionID + ":" + sourceID
	if _, exists := m.rateLimiter[rateKey]; !exists {
		m.rateLimiter[rateKey] = make([]time.Time, 0, ratePerMinute)
	}
	m.mu.Unlock()

	return sub, nil
}

func (m *UISubscriptionManager) Unsubscribe(subscriptionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subscriptions[subscriptionID]
	if !ok {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}
	sub.Active = false
	if sub.cancel != nil {
		sub.cancel()
	}
	delete(m.subscriptions, subscriptionID)
	rateKey := sub.SessionID + ":" + sub.DataSourceID
	delete(m.rateLimiter, rateKey)
	return nil
}

func (m *UISubscriptionManager) UnsubscribeAll(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, sub := range m.subscriptions {
		if sub.SessionID == sessionID {
			sub.Active = false
			if sub.cancel != nil {
				sub.cancel()
			}
			rateKey := sub.SessionID + ":" + sub.DataSourceID
			delete(m.rateLimiter, rateKey)
			delete(m.subscriptions, id)
		}
	}
}

func (m *UISubscriptionManager) Publish(sessionID, sourceID string, payload json.RawMessage) {
	m.mu.RLock()
	var targets []*UISubscription
	for _, sub := range m.subscriptions {
		if sub.SessionID == sessionID && sub.DataSourceID == sourceID && sub.Active {
			targets = append(targets, sub)
		}
	}
	m.mu.RUnlock()

	now := time.Now().UTC()
	for _, sub := range targets {
		if !m.checkRateLimit(sub.SessionID, sub.DataSourceID, sub.RatePerMinute, now) {
			continue
		}
		sub.LastUpdate = now
		if sub.Callback != nil {
			sub.Callback(payload)
		}
	}
}

func (m *UISubscriptionManager) checkRateLimit(sessionID, sourceID string, ratePerMinute int, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rateKey := sessionID + ":" + sourceID
	timestamps := m.rateLimiter[rateKey]
	cutoff := now.Add(-time.Minute)
	filtered := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	if len(filtered) >= ratePerMinute {
		m.rateLimiter[rateKey] = filtered
		return false
	}
	filtered = append(filtered, now)
	m.rateLimiter[rateKey] = filtered
	return true
}
