package proactive

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type OutputPriority int

const (
	PriorityLow    OutputPriority = 1
	PriorityNormal OutputPriority = 2
	PriorityHigh   OutputPriority = 3
	PriorityCrit   OutputPriority = 4
)

func (p OutputPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCrit:
		return "critical"
	default:
		return "unknown"
	}
}

func (p OutputPriority) IsLowPriority() bool {
	return p <= PriorityLow
}

type OutputLease struct {
	ID             string         `json:"id"`
	Priority       OutputPriority `json:"priority"`
	CharacterID    string         `json:"characterId"`
	ConversationID string         `json:"conversationId"`
	Channel        string         `json:"channel"`
	ChannelGroup   string         `json:"channelGroup,omitempty"`
	CorrelationID  string         `json:"correlationId"`
	ExpiresAt      time.Time      `json:"expiresAt"`
	AcquiredAt     time.Time      `json:"acquiredAt"`
	Cancelled      bool           `json:"cancelled"`
	mu             sync.Mutex
}

func (l *OutputLease) Cancel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Cancelled = true
}

func (l *OutputLease) IsCancelled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Cancelled
}

func (l *OutputLease) IsExpired(now time.Time) bool {
	return now.After(l.ExpiresAt)
}

func (l *OutputLease) IsValid(now time.Time) bool {
	return !l.IsCancelled() && !l.IsExpired(now)
}

type OutputLeaseManager struct {
	leases map[string]*OutputLease
	mu     sync.RWMutex
}

var GlobalLeaseManager = &OutputLeaseManager{
	leases: make(map[string]*OutputLease),
}

func (m *OutputLeaseManager) AcquireLease(priority OutputPriority, characterID, conversationID, channel, correlationID string, ttl time.Duration) *OutputLease {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	lease := &OutputLease{
		ID:             uuid.New().String(),
		Priority:       priority,
		CharacterID:    characterID,
		ConversationID: conversationID,
		Channel:        channel,
		CorrelationID:  correlationID,
		ExpiresAt:      now.Add(ttl),
		AcquiredAt:     now,
	}
	m.leases[lease.ID] = lease
	log.Printf("[OutputLease] acquired id=%s priority=%s char=%s channel=%s ttl=%v", lease.ID, priority, characterID, channel, ttl)
	return lease
}

func (m *OutputLeaseManager) ReleaseLease(leaseID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.leases, leaseID)
	log.Printf("[OutputLease] released id=%s", leaseID)
}

func (m *OutputLeaseManager) IsLeaseValid(leaseID string) bool {
	m.mu.RLock()
	lease, ok := m.leases[leaseID]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return lease.IsValid(time.Now())
}

func (m *OutputLeaseManager) CancelByUserInput(characterID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cancelled := 0
	now := time.Now()
	for id, lease := range m.leases {
		if lease.CharacterID != characterID {
			continue
		}
		if lease.IsExpired(now) {
			delete(m.leases, id)
			continue
		}
		if lease.Priority.IsLowPriority() {
			lease.Cancel()
			delete(m.leases, id)
			cancelled++
		}
	}
	if cancelled > 0 {
		log.Printf("[OutputLease] user input cancelled %d low-priority leases for char=%s", cancelled, characterID)
	}
	return cancelled
}

func (m *OutputLeaseManager) CancelByUserInputAllChannels(characterID string) int {
	return m.CancelByUserInput(characterID)
}

func (m *OutputLeaseManager) GetActiveLeases(characterID string) []*OutputLease {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	var active []*OutputLease
	for _, lease := range m.leases {
		if characterID != "" && lease.CharacterID != characterID {
			continue
		}
		if lease.IsExpired(now) {
			continue
		}
		if lease.IsCancelled() {
			continue
		}
		active = append(active, lease)
	}
	if active == nil {
		active = []*OutputLease{}
	}
	return active
}

func (m *OutputLeaseManager) CountActive(characterID string) int {
	return len(m.GetActiveLeases(characterID))
}

func (m *OutputLeaseManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	cleaned := 0
	for id, lease := range m.leases {
		if lease.IsExpired(now) || lease.IsCancelled() {
			delete(m.leases, id)
			cleaned++
		}
	}
	if cleaned > 0 {
		log.Printf("[OutputLease] cleaned %d expired/cancelled leases", cleaned)
	}
	return cleaned
}

func (m *OutputLeaseManager) PreemptLease(characterID string, minPriority OutputPriority) *OutputLease {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, lease := range m.leases {
		if lease.CharacterID != characterID {
			continue
		}
		if lease.IsExpired(now) {
			delete(m.leases, id)
			continue
		}
		if lease.Priority < minPriority && !lease.IsCancelled() {
			lease.Cancel()
			delete(m.leases, id)
			log.Printf("[OutputLease] preempted id=%s priority=%s for char=%s", id, lease.Priority, characterID)
			return lease
		}
	}
	return nil
}

func (m *OutputLeaseManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leases = make(map[string]*OutputLease)
}
