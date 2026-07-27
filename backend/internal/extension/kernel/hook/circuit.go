package hook

import (
	"sync"
	"time"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type circuitEntry struct {
	state            CircuitState
	consecutiveFails int
	lastFailTime     time.Time
	lastFailCode     HookErrorCode
	openedAt         time.Time
	totalFails       int64
	totalSuccess     int64
}

type CircuitBreaker struct {
	mu               sync.RWMutex
	entries          map[string]*circuitEntry
	failureThreshold int
	recoveryTimeout  time.Duration
	halfOpenMax      int
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		entries:          make(map[string]*circuitEntry),
		failureThreshold: 5,
		recoveryTimeout:  30 * time.Second,
		halfOpenMax:      1,
	}
}

func (cb *CircuitBreaker) State(contributionID string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	entry, ok := cb.entries[contributionID]
	if !ok {
		return CircuitClosed
	}
	cb.refreshStateLocked(entry)
	return entry.state
}

func (cb *CircuitBreaker) IsOpen(contributionID string) bool {
	return cb.State(contributionID) == CircuitOpen
}

func (cb *CircuitBreaker) RecordSuccess(contributionID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	entry := cb.getOrCreateLocked(contributionID)
	entry.totalSuccess++
	if entry.state == CircuitHalfOpen {
		entry.state = CircuitClosed
		entry.consecutiveFails = 0
	} else if entry.state == CircuitClosed {
		entry.consecutiveFails = 0
	}
}

func (cb *CircuitBreaker) RecordFailure(contributionID string, code HookErrorCode) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	entry := cb.getOrCreateLocked(contributionID)
	entry.consecutiveFails++
	entry.totalFails++
	entry.lastFailTime = time.Now()
	entry.lastFailCode = code
	if entry.state == CircuitHalfOpen {
		entry.state = CircuitOpen
		entry.openedAt = time.Now()
	} else if entry.state == CircuitClosed && entry.consecutiveFails >= cb.failureThreshold {
		entry.state = CircuitOpen
		entry.openedAt = time.Now()
	}
}

func (cb *CircuitBreaker) Reset(contributionID string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.entries, contributionID)
}

func (cb *CircuitBreaker) ResetAll() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.entries = make(map[string]*circuitEntry)
}

func (cb *CircuitBreaker) GetStats(contributionID string) CircuitStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	entry, ok := cb.entries[contributionID]
	if !ok {
		return CircuitStats{State: CircuitClosed}
	}
	cb.refreshStateLocked(entry)
	return CircuitStats{
		State:            entry.state,
		ConsecutiveFails: entry.consecutiveFails,
		TotalFails:       entry.totalFails,
		TotalSuccess:     entry.totalSuccess,
		LastFailCode:     string(entry.lastFailCode),
		LastFailTime:     entry.lastFailTime,
		OpenedAt:         entry.openedAt,
	}
}

type CircuitStats struct {
	State            CircuitState `json:"state"`
	ConsecutiveFails int          `json:"consecutiveFails"`
	TotalFails       int64        `json:"totalFails"`
	TotalSuccess     int64        `json:"totalSuccess"`
	LastFailCode     string       `json:"lastFailCode,omitempty"`
	LastFailTime     time.Time    `json:"lastFailTime,omitempty"`
	OpenedAt         time.Time    `json:"openedAt,omitempty"`
}

func (cb *CircuitBreaker) getOrCreateLocked(contributionID string) *circuitEntry {
	entry, ok := cb.entries[contributionID]
	if !ok {
		entry = &circuitEntry{state: CircuitClosed}
		cb.entries[contributionID] = entry
	}
	return entry
}

func (cb *CircuitBreaker) refreshStateLocked(entry *circuitEntry) {
	if entry.state == CircuitOpen && time.Since(entry.openedAt) > cb.recoveryTimeout {
		entry.state = CircuitHalfOpen
	}
}
