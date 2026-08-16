package control

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

func TestControlHandler_OutputDispatchesRegisteredEffect(t *testing.T) {
	gate, topology, _ := newCompleteTestGate(t)
	topology.RegisterService("rt-1", "svc-1")
	registry := NewControlSinkRegistry()
	sink := NewRecordingControlEffectSink()
	if err := registry.RegisterEffect(ControlSinkDescriptor{
		SinkID: "sink-1", RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Kind: KindCustomRPC, Generation: 1,
	}, sink); err != nil {
		t.Fatalf("register effect sink: %v", err)
	}
	handler := NewControlHandler(gate, registry)
	payload, _ := json.Marshal(ControlOutputInput{OutputID: "output-1", SinkID: "sink-1", ServiceID: "svc-1", Epoch: 10, Payload: json.RawMessage(`{"operation":"run"}`)})
	response, err := handler.handleControlOutput(context.Background(), rpc.RPCRequest{
		ID: "request-1", PluginID: "plugin-1", RuntimeID: "rt-1", ServiceID: "svc-1", Generation: 1, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle control output: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected response error: %v", response.Error)
	}
	var result ControlOutputResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatalf("decode output result: %v", err)
	}
	if !result.Allowed || sink.Calls() != 1 {
		t.Fatalf("expected one committed effect, allowed=%t calls=%d", result.Allowed, sink.Calls())
	}
}

func TestControlHandler_OutputRejectsSinkIdentityMismatch(t *testing.T) {
	gate, topology, _ := newCompleteTestGate(t)
	topology.RegisterService("rt-1", "svc-1")
	registry := NewControlSinkRegistry()
	if err := registry.RegisterEffect(ControlSinkDescriptor{
		SinkID: "sink-1", RuntimeID: "rt-1", PluginID: "plugin-1", ServiceID: "svc-1", Kind: KindCustomRPC, Generation: 1,
	}, NewRecordingControlEffectSink()); err != nil {
		t.Fatalf("register effect sink: %v", err)
	}
	handler := NewControlHandler(gate, registry)
	payload, _ := json.Marshal(ControlOutputInput{OutputID: "output-1", SinkID: "sink-1", ServiceID: "svc-1", Epoch: 10, Payload: json.RawMessage(`{}`)})
	response, err := handler.handleControlOutput(context.Background(), rpc.RPCRequest{
		ID: "request-1", PluginID: domain.PluginID("plugin-1"), RuntimeID: "rt-1", ServiceID: "svc-1", Generation: 2, Payload: payload,
	})
	if err != nil {
		t.Fatalf("handle control output: %v", err)
	}
	var result ControlOutputResult
	if err := json.Unmarshal(response.Payload, &result); err != nil {
		t.Fatalf("decode output result: %v", err)
	}
	if result.Allowed || result.Reason != "sink_identity_mismatch" {
		t.Fatalf("expected sink identity denial, got allowed=%t reason=%s", result.Allowed, result.Reason)
	}
}
