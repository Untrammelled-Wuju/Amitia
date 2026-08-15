package control

import (
	"context"
	"errors"
	"log"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

var ErrEffectSinkUnavailable = errors.New("effect sink unavailable: no commit function registered")

type DefaultControlEffectSink struct {
	runtimeID domain.RuntimeInstanceID
	pluginID  domain.PluginID
	sinkID    string
	commitFn  func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, payload []byte) error
}

func NewDefaultControlEffectSink(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, sinkID string, commitFn func(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, payload []byte) error) *DefaultControlEffectSink {
	return &DefaultControlEffectSink{
		runtimeID: runtimeID,
		pluginID:  pluginID,
		sinkID:    sinkID,
		commitFn:  commitFn,
	}
}

func (s *DefaultControlEffectSink) ExecuteAuthorized(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID, permit OutputPermit, payload []byte) error {
	if s.commitFn == nil {
		return ErrEffectSinkUnavailable
	}
	return s.commitFn(ctx, runtimeID, serviceID, pluginID, payload)
}

func ExtractControlEffectSink(sink ControlEffectSink) (*DefaultControlEffectSink, bool) {
	if ds, ok := sink.(*DefaultControlEffectSink); ok {
		return ds, true
	}
	return nil, false
}
