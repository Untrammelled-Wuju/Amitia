package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/stream"
)

type EmergencyStreamAdapter struct {
	manager *stream.StreamManager
}

func NewEmergencyStreamAdapter(manager *stream.StreamManager) *EmergencyStreamAdapter {
	return &EmergencyStreamAdapter{manager: manager}
}

func (a *EmergencyStreamAdapter) StopRuntimeStreams(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.manager == nil {
		return nil
	}
	a.manager.RemoveByRuntime(ctx, runtimeID)
	return nil
}

var _ control.StreamStopper = (*EmergencyStreamAdapter)(nil)

type EmergencyStreamVerifier struct {
	manager *stream.StreamManager
}

func NewEmergencyStreamVerifier(manager *stream.StreamManager) *EmergencyStreamVerifier {
	return &EmergencyStreamVerifier{manager: manager}
}

func (v *EmergencyStreamVerifier) CountRuntimeStreams(runtimeID domain.RuntimeInstanceID) int {
	if v.manager == nil {
		return 0
	}
	return v.manager.CountByRuntime(runtimeID)
}

var _ control.StreamVerifier = (*EmergencyStreamVerifier)(nil)
