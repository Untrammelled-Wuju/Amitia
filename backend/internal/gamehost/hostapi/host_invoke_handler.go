package hostapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

const HostInvokeMethod = "host.invoke"

type hostInvokeHandler struct {
	adapter *HostAPIAdapter
}

func NewHostInvokeHandler(adapter *HostAPIAdapter) rpc.Handler {
	return &hostInvokeHandler{adapter: adapter}
}

func (h *hostInvokeHandler) Handle(ctx context.Context, req rpc.RPCRequest) (rpc.RPCResponse, error) {
	var invokeInput struct {
		Method    string          `json:"method"`
		Version   int             `json:"version,omitempty"`
		Input     json.RawMessage `json:"input"`
		SideEffect string         `json:"sideEffect,omitempty"`
		RequestID string          `json:"requestId,omitempty"`
		TimeoutMs int             `json:"timeoutMs,omitempty"`
	}
	if err := json.Unmarshal(req.Payload, &invokeInput); err != nil {
		return rpc.RPCResponse{
			RequestID: req.ID,
			Error: &rpc.RPCRoutedError{
				Code:    CodeInvalidRequest,
				Message: fmt.Sprintf("invalid host.invoke input: %v", err),
			},
		}, nil
	}

	if invokeInput.Method == "" {
		return rpc.RPCResponse{
			RequestID: req.ID,
			Error: &rpc.RPCRoutedError{
				Code:    CodeInvalidRequest,
				Message: "host.invoke requires method field",
			},
		}, nil
	}

	peer := ipc.Peer{
		PluginID:   req.PluginID,
		RuntimeID:  req.RuntimeID,
		ServiceID:  req.ServiceID,
		Generation: req.Generation,
	}

	callReq := Request{
		Peer:    peer,
		Route:   invokeInput.Method,
		Version: invokeInput.Version,
		Input:   invokeInput.Input,
		ConnKey: req.ConnectionID,
	}
	if invokeInput.TimeoutMs > 0 {
		callReq.Deadline = time.Now().Add(time.Duration(invokeInput.TimeoutMs) * time.Millisecond)
	}

	resp, err := h.adapter.Call(ctx, callReq)
	if err != nil {
		return rpc.RPCResponse{
			RequestID: req.ID,
			Error: &rpc.RPCRoutedError{
				Code:    extractErrorCode(err),
				Message: err.Error(),
			},
		}, nil
	}

	output := normalizeResult(resp.Output)

	var statusResult map[string]any
	if err := json.Unmarshal(output, &statusResult); err != nil {
		statusResult = map[string]any{"status": resp.Status, "output": json.RawMessage(output)}
	}
	if _, hasStatus := statusResult["status"]; !hasStatus {
		statusResult["status"] = resp.Status
	}
	resultPayload, _ := json.Marshal(statusResult)

	return rpc.RPCResponse{
		RequestID: req.ID,
		Payload:   resultPayload,
	}, nil
}
