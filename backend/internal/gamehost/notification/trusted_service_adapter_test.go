package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestTrustedServiceNotificationAdapter_Handle(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)
	adapter := NewTrustedServiceNotificationAdapter(bridge)

	adapter.RegisterRoute("svc-1", RouteContext{
		PluginID:  "plugin-a",
		RuntimeID: "runtime-1",
		ServiceID: "svc-1",
	})

	err := adapter.Handle(context.Background(), "svc-1", "service.event", json.RawMessage(`{"data":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := sink.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1, got %d", len(snapshot))
	}
	n := snapshot[0]
	if n.PluginID != "plugin-a" {
		t.Errorf("PluginID: got %q, want %q", n.PluginID, "plugin-a")
	}
	if n.RuntimeID != "runtime-1" {
		t.Errorf("RuntimeID: got %q, want %q", n.RuntimeID, "runtime-1")
	}
	if n.ServiceID != "svc-1" {
		t.Errorf("ServiceID: got %q, want %q", n.ServiceID, "svc-1")
	}
	if n.Method != "service.event" {
		t.Errorf("Method: got %q, want %q", n.Method, "service.event")
	}
	if string(n.Payload) != `{"data":true}` {
		t.Errorf("Payload: got %q, want %q", n.Payload, `{"data":true}`)
	}
}

func TestTrustedServiceNotificationAdapter_UnknownService(t *testing.T) {
	bridge := NewBridge(NewMemorySink())
	adapter := NewTrustedServiceNotificationAdapter(bridge)

	err := adapter.Handle(context.Background(), "unknown", "method", nil)
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestTrustedServiceNotificationAdapter_Unregister(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)
	adapter := NewTrustedServiceNotificationAdapter(bridge)

	adapter.RegisterRoute("svc", RouteContext{PluginID: "p", RuntimeID: "r", ServiceID: "svc"})
	adapter.UnregisterRoute("svc")

	err := adapter.Handle(context.Background(), "svc", "method", nil)
	if err == nil {
		t.Fatal("expected error after unregister")
	}
}

func TestTrustedServiceNotificationAdapter_MultipleServices(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)
	adapter := NewTrustedServiceNotificationAdapter(bridge)

	adapter.RegisterRoute("svc-a", RouteContext{PluginID: "p", RuntimeID: "r1", ServiceID: "svc-a"})
	adapter.RegisterRoute("svc-b", RouteContext{PluginID: "p", RuntimeID: "r2", ServiceID: "svc-b"})

	_ = adapter.Handle(context.Background(), "svc-a", "evt.a", json.RawMessage(`{}`))
	_ = adapter.Handle(context.Background(), "svc-b", "evt.b", json.RawMessage(`{}`))

	snapshot := sink.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("expected 2, got %d", len(snapshot))
	}
	if snapshot[0].ServiceID != "svc-a" {
		t.Errorf("first should be svc-a, got %q", snapshot[0].ServiceID)
	}
	if snapshot[1].ServiceID != "svc-b" {
		t.Errorf("second should be svc-b, got %q", snapshot[1].ServiceID)
	}
}

func TestTrustedServiceNotificationAdapter_BridgeError(t *testing.T) {
	bridge := NewBridge(&errorSink{err: errors.New("bridge fail")})
	adapter := NewTrustedServiceNotificationAdapter(bridge)

	adapter.RegisterRoute("svc", RouteContext{PluginID: "p", RuntimeID: "r", ServiceID: "svc"})
	err := adapter.Handle(context.Background(), "svc", "method", nil)
	if err == nil {
		t.Fatal("expected error from bridge")
	}
}

type directSrc struct {
	Route RouteContext
}

func (s directSrc) AsRoute() RouteContext {
	return s.Route
}

func TestBuildRoute(t *testing.T) {
	r := BuildRoute("plugin", "runtime", "service")
	if r.PluginID != "plugin" || r.RuntimeID != "runtime" || r.ServiceID != "service" {
		t.Errorf("bad route: %+v", r)
	}
}
