package kernel

import (
	"context"
	"sync"
	"time"
)

// RuntimeState provides a unified snapshot of the kernel's runtime state,
// aggregating status from execution, capabilities, providers, and UI systems.
type RuntimeState struct {
	mu        sync.RWMutex
	status    RuntimeStatus
	summary   RuntimeSummary
	issues    []RuntimeIssue
	updatedAt time.Time
}

// RuntimeStatus indicates the overall health of the kernel runtime.
type RuntimeStatus string

const (
	RuntimeStatusHealthy   RuntimeStatus = "healthy"
	RuntimeStatusDegraded  RuntimeStatus = "degrated"
	RuntimeStatusStarting  RuntimeStatus = "starting"
	RuntimeStatusUnhealthy RuntimeStatus = "unhealthy"
)

// RuntimeSummary provides aggregated counters.
type RuntimeSummary struct {
	ActiveExecutions    int `json:"activeExecutions"`
	ActiveResumes       int `json:"activeResumes"`
	RegisteredTools     int `json:"registeredTools"`
	RegisteredProviders int `json:"registeredProviders"`
	ActiveOperations    int `json:"activeOperations"`
	PendingOperations   int `json:"pendingOperations"`
}

// RuntimeIssue records a runtime-level concern.
type RuntimeIssue struct {
	Code      string    `json:"code"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// NewRuntimeState creates an initial RuntimeState.
func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		status:    RuntimeStatusStarting,
		issues:    make([]RuntimeIssue, 0),
		updatedAt: time.Now().UTC(),
	}
}

// Status returns the current runtime status.
func (s *RuntimeState) Status() RuntimeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// Summary returns the current summary counters.
func (s *RuntimeState) Summary() RuntimeSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.summary
}

// Issues returns recorded runtime issues.
func (s *RuntimeState) Issues() []RuntimeIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()
	issues := make([]RuntimeIssue, len(s.issues))
	copy(issues, s.issues)
	return issues
}

// Refresh recomputes the runtime state from the kernel container.
func (s *RuntimeState) Refresh(ctx context.Context, c *Container) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = RuntimeStatusHealthy
	s.issues = nil
	s.updatedAt = time.Now().UTC()

	if c == nil {
		s.status = RuntimeStatusUnhealthy
		s.issues = append(s.issues, RuntimeIssue{
			Code: "container_unavailable", Severity: "critical",
			Message: "kernel container is nil", Source: "runtime_state",
			Timestamp: s.updatedAt,
		})
		return
	}

	if c.ExecutionService != nil {
		s.summary.ActiveExecutions = len(c.ExecutionService.ListActiveExecutions())
		s.summary.ActiveResumes = len(c.ExecutionService.ListResumes())
	}

	if c.ToolRegistry != nil {
		s.summary.RegisteredTools = c.ToolRegistry.Count()
	}

	if c.CapabilityService != nil {
		s.summary.RegisteredProviders = len(c.CapabilityService.ListProviders())
	}

	if len(s.issues) > 0 {
		s.status = RuntimeStatusDegraded
	}
}

// SetStatus manually overrides the runtime status.
func (s *RuntimeState) SetStatus(status RuntimeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.updatedAt = time.Now().UTC()
}

// AddIssue records a runtime issue.
func (s *RuntimeState) AddIssue(code, severity, message, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues = append(s.issues, RuntimeIssue{
		Code: code, Severity: severity,
		Message: message, Source: source,
		Timestamp: time.Now().UTC(),
	})
}
