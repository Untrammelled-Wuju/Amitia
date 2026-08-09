package ipc_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func newTestHandshakeManager() *handshake.HandshakeManager {
	validator := newHandshakeTestValidator()
	registry := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})
	adapter := handshake.NewNamespaceAdapter(registry)
	advertiser := handshake.NoopChannelAdvertiser{}
	return handshake.NewHandshakeManager(handshake.HandshakeManagerConfig{
		HostSupportedProtocols: []string{"amitia-game-host/1"},
		HostCapabilities: []domain.Capability{
			domain.CapabilityCustomRPC,
			domain.CapabilityEventStreaming,
			domain.CapabilityStateStreaming,
			domain.CapabilityBinaryStreaming,
			domain.CapabilityHostAPI,
		},
		NamespaceAdapter:  adapter,
		ChannelAdvertiser: advertiser,
		RuntimeValidator:  validator,
	})
}

func TestControlPlane_HandshakeGate_BusinessBlockedBeforeHello(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{PluginID: "gate.plugin"}
	mgr := newTestHandshakeManager()
	controller := handshake.NewHandshakeControllerAdapter(mgr)

	var dispatched []protocol.Envelope
	var mu sync.Mutex

	dispatcher := &recordingDispatcher{
		recordFn: func(_ ipc.Peer, e protocol.Envelope) {
			mu.Lock()
			dispatched = append(dispatched, e)
			mu.Unlock()
		},
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:            resolver,
		HandshakeController: controller,
		Dispatcher:          dispatcher,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "gate.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	bizEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "biz-1",
		Method:   "vendor.custom.action",
	}

	if err := hostTransport.Send(ctx, bizEnv); err != nil {
		t.Fatalf("host send biz failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	for _, env := range dispatched {
		if env.ID == "biz-1" {
			t.Fatal("business envelope must be blocked before handshake completes")
		}
	}
}

func TestControlPlane_HandshakeGate_HelloSucceedsAndUnlocksTraffic(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{PluginID: "hello.plugin"}
	mgr := newTestHandshakeManager()
	controller := handshake.NewHandshakeControllerAdapter(mgr)

	var dispatched []protocol.Envelope
	var mu sync.Mutex

	dispatcher := &recordingDispatcher{
		recordFn: func(_ ipc.Peer, e protocol.Envelope) {
			mu.Lock()
			dispatched = append(dispatched, e)
			mu.Unlock()
		},
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:            resolver,
		HandshakeController: controller,
		Dispatcher:          dispatcher,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "hello.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	helloPayload, _ := json.Marshal(map[string]interface{}{
		"supportedProtocols": []string{"amitia-game-host/1"},
		"capabilities":       []string{"custom_rpc"},
	})

	helloEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "hello-1",
		Method:   ipc.HandshakeMethod,
		Payload:  helloPayload,
	}

	if err := hostTransport.Send(ctx, helloEnv); err != nil {
		t.Fatalf("hello send failed: %v", err)
	}

	select {
	case resp := <-recvFromTransport(ctx, hostTransport):
		if resp.Error != nil {
			t.Fatalf("hello response contained an error: %+v", resp.Error)
		}
		if resp.Type != protocol.MessageTypeResponse {
			t.Errorf("expected response message type, got %s", resp.Type)
		}
		var body handshake.HelloResponse
		if err := json.Unmarshal(resp.Payload, &body); err != nil {
			t.Fatalf("hello response payload is not valid: %v", err)
		}
		if body.Protocol != "amitia-game-host/1" {
			t.Errorf("negotiated protocol mismatch: got %s, want amitia-game-host/1", body.Protocol)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for hello response")
	}

	bizEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "biz-2",
		Method:   "vendor.custom.action",
	}

	if err := hostTransport.Send(ctx, bizEnv); err != nil {
		t.Fatalf("biz send failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	var found bool
	for _, env := range dispatched {
		if env.ID == "biz-2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("business envelope should have been forwarded after successful hello; got %d envelopes", len(dispatched))
	}
}

func TestControlPlane_HandshakeGate_ProtocolMismatchFails(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{PluginID: "mismatch.plugin"}
	mgr := newTestHandshakeManager()
	controller := handshake.NewHandshakeControllerAdapter(mgr)

	var errors []*ipc.IPCError
	var mu sync.Mutex

	errorHandler := func(evt ipc.ConnectionEvent) {
		if evt.Error == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if ipcErr, ok := evt.Error.(*ipc.IPCError); ok {
			errors = append(errors, ipcErr)
		}
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:            resolver,
		HandshakeController: controller,
		Dispatcher:          ipc.NewNoopDispatcher(),
		EventHandler:        errorHandler,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "mismatch.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	helloPayload, _ := json.Marshal(map[string]interface{}{
		"supportedProtocols": []string{"amitia-game-host/99"},
		"capabilities":       []string{"realtime_control"},
	})

	helloEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "hello-bad",
		Method:   ipc.HandshakeMethod,
		Payload:  helloPayload,
	}

	if err := hostTransport.Send(ctx, helloEnv); err != nil {
		t.Fatalf("hello send failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	var sawMismatch bool
	for _, e := range errors {
		if e.Code == domain.ErrInvalidArgument || e.Code == domain.ErrProtocolMismatch {
			sawMismatch = true
			break
		}
	}
	if !sawMismatch {
		t.Errorf("expected handshake error for protocol mismatch, got %+v", errors)
	}
}

func TestControlPlane_HandshakeGate_NotReadyFailsDuplicateHello(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{PluginID: "duphello.plugin"}
	mgr := newTestHandshakeManager()
	controller := handshake.NewHandshakeControllerAdapter(mgr)

	cfg := ipc.ControlPlaneConfig{
		Resolver:            resolver,
		HandshakeController: controller,
		Dispatcher:          ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "duphello.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	helloPayload, _ := json.Marshal(map[string]interface{}{
		"supportedProtocols": []string{"amitia-game-host/1"},
		"capabilities":       []string{"custom_rpc"},
	})

	firstHello := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "hello-first",
		Method:   ipc.HandshakeMethod,
		Payload:  helloPayload,
	}

	if err := hostTransport.Send(ctx, firstHello); err != nil {
		t.Fatalf("first hello send failed: %v", err)
	}

	select {
	case <-recvFromTransport(ctx, hostTransport):
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first hello response")
	}

	secondHello := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "hello-second",
		Method:   ipc.HandshakeMethod,
		Payload:  helloPayload,
	}

	if err := hostTransport.Send(ctx, secondHello); err != nil {
		t.Fatalf("second hello send failed: %v", err)
	}

	select {
	case resp := <-recvFromTransport(ctx, hostTransport):
		if resp.Error == nil {
			t.Error("expected error response for duplicate hello after ready")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second hello response")
	}
}

func recvFromTransport(ctx context.Context, tr ipc.Transport) <-chan protocol.Envelope {
	ch := make(chan protocol.Envelope, 1)
	go func() {
		env, err := tr.Receive(ctx)
		if err == nil {
			ch <- env
		}
		close(ch)
	}()
	return ch
}

type testHandshakeValidator struct{}

func newHandshakeTestValidator() *testHandshakeValidator {
	return &testHandshakeValidator{}
}

func (v *testHandshakeValidator) RuntimeExists(runtimeID string) (bool, error) {
	return runtimeID == "runtime-1" || runtimeID == "runtime-2", nil
}

func (v *testHandshakeValidator) ServiceBelongsToRuntime(runtimeID, serviceID, pluginID string) error {
	valid := map[string]bool{
		"runtime-1/service-a": true,
		"runtime-1/service-b": true,
		"runtime-2/service-a": true,
	}
	if !valid[runtimeID+"/"+serviceID] {
		return domain.NewHostError(domain.ErrNotFound, "service not found")
	}
	return nil
}
