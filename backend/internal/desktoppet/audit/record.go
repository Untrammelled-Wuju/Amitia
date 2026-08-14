// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package audit

import (
	"time"

	"github.com/u-ai/backend/internal/auth"
)

type AuditAction string

const (
	ActionLoginSuccess          AuditAction = "auth.login.success"
	ActionLoginFailure          AuditAction = "auth.login.failure"
	ActionAccessDenied          AuditAction = "security.access_denied"
	ActionResourceDelete        AuditAction = "resource.delete"
	ActionImportUpload          AuditAction = "import.upload"
	ActionImportConsume         AuditAction = "import.consume"
	ActionReleaseRevoke         AuditAction = "release.revoke"
	ActionInstallationUninstall AuditAction = "installation.uninstall"
	ActionRevisionActivate      AuditAction = "revision.activate"
	ActionCandidateAccept       AuditAction = "candidate.accept"
	ActionQualityManualReview   AuditAction = "quality.manual_review"
	ActionBehaviorGlobalConfig  AuditAction = "behavior.global_config"
	ActionRuntimeManualCommand  AuditAction = "runtime.manual_command"
	ActionMigrationStart        AuditAction = "migration.start"
	ActionMigrationComplete     AuditAction = "migration.complete"
	ActionMigrationFailure      AuditAction = "migration.failure"
	ActionDoctorRepair          AuditAction = "doctor.repair"
	ActionDeadLetterReplay      AuditAction = "dead_letter.replay"
)

type AuditResult string

const (
	ResultSuccess AuditResult = "success"
	ResultFailure AuditResult = "failure"
	ResultDenied  AuditResult = "denied"
)

type ActorSnapshot struct {
	ActorType     string   `json:"actorType"`
	UserID        string   `json:"userId"`
	DeviceID      string   `json:"deviceId,omitempty"`
	Roles         []string `json:"roles,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
	AuthMethod    string   `json:"authMethod"`
	CorrelationID string   `json:"correlationId"`
	RequestID     string   `json:"requestId"`
}

type SecurityAuditRecord struct {
	ID            string        `json:"id"`
	Actor         ActorSnapshot `json:"actor"`
	Action        AuditAction   `json:"action"`
	ResourceType  string        `json:"resourceType"`
	ResourceID    string        `json:"resourceId"`
	Result        AuditResult   `json:"result"`
	Reason        string        `json:"reason,omitempty"`
	CorrelationID string        `json:"correlationId"`
	MetadataJSON  string        `json:"metadata,omitempty"`
	OccurredAt    string        `json:"occurredAt"`
}

func NewSnapshot(actor *auth.ActorContext) ActorSnapshot {
	if actor == nil {
		return ActorSnapshot{}
	}
	return ActorSnapshot{
		ActorType:     string(actor.ActorType),
		UserID:        string(actor.UserID),
		DeviceID:      string(actor.DeviceID),
		Roles:         actor.Roles,
		Permissions:   actor.Permissions,
		AuthMethod:    actor.AuthMethod,
		CorrelationID: actor.CorrelationID,
		RequestID:     actor.RequestID,
	}
}

func NewAuditRecord(action AuditAction, resourceType, resourceID string, result AuditResult, reason string, actor *auth.ActorContext, metadataJSON string) *SecurityAuditRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return &SecurityAuditRecord{
		ID:            generateAuditID(),
		Actor:         NewSnapshot(actor),
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Result:        result,
		Reason:        reason,
		CorrelationID: actor.CorrelationID,
		MetadataJSON:  metadataJSON,
		OccurredAt:    now,
	}
}

func generateAuditID() string {
	return "audit_" + time.Now().UTC().Format("20060102T150405") + "_" + randomHex(8)
}

func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[(time.Now().UnixNano()+int64(i))%16]
	}
	return string(b)
}

type AuditRecorder interface {
	Record(record *SecurityAuditRecord) error
}
