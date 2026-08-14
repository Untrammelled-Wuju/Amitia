package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
)

type EmergencyBinaryAdapter struct {
	registry binary.ObjectRegistry
}

func NewEmergencyBinaryAdapter(registry binary.ObjectRegistry) *EmergencyBinaryAdapter {
	return &EmergencyBinaryAdapter{registry: registry}
}

func (a *EmergencyBinaryAdapter) ReleaseRuntimeTransientBinary(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.registry == nil {
		return nil
	}
	conns, err := a.registry.ListByRuntime(runtimeID)
	if err != nil {
		return err
	}
	for _, obj := range conns {
		if err := a.registry.Release(ctx, obj.ID); err != nil {
			continue
		}
	}
	return nil
}

var _ control.BinaryReleaser = (*EmergencyBinaryAdapter)(nil)

type EmergencyBinaryVerifier struct {
	registry binary.ObjectRegistry
}

func NewEmergencyBinaryVerifier(registry binary.ObjectRegistry) *EmergencyBinaryVerifier {
	return &EmergencyBinaryVerifier{registry: registry}
}

func (v *EmergencyBinaryVerifier) CountRuntimeBinary(runtimeID domain.RuntimeInstanceID) int {
	if v.registry == nil {
		return 0
	}
	objs, err := v.registry.ListByRuntime(runtimeID)
	if err != nil {
		return 0
	}
	count := 0
	for _, obj := range objs {
		if obj.State != binary.ObjectStateReleased {
			count++
		}
	}
	return count
}

var _ control.BinaryVerifier = (*EmergencyBinaryVerifier)(nil)
