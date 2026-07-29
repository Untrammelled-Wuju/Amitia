package developer_console

import (
	"context"
	"time"
)

type HostAPIAuditEntry struct {
	CallID               string     `json:"callId"`
	TraceID              string     `json:"traceId"`
	OperationID          string     `json:"operationId"`
	InvocationID         string     `json:"invocationId"`
	ExtensionID          string     `json:"extensionId"`
	ModuleID             string     `json:"moduleId"`
	Method               string     `json:"method"`
	Generation           int64      `json:"generation"`
	PermissionSnapshotID string     `json:"permissionSnapshotId"`
	ScopeSnapshotID      string     `json:"scopeSnapshotId"`
	StartedAt            time.Time  `json:"startedAt"`
	FinishedAt           *time.Time `json:"finishedAt,omitempty"`
	Result               string     `json:"result"`
	ErrorCode            string     `json:"errorCode,omitempty"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	SideEffect           string     `json:"sideEffect,omitempty"`
	InputMasked          string     `json:"inputMasked,omitempty"`
	Phase                string     `json:"phase"`
}

type HostAPIAuditQuery interface {
	ListAuditLogs(ctx context.Context, extensionID, method, result, traceID string, limit, offset int) ([]HostAPIAuditEntry, error)
	CountAuditLogs(ctx context.Context, extensionID, method, result, traceID string) (int64, error)
}
