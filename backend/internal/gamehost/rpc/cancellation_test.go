package rpc

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestParseCancelEnvelope_Valid(t *testing.T) {
	env := &protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeNotification,
		Method:   CancelMethod,
		Payload:  json.RawMessage(`{"requestId":"req-1"}`),
	}

	req, ok := ParseCancelEnvelope(env)
	if !ok {
		t.Fatal("should parse valid cancel envelope")
	}
	if req.RequestID != "req-1" {
		t.Errorf("request ID mismatch: got %q", req.RequestID)
	}
}

func TestParseCancelEnvelope_WrongMethod(t *testing.T) {
	env := &protocol.Envelope{
		Type:   protocol.MessageTypeNotification,
		Method: "some.other.method",
	}

	_, ok := ParseCancelEnvelope(env)
	if ok {
		t.Error("wrong method should not parse as cancel")
	}
}

func TestParseCancelEnvelope_EmptyPayload(t *testing.T) {
	env := &protocol.Envelope{
		Type:   protocol.MessageTypeNotification,
		Method: CancelMethod,
	}

	_, ok := ParseCancelEnvelope(env)
	if ok {
		t.Error("empty payload should not parse as cancel")
	}
}

func TestParseCancelEnvelope_InvalidJSON(t *testing.T) {
	env := &protocol.Envelope{
		Type:    protocol.MessageTypeNotification,
		Method:  CancelMethod,
		Payload: json.RawMessage(`not valid json`),
	}

	_, ok := ParseCancelEnvelope(env)
	if ok {
		t.Error("invalid JSON should not parse as cancel")
	}
}

func TestParseCancelEnvelope_EmptyRequestID(t *testing.T) {
	env := &protocol.Envelope{
		Type:    protocol.MessageTypeNotification,
		Method:  CancelMethod,
		Payload: json.RawMessage(`{"requestId":"  "}`),
	}

	_, ok := ParseCancelEnvelope(env)
	if ok {
		t.Error("empty request ID should not parse")
	}
}

func TestParseCancelEnvelope_ReasonTruncated(t *testing.T) {
	longReason := strings.Repeat("x", 200)
	payload := `{"requestId":"req-1","reason":"` + longReason + `"}`
	env := &protocol.Envelope{
		Type:    protocol.MessageTypeNotification,
		Method:  CancelMethod,
		Payload: json.RawMessage(payload),
	}

	req, ok := ParseCancelEnvelope(env)
	if !ok {
		t.Fatal("should parse with long reason")
	}
	if len(req.Reason) > MaxCancelReasonLen {
		t.Errorf("reason should be truncated to %d, got %d", MaxCancelReasonLen, len(req.Reason))
	}
}

func TestBuildCancelEnvelope(t *testing.T) {
	env := BuildCancelEnvelope("req-1", "timeout")

	if env.Type != protocol.MessageTypeNotification {
		t.Errorf("type should be notification, got %s", env.Type)
	}
	if env.Method != CancelMethod {
		t.Errorf("method mismatch: got %s", env.Method)
	}

	var req CancelRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if req.RequestID != "req-1" {
		t.Error("request ID mismatch")
	}
	if req.Reason != "timeout" {
		t.Error("reason mismatch")
	}
}

func TestValidateCancelSender_Matching(t *testing.T) {
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1"}
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s1"}

	if err := ValidateCancelSender(key, peer); err != nil {
		t.Errorf("matching sender should not error: %v", err)
	}
}

func TestValidateCancelSender_DifferentRuntime(t *testing.T) {
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1"}
	peer := ipc.Peer{RuntimeID: "r2", ServiceID: "s1"}

	if err := ValidateCancelSender(key, peer); err == nil {
		t.Error("different runtime should be rejected")
	}
}

func TestValidateCancelSender_DifferentService(t *testing.T) {
	key := RequestKey{RuntimeID: "r1", ServiceID: "s1"}
	peer := ipc.Peer{RuntimeID: "r1", ServiceID: "s2"}

	if err := ValidateCancelSender(key, peer); err == nil {
		t.Error("different service should be rejected")
	}
}
