package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type EmergencyRPCAdapter struct {
	registry rpc.PendingRequestRegistry
}

func NewEmergencyRPCAdapter(registry rpc.PendingRequestRegistry) *EmergencyRPCAdapter {
	return &EmergencyRPCAdapter{registry: registry}
}

func (a *EmergencyRPCAdapter) CancelRuntimeRequests(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	if a.registry == nil {
		return 0, nil
	}
	count := a.registry.CancelByRuntime(runtimeID)
	return count, nil
}

var _ control.PendingWorkCanceller = (*EmergencyRPCAdapter)(nil)

type EmergencyPendingVerifier struct {
	registry rpc.PendingRequestRegistry
}

func NewEmergencyPendingVerifier(registry rpc.PendingRequestRegistry) *EmergencyPendingVerifier {
	return &EmergencyPendingVerifier{registry: registry}
}

func (v *EmergencyPendingVerifier) CountRuntimePending(runtimeID domain.RuntimeInstanceID) int {
	if v.registry == nil {
		return 0
	}
	return v.registry.CountByRuntime(runtimeID)
}

var _ control.PendingVerifier = (*EmergencyPendingVerifier)(nil)
