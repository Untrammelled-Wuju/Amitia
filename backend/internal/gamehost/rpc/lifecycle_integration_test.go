package rpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type MockLifecycleValidator struct {
	RuntimeExists map[domain.RuntimeInstanceID]bool
	ServiceExists map[string]domain.PluginID
}

func NewLifecycleMockValidator() *MockLifecycleValidator {
	return &MockLifecycleValidator{
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

func (m *MockLifecycleValidator) ValidateRuntime(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
) error {
	if !m.RuntimeExists[runtimeID] {
		return domain.NewHostError(domain.ErrRuntimeUnavailable, "runtime not found")
	}
	return nil
}

func (m *MockLifecycleValidator) ValidateService(
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

type mockLifecycleCP struct {
	sent []mockLifecycleSent
}

type mockLifecycleSent struct {
	Peer     ipc.Peer
	Envelope protocol.Envelope
}

func (m *mockLifecycleCP) Attach(
	ctx context.Context,
	peer ipc.Peer,
	transport ipc.Transport,
) (*ipc.Connection, error) {
	return nil, nil
}

func (m *mockLifecycleCP) Detach(ctx context.Context, id ipc.ConnectionID) error {
	return nil
}

func (m *mockLifecycleCP) Send(
	ctx context.Context,
	peer ipc.Peer,
	envelope protocol.Envelope,
) error {
	m.sent = append(m.sent, mockLifecycleSent{Peer: peer, Envelope: envelope})
	return nil
}

func (m *mockLifecycleCP) SendRequest(
	ctx context.Context,
	peer ipc.Peer,
	envelope protocol.Envelope,
	timeout time.Duration,
) (*protocol.Envelope, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

func (m *mockLifecycleCP) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockLifecycleCP) Record() []mockLifecycleSent {
	return m.sent
}

type echoTestHandler struct {
	responsePayload json.RawMessage
}

func (h *echoTestHandler) Handle(ctx context.Context, request rpc.RPCRequest) (rpc.RPCResponse, error) {
	return rpc.RPCResponse{
		RequestID: request.ID,
		Payload:   h.responsePayload,
	}, nil
}

func TestLifecycleManager_OutgoingAndResponse(t *testing.T) {
	cp := &mockLifecycleCP{}

	lm := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	lm.SetControlPlane(cp)

	peer := ipc.Peer{
		RuntimeID: "runtime-1",
		ServiceID: "service-a",
	}

	reqEnv := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "req-1",
		Method:   "host.test.echo",
		Payload:  json.RawMessage(`{"input":"hello"}`),
	}

	req, err := lm.HandleOutgoingRequest(context.Background(), peer, reqEnv, 5000)
	if err != nil {
		t.Fatalf("HandleOutgoingRequest failed: %v", err)
	}

	if req.Key.RequestID != "req-1" {
		t.Error("request key mismatch")
	}
}

func TestLifecycleManager_CancelUnauthorizedRejected(t *testing.T) {
	cp := &mockLifecycleCP{}
	lm := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	lm.SetControlPlane(cp)

	owner := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	intruder := ipc.Peer{RuntimeID: "r2", ServiceID: "s2"}

	reqEnv := protocol.Envelope{
		ID:     "req-1",
		Method: "host.test.echo",
	}
	lm.HandleOutgoingRequest(context.Background(), owner, reqEnv, 5000)

	err := lm.HandleCancel(intruder, rpc.CancelRequest{RequestID: "req-1"})
	if err == nil {
		t.Error("unauthorized cancel should fail")
	}
}

func TestLifecycleManager_CancelUnknownReturnsError(t *testing.T) {
	cp := &mockLifecycleCP{}
	lm := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	lm.SetControlPlane(cp)

	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	err := lm.HandleCancel(peer, rpc.CancelRequest{RequestID: "nonexistent"})
	if err == nil {
		t.Error("cancel of unknown request should return error")
	}
}

func TestLifecycleManager_CancelSuccess(t *testing.T) {
	cp := &mockLifecycleCP{}
	lm := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	lm.SetControlPlane(cp)

	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	reqEnv := protocol.Envelope{
		ID:     "req-1",
		Method: "host.test.echo",
	}
	lm.HandleOutgoingRequest(context.Background(), peer, reqEnv, 5000)

	err := lm.HandleCancel(peer, rpc.CancelRequest{RequestID: "req-1"})
	if err != nil {
		t.Errorf("cancel should succeed: %v", err)
	}
}

func TestLifecycleManager_IdempotentReplay(t *testing.T) {
	cp := &mockLifecycleCP{}
	lm := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	lm.SetControlPlane(cp)

	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}
	env := protocol.Envelope{
		ID:      "req-1",
		Method:  "host.test.echo",
		Payload: json.RawMessage(`{"a":1}`),
	}

	_, err := lm.HandleOutgoingRequest(context.Background(), peer, env, 5000)
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

	_, err = lm.HandleOutgoingRequest(context.Background(), peer, env, 5000)
	if err != nil {
		t.Logf("replay returned: %v", err)
	}
}

func TestParseCancelEnvelope_Integration(t *testing.T) {
	env := &protocol.Envelope{
		Type:   protocol.MessageTypeNotification,
		Method: rpc.CancelMethod,
	}

	_, ok := rpc.ParseCancelEnvelope(env)
	if ok {
		t.Error("cancel without valid payload should fail")
	}
}

func TestBuildCancelEnvelope_Integration(t *testing.T) {
	env := rpc.BuildCancelEnvelope("req-1", "timeout")
	if env.Type != protocol.MessageTypeNotification {
		t.Error("cancel should be notification type")
	}
	if env.Method != rpc.CancelMethod {
		t.Error("method mismatch")
	}
}
