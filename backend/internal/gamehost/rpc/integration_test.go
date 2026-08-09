package rpc_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type MockRuntimeValidator struct {
	RuntimeExists map[domain.RuntimeInstanceID]bool
	ServiceExists map[string]domain.PluginID
}

func NewMockRuntimeValidator() *MockRuntimeValidator {
	return &MockRuntimeValidator{
		RuntimeExists: map[domain.RuntimeInstanceID]bool{
			"runtime-1": true,
			"runtime-2": true,
		},
		ServiceExists: map[string]domain.PluginID{
			"runtime-1/service-a": "plugin-1",
			"runtime-1/service-b": "plugin-1",
			"runtime-2/service-a": "plugin-2",
		},
	}
}

func (m *MockRuntimeValidator) ValidateRuntime(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
) error {
	if !m.RuntimeExists[runtimeID] {
		return domain.NewHostError(domain.ErrRuntimeUnavailable, "runtime not found")
	}
	return nil
}

func (m *MockRuntimeValidator) ValidateService(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	expectedPluginID domain.PluginID,
) error {
	key := string(runtimeID) + "/" + string(serviceID)
	pid, exists := m.ServiceExists[key]
	if !exists {
		return domain.NewHostError(domain.ErrNotFound, "service not found")
	}
	if pid != expectedPluginID {
		return domain.NewHostError(domain.ErrInvalidArgument, "plugin mismatch")
	}
	return nil
}

func (id domain.RuntimeInstanceID) String() string {
	return string(id)
}

func (id domain.ServiceID) String() string {
	return string(id)
}

func TestParseMethod_Valid(t *testing.T) {
	tests := []struct {
		method      string
		ns          string
		segmentLen  int
		expectError bool
	}{
		{"minecraft.move", "minecraft", 2, false},
		{"minecraft.bot.move", "minecraft", 3, false},
		{"vendor.foo.bar.baz", "vendor", 4, false},
		{"minecraft", "", 0, true},
		{".minecraft.move", "", 0, true},
		{"minecraft.", "", 0, true},
		{"minecraft..move", "", 0, true},
		{"UPPER.foo", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			ns, segs, err := rpc.ParseMethod(tt.method)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for method %q", tt.method)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.method, err)
			}
			if string(ns) != tt.ns {
				t.Errorf("namespace: got %q, want %q", ns, tt.ns)
			}
			if len(segs) != tt.segmentLen {
				t.Errorf("segment count: got %d, want %d", len(segs), tt.segmentLen)
			}
		})
	}
}

func TestReservedNamespace(t *testing.T) {
	reserved := []string{"host", "runtime", "service", "control", "plugin", "channel"}
	for _, ns := range reserved {
		if !rpc.IsReservedNamespace(rpc.Namespace(ns)) {
			t.Errorf("expected %q to be reserved", ns)
		}
	}

	custom := []string{"minecraft", "factorio", "mygame", "vendor"}
	for _, ns := range custom {
		if rpc.IsReservedNamespace(rpc.Namespace(ns)) {
			t.Errorf("expected %q to be available for plugins", ns)
		}
	}
}

func TestNamespaceRegistry_RegisterAndResolve(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	route := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	err := reg.Register(ctx, route)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	resolved, err := reg.Resolve(ctx, "runtime-1", "minecraft.bot.move")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if resolved.ServiceID != "service-a" {
		t.Errorf("resolved service: got %q, want service-a", resolved.ServiceID)
	}
}

func TestNamespaceRegistry_ReservedRejected(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	route := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "host",
	}

	err := reg.Register(ctx, route)
	if err == nil {
		t.Fatal("reserved namespace should be rejected")
	}
}

func TestNamespaceRegistry_SameRuntimeConflict(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	route1 := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	route2 := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-b",
		Namespace: "minecraft",
	}

	if err := reg.Register(ctx, route1); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := reg.Register(ctx, route2)
	if err == nil {
		t.Fatal("conflicting namespace registration should fail")
	}
}

func TestNamespaceRegistry_CrossRuntimeSameNamespace(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	routeA := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	routeB := rpc.Route{
		RuntimeID: "runtime-2",
		PluginID:  "plugin-2",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	if err := reg.Register(ctx, routeA); err != nil {
		t.Fatalf("routeA register failed: %v", err)
	}

	if err := reg.Register(ctx, routeB); err != nil {
		t.Fatalf("routeB register failed: %v", err)
	}

	resolved1, err := reg.Resolve(ctx, "runtime-1", "minecraft.foo")
	if err != nil {
		t.Fatalf("resolve runtime-1 failed: %v", err)
	}
	if resolved1.ServiceID != "service-a" {
		t.Errorf("runtime-1 should resolve to service-a")
	}

	resolved2, err := reg.Resolve(ctx, "runtime-2", "minecraft.bar")
	if err != nil {
		t.Fatalf("resolve runtime-2 failed: %v", err)
	}
	if resolved2.ServiceID != "service-a" {
		t.Errorf("runtime-2 should resolve to service-a")
	}
}

func TestNamespaceRegistry_MultiNamespacePerService(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	namespaces := []string{"minecraft", "gamebridge", "vendor"}
	for _, ns := range namespaces {
		route := rpc.Route{
			RuntimeID: "runtime-1",
			PluginID:  "plugin-1",
			ServiceID: "service-a",
			Namespace: rpc.Namespace(ns),
		}
		if err := reg.Register(ctx, route); err != nil {
			t.Fatalf("register namespace %q failed: %v", ns, err)
		}
	}

	list, err := reg.List(ctx, "runtime-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 namespaces, got %d", len(list))
	}
}

func TestNamespaceRegistry_UnknownNamespaceResolve(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	_, err := reg.Resolve(ctx, "runtime-1", "factorio.foo")
	if err == nil {
		t.Fatal("resolve unknown namespace should fail")
	}
}

func TestNamespaceRegistry_RuntimeIsolation(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	routeA := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	routeB := rpc.Route{
		RuntimeID: "runtime-2",
		PluginID:  "plugin-2",
		ServiceID: "service-a",
		Namespace: "agent",
	}

	if err := reg.Register(ctx, routeA); err != nil {
		t.Fatalf("routeA failed: %v", err)
	}
	if err := reg.Register(ctx, routeB); err != nil {
		t.Fatalf("routeB failed: %v", err)
	}

	_, err := reg.Resolve(ctx, "runtime-1", "agent.foo")
	if err == nil {
		t.Error("runtime-1 should not access runtime-2's agent namespace")
	}

	_, err = reg.Resolve(ctx, "runtime-2", "minecraft.foo")
	if err == nil {
		t.Error("runtime-2 should not access runtime-1's minecraft namespace")
	}
}

func TestNamespaceRegistry_RuntimeCleanup(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	routeA := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	routeB := rpc.Route{
		RuntimeID: "runtime-2",
		PluginID:  "plugin-2",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	if err := reg.Register(ctx, routeA); err != nil {
		t.Fatalf("routeA failed: %v", err)
	}
	if err := reg.Register(ctx, routeB); err != nil {
		t.Fatalf("routeB failed: %v", err)
	}

	if err := reg.UnregisterByRuntime(ctx, "runtime-1"); err != nil {
		t.Fatalf("unregister runtime-1 failed: %v", err)
	}

	_, err := reg.Resolve(ctx, "runtime-1", "minecraft.foo")
	if err == nil {
		t.Error("runtime-1 namespaces should be removed")
	}

	resolved, err := reg.Resolve(ctx, "runtime-2", "minecraft.foo")
	if err != nil {
		t.Errorf("runtime-2 minecraft should still exist: %v", err)
	}
	if resolved.RuntimeID != "runtime-2" {
		t.Error("wrong route resolved")
	}
}

func TestNamespaceRegistry_ServiceCleanup(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	routeA := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-a",
		Namespace: "minecraft",
	}

	routeB := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "service-b",
		Namespace: "agent",
	}

	if err := reg.Register(ctx, routeA); err != nil {
		t.Fatalf("routeA failed: %v", err)
	}
	if err := reg.Register(ctx, routeB); err != nil {
		t.Fatalf("routeB failed: %v", err)
	}

	if err := reg.UnregisterByService(ctx, "runtime-1", "service-a"); err != nil {
		t.Fatalf("unregister service-a failed: %v", err)
	}

	_, err := reg.Resolve(ctx, "runtime-1", "minecraft.foo")
	if err == nil {
		t.Error("service-a's minecraft namespace should be removed")
	}

	_, err = reg.Resolve(ctx, "runtime-1", "agent.foo")
	if err != nil {
		t.Errorf("service-b's agent namespace should persist: %v", err)
	}
}

func TestNamespaceRegistry_StableList(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	reg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	namespaces := []string{"zebra", "alpha", "minecraft", "factorio"}
	for _, ns := range namespaces {
		route := rpc.Route{
			RuntimeID: "runtime-1",
			PluginID:  "plugin-1",
			ServiceID: "service-a",
			Namespace: rpc.Namespace(ns),
		}
		if err := reg.Register(ctx, route); err != nil {
			t.Fatalf("register %s failed: %v", ns, err)
		}
	}

	list, err := reg.List(ctx, "runtime-1")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(list) != 4 {
		t.Fatalf("expected 4 namespaces, got %d", len(list))
	}

	expected := []string{"alpha", "factorio", "minecraft", "zebra"}
	for i, ns := range expected {
		if string(list[i].Namespace) != ns {
			t.Errorf("position %d: got %q, want %q", i, list[i].Namespace, ns)
		}
	}
}

func TestHostHandlerRegistry(t *testing.T) {
	reg := rpc.NewHostHandlerRegistry()

	echoHandler := &echoHandler{
		responsePayload: json.RawMessage(`{"echo":true}`),
	}

	err := reg.Register("host.test.echo", echoHandler)
	if err != nil {
		t.Fatalf("register handler failed: %v", err)
	}

	handler, err := reg.Resolve("host.test.echo")
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
}

func TestHostHandlerRegistry_NotFound(t *testing.T) {
	reg := rpc.NewHostHandlerRegistry()

	_, err := reg.Resolve("host.does.not.exist")
	if err == nil {
		t.Fatal("resolve unknown handler should fail")
	}
}

type echoHandler struct {
	responsePayload json.RawMessage
}

func (h *echoHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return rpc.RPCResponse{
		RequestID: request.ID,
		Payload:   h.responsePayload,
	}, nil
}

func TestRPCDispatcher_Integration(t *testing.T) {
	ctx := context.Background()
	validator := NewMockRuntimeValidator()

	nsReg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	handlerReg := rpc.NewHostHandlerRegistry()
	echo := &echoHandler{responsePayload: json.RawMessage(`{"result":"ok"}`)}
	if err := handlerReg.Register("host.test.ping", echo); err != nil {
		t.Fatalf("register echo handler failed: %v", err)
	}

	disp := rpc.NewRPCDispatcher(rpc.DispatcherConfig{
		Namespaces:   nsReg,
		HostHandlers: handlerReg,
	})

	route := rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "bridge",
		Namespace: "minecraft",
	}
	if err := nsReg.Register(ctx, route); err != nil {
		t.Fatalf("register namespace failed: %v", err)
	}

	err := disp.Dispatch(ctx, ipc.Peer{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "agent",
	}, protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "req-1",
		Method:   "host.test.ping",
	})

	if err != nil {
		t.Logf("dispatch error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestRPCDispatcher_CustomRouteForward(t *testing.T) {
	ctx := context.Background()

	validator := NewMockRuntimeValidator()
	nsReg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	if err := nsReg.Register(ctx, rpc.Route{
		RuntimeID: "runtime-1",
		PluginID:  "plugin-1",
		ServiceID: "bridge",
		Namespace: "minecraft",
	}); err != nil {
		t.Fatalf("register namespace failed: %v", err)
	}

	controlPlane := &mockControlPlane{}

	disp := rpc.NewRPCDispatcher(rpc.DispatcherConfig{
		Namespaces: nsReg,
	})
	disp.SetControlPlane(controlPlane)

	err := disp.Dispatch(ctx, ipc.Peer{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "agent",
	}, protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "custom-req-1",
		Method:   "minecraft.bot.move",
		Payload:  json.RawMessage(`{"x":1,"y":2,"z":3}`),
	})

	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	if len(controlPlane.sent) != 1 {
		t.Fatalf("expected 1 forwarded message, got %d", len(controlPlane.sent))
	}

	forwarded := controlPlane.sent[0]
	if forwarded.Peer.ServiceID != "bridge" {
		t.Errorf("forward target should be bridge, got %s", forwarded.Peer.ServiceID)
	}
	if forwarded.Envelope.ID != "custom-req-1" {
		t.Errorf("request ID should be preserved")
	}
	if forwarded.Envelope.Method != "minecraft.bot.move" {
		t.Errorf("method should be preserved")
	}
}

func TestRPCDispatcher_CustomNamespaceNotFound(t *testing.T) {
	ctx := context.Background()

	validator := NewMockRuntimeValidator()
	nsReg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{
		Validator: validator,
	})

	controlPlane := &mockControlPlane{}

	disp := rpc.NewRPCDispatcher(rpc.DispatcherConfig{
		Namespaces: nsReg,
	})
	disp.SetControlPlane(controlPlane)

	err := disp.Dispatch(ctx, ipc.Peer{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "agent",
	}, protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "req-unknown",
		Method:   "factorio.foo.bar",
	})

	if err == nil {
		t.Fatal("dispatch to unknown namespace should fail")
	}

	if len(controlPlane.sent) != 0 {
		t.Error("no message should be forwarded for unknown namespace")
	}
}

type mockControlPlane struct {
	mu   sync.Mutex
	sent []mockSent
}

type mockSent struct {
	Peer      ipc.Peer
	Envelope  protocol.Envelope
}

func (m *mockControlPlane) Attach(
	ctx context.Context,
	peer ipc.Peer,
	transport ipc.Transport,
) (*ipc.Connection, error) {
	return nil, nil
}

func (m *mockControlPlane) Detach(
	ctx context.Context,
	connectionID ipc.ConnectionID,
) error {
	return nil
}

func (m *mockControlPlane) Send(
	ctx context.Context,
	peer ipc.Peer,
	envelope protocol.Envelope,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, mockSent{
		Peer:     peer,
		Envelope: envelope,
	})
	return nil
}

func (m *mockControlPlane) Shutdown(ctx context.Context) error {
	return nil
}

func init() {
	_ = ipc.NewNoopDispatcher
}

var _ ipc.ControlPlane = (*mockControlPlane)(nil)
