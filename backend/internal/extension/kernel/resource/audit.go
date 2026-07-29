package resource

import (
	"context"
	"time"
)

type AuditEventType string

const (
	AuditResourceCreated     AuditEventType = "resource.created"
	AuditResourceUpdated     AuditEventType = "resource.updated"
	AuditResourceDeleted     AuditEventType = "resource.deleted"
	AuditResourceTransferred AuditEventType = "resource.transferred"
	AuditResourceReleased    AuditEventType = "resource.released"
	AuditResourceOrphaned    AuditEventType = "resource.orphaned"
	AuditCleanupStarted      AuditEventType = "cleanup.started"
	AuditCleanupCompleted    AuditEventType = "cleanup.completed"
	AuditCleanupFailed       AuditEventType = "cleanup.failed"
)

type ResourceAuditEvent struct {
	EventID    string         `json:"event_id"`
	EventType  AuditEventType `json:"event_type"`
	ResourceID string         `json:"resource_id"`
	Owner      ResourceOwner  `json:"owner"`
	Actor      string         `json:"actor"`
	Timestamp  time.Time      `json:"timestamp"`
	Details    map[string]any `json:"details,omitempty"`
}

type AuditWriter interface {
	WriteAudit(ctx context.Context, event ResourceAuditEvent) error
}

type defaultAuditWriter struct{}

func NewDefaultAuditWriter() AuditWriter {
	return &defaultAuditWriter{}
}

func (w *defaultAuditWriter) WriteAudit(ctx context.Context, event ResourceAuditEvent) error {
	return nil
}
