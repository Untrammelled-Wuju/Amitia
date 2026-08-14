package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type EmergencyChannelAdapter struct {
	registry channel.Registry
}

func NewEmergencyChannelAdapter(registry channel.Registry) *EmergencyChannelAdapter {
	return &EmergencyChannelAdapter{registry: registry}
}

func (a *EmergencyChannelAdapter) CleanupRuntimeChannels(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.registry == nil {
		return nil
	}
	_, err := a.registry.RemoveByRuntime(ctx, runtimeID)
	return err
}

var _ control.ChannelCleaner = (*EmergencyChannelAdapter)(nil)

type EmergencyChannelVerifier struct {
	registry channel.Registry
}

func NewEmergencyChannelVerifier(registry channel.Registry) *EmergencyChannelVerifier {
	return &EmergencyChannelVerifier{registry: registry}
}

func (v *EmergencyChannelVerifier) CountRuntimeChannels(runtimeID domain.RuntimeInstanceID) int {
	if v.registry == nil {
		return 0
	}
	return v.registry.CountByRuntime(runtimeID)
}

var _ control.ChannelVerifier = (*EmergencyChannelVerifier)(nil)
