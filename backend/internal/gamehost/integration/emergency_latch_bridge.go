package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ManagerRuntimeManager interface {
	IsEmergencyLatched(runtimeID domain.RuntimeInstanceID) bool
	SetEmergencyLatch(runtimeID domain.RuntimeInstanceID, latched bool)
}

type ManagerEmergencyLatchBridge struct {
	mgr  ManagerRuntimeManager
	delegate control.EmergencyIntentStore
}

func NewManagerEmergencyLatchBridge(mgr ManagerRuntimeManager, delegate control.EmergencyIntentStore) *ManagerEmergencyLatchBridge {
	return &ManagerEmergencyLatchBridge{
		mgr:      mgr,
		delegate: delegate,
	}
}

func (b *ManagerEmergencyLatchBridge) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	if b.mgr != nil {
		b.mgr.SetEmergencyLatch(runtimeID, true)
	}
	if b.delegate != nil {
		return b.delegate.CommitEmergencyIntent(ctx, runtimeID, operationID)
	}
	return nil
}

func (b *ManagerEmergencyLatchBridge) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	if b.mgr != nil {
		return b.mgr.IsEmergencyLatched(runtimeID)
	}
	if b.delegate != nil {
		return b.delegate.IsEmergencyLatched(ctx, runtimeID)
	}
	return false
}

func (b *ManagerEmergencyLatchBridge) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	if b.delegate != nil {
		return b.delegate.GetEmergencyOperationID(ctx, runtimeID)
	}
	return "", false
}

func (b *ManagerEmergencyLatchBridge) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	if b.mgr != nil {
		b.mgr.SetEmergencyLatch(runtimeID, false)
	}
	if b.delegate != nil {
		return b.delegate.ClearEmergencyLatch(ctx, runtimeID, actor)
	}
	return nil
}

var _ control.EmergencyIntentStore = (*ManagerEmergencyLatchBridge)(nil)
