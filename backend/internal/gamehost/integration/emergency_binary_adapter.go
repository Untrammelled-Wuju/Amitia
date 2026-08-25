package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
)

type BinaryRuntimeCleaner interface {
	CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}

type EmergencyBinaryAdapter struct {
	cleaner BinaryRuntimeCleaner
}

func NewEmergencyBinaryAdapter(cleaner BinaryRuntimeCleaner) *EmergencyBinaryAdapter {
	return &EmergencyBinaryAdapter{cleaner: cleaner}
}

func (a *EmergencyBinaryAdapter) ReleaseRuntimeTransientBinary(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a == nil || a.cleaner == nil {
		return nil
	}
	return a.cleaner.CleanupRuntime(ctx, runtimeID)
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
