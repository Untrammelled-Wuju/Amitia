package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func (r *Runtime) recordRuntimeOperation(ctx context.Context, extID domain.ExtensionID, opType string, rtErr *runtime_supervisor.RuntimeError) {
	if r.container == nil || r.container.OperationRepository == nil {
		return
	}
	now := time.Now().UTC()
	_ = r.container.OperationRepository.PutOperation(ctx, sqlite.Operation{
		OperationID:   fmt.Sprintf("rtop-%s-%d", extID, now.UnixNano()),
		OperationType: opType,
		ExtensionID:   extID,
		Status:        "failed",
		ErrorCode:     rtErr.Code,
		ErrorMessage:  rtErr.Error(),
		StartedAt:     now,
		FinishedAt:    &now,
	})
}

func (r *Runtime) recordRuntimeStopFailure(ctx context.Context, extID domain.ExtensionID, opType string, instanceID string, err error) {
	if r.container == nil || r.container.OperationRepository == nil {
		return
	}
	rtErr := runtime_supervisor.NewRuntimeError(
		runtime_supervisor.CodeRuntimeStopFailed,
		fmt.Sprintf("extension=%s instance=%s", extID, instanceID),
		err,
	)
	now := time.Now().UTC()
	_ = r.container.OperationRepository.PutOperation(ctx, sqlite.Operation{
		OperationID:   fmt.Sprintf("rtop-%s-%d", extID, now.UnixNano()),
		OperationType: opType,
		ExtensionID:   extID,
		Status:        "failed",
		ErrorCode:     rtErr.Code,
		ErrorMessage:  rtErr.Error(),
		StartedAt:     now,
		FinishedAt:    &now,
	})
}

func (c *Container) recordReconcileFailure(ctx context.Context, extID domain.ExtensionID, result runtime_supervisor.ReconcileResult) {
	if c == nil || c.OperationRepository == nil {
		return
	}
	rtErr := runtime_supervisor.ClassifyReconcileError(result)
	if rtErr == nil {
		return
	}
	now := time.Now().UTC()
	_ = c.OperationRepository.PutOperation(ctx, sqlite.Operation{
		OperationID:   fmt.Sprintf("rtop-%s-%d", extID, now.UnixNano()),
		OperationType: "recover",
		ExtensionID:   extID,
		Status:        "failed",
		ErrorCode:     rtErr.Code,
		ErrorMessage:  rtErr.Error(),
		StartedAt:     now,
		FinishedAt:    &now,
	})
}
