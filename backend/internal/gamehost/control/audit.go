package control

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type TransitionAuditResult string

const (
	AuditResultSuccess TransitionAuditResult = "success"
	AuditResultDenied  TransitionAuditResult = "denied"
	AuditResultNoop    TransitionAuditResult = "noop"
	AuditResultError   TransitionAuditResult = "error"
)

type AuthorityAuditEvent struct {
	RuntimeID domain.RuntimeInstanceID
	PluginID  domain.PluginID

	PreviousMode  domain.ControlMode
	NewMode       domain.ControlMode
	PreviousEpoch uint64
	NewEpoch      uint64

	Actor  TransitionActor
	Reason TransitionReason

	Result TransitionAuditResult
	Error  string

	Timestamp time.Time
}

type AuthorityAuditSink interface {
	RecordTransition(event AuthorityAuditEvent)
}

type InMemoryAuthorityAuditSink struct {
	mu     sync.RWMutex
	events []AuthorityAuditEvent
}

func NewInMemoryAuthorityAuditSink() *InMemoryAuthorityAuditSink {
	return &InMemoryAuthorityAuditSink{
		events: make([]AuthorityAuditEvent, 0),
	}
}

func (s *InMemoryAuthorityAuditSink) RecordTransition(event AuthorityAuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *InMemoryAuthorityAuditSink) Events() []AuthorityAuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AuthorityAuditEvent, len(s.events))
	copy(result, s.events)
	return result
}

func (s *InMemoryAuthorityAuditSink) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *InMemoryAuthorityAuditSink) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = make([]AuthorityAuditEvent, 0)
}

type NoopAuthorityAuditSink struct{}

func (NoopAuthorityAuditSink) RecordTransition(AuthorityAuditEvent) {}
