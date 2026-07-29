package permission

import (
	"context"
	"sync"
	"time"
)

type AuditAction string

const (
	AuditEvaluate AuditAction = "evaluate"
	AuditGrant    AuditAction = "grant"
	AuditRevoke   AuditAction = "revoke"
	AuditDeny     AuditAction = "deny"
)

type PermissionAuditEntry struct {
	Action       AuditAction        `json:"action"`
	Subject      PermissionSubject  `json:"subject"`
	PermissionID string             `json:"permissionId"`
	Decision     PermissionDecision `json:"decision"`
	GrantID      string             `json:"grantId,omitempty"`
	Reasons      []PermissionReason `json:"reasons,omitempty"`
	Timestamp    time.Time          `json:"timestamp"`
}

type PermissionAuditRecorder struct {
	entries []PermissionAuditEntry
	mu      sync.RWMutex
}

func NewPermissionAuditRecorder() *PermissionAuditRecorder {
	return &PermissionAuditRecorder{
		entries: make([]PermissionAuditEntry, 0),
	}
}

func (r *PermissionAuditRecorder) RecordEvaluation(ctx context.Context, request PermissionEvaluationRequest, result PermissionEvaluationResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, PermissionAuditEntry{
		Action:       AuditEvaluate,
		Subject:      request.Subject,
		PermissionID: "",
		Decision:     result.Decision,
		Reasons:      result.Reasons,
		Timestamp:    time.Now(),
	})
}

func (r *PermissionAuditRecorder) RecordGrant(ctx context.Context, grant PermissionGrant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, PermissionAuditEntry{
		Action:       AuditGrant,
		Subject:      grant.Subject,
		PermissionID: grant.PermissionID,
		Decision:     grant.Decision,
		GrantID:      grant.GrantID,
		Timestamp:    time.Now(),
	})
}

func (r *PermissionAuditRecorder) RecordRevoke(ctx context.Context, grantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, PermissionAuditEntry{
		Action:    AuditRevoke,
		GrantID:   grantID,
		Timestamp: time.Now(),
	})
}

func (r *PermissionAuditRecorder) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func (r *PermissionAuditRecorder) Entries() []PermissionAuditEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]PermissionAuditEntry, len(r.entries))
	copy(result, r.entries)
	return result
}
