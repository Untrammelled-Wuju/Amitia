package permission

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type ApprovalDecision string

const (
	ApprovalDecisionApproved ApprovalDecision = "approved"
	ApprovalDecisionDenied   ApprovalDecision = "denied"
)

type PermissionApprovalRecordRequest struct {
	InvocationID        string
	PermissionIDs       []string
	ScopeSnapshotID     string
	Decision            ApprovalDecision
	ApprovedBy          string
	ExpiresAt           *time.Time
	ExecutionContext    PermissionExecutionContext
	ExecutionBindingKey string
	RiskLevel           string
}

type PermissionApprovalRecord struct {
	RecordID            string
	InvocationID        string
	PermissionIDs       []string
	ScopeSnapshotID     string
	Decision            ApprovalDecision
	ApprovedBy          string
	CreatedAt           time.Time
	ExpiresAt           *time.Time
	ExecutionContext    PermissionExecutionContext
	ExecutionBindingKey string
	RiskLevel           string
}

func (b *DefaultPermissionBroker) RecordApproval(ctx context.Context, request PermissionApprovalRecordRequest) (PermissionApprovalRecord, error) {
	if request.Decision != ApprovalDecisionApproved && request.Decision != ApprovalDecisionDenied {
		return PermissionApprovalRecord{}, fmt.Errorf("invalid approval decision: %s", request.Decision)
	}

	if request.ExecutionBindingKey == "" {
		return PermissionApprovalRecord{}, fmt.Errorf("execution binding key is required")
	}
	if request.ScopeSnapshotID == "" {
		return PermissionApprovalRecord{}, fmt.Errorf("scope snapshot ID is required")
	}

	bindingKey := request.ExecutionBindingKey

	recordID := generateApprovalRecordID(request, bindingKey)

	record := PermissionApprovalRecord{
		RecordID:            recordID,
		InvocationID:        request.InvocationID,
		PermissionIDs:       append([]string{}, request.PermissionIDs...),
		ScopeSnapshotID:     request.ScopeSnapshotID,
		Decision:            request.Decision,
		ApprovedBy:          request.ApprovedBy,
		CreatedAt:           time.Now().UTC(),
		ExpiresAt:           request.ExpiresAt,
		ExecutionContext:    request.ExecutionContext,
		ExecutionBindingKey: bindingKey,
		RiskLevel:           request.RiskLevel,
	}

	b.mu.Lock()
	if b.approvalRecords == nil {
		b.approvalRecords = make(map[string]PermissionApprovalRecord)
	}
	b.approvalRecords[recordID] = record
	b.mu.Unlock()

	if b.auditRec != nil {
		b.auditRec.RecordApproval(ctx, record)
	}

	return record, nil
}

func (b *DefaultPermissionBroker) getApprovalRecord(recordID string) (PermissionApprovalRecord, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.approvalRecords == nil {
		return PermissionApprovalRecord{}, false
	}
	r, ok := b.approvalRecords[recordID]
	return r, ok
}

func generateApprovalRecordID(request PermissionApprovalRecordRequest, bindingKey string) string {
	input := fmt.Sprintf("%s:%s:%s:%d", request.InvocationID, request.Decision, bindingKey, time.Now().UnixNano())
	h := sha256.Sum256([]byte(input))
	return "apr_" + hex.EncodeToString(h[:16])
}
