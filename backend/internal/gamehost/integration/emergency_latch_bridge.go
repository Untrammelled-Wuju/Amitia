package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ManagerRuntimeManager interface {
	IsEmergencyLatched(runtimeID domain.RuntimeInstanceID) bool
	SetEmergencyLatch(runtimeID domain.RuntimeInstanceID, latched bool)
	Get(ctx context.Context, runtimeID domain.RuntimeInstanceID) (*domain.RuntimeInstance, error)
}

type pluginAwareEmergencyIntentStore interface {
	CommitEmergencyIntentForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, operationID string) error
	IsEmergencyLatchedForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) bool
	GetEmergencyOperationIDForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID) (string, bool)
	ClearEmergencyLatchForPlugin(ctx context.Context, runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, actor string) error
}

type ManagerEmergencyLatchBridge struct {
	mgr      ManagerRuntimeManager
	delegate control.EmergencyIntentStore
}

func NewManagerEmergencyLatchBridge(mgr ManagerRuntimeManager, delegate control.EmergencyIntentStore) *ManagerEmergencyLatchBridge {
	return &ManagerEmergencyLatchBridge{
		mgr:      mgr,
		delegate: delegate,
	}
}

func (b *ManagerEmergencyLatchBridge) pluginID(ctx context.Context, runtimeID domain.RuntimeInstanceID) domain.PluginID {
	if b == nil || b.mgr == nil {
		return ""
	}
	runtime, err := b.mgr.Get(ctx, runtimeID)
	if err != nil || runtime == nil {
		return ""
	}
	return runtime.PluginID
}

func (b *ManagerEmergencyLatchBridge) CommitEmergencyIntent(ctx context.Context, runtimeID domain.RuntimeInstanceID, operationID string) error {
	// Set the in-process latch before disk I/O so even a persistence failure does
	// not allow the current host to restart the runtime while the emergency stop
	// reports its failure.
	if b.mgr != nil {
		b.mgr.SetEmergencyLatch(runtimeID, true)
	}
	if b.delegate == nil {
		return nil
	}
	if store, ok := b.delegate.(pluginAwareEmergencyIntentStore); ok {
		return store.CommitEmergencyIntentForPlugin(ctx, runtimeID, b.pluginID(ctx, runtimeID), operationID)
	}
	return b.delegate.CommitEmergencyIntent(ctx, runtimeID, operationID)
}

func (b *ManagerEmergencyLatchBridge) IsEmergencyLatched(ctx context.Context, runtimeID domain.RuntimeInstanceID) bool {
	pluginID := b.pluginID(ctx, runtimeID)
	if store, ok := b.delegate.(pluginAwareEmergencyIntentStore); ok {
		if store.IsEmergencyLatchedForPlugin(ctx, runtimeID, pluginID) {
			if b.mgr != nil {
				b.mgr.SetEmergencyLatch(runtimeID, true)
			}
			return true
		}
	} else if b.delegate != nil && b.delegate.IsEmergencyLatched(ctx, runtimeID) {
		if b.mgr != nil {
			b.mgr.SetEmergencyLatch(runtimeID, true)
		}
		return true
	}
	if b.mgr != nil {
		return b.mgr.IsEmergencyLatched(runtimeID)
	}
	return false
}

func (b *ManagerEmergencyLatchBridge) GetEmergencyOperationID(ctx context.Context, runtimeID domain.RuntimeInstanceID) (string, bool) {
	if store, ok := b.delegate.(pluginAwareEmergencyIntentStore); ok {
		return store.GetEmergencyOperationIDForPlugin(ctx, runtimeID, b.pluginID(ctx, runtimeID))
	}
	if b.delegate != nil {
		return b.delegate.GetEmergencyOperationID(ctx, runtimeID)
	}
	return "", false
}

func (b *ManagerEmergencyLatchBridge) ClearEmergencyLatch(ctx context.Context, runtimeID domain.RuntimeInstanceID, actor string) error {
	pluginID := b.pluginID(ctx, runtimeID)
	if b.delegate != nil {
		var err error
		if store, ok := b.delegate.(pluginAwareEmergencyIntentStore); ok {
			err = store.ClearEmergencyLatchForPlugin(ctx, runtimeID, pluginID, actor)
		} else {
			err = b.delegate.ClearEmergencyLatch(ctx, runtimeID, actor)
		}
		if err != nil {
			return err
		}
	}
	if b.mgr != nil {
		b.mgr.SetEmergencyLatch(runtimeID, false)
	}
	return nil
}

var _ control.EmergencyIntentStore = (*ManagerEmergencyLatchBridge)(nil)
