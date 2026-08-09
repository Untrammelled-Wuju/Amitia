package sdk

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestBuildDescriptor(t *testing.T) {
	metadata := json.RawMessage(`{"key": "value"}`)
	desc, err := NewDescriptor("example.game", "Example Game", "1.0.0").
		WithService(protocol.ServiceDescriptor{
			ID:   "agent",
			Kind: protocol.ServiceKindProcess,
		}).
		WithChannel(protocol.ChannelDescriptor{
			ID:   "events",
			Kind: protocol.ChannelKindEvent,
		}).
		WithCapability(protocol.CapabilityCustomRPC).
		WithMetadata("custom", metadata).
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if desc.ID != "example.game" {
		t.Fatalf("expected ID 'example.game', got '%s'", desc.ID)
	}
	if desc.Name != "Example Game" {
		t.Fatalf("expected Name 'Example Game', got '%s'", desc.Name)
	}
	if desc.Version != "1.0.0" {
		t.Fatalf("expected Version '1.0.0', got '%s'", desc.Version)
	}
	if desc.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected ProtocolVersion '%s', got '%s'", protocol.ProtocolVersion, desc.ProtocolVersion)
	}
	if len(desc.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(desc.Services))
	}
	if len(desc.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(desc.Capabilities))
	}
}

func TestBuildDescriptorRejectsDuplicateCapability(t *testing.T) {
	_, err := NewDescriptor("example.game", "Example Game", "1.0.0").
		WithCapability(protocol.CapabilityCustomRPC).
		WithCapability(protocol.CapabilityCustomRPC).
		Build()

	if err == nil {
		t.Fatal("expected error for duplicate capability, got nil")
	}
}

func TestNewRequest(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001")
	client := NewClient(transport, WithIDGenerator(idGen))

	payload := map[string]any{
		"goal": "build a shelter",
	}
	envelope, err := client.NewRequest("minecraft.agent.submit_goal", payload)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	if envelope.Protocol != protocol.ProtocolVersion {
		t.Fatalf("expected Protocol '%s', got '%s'", protocol.ProtocolVersion, envelope.Protocol)
	}
	if envelope.Type != protocol.MessageTypeRequest {
		t.Fatalf("expected Type 'request', got '%s'", envelope.Type)
	}
	if envelope.ID != "msg-001" {
		t.Fatalf("expected ID 'msg-001', got '%s'", envelope.ID)
	}
	if envelope.Method != "minecraft.agent.submit_goal" {
		t.Fatalf("expected Method 'minecraft.agent.submit_goal', got '%s'", envelope.Method)
	}
	if len(envelope.Payload) == 0 {
		t.Fatal("expected non-empty payload")
	}
}

func TestNewResponse(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001", "msg-002")
	client := NewClient(transport, WithIDGenerator(idGen))

	request, err := client.NewRequest("minecraft.agent.submit_goal", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	payload := map[string]any{
		"status": "accepted",
	}
	response, err := client.NewResponse(request, payload)
	if err != nil {
		t.Fatalf("NewResponse failed: %v", err)
	}

	if response.RequestID != request.ID {
		t.Fatalf("expected RequestID '%s', got '%s'", request.ID, response.RequestID)
	}
	if response.Type != protocol.MessageTypeResponse {
		t.Fatalf("expected Type 'response', got '%s'", response.Type)
	}
	if response.Protocol != protocol.ProtocolVersion {
		t.Fatalf("expected Protocol '%s', got '%s'", protocol.ProtocolVersion, response.Protocol)
	}
}

func TestNewNotification(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-003")
	client := NewClient(transport, WithIDGenerator(idGen))

	payload := map[string]any{
		"state": "running",
	}
	notification, err := client.NewNotification("vendor.runtime.state_changed", payload)
	if err != nil {
		t.Fatalf("NewNotification failed: %v", err)
	}

	if notification.Type != protocol.MessageTypeNotification {
		t.Fatalf("expected Type 'notification', got '%s'", notification.Type)
	}
	if notification.Method != "vendor.runtime.state_changed" {
		t.Fatalf("expected Method 'vendor.runtime.state_changed', got '%s'", notification.Method)
	}
	if notification.RequestID != "" {
		t.Fatalf("expected empty RequestID for notification, got '%s'", notification.RequestID)
	}
}

func TestNewErrorResponse(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001", "msg-004")
	client := NewClient(transport, WithIDGenerator(idGen))

	request, err := client.NewRequest("minecraft.agent.submit_goal", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	data := map[string]any{
		"reason": "goal not allowed",
	}
	envelope, err := client.NewError(request, protocol.ErrorPermissionDenied, "permission denied", false, data)
	if err != nil {
		t.Fatalf("NewError failed: %v", err)
	}

	if envelope.Error == nil {
		t.Fatal("expected Error to be non-nil")
	}
	if envelope.Error.Code != protocol.ErrorPermissionDenied {
		t.Fatalf("expected Error.Code '%s', got '%s'", protocol.ErrorPermissionDenied, envelope.Error.Code)
	}
	if envelope.Error.Message != "permission denied" {
		t.Fatalf("expected Error.Message 'permission denied', got '%s'", envelope.Error.Message)
	}
	if envelope.RequestID != request.ID {
		t.Fatalf("expected RequestID '%s', got '%s'", request.ID, envelope.RequestID)
	}
}

func TestDecodePayload(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport)

	originalPayload := map[string]any{
		"player": "steve",
		"position": map[string]any{
			"x": 100,
			"y": 64,
			"z": -200,
		},
	}

	envelope, err := client.NewRequest("game.event.player_moved", originalPayload)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	result, err := DecodePayload[map[string]any](envelope)
	if err != nil {
		t.Fatalf("DecodePayload failed: %v", err)
	}

	if result["player"] != "steve" {
		t.Fatalf("expected player 'steve', got '%v'", result["player"])
	}
	pos := result["position"].(map[string]any)
	if pos["x"] != float64(100) {
		t.Fatalf("expected position.x 100, got %v", pos["x"])
	}
}

func TestRawPayloadPreserved(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport)

	rawData := json.RawMessage(`{"unknown_field": [1, 2, 3], "another": {"nested": true}}`)

	envelope, err := client.NewRequest("custom.action", rawData)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	decoded := DecodeRawPayload(envelope)
	if string(decoded) != string(rawData) {
		t.Fatalf("expected raw payload to be preserved, got '%s'", string(decoded))
	}
}

func TestCustomMethod(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001")
	client := NewClient(transport, WithIDGenerator(idGen))

	methods := []string{
		"minecraft.agent.submit_goal",
		"minecraft.state.query",
		"vendor.control.execute",
		"custom.foo.bar",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			_, err := client.NewRequest(method, nil)
			if err != nil {
				t.Fatalf("NewRequest failed for method '%s': %v", method, err)
			}
		})
	}
}

func TestReservedHostMethodRejectedForPlugin(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport)
	ctx := context.Background()

	reservedMethods := []string{
		"host.secret.read",
		"host.runtime.health",
		"runtime.internal.state",
		"service.register",
	}

	for _, method := range reservedMethods {
		t.Run(method, func(t *testing.T) {
			_, err := client.SendRequest(ctx, method, nil)
			if err == nil {
				t.Fatalf("expected error for reserved method '%s', got nil", method)
			}
		})
	}
}

func TestDeterministicIDGenerator(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001", "msg-002", "msg-003")
	client := NewClient(transport, WithIDGenerator(idGen))

	for i, expected := range []string{"msg-001", "msg-002", "msg-003"} {
		envelope, err := client.NewRequest("custom.action", nil)
		if err != nil {
			t.Fatalf("NewRequest #%d failed: %v", i, err)
		}
		if envelope.ID != expected {
			t.Fatalf("expected ID '%s', got '%s'", expected, envelope.ID)
		}
	}
}

func TestSendRequestWithTransport(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001")
	client := NewClient(transport, WithIDGenerator(idGen))

	ctx := context.Background()
	_, err := client.SendRequest(ctx, "minecraft.agent.submit_goal", map[string]any{
		"goal": "mine diamonds",
	})
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(messages))
	}

	if messages[0].Method != "minecraft.agent.submit_goal" {
		t.Fatalf("expected method 'minecraft.agent.submit_goal', got '%s'", messages[0].Method)
	}
}

func TestSendResponseWithTransport(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001", "msg-002")
	client := NewClient(transport, WithIDGenerator(idGen))

	ctx := context.Background()

	request, err := client.NewRequest("minecraft.agent.submit_goal", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	_, err = client.SendResponse(ctx, request, map[string]any{"status": "accepted"})
	if err != nil {
		t.Fatalf("SendResponse failed: %v", err)
	}

	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(messages))
	}

	if messages[0].RequestID != request.ID {
		t.Fatalf("expected RequestID '%s', got '%s'", request.ID, messages[0].RequestID)
	}
}

func TestSendNotificationWithTransport(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-003")
	client := NewClient(transport, WithIDGenerator(idGen))

	ctx := context.Background()
	_, err := client.SendNotification(ctx, "vendor.world.state_changed", map[string]any{
		"state": "running",
	})
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(messages))
	}

	if messages[0].Type != protocol.MessageTypeNotification {
		t.Fatalf("expected Type 'notification', got '%s'", messages[0].Type)
	}
}

func TestSendErrorWithTransport(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001", "msg-004")
	client := NewClient(transport, WithIDGenerator(idGen))

	ctx := context.Background()

	request, err := client.NewRequest("minecraft.agent.submit_goal", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	_, err = client.SendError(ctx, request, protocol.ErrorPermissionDenied, "permission denied", false, nil)
	if err != nil {
		t.Fatalf("SendError failed: %v", err)
	}

	messages := transport.GetSentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(messages))
	}

	if messages[0].Error == nil {
		t.Fatal("expected Error to be non-nil")
	}
	if messages[0].Error.Code != protocol.ErrorPermissionDenied {
		t.Fatalf("expected Error.Code '%s', got '%s'", protocol.ErrorPermissionDenied, messages[0].Error.Code)
	}
}

func TestReceiveMessage(t *testing.T) {
	transport := NewMockTransport()
	client := NewClient(transport)

	expectedEnvelope := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeRequest,
		ID:       "msg-001",
		Method:   "minecraft.agent.submit_goal",
	}
	transport.QueueMessage(expectedEnvelope)

	ctx := context.Background()
	envelope, err := client.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if envelope.ID != expectedEnvelope.ID {
		t.Fatalf("expected ID '%s', got '%s'", expectedEnvelope.ID, envelope.ID)
	}
	if envelope.Method != expectedEnvelope.Method {
		t.Fatalf("expected Method '%s', got '%s'", expectedEnvelope.Method, envelope.Method)
	}
}

func TestClientRouteHelpers(t *testing.T) {
	transport := NewMockTransport()
	idGen := NewFixedIDGenerator("msg-001")
	client := NewClient(transport,
		WithIDGenerator(idGen),
		WithClientPluginID("plugin-123"),
		WithClientRuntimeID("runtime-456"),
		WithClientServiceID("service-789"),
	)

	envelope, err := client.NewRequest("minecraft.agent.submit_goal", nil,
		WithMetadata("traceId", json.RawMessage(`"trace-001"`)),
	)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	if envelope.PluginID != "plugin-123" {
		t.Fatalf("expected PluginID 'plugin-123', got '%s'", envelope.PluginID)
	}
	if envelope.RuntimeID != "runtime-456" {
		t.Fatalf("expected RuntimeID 'runtime-456', got '%s'", envelope.RuntimeID)
	}
	if envelope.ServiceID != "service-789" {
		t.Fatalf("expected ServiceID 'service-789', got '%s'", envelope.ServiceID)
	}
	if envelope.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}
}

func TestSDKVersion(t *testing.T) {
	if SDKName == "" || SDKVersion == "" {
		t.Fatal("SDK version info must be defined")
	}
}
