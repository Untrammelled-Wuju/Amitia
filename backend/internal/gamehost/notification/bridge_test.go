package notification

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestBridge_Handle_BasicNotification(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin-a",
		RuntimeID: "runtime-1",
		ServiceID: "service-main",
	}
	payload := json.RawMessage(`{"foo":"bar"}`)

	if err := bridge.Handle(context.Background(), source, "vendor.event.happened", payload, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot := sink.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(snapshot))
	}

	n := snapshot[0]
	if n.PluginID != "plugin-a" {
		t.Errorf("PluginID: got %q, want %q", n.PluginID, "plugin-a")
	}
	if n.RuntimeID != "runtime-1" {
		t.Errorf("RuntimeID: got %q, want %q", n.RuntimeID, "runtime-1")
	}
	if n.ServiceID != "service-main" {
		t.Errorf("ServiceID: got %q, want %q", n.ServiceID, "service-main")
	}
	if n.Method != "vendor.event.happened" {
		t.Errorf("Method: got %q, want %q", n.Method, "vendor.event.happened")
	}
	if string(n.Payload) != `{"foo":"bar"}` {
		t.Errorf("Payload: got %q, want %q", n.Payload, `{"foo":"bar"}`)
	}
	if n.ReceivedAt.IsZero() {
		t.Errorf("ReceivedAt should not be zero")
	}
}

func TestBridge_Handle_PayloadOpaque(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin-b",
		RuntimeID: "runtime-2",
		ServiceID: "svc",
	}
	payload := json.RawMessage(`{"foo":{"unknown":[1,2,3]}}`)

	if err := bridge.Handle(context.Background(), source, "custom.method", payload, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := sink.Snapshot()[0]
	if string(n.Payload) != string(payload) {
		t.Errorf("Payload changed during bridge: got %q, want %q", n.Payload, payload)
	}
}

func TestBridge_Handle_PayloadDeepCopy(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin-c",
		RuntimeID: "runtime-3",
		ServiceID: "svc",
	}
	payload := json.RawMessage(`{"key":"value"}`)

	if err := bridge.Handle(context.Background(), source, "method", payload, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := sink.Snapshot()[0]

	payload[0] = 'X'
	if string(n.Payload) != `{"key":"value"}` {
		t.Errorf("Payload not deep copied, got %q", n.Payload)
	}
}

func TestBridge_Handle_MetadataDeepCopy(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin-d",
		RuntimeID: "runtime-4",
		ServiceID: "svc",
	}
	metadata := map[string]json.RawMessage{
		"trace": json.RawMessage(`"abc"`),
		"extra": json.RawMessage(`{"nested":true}`),
	}

	if err := bridge.Handle(context.Background(), source, "method", nil, metadata); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := sink.Snapshot()[0]

	metadata["trace"][0] = 'X'
	if string(n.Metadata["trace"]) != `"abc"` {
		t.Errorf("Metadata not deep copied, got %q", n.Metadata["trace"])
	}
}

func TestBridge_Handle_MissingPluginID(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		RuntimeID: "runtime-1",
		ServiceID: "svc",
	}
	err := bridge.Handle(context.Background(), source, "method", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing plugin id")
	}
	if !domain.IsHostError(err, domain.ErrInvalidArgument) {
		t.Errorf("expected invalid_argument error, got %v", err)
	}
}

func TestBridge_Handle_MissingRuntimeID(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		ServiceID: "svc",
	}
	err := bridge.Handle(context.Background(), source, "method", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing runtime id")
	}
}

func TestBridge_Handle_MissingServiceID(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
	}
	err := bridge.Handle(context.Background(), source, "method", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing service id")
	}
}

func TestBridge_Handle_EmptyMethod(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	err := bridge.Handle(context.Background(), source, "", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty method")
	}
}

func TestBridge_Handle_MethodWithControlChars(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	badMethod := "bad" + strings.Repeat("\x00", 1) + "method"
	err := bridge.Handle(context.Background(), source, badMethod, nil, nil)
	if err == nil {
		t.Fatal("expected error for method with control characters")
	}
}

func TestBridge_Handle_TooLongMethod(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	longMethod := strings.Repeat("a", maxMethodLength+1)
	err := bridge.Handle(context.Background(), source, longMethod, nil, nil)
	if err == nil {
		t.Fatal("expected error for too long method")
	}
}

type errSink struct {
	err error
}

func (e *errSink) Publish(ctx context.Context, n Notification) error {
	return e.err
}

func TestBridge_Handle_SinkError(t *testing.T) {
	expectedErr := errors.New("sink broken")
	bridge := NewBridge(&errorSink{err: expectedErr})

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	err := bridge.Handle(context.Background(), source, "method", nil, nil)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected sink error, got %v", err)
	}
}

func TestBridge_Handle_NilSink(t *testing.T) {
	bridge := NewBridge(nil)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	err := bridge.Handle(context.Background(), source, "method", nil, nil)
	if err == nil {
		t.Fatal("expected error for nil sink")
	}
}

func TestBridge_Handle_ContextCancel(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := bridge.Handle(ctx, source, "method", nil, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestBridge_MultiServiceNotification(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	services := []struct {
		Route  RouteContext
		Method string
	}{
		{RouteContext{"plugin", "runtime", "bridge"}, "bridge.event"},
		{RouteContext{"plugin", "runtime", "agent"}, "agent.event"},
		{RouteContext{"plugin", "runtime", "vision"}, "vision.event"},
	}

	for _, svc := range services {
		payloadStr := `{"svc":"` + string(svc.Route.ServiceID) + `"}`
		payload := json.RawMessage(payloadStr)
		if err := bridge.Handle(context.Background(), svc.Route, svc.Method, payload, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	snapshot := sink.Snapshot()
	if len(snapshot) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(snapshot))
	}

	for i, svc := range services {
		if snapshot[i].ServiceID != svc.Route.ServiceID {
			t.Errorf("[%d] ServiceID: got %q, want %q", i, snapshot[i].ServiceID, svc.Route.ServiceID)
		}
		if snapshot[i].Method != svc.Method {
			t.Errorf("[%d] Method: got %q, want %q", i, snapshot[i].Method, svc.Method)
		}
	}
}

func TestBridge_MultiRuntimeNotification(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	runtimes := []RouteContext{
		{"plugin-beta", "runtime-1", "svc"},
		{"plugin-beta", "runtime-2", "svc"},
	}

	for _, rt := range runtimes {
		if err := bridge.Handle(context.Background(), rt, "example.game.entity.updated", json.RawMessage(`{}`), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	snapshot := sink.Snapshot()
	if snapshot[0].RuntimeID != "runtime-1" {
		t.Errorf("expected runtime-1, got %s", snapshot[0].RuntimeID)
	}
	if snapshot[1].RuntimeID != "runtime-2" {
		t.Errorf("expected runtime-2, got %s", snapshot[1].RuntimeID)
	}
}

func TestBridge_BigIntPrecision(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	payload := json.RawMessage(`{"tick":9007199254740993}`)

	if err := bridge.Handle(context.Background(), source, "method", payload, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := sink.Snapshot()[0]
	if string(n.Payload) != `{"tick":9007199254740993}` {
		t.Errorf("big int precision lost: got %s", n.Payload)
	}
}

func TestBridge_DoNotInterpretMethodName(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	methods := []string{
		"example.game.entity.updated",
		"example.game.inventory.updated",
		"vendor.agent.thinking",
		"vendor.game.state.updated",
		"plugin.custom.notification",
	}
	for _, m := range methods {
		if err := bridge.Handle(context.Background(), source, m, json.RawMessage(`{}`), nil); err != nil {
			t.Fatalf("method %q rejected: %v", m, err)
		}
	}

	snapshot := sink.Snapshot()
	for i, m := range methods {
		if snapshot[i].Method != m {
			t.Errorf("[%d] method got %q, want %q", i, snapshot[i].Method, m)
		}
	}
}

func TestBridge_NowFuncOverride(t *testing.T) {
	sink := NewMemorySink()
	bridge := NewBridge(sink)

	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	bridge.SetNowFunc(func() time.Time { return fixedTime })

	source := RouteContext{
		PluginID:  "plugin",
		RuntimeID: "runtime",
		ServiceID: "svc",
	}
	if err := bridge.Handle(context.Background(), source, "method", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n := sink.Snapshot()[0]
	if !n.ReceivedAt.Equal(fixedTime) {
		t.Errorf("ReceivedAt: got %v, want %v", n.ReceivedAt, fixedTime)
	}
}

func TestNotification_Route(t *testing.T) {
	n := Notification{
		PluginID:  "p",
		RuntimeID: "r",
		ServiceID: "s",
	}
	r := n.Route()
	if r.PluginID != "p" || r.RuntimeID != "r" || r.ServiceID != "s" {
		t.Errorf("Route() returned wrong values: %+v", r)
	}
}
