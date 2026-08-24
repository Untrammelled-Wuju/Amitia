package hostapi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/gamehost/rpc"
)

func TestHostInvokeHandlerKeepsGatewayObjectUnderOutput(t *testing.T) {
	gw := &fakeGateway{result: host_api.CallResult{
		Status: host_api.StatusSuccess,
		Output: json.RawMessage(`{"moduleId":"module-1","instances":["runtime-1"]}`),
	}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: "example.game/plugin",
		ModuleID:    "module-1",
		RuntimeType: "service",
		Generation:  3,
	}}
	adapter := newTestAdapter(gw, mapper)
	handler := NewHostInvokeHandler(adapter)

	response, err := handler.Handle(context.Background(), rpc.RPCRequest{
		ID:         "request-1",
		PluginID:   "plugin-1",
		RuntimeID:  "runtime-1",
		ServiceID:  "service-1",
		Generation: 3,
		Payload:    json.RawMessage(`{"method":"host.runtime.health","version":1,"input":{"probe":true}}`),
	})
	if err != nil {
		t.Fatalf("host.invoke: %v", err)
	}
	if response.Error != nil {
		t.Fatalf("unexpected routed error: %+v", response.Error)
	}

	var wire struct {
		Status     string          `json:"status"`
		Output     json.RawMessage `json:"output"`
		Method     string          `json:"method"`
		DurationMs int64           `json:"durationMs"`
	}
	if err := json.Unmarshal(response.Payload, &wire); err != nil {
		t.Fatalf("decode host.invoke result: %v", err)
	}
	if wire.Status != host_api.StatusSuccess || wire.Method != "host.runtime.health" {
		t.Fatalf("unexpected result: %+v", wire)
	}
	var output map[string]any
	if err := json.Unmarshal(wire.Output, &output); err != nil {
		t.Fatalf("decode nested output: %v", err)
	}
	if output["moduleId"] != "module-1" {
		t.Fatalf("gateway object was not preserved under output: %v", output)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(response.Payload, &top); err != nil {
		t.Fatalf("decode top level: %v", err)
	}
	if _, flattened := top["moduleId"]; flattened {
		t.Fatal("gateway object must not be flattened into the host.invoke response envelope")
	}
}
