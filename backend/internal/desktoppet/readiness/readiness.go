// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package readiness

import (
	"sync"
	"time"
)

type SystemStatus string

const (
	StatusReady       SystemStatus = "ready"
	StatusDegraded    SystemStatus = "degraded"
	StatusBlocked     SystemStatus = "blocked"
	StatusMaintenance SystemStatus = "maintenance"
)

type CheckResult struct {
	Name     string       `json:"name"`
	Status   SystemStatus `json:"status"`
	Required bool         `json:"required"`
	Message  string       `json:"message,omitempty"`
	Duration int64        `json:"durationMs"`
}

type ReadinessSnapshot struct {
	OverallStatus SystemStatus           `json:"overallStatus"`
	Checks        map[string]CheckResult `json:"checks"`
	BlockingCount int                    `json:"blockingCount"`
	DegradedCount int                    `json:"degradedCount"`
	Timestamp     string                 `json:"timestamp"`
}

type ReadinessChecker interface {
	Name() string
	IsRequired() bool
	Evaluate() (SystemStatus, string)
}

type ReadinessService struct {
	mu       sync.RWMutex
	checkers []ReadinessChecker
	nowFn    func() time.Time
}

func NewReadinessService() *ReadinessService {
	return &ReadinessService{
		checkers: make([]ReadinessChecker, 0),
		nowFn:    time.Now,
	}
}

func (s *ReadinessService) Register(checker ReadinessChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkers = append(s.checkers, checker)
}

func (s *ReadinessService) Snapshot() ReadinessSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	checks := make(map[string]CheckResult)
	var blocking, degraded int
	overall := StatusReady
	for _, c := range s.checkers {
		start := s.nowFn()
		status, message := c.Evaluate()
		duration := s.nowFn().Sub(start).Milliseconds()
		result := CheckResult{
			Name:     c.Name(),
			Status:   status,
			Required: c.IsRequired(),
			Message:  message,
			Duration: duration,
		}
		checks[c.Name()] = result
		switch status {
		case StatusBlocked:
			if c.IsRequired() {
				blocking++
			} else {
				degraded++
			}
		case StatusDegraded:
			degraded++
		}
	}
	if blocking > 0 {
		overall = StatusBlocked
	} else if degraded > 0 {
		overall = StatusDegraded
	}
	return ReadinessSnapshot{
		OverallStatus: overall,
		Checks:        checks,
		BlockingCount: blocking,
		DegradedCount: degraded,
		Timestamp:     s.nowFn().UTC().Format(time.RFC3339Nano),
	}
}

type MaintenanceGateFunc func() bool

type SafeModeController struct {
	mu         sync.RWMutex
	inSafeMode bool
	reason     string
	setAt      time.Time
}

func NewSafeModeController() *SafeModeController {
	return &SafeModeController{}
}

func (s *SafeModeController) Enter(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inSafeMode = true
	s.reason = reason
	s.setAt = time.Now()
}

func (s *SafeModeController) Exit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inSafeMode = false
	s.reason = ""
	s.setAt = time.Time{}
}

func (s *SafeModeController) IsInSafeMode() (bool, string, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inSafeMode, s.reason, s.setAt
}

type WorkerRegistryEntry struct {
	WorkerName     string    `json:"workerName"`
	Required       bool      `json:"required"`
	StartedAt      time.Time `json:"startedAt"`
	LastHeartbeat  time.Time `json:"lastHeartbeat"`
	Status         string    `json:"status"`
	DependencyHash string    `json:"dependencyHash,omitempty"`
}

type WorkerRegistry struct {
	mu      sync.RWMutex
	entries map[string]WorkerRegistryEntry
	nowFn   func() time.Time
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		entries: make(map[string]WorkerRegistryEntry),
		nowFn:   time.Now,
	}
}

func (r *WorkerRegistry) Register(name string, required bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[name] = WorkerRegistryEntry{
		WorkerName: name,
		Required:   required,
		StartedAt:  r.nowFn(),
		Status:     "started",
	}
}

func (r *WorkerRegistry) Heartbeat(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.LastHeartbeat = r.nowFn()
		entry.Status = "running"
		r.entries[name] = entry
	}
}

func (r *WorkerRegistry) Stop(name string, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[name]; ok {
		entry.LastHeartbeat = r.nowFn()
		entry.Status = status
		r.entries[name] = entry
	}
}

func (r *WorkerRegistry) GetStatus() []WorkerRegistryEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WorkerRegistryEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		result = append(result, entry)
	}
	return result
}

func (r *WorkerRegistry) AllRequiredHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.entries {
		if entry.Required && entry.Status != "started" && entry.Status != "running" {
			return false
		}
	}
	return true
}
