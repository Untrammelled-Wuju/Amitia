package hostapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

type hostAPIHandler struct {
	adapter *HostAPIAdapter
	idGen   func() string
}

func NewHostAPIHandler(adapter *HostAPIAdapter) rpc.Handler {
	return &hostAPIHandler{
		adapter: adapter,
		idGen:   DefaultIDGenerator(),
	}
}

func (h *hostAPIHandler) Handle(ctx context.Context, req rpc.RPCRequest) (rpc.RPCResponse, error) {
	peer := ipc.Peer{
		PluginID:  req.PluginID,
		RuntimeID: req.RuntimeID,
		ServiceID: req.ServiceID,
	}

	resp, err := h.adapter.Call(ctx, Request{
		Peer:    peer,
		Route:   string(req.Method),
		Input:   req.Payload,
		ConnKey: req.ConnectionID,
	})
	if err != nil {
		return rpc.RPCResponse{
			RequestID: req.ID,
			Error: &rpc.RPCRoutedError{
				Code:    extractErrorCode(err),
				Message: err.Error(),
			},
		}, nil
	}

	return rpc.RPCResponse{
		RequestID: req.ID,
		Payload:   normalizeResult(resp.Output),
	}, nil
}

func RegisterHostAPIMethods(adapter *HostAPIAdapter, registry rpc.HandlerRegistry, methods []host_api.Method) error {
	if adapter == nil {
		return fmt.Errorf("hostapi: adapter is required")
	}
	if registry == nil {
		return fmt.Errorf("hostapi: handler registry is required")
	}
	handler := NewHostAPIHandler(adapter)
	for _, m := range methods {
		if m == "" {
			continue
		}
		if err := registry.Register(rpc.Method(string(m)), handler); err != nil {
			return fmt.Errorf("hostapi: register %s: %w", m, err)
		}
	}
	return nil
}

func extractErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if he, ok := err.(*Error); ok && he.Code != "" {
		return he.Code
	}
	return CodeInternal
}

func normalizeResult(out json.RawMessage) json.RawMessage {
	if len(out) == 0 {
		return json.RawMessage(`{}`)
	}
	return out
}
