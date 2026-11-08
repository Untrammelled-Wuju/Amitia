package agent

import (
	"context"
	"errors"
)

var ErrCoreInferenceRequired = errors.New("当前设备已使用云端服务，对话与记忆计算必须由当前 Core 提供")

func (h *LocalHandler) LocalInferenceContext(ctx context.Context) (context.Context, func(), error) {
	if h == nil {
		return nil, nil, ErrCoreInferenceRequired
	}
	h.mu.RLock()
	coreMode := h.coreMode
	h.mu.RUnlock()
	if coreMode {
		if err := context.Cause(ctx); err != nil {
			return nil, nil, err
		}
		child, cancel := context.WithCancelCause(ctx)
		return child, func() { cancel(context.Canceled) }, nil
	}
	h.providerMu.Lock()
	defer h.providerMu.Unlock()
	if err := context.Cause(ctx); err != nil {
		return nil, nil, err
	}
	if h.providerPaused || h.providerContext == nil || h.providerContext.Err() != nil {
		return nil, nil, ErrCoreInferenceRequired
	}
	credential, err := h.credStore.LoadCredential()
	if err != nil {
		return nil, nil, err
	}
	if credential != nil {
		return nil, nil, ErrCoreInferenceRequired
	}
	child, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(h.providerContext, func() { cancel(ErrCoreInferenceRequired) })
	return child, func() { stop(); cancel(context.Canceled) }, nil
}
