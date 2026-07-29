package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type StartupAuditEvent struct {
	StartupID   string
	ComponentID string
	Phase       StartupPhase
	Status      string
	Attempt     int
	ErrorCode   string
	Error       string
	Timestamp   time.Time
	Metadata    map[string]any
}

type ShutdownAuditEvent struct {
	ShutdownID  string
	ComponentID string
	Phase       ShutdownPhase
	Status      string
	Reason      string
	Error       string
	Clean       bool
	Timestamp   time.Time
	Metadata    map[string]any
}

type ReadinessAuditEvent struct {
	Ready      bool
	StartupID  string
	Components map[string]string
	Timestamp  time.Time
	Reason     string
}

type LifecycleAuditWriter interface {
	RecordStartupEvent(ctx context.Context, event StartupAuditEvent)
	RecordShutdownEvent(ctx context.Context, event ShutdownAuditEvent)
	RecordReadinessEvent(ctx context.Context, event ReadinessAuditEvent)
}

type InMemoryAuditWriter struct {
	mu        sync.Mutex
	startup   []StartupAuditEvent
	shutdown  []ShutdownAuditEvent
	readiness []ReadinessAuditEvent
}

func NewInMemoryAuditWriter() *InMemoryAuditWriter {
	return &InMemoryAuditWriter{}
}

func (w *InMemoryAuditWriter) RecordStartupEvent(_ context.Context, event StartupAuditEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.startup = append(w.startup, event)
}

func (w *InMemoryAuditWriter) RecordShutdownEvent(_ context.Context, event ShutdownAuditEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shutdown = append(w.shutdown, event)
}

func (w *InMemoryAuditWriter) RecordReadinessEvent(_ context.Context, event ReadinessAuditEvent) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.readiness = append(w.readiness, event)
}

func (w *InMemoryAuditWriter) StartupEvents() []StartupAuditEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]StartupAuditEvent{}, w.startup...)
}

func (w *InMemoryAuditWriter) ShutdownEvents() []ShutdownAuditEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ShutdownAuditEvent{}, w.shutdown...)
}

func (w *InMemoryAuditWriter) ReadinessEvents() []ReadinessAuditEvent {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ReadinessAuditEvent{}, w.readiness...)
}

var _ LifecycleAuditWriter = (*InMemoryAuditWriter)(nil)

type NoopAuditWriter struct{}

func (NoopAuditWriter) RecordStartupEvent(context.Context, StartupAuditEvent)     {}
func (NoopAuditWriter) RecordShutdownEvent(context.Context, ShutdownAuditEvent)   {}
func (NoopAuditWriter) RecordReadinessEvent(context.Context, ReadinessAuditEvent) {}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
