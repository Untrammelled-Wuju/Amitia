package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestRequestState_Terminal(t *testing.T) {
	tests := map[RequestState]bool{
		RequestStateCreated:   false,
		RequestStatePending:   false,
		RequestStateRunning:   false,
		RequestStateCompleted: true,
		RequestStateFailed:    true,
		RequestStateTimedOut:  true,
		RequestStateCancelled: true,
	}

	for state, expected := range tests {
		if state.IsTerminal() != expected {
			t.Errorf("state %s terminal state mismatch: got %v, want %v", state, state.IsTerminal(), expected)
		}
	}
}

func TestRequestKey_FromIPC(t *testing.T) {
	peer := ipc.Peer{
		PluginID:  "plugin-1",
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}
	key := RequestKeyFromIPC("req-123", peer)
	if key.RuntimeID != "runtime-1" {
		t.Error("runtime ID mismatch")
	}
	if key.ServiceID != "service-a" {
		t.Error("service ID mismatch")
	}
	if key.RequestID != "req-123" {
		t.Error("request ID mismatch")
	}
}

func TestRequestKey_String(t *testing.T) {
	key := RequestKey{
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
		RequestID: "req-1",
	}
	expected := "runtime-1/service-a/req-1"
	if key.String() != expected {
		t.Errorf("RequestKey.String() = %q, want %q", key.String(), expected)
	}
}

func TestRequestKey_Validate(t *testing.T) {
	valid := RequestKey{RuntimeID: "r", ServiceID: "s", RequestID: "id"}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid key should not return error: %v", err)
	}

	invalidKeys := []RequestKey{
		{ServiceID: "s", RequestID: "id"},
		{RuntimeID: "r", RequestID: "id"},
		{RuntimeID: "r", ServiceID: "s"},
	}
	for _, k := range invalidKeys {
		if err := k.Validate(); err == nil {
			t.Errorf("invalid key should return error: %v", k)
		}
	}
}

func TestPendingRegistry_BasicCRUD(t *testing.T) {
	reg := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	req := &PendingRequest{
		Key:       RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		State:     RequestStatePending,
		CreatedAt: time.Now().UTC(),
		Done:      make(chan struct{}),
	}

	ok, err := reg.Register(req)
	if !ok || err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if reg.Count() != 1 {
		t.Errorf("expected count 1, got %d", reg.Count())
	}

	env := protocol.Envelope{
		Type:    protocol.MessageTypeResponse,
		ID:      "req-1",
		Payload: json.RawMessage(`{"result":"ok"}`),
	}
	ok, err = reg.Complete(req.Key, env)
	if !ok || err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	ok, _ = reg.Complete(req.Key, env)
	if ok {
		t.Error("double complete should not succeed")
	}

	reg.Remove(req.Key)
	if reg.Count() != 0 {
		t.Errorf("expected count 0 after remove, got %d", reg.Count())
	}
}

func TestPendingRegistry_Timeout(t *testing.T) {
	reg := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	req := &PendingRequest{
		Key:       RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		State:     RequestStatePending,
		CreatedAt: time.Now().UTC(),
		Done:      make(chan struct{}),
	}

	reg.Register(req)

	ok, _ := reg.Timeout(req.Key)
	if !ok {
		t.Error("timeout should succeed")
	}

	ok, _ = reg.Complete(req.Key, protocol.Envelope{})
	if ok {
		t.Error("complete after timeout should fail")
	}
}

func TestPendingRegistry_Cancel(t *testing.T) {
	reg := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	req := &PendingRequest{
		Key:       RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: "req-1"},
		State:     RequestStatePending,
		CreatedAt: time.Now().UTC(),
		Done:      make(chan struct{}),
	}

	reg.Register(req)

	ok, _ := reg.Cancel(req.Key)
	if !ok {
		t.Error("cancel should succeed")
	}

	ok, _ = reg.Cancel(req.Key)
	if ok {
		t.Error("double cancel should fail")
	}
}

func TestPendingRegistry_ListByPeer(t *testing.T) {
	reg := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	for i := 0; i < 3; i++ {
		req := &PendingRequest{
			Key:       RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: fmt.Sprintf("req-%d", i)},
			State:     RequestStatePending,
			CreatedAt: time.Now().UTC(),
			Done:      make(chan struct{}),
		}
		reg.Register(req)
	}

	list := reg.ListByPeer("r1", "s1")
	if len(list) != 3 {
		t.Errorf("expected 3 requests, got %d", len(list))
	}
}

func TestPendingRegistry_Shutdown(t *testing.T) {
	reg := NewPendingRequestRegistry(DefaultPendingRegistryConfig())

	for i := 0; i < 5; i++ {
		req := &PendingRequest{
			Key:       RequestKey{RuntimeID: "r1", ServiceID: "s1", RequestID: fmt.Sprintf("req-%d", i)},
			State:     RequestStatePending,
			CreatedAt: time.Now().UTC(),
			Done:      make(chan struct{}),
		}
		reg.Register(req)
	}

	if reg.Count() != 5 {
		t.Fatalf("expected 5 pending, got %d", reg.Count())
	}

	if r, ok := reg.(*pendingRequestRegistry); ok {
		r.shutdown()
	}

	if reg.Count() != 0 {
		t.Errorf("expected 0 pending after shutdown, got %d", reg.Count())
	}
}

func TestLifecycleManager_BasicOutgoing(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	ctx := context.Background()
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}

	env := protocol.Envelope{
		ID:      "req-1",
		Method:  "minecraft.move",
		Payload: json.RawMessage(`{"x":1}`),
	}

	req, err := lm.HandleOutgoingRequest(ctx, peer, env, 0)
	if err != nil {
		t.Fatalf("HandleOutgoingRequest failed: %v", err)
	}

	if req.Key.RuntimeID != "r1" {
		t.Error("runtime ID mismatch")
	}
	if req.State == RequestStateCompleted {
		t.Error("fresh request should not be completed")
	}
}

func TestLifecycleManager_IdempotentReplay(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	ctx := context.Background()

	env := protocol.Envelope{
		ID:      "req-1",
		Method:  "minecraft.move",
		Payload: json.RawMessage(`{"x":1}`),
	}

	_, err := lm.HandleOutgoingRequest(ctx, peer, env, 0)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	resp := protocol.Envelope{
		Type:      protocol.MessageTypeResponse,
		ID:        "req-1",
		RequestID: "req-1",
		Payload:   json.RawMessage(`{"result":"ok"}`),
	}
	lm.HandleIncomingResponse(peer, resp)

	req2, err := lm.HandleOutgoingRequest(ctx, peer, env, 0)
	if err != nil {
		t.Errorf("second request should replay: %v", err)
	}

	if req2 != nil && req2.Key.RuntimeID != "r1" {
		t.Error("replay should have correct key")
	}
}

func TestLifecycleManager_UnauthorizedCancel(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	owner := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	intruder := ipc.Peer{RuntimeID: "r2", ServiceID: "s2"}
	ctx := context.Background()

	env := protocol.Envelope{
		ID:      "req-1",
		Method:  "minecraft.move",
		Payload: json.RawMessage(`{"x":1}`),
	}

	lm.HandleOutgoingRequest(ctx, owner, env, 0)

	err := lm.HandleCancel(intruder, CancelRequest{RequestID: "req-1"})
	if err == nil {
		t.Error("unauthorized cancel should fail")
	}
}

func TestLifecycleManager_CancelSuccess(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	ctx := context.Background()

	env := protocol.Envelope{
		ID:      "req-1",
		Method:  "minecraft.move",
		Payload: json.RawMessage(`{"x":1}`),
	}

	lm.HandleOutgoingRequest(ctx, peer, env, 0)

	err := lm.HandleCancel(peer, CancelRequest{RequestID: "req-1"})
	if err != nil {
		t.Errorf("cancel should succeed: %v", err)
	}
}

func TestLifecycleManager_UnknownCancelReturnsError(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}

	err := lm.HandleCancel(peer, CancelRequest{RequestID: "unknown-req"})
	if err == nil {
		t.Error("unknown cancel should return error")
	}
}

func TestLifecycleManager_Shutdown(t *testing.T) {
	lm := NewLifecycleManager(LifecycleManagerConfig{})
	ctx := context.Background()

	err := lm.Shutdown(ctx)
	if err != nil {
		t.Errorf("shutdown should succeed: %v", err)
	}
}

func TestTimeoutConfig_Defaults(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	if cfg.Default == 0 {
		t.Error("default timeout should not be 0")
	}
	if cfg.Maximum == 0 {
		t.Error("maximum timeout should not be 0")
	}
	if cfg.Minimum == 0 {
		t.Error("minimum timeout should not be 0")
	}
}

func TestEffectiveDeadline_DefaultOnly(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	now := time.Now()

	deadline, _ := EffectiveDeadline(context.Background(), 0, cfg, now)
	if !deadline.After(now) {
		t.Error("deadline should be in the future")
	}
}

func TestEffectiveDeadline_ProtocolTimeout(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	now := time.Now()

	deadline, _ := EffectiveDeadline(context.Background(), 5000, cfg, now)
	expected := now.Add(5 * time.Second)
	diff := deadline.Sub(expected)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("protocol timeout 5000ms should yield ~5s deadline, got %v", deadline.Sub(now))
	}
}

func TestEffectiveDeadline_MaximumClamp(t *testing.T) {
	cfg := TimeoutConfig{
		Default: DefaultRPCTimeout,
		Maximum: 10 * time.Second,
		Minimum: MinimumRPCTimeout,
	}
	now := time.Now()

	deadline, _ := EffectiveDeadline(context.Background(), int64((60 * time.Second).Milliseconds()), cfg, now)
	maxDeadline := now.Add(cfg.Maximum + time.Second)
	if deadline.After(maxDeadline) {
		t.Error("deadline should be clamped to maximum")
	}
}
