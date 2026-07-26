package package_security

import (
	"context"
	"sync"
	"time"
)

type AuditEventType string

const (
	AuditPackageInspect     AuditEventType = "package_inspect"
	AuditPackageReject      AuditEventType = "package_reject"
	AuditSignatureFailure   AuditEventType = "signature_failure"
	AuditUnknownPublisher   AuditEventType = "unknown_publisher"
	AuditUserTrustPublisher AuditEventType = "user_trust_publisher"
	AuditCommit             AuditEventType = "commit"
	AuditRollback           AuditEventType = "rollback"
	AuditRecovery           AuditEventType = "recovery"
	AuditCleanupFailure     AuditEventType = "cleanup_failure"
	AuditHashMismatch       AuditEventType = "hash_mismatch"
	AuditDevModeBypass      AuditEventType = "dev_mode_bypass"
)

type ResourceAuditEvent struct {
	EventID     string         `json:"event_id"`
	EventType   AuditEventType `json:"event_type"`
	PackageID   string         `json:"package_id,omitempty"`
	Version     string         `json:"version,omitempty"`
	PublisherID string         `json:"publisher_id,omitempty"`
	ReportID    string         `json:"report_id,omitempty"`
	StagingID   string         `json:"staging_id,omitempty"`
	SnapshotID  string         `json:"snapshot_id,omitempty"`
	OperationID string         `json:"operation_id,omitempty"`
	Details     string         `json:"details,omitempty"`
	Success     bool           `json:"success"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AuditWriter interface {
	WriteAuditEvent(ctx context.Context, event ResourceAuditEvent) error
}

type MemoryAuditWriter struct {
	mu     sync.RWMutex
	events []ResourceAuditEvent
}

func NewMemoryAuditWriter() *MemoryAuditWriter {
	return &MemoryAuditWriter{
		events: make([]ResourceAuditEvent, 0),
	}
}

var _ AuditWriter = (*MemoryAuditWriter)(nil)

func (w *MemoryAuditWriter) WriteAuditEvent(ctx context.Context, event ResourceAuditEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	w.events = append(w.events, event)
	return nil
}

func (w *MemoryAuditWriter) GetEvents(ctx context.Context) []ResourceAuditEvent {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]ResourceAuditEvent, len(w.events))
	copy(result, w.events)
	return result
}

func (w *MemoryAuditWriter) GetEventsByType(ctx context.Context, eventType AuditEventType) []ResourceAuditEvent {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var result []ResourceAuditEvent
	for _, e := range w.events {
		if e.EventType == eventType {
			result = append(result, e)
		}
	}
	return result
}
