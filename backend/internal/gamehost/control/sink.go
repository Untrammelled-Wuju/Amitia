package control

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type OpaqueControlPayload interface {
	PayloadRef() string
	PayloadKind() string
}

type ControlEffectSink interface {
	ExecuteAuthorized(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error
}

type ControlEffectSinkFunc func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error

func (f ControlEffectSinkFunc) ExecuteAuthorized(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
	return f(ctx, runtimeID, serviceID, pluginID, permit, payload)
}

var _ ControlEffectSink = ControlEffectSinkFunc(nil)

func NewNoopControlEffectSink() ControlEffectSink {
	return ControlEffectSinkFunc(func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
		return nil
	})
}
