package ipc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestControlPlane_Attach_Success(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, _ := NewMemoryTransportPair()

	conn, err := cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	if conn == nil {
		t.Fatal("connection should not be nil")
	}

	if conn.ID == "" {
		t.Fatal("connection ID should not be empty")
	}

	if conn.Peer.PluginID != peer.PluginID {
		t.Errorf("peer plugin ID mismatch: got %s, want %s", conn.Peer.PluginID, peer.PluginID)
	}

	if conn.State() != ipc.ConnectionStateAttached {
		t.Errorf("connection state should be attached, got %s", conn.State())
	}
}

func TestControlPlane_Attach_UnknownRuntime(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		Err: domain.NewHostError(domain.ErrNotFound, "runtime not found"),
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-missing",
		ServiceID: "service-1",
	}

	pluginTransport, _ := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err == nil {
		t.Fatal("Attach should fail for unknown runtime")
	}
}

func TestControlPlane_Attach_UnknownService(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		Err: domain.NewHostError(domain.ErrNotFound, "service not found"),
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-missing",
	}

	pluginTransport, _ := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err == nil {
		t.Fatal("Attach should fail for unknown service")
	}
}

func TestControlPlane_Attach_PluginMismatch(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "different.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, _ := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err == nil {
		t.Fatal("Attach should fail when plugin ID does not match runtime's plugin")
	}
}

func TestControlPlane_Attach_DuplicatePeerConnection(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	transport1, _ := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, transport1)
	if err != nil {
		t.Fatalf("first Attach failed: %v", err)
	}

	transport2, _ := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, transport2)
	if err == nil {
		t.Fatal("second Attach should fail with already_exists error")
	}
}

func TestControlPlane_Send_Success(t *testing.T) {
	ctx := context.Background()

	var receivedEnv protocol.Envelope
	var mu sync.Mutex

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	go func() {
		env, _ := hostTransport.Receive(ctx)
		mu.Lock()
		receivedEnv = env
		mu.Unlock()
	}()

	env := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "msg-1",
		Method:   "vendor.custom.action",
	}

	err = cp.Send(ctx, peer, env)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if receivedEnv.ID != "msg-1" {
		t.Errorf("received envelope ID mismatch: got %s, want msg-1", receivedEnv.ID)
	}
}

func TestControlPlane_Send_NoConnection(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-missing",
		ServiceID: "service-1",
	}

	env := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "msg-1",
		Method:   "vendor.custom.action",
	}

	err = cp.Send(ctx, peer, env)
	if err == nil {
		t.Fatal("Send should fail when no connection exists for peer")
	}
}

func TestControlPlane_Detach_Idempotent(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, _ := NewMemoryTransportPair()

	conn, err := cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	err = cp.Detach(ctx, conn.ID)
	if err != nil {
		t.Fatalf("first Detach failed: %v", err)
	}

	err = cp.Detach(ctx, conn.ID)
	if err != nil {
		t.Fatalf("second Detach should be idempotent: %v", err)
	}
}

func TestControlPlane_Shutdown(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peers := []ipc.Peer{
		{PluginID: "test.plugin", RuntimeID: "runtime-1", ServiceID: "service-1"},
		{PluginID: "test.plugin", RuntimeID: "runtime-1", ServiceID: "service-2"},
		{PluginID: "test.plugin", RuntimeID: "runtime-2", ServiceID: "service-1"},
	}

	for _, peer := range peers {
		tp, _ := NewMemoryTransportPair()
		_, err := cp.Attach(ctx, peer, tp)
		if err != nil {
			t.Fatalf("Attach failed for peer %v: %v", peer, err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = cp.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	err = cp.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("second Shutdown should be no-op: %v", err)
	}
}

func TestControlPlane_Attach_AfterShutdown(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: ipc.NewNoopDispatcher(),
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = cp.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	tp, _ := NewMemoryTransportPair()
	_, err = cp.Attach(ctx, peer, tp)
	if err == nil {
		t.Fatal("Attach should fail after Shutdown")
	}
}

func TestControlPlane_ReceiveLoop_EnvelopeValidation(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	var events []ipc.ConnectionEvent
	var mu sync.Mutex

	handler := func(evt ipc.ConnectionEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:     resolver,
		Dispatcher:   ipc.NewNoopDispatcher(),
		EventHandler: handler,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	hasAttach := false
	for _, evt := range events {
		if evt.Type == ipc.EventConnectionAttached {
			hasAttach = true
		}
	}
	mu.Unlock()

	if !hasAttach {
		t.Error("expected ipc.connection.attached event")
	}

	invalidEnv := protocol.Envelope{
		Protocol: "wrong-protocol/99",
		Type:     protocol.MessageTypeRequest,
		ID:       "bad-env",
		Method:   "test.method",
	}

	sendCtx := context.Background()
	err = hostTransport.Send(sendCtx, invalidEnv)
	if err != nil {
		t.Fatalf("host send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	hasError := false
	for _, evt := range events {
		if evt.Type == ipc.EventConnectionError {
			hasError = true
		}
	}
	mu.Unlock()

	if !hasError {
		t.Error("expected ipc.connection.error event for invalid envelope")
	}
}

func TestControlPlane_PeerSpoofPrevention(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	var events []ipc.ConnectionEvent
	var mu sync.Mutex

	handler := func(evt ipc.ConnectionEvent) {
		mu.Lock()
		events = append(events, evt)
		mu.Unlock()
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:     resolver,
		Dispatcher:   ipc.NewNoopDispatcher(),
		EventHandler: handler,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-a",
		ServiceID: "service-a",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	spoofEnv := protocol.Envelope{
		Protocol:  protocol.ProtocolVersion,
		Type:      protocol.MessageTypeRequest,
		ID:        "spoof-msg",
		Method:    "vendor.custom.action",
		RuntimeID: "runtime-b",
	}

	sendCtx := context.Background()
	err = hostTransport.Send(sendCtx, spoofEnv)
	if err != nil {
		t.Fatalf("host send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	hasError := false
	for _, evt := range events {
		if evt.Type == ipc.EventConnectionError {
			hasError = true
		}
	}
	mu.Unlock()

	if !hasError {
		t.Error("expected connection error for peer spoofing attempt")
	}
}

func TestControlPlane_CustomMethodNoBusinessInterpretation(t *testing.T) {
	ctx := context.Background()

	resolver := &MockResolver{
		PluginID: "test.plugin",
	}

	type recorded struct {
		peer      ipc.Peer
		envelope protocol.Envelope
	}

	var recordedMu sync.Mutex
	var recordedCalls []recorded

	testDispatcher := &recordingDispatcher{
		recordFn: func(p ipc.Peer, e protocol.Envelope) {
			recordedMu.Lock()
			recordedCalls = append(recordedCalls, recorded{peer: p, envelope: e})
			recordedMu.Unlock()
		},
	}

	cfg := ipc.ControlPlaneConfig{
		Resolver:   resolver,
		Dispatcher: testDispatcher,
	}

	cp, err := ipc.NewControlPlane(cfg)
	if err != nil {
		t.Fatalf("NewControlPlane failed: %v", err)
	}

	peer := ipc.Peer{
		PluginID:  "test.plugin",
		RuntimeID: "runtime-1",
		ServiceID: "service-1",
	}

	pluginTransport, hostTransport := NewMemoryTransportPair()

	_, err = cp.Attach(ctx, peer, pluginTransport)
	if err != nil {
		t.Fatalf("Attach failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	customEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "custom-msg",
		Method:   "minecraft.agent.execute",
	}

	sendCtx := context.Background()
	err = hostTransport.Send(sendCtx, customEnv)
	if err != nil {
		t.Fatalf("host send failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	recordedMu.Lock()
	defer recordedMu.Unlock()
	if len(recordedCalls) == 0 {
		t.Fatal("dispatcher should have received the custom method envelope")
	}

	if recordedCalls[0].envelope.Method != "minecraft.agent.execute" {
		t.Errorf("method should be preserved as-is, got %s", recordedCalls[0].envelope.Method)
	}
}

type recordingDispatcher struct {
	recordFn func(peer ipc.Peer, envelope protocol.Envelope)
}

func (d *recordingDispatcher) Dispatch(ctx context.Context, peer ipc.Peer, envelope protocol.Envelope) error {
	if d.recordFn != nil {
		d.recordFn(peer, envelope)
	}
	return nil
}
