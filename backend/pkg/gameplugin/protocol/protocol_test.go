package protocol

import (
	"encoding/json"
	"testing"
)

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != "amitia-game-host/1" {
		t.Fatalf("expected ProtocolVersion to be 'amitia-game-host/1', got '%s'", ProtocolVersion)
	}
	if ProtocolName != "amitia-game-host" {
		t.Fatalf("expected ProtocolName to be 'amitia-game-host', got '%s'", ProtocolName)
	}
	if ProtocolMajor != 1 {
		t.Fatalf("expected ProtocolMajor to be 1, got %d", ProtocolMajor)
	}
}

func TestEncodeDecodeRequest(t *testing.T) {
	payload := map[string]any{
		"status": "healthy",
	}
	env, err := NewRequest("msg-001", "host.runtime.health", payload)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	env.RuntimeID = "runtime-123"

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ID != env.ID {
		t.Fatalf("expected ID '%s', got '%s'", env.ID, decoded.ID)
	}
	if decoded.Method != env.Method {
		t.Fatalf("expected Method '%s', got '%s'", env.Method, decoded.Method)
	}
	if decoded.Protocol != env.Protocol {
		t.Fatalf("expected Protocol '%s', got '%s'", env.Protocol, decoded.Protocol)
	}
	if decoded.RuntimeID != env.RuntimeID {
		t.Fatalf("expected RuntimeID '%s', got '%s'", env.RuntimeID, decoded.RuntimeID)
	}
	if string(decoded.Payload) != string(env.Payload) {
		t.Fatalf("expected Payload '%s', got '%s'", string(env.Payload), string(decoded.Payload))
	}
}

func TestEncodeDecodeResponse(t *testing.T) {
	payload := map[string]any{
		"status": "healthy",
	}
	env, err := NewResponse("msg-002", "msg-001", payload)
	if err != nil {
		t.Fatalf("NewResponse failed: %v", err)
	}

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.RequestID != env.RequestID {
		t.Fatalf("expected RequestID '%s', got '%s'", env.RequestID, decoded.RequestID)
	}
	if string(decoded.Payload) != string(env.Payload) {
		t.Fatalf("expected Payload '%s', got '%s'", string(env.Payload), string(decoded.Payload))
	}
}

func TestEncodeDecodeNotification(t *testing.T) {
	payload := map[string]any{
		"state": "running",
	}
	env, err := NewNotification("msg-003", "plugin.runtime.state_changed", payload)
	if err != nil {
		t.Fatalf("NewNotification failed: %v", err)
	}
	env.RuntimeID = "runtime-123"

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.ID != env.ID {
		t.Fatalf("expected ID '%s', got '%s'", env.ID, decoded.ID)
	}
	if decoded.Method != env.Method {
		t.Fatalf("expected Method '%s', got '%s'", env.Method, decoded.Method)
	}
	if decoded.Type != MessageTypeNotification {
		t.Fatalf("expected Type 'notification', got '%s'", decoded.Type)
	}
	if decoded.RequestID != "" {
		t.Fatalf("expected empty RequestID, got '%s'", decoded.RequestID)
	}
}

func TestEncodeDecodeError(t *testing.T) {
	env := NewErrorEnvelope("msg-004", "msg-001", ErrorRuntimeUnavailable, "runtime is unavailable", true)

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected Error to be non-nil")
	}
	if decoded.Error.Code != ErrorRuntimeUnavailable {
		t.Fatalf("expected Error.Code '%s', got '%s'", ErrorRuntimeUnavailable, decoded.Error.Code)
	}
	if decoded.Error.Message != "runtime is unavailable" {
		t.Fatalf("expected Error.Message 'runtime is unavailable', got '%s'", decoded.Error.Message)
	}
	if !decoded.Error.Retryable {
		t.Fatal("expected Error.Retryable to be true")
	}
	if decoded.RequestID != "msg-001" {
		t.Fatalf("expected RequestID 'msg-001', got '%s'", decoded.RequestID)
	}
}

func TestEncodeDecodeErrorWithData(t *testing.T) {
	errorData := json.RawMessage(`{"detail": "connection refused"}`)
	env := NewErrorEnvelopeWithData("msg-005", "msg-002", ErrorCode("vendor.connection_failed"), "connection failed", true, errorData)

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected Error to be non-nil")
	}

	var expectedData map[string]any
	if err := json.Unmarshal(errorData, &expectedData); err != nil {
		t.Fatalf("failed to unmarshal expected error data: %v", err)
	}
	var actualData map[string]any
	if err := json.Unmarshal(decoded.Error.Data, &actualData); err != nil {
		t.Fatalf("failed to unmarshal actual error data: %v", err)
	}
	if expectedData["detail"] != actualData["detail"] {
		t.Fatalf("expected Error.Data detail '%v', got '%v'", expectedData["detail"], actualData["detail"])
	}
}

func TestRejectUnknownProtocol(t *testing.T) {
	data := []byte(`{"protocol": "amitia-game-host/2", "type": "request", "id": "msg-001", "method": "test"}`)
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for unknown protocol, got nil")
	}
}

func TestRejectInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing id",
			data: `{"protocol": "amitia-game-host/1", "type": "request", "method": "test"}`,
		},
		{
			name: "missing method",
			data: `{"protocol": "amitia-game-host/1", "type": "request", "id": "msg-001"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.data))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestRejectInvalidResponse(t *testing.T) {
	data := []byte(`{"protocol": "amitia-game-host/1", "type": "response", "id": "msg-001"}`)
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for missing requestId, got nil")
	}
}

func TestRejectInvalidError(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing error code",
			data: `{"protocol": "amitia-game-host/1", "type": "error", "id": "msg-001", "error": {"message": "test"}}`,
		},
		{
			name: "missing error message",
			data: `{"protocol": "amitia-game-host/1", "type": "error", "id": "msg-001", "error": {"code": "ERR_TEST"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.data))
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestOpaquePayloadRoundTrip(t *testing.T) {
	payload := map[string]any{
		"foo": map[string]any{
			"bar": []any{float64(1), float64(2), float64(3)},
		},
	}
	env, err := NewRequest("msg-001", "custom.action", payload)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	data, err := Encode(env)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(decoded.Payload, &result); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	foo, ok := result["foo"].(map[string]any)
	if !ok {
		t.Fatal("expected foo to be a map")
	}
	bar, ok := foo["bar"].([]any)
	if !ok {
		t.Fatal("expected bar to be an array")
	}
	if len(bar) != 3 {
		t.Fatalf("expected bar to have 3 elements, got %d", len(bar))
	}
}

func TestCustomMethodAccepted(t *testing.T) {
	methods := []string{
		"minecraft.agent.submit_goal",
		"custom.vendor.action",
		"factorio.entity.create",
		"terraria.world.generate",
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			env, err := NewRequest("msg-001", method, nil)
			if err != nil {
				t.Fatalf("NewRequest failed for method '%s': %v", method, err)
			}
			if env.Method != method {
				t.Fatalf("expected Method '%s', got '%s'", method, env.Method)
			}
		})
	}
}

func TestReservedHostNamespaceValidation(t *testing.T) {
	reservedMethods := []string{
		"host.runtime.health",
		"plugin.custom.action",
		"runtime.state.update",
		"service.register",
		"channel.create",
		"control.stop",
	}
	for _, method := range reservedMethods {
		t.Run(method, func(t *testing.T) {
			if !IsReservedNamespace(method) {
				t.Fatalf("expected method '%s' to be reserved", method)
			}
		})
	}

	customMethods := []string{
		"minecraft.agent.submit_goal",
		"factorio.entity.create",
		"myplugin.custom.action",
	}
	for _, method := range customMethods {
		t.Run(method, func(t *testing.T) {
			if IsReservedNamespace(method) {
				t.Fatalf("expected method '%s' to not be reserved", method)
			}
		})
	}
}

func TestMessageIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"valid simple", "msg-001", false},
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", false},
		{"empty", "", true},
		{"too long", string(make([]byte, 300)), true},
		{"control char", "msg-001\x00", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMessageID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestMethodValidation(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		wantErr bool
	}{
		{"valid", "host.runtime.health", false},
		{"valid custom", "minecraft.agent.submit_goal", false},
		{"empty", "", true},
		{"single part", "health", true},
		{"uppercase", "Host.Runtime.Health", true},
		{"empty part", "host..health", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMethod(tt.method)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMethod(%q) error = %v, wantErr %v", tt.method, err, tt.wantErr)
			}
		})
	}
}

func TestRequestValidationRules(t *testing.T) {
	env, err := NewRequest("msg-001", "host.runtime.health", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	env.RequestID = "msg-000"
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for request with requestId, got nil")
	}
	env.RequestID = ""

	env.Error = &ProtocolError{Code: ErrorInvalidRequest, Message: "test"}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for request with error, got nil")
	}
	env.Error = nil
}

func TestResponseValidationRules(t *testing.T) {
	env, err := NewResponse("msg-002", "msg-001", nil)
	if err != nil {
		t.Fatalf("NewResponse failed: %v", err)
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestNotificationValidationRules(t *testing.T) {
	env, err := NewNotification("msg-003", "plugin.runtime.state_changed", nil)
	if err != nil {
		t.Fatalf("NewNotification failed: %v", err)
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	env.RequestID = "msg-001"
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for notification with requestId, got nil")
	}
	env.RequestID = ""

	env.Error = &ProtocolError{Code: ErrorInvalidRequest, Message: "test"}
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for notification with error, got nil")
	}
	env.Error = nil
}

func TestErrorEnvelopeValidation(t *testing.T) {
	env := NewErrorEnvelope("msg-004", "msg-001", ErrorRuntimeUnavailable, "runtime is unavailable", true)
	if err := env.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	env.Error = nil
	if err := env.Validate(); err == nil {
		t.Fatal("expected error for error envelope without error field, got nil")
	}
}

func TestRequestWithRoute(t *testing.T) {
	env, err := NewRequestWithRoute("msg-001", "host.runtime.health", nil, "runtime-123", "plugin-456", "service-789")
	if err != nil {
		t.Fatalf("NewRequestWithRoute failed: %v", err)
	}
	if env.RuntimeID != "runtime-123" {
		t.Fatalf("expected RuntimeID 'runtime-123', got '%s'", env.RuntimeID)
	}
	if env.PluginID != "plugin-456" {
		t.Fatalf("expected PluginID 'plugin-456', got '%s'", env.PluginID)
	}
	if env.ServiceID != "service-789" {
		t.Fatalf("expected ServiceID 'service-789', got '%s'", env.ServiceID)
	}
}

func TestNotificationWithRoute(t *testing.T) {
	env, err := NewNotificationWithRoute("msg-003", "plugin.runtime.state_changed", nil, "runtime-123", "plugin-456", "service-789")
	if err != nil {
		t.Fatalf("NewNotificationWithRoute failed: %v", err)
	}
	if env.RuntimeID != "runtime-123" {
		t.Fatalf("expected RuntimeID 'runtime-123', got '%s'", env.RuntimeID)
	}
	if env.PluginID != "plugin-456" {
		t.Fatalf("expected PluginID 'plugin-456', got '%s'", env.PluginID)
	}
	if env.ServiceID != "service-789" {
		t.Fatalf("expected ServiceID 'service-789', got '%s'", env.ServiceID)
	}
}

func TestResponseWithMetadata(t *testing.T) {
	metadata := map[string]json.RawMessage{
		"traceId": json.RawMessage(`"trace-123"`),
	}
	env, err := NewResponseWithMetadata("msg-002", "msg-001", nil, metadata)
	if err != nil {
		t.Fatalf("NewResponseWithMetadata failed: %v", err)
	}
	if env.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}
	if string(env.Metadata["traceId"]) != `"trace-123"` {
		t.Fatalf("expected metadata traceId '\"trace-123\"', got '%s'", string(env.Metadata["traceId"]))
	}
}

func TestDecodeInvalidEnvelope(t *testing.T) {
	invalidData := []byte(`{"protocol": "amitia-game-host/1", "type": "unknown", "id": "msg-001"}`)
	_, err := Decode(invalidData)
	if err == nil {
		t.Fatal("expected error for invalid message type, got nil")
	}
}

func TestNoGameSpecificFields(t *testing.T) {
	payloads := []map[string]any{
		{"inventory": []any{"sword", "shield"}},
		{"player": map[string]any{"name": "steve"}},
		{"world": map[string]any{"seed": 12345}},
		{"block": map[string]any{"type": "stone"}},
		{"npc": map[string]any{"id": "npc-001"}},
		{"position": map[string]any{"x": 0, "y": 64, "z": 0}},
		{"goal": "build a shelter"},
		{"task": map[string]any{"id": "task-001"}},
		{"plan": []any{"step1", "step2"}},
		{"combatState": "idle"},
	}

	for _, payload := range payloads {
		env, err := NewRequest("msg-001", "game.custom.action", payload)
		if err != nil {
			t.Fatalf("NewRequest failed for payload %v: %v", payload, err)
		}
		data, err := Encode(env)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		if string(decoded.Payload) != string(env.Payload) {
			t.Fatalf("payload mismatch after round trip")
		}
	}
}

func TestReservedNamespaces(t *testing.T) {
	expectedReserved := []string{"host.", "plugin.", "runtime.", "service.", "channel.", "control."}
	tests := map[string]bool{
		"host.runtime.health":   true,
		"plugin.custom.action":  true,
		"runtime.state.update":  true,
		"service.register":      true,
		"channel.create":        true,
		"control.stop":          true,
		"minecraft.agent.goal":  false,
		"factorio.entity.move":  false,
		"terraria.world.load":   false,
		"myplugin.custom.test":  false,
	}

	for method, expected := range tests {
		t.Run(method, func(t *testing.T) {
			result := IsReservedNamespace(method)
			if result != expected {
				t.Fatalf("IsReservedNamespace(%q) = %v, want %v", method, result, expected)
			}
		})
	}

	for _, ns := range expectedReserved {
		method := ns + "test"
		if !IsReservedNamespace(method) {
			t.Fatalf("expected method with prefix '%s' to be reserved", ns)
		}
	}
}

func TestValidateServiceDescriptor(t *testing.T) {
	tests := []struct {
		name    string
		svc     ServiceDescriptor
		wantErr bool
	}{
		{
			name: "valid agent-service",
			svc: ServiceDescriptor{
				ID:   "agent-service",
				Kind: ServiceKindProcess,
			},
			wantErr: false,
		},
		{
			name: "valid game-bridge",
			svc: ServiceDescriptor{
				ID:   "game-bridge",
				Kind: ServiceKindExternal,
			},
			wantErr: false,
		},
		{
			name: "valid vision-service",
			svc: ServiceDescriptor{
				ID:   "vision-service",
				Kind: ServiceKindProcess,
			},
			wantErr: false,
		},
		{
			name: "valid python.worker",
			svc: ServiceDescriptor{
				ID:   "python.worker",
				Kind: ServiceKindProcess,
			},
			wantErr: false,
		},
		{
			name: "valid service_1",
			svc: ServiceDescriptor{
				ID:   "service_1",
				Kind: ServiceKindExternal,
			},
			wantErr: false,
		},
		{
			name: "empty id",
			svc: ServiceDescriptor{
				ID:   "",
				Kind: ServiceKindProcess,
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			svc: ServiceDescriptor{
				ID:   "test-service",
				Kind: "invalid",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.svc.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ServiceDescriptor.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRejectSelfDependency(t *testing.T) {
	svc := ServiceDescriptor{
		ID:        "service-a",
		Kind:      ServiceKindProcess,
		DependsOn: []ServiceID{"service-a"},
	}
	if err := svc.Validate(); err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
}

func TestRejectDuplicateDependency(t *testing.T) {
	svc := ServiceDescriptor{
		ID:        "service-a",
		Kind:      ServiceKindProcess,
		DependsOn: []ServiceID{"service-b", "service-b"},
	}
	if err := svc.Validate(); err == nil {
		t.Fatal("expected error for duplicate dependency, got nil")
	}
}

func TestValidateKnownCapabilities(t *testing.T) {
	knownCaps := []Capability{
		CapabilityRealtimeControl,
		CapabilityStateStreaming,
		CapabilityEventStreaming,
		CapabilityBinaryStreaming,
		CapabilityCustomRPC,
		CapabilityHostAPI,
		CapabilitySharedControl,
		CapabilityCustomUI,
		CapabilityMultiService,
	}
	for _, cap := range knownCaps {
		t.Run(string(cap), func(t *testing.T) {
			if err := ValidateCapability(cap); err != nil {
				t.Fatalf("ValidateCapability(%q) failed: %v", cap, err)
			}
			if !IsKnownCapability(cap) {
				t.Fatalf("IsKnownCapability(%q) should return true", cap)
			}
		})
	}
}

func TestValidateCustomCapability(t *testing.T) {
	customCaps := []Capability{
		"minecraft.pathfinding",
		"vendor.experimental_ai",
		"factorio.logistics",
		"terraria.crafting",
	}
	for _, cap := range customCaps {
		t.Run(string(cap), func(t *testing.T) {
			if err := ValidateCapability(cap); err != nil {
				t.Fatalf("ValidateCapability(%q) failed: %v", cap, err)
			}
		})
	}
}

func TestUnknownCapabilityIsNotKnown(t *testing.T) {
	if IsKnownCapability("minecraft.pathfinding") {
		t.Fatal("IsKnownCapability should return false for custom capability")
	}
	if !IsKnownCapability(CapabilityCustomRPC) {
		t.Fatal("IsKnownCapability should return true for standard capability")
	}
}

func TestValidateChannelDescriptor(t *testing.T) {
	channels := []ChannelDescriptor{
		{ID: "event-channel", Kind: ChannelKindEvent},
		{ID: "state-channel", Kind: ChannelKindState},
		{ID: "log-channel", Kind: ChannelKindLog},
		{ID: "metric-channel", Kind: ChannelKindMetric},
		{ID: "binary-channel", Kind: ChannelKindBinary},
		{ID: "custom-channel", Kind: ChannelKindCustom},
	}
	for i := range channels {
		t.Run(string(channels[i].Kind), func(t *testing.T) {
			if err := channels[i].Validate(); err != nil {
				t.Fatalf("ChannelDescriptor.Validate() failed for kind '%s': %v", channels[i].Kind, err)
			}
		})
	}

	normalHint := FrequencyHintNormal
	realtimeHint := FrequencyHintRealtime

	tests := []struct {
		name    string
		ch      ChannelDescriptor
		wantErr bool
	}{
		{
			name: "valid event with direction and hint",
			ch: ChannelDescriptor{
				ID:            "ch-1",
				Kind:          ChannelKindEvent,
				Direction:     ChannelDirectionPluginToHost,
				FrequencyHint: &normalHint,
			},
			wantErr: false,
		},
		{
			name: "valid state with bidirectional",
			ch: ChannelDescriptor{
				ID:            "ch-2",
				Kind:          ChannelKindState,
				Direction:     ChannelDirectionBidirectional,
				FrequencyHint: &realtimeHint,
			},
			wantErr: false,
		},
		{
			name: "empty kind",
			ch: ChannelDescriptor{
				ID: "ch-3",
			},
			wantErr: true,
		},
		{
			name: "invalid direction",
			ch: ChannelDescriptor{
				ID:        "ch-4",
				Kind:      ChannelKindLog,
				Direction: "invalid",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ch.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ChannelDescriptor.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCustomSchemaID(t *testing.T) {
	schemaID := "minecraft.world-state/v1"
	ch := ChannelDescriptor{
		ID:       "state-ch",
		Kind:     ChannelKindState,
		SchemaID: schemaID,
	}
	if err := ch.Validate(); err != nil {
		t.Fatalf("ChannelDescriptor.Validate() failed: %v", err)
	}

	data, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ChannelDescriptor
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.SchemaID != schemaID {
		t.Fatalf("expected SchemaID '%s', got '%s'", schemaID, decoded.SchemaID)
	}
}

func TestProtocolErrorKnownCode(t *testing.T) {
	knownCodes := []ErrorCode{
		ErrorInvalidRequest,
		ErrorInvalidArgument,
		ErrorNotFound,
		ErrorAlreadyExists,
		ErrorUnsupported,
		ErrorProtocolMismatch,
		ErrorCapabilityUnsupported,
		ErrorRuntimeUnavailable,
		ErrorServiceUnavailable,
		ErrorInvalidRuntimeState,
		ErrorPermissionDenied,
		ErrorResourceExhausted,
		ErrorTimeout,
		ErrorCancelled,
		ErrorInternal,
	}
	for _, code := range knownCodes {
		t.Run(string(code), func(t *testing.T) {
			if err := ValidateErrorCode(code); err != nil {
				t.Fatalf("ValidateErrorCode(%q) failed: %v", code, err)
			}
		})
	}
}

func TestProtocolErrorExtensionCode(t *testing.T) {
	extensionCodes := []ErrorCode{
		"minecraft.connection_failed",
		"vendor.auth_failed",
		"factorio.protocol_error",
	}
	for _, code := range extensionCodes {
		t.Run(string(code), func(t *testing.T) {
			if err := ValidateErrorCode(code); err != nil {
				t.Fatalf("ValidateErrorCode(%q) failed: %v", code, err)
			}
		})
	}
}

func TestRejectInvalidBareErrorCode(t *testing.T) {
	invalidCodes := []ErrorCode{
		"some_random_unknown_error",
		"MY_CUSTOM_ERROR",
		"",
	}
	for _, code := range invalidCodes {
		t.Run(string(code), func(t *testing.T) {
			if err := ValidateErrorCode(code); err == nil {
				t.Fatalf("ValidateErrorCode(%q) should fail for invalid bare code", code)
			}
		})
	}
}

func TestDescriptorRoundTrip(t *testing.T) {
	ps := PluginSchema{
		Services: []ServiceDescriptor{
			{
				ID:           "agent",
				Name:         "Agent Service",
				Kind:         ServiceKindProcess,
				Required:     true,
				Capabilities: []Capability{CapabilityCustomRPC, CapabilityStateStreaming},
			},
			{
				ID:       "game-bridge",
				Kind:     ServiceKindExternal,
				Required: false,
				DependsOn: []ServiceID{"agent"},
			},
		},
		Channels: []ChannelDescriptor{
			{
				ID:        "events",
				Kind:      ChannelKindEvent,
				Direction: ChannelDirectionPluginToHost,
			},
			{
				ID:       "world-state",
				Kind:     ChannelKindState,
				SchemaID: "minecraft.world-state/v1",
			},
		},
		Capabilities: []Capability{
			CapabilityCustomRPC,
			CapabilityStateStreaming,
			"minecraft.pathfinding",
		},
	}

	data, err := json.Marshal(ps)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded PluginSchema
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(decoded.Services) != len(ps.Services) {
		t.Fatalf("expected %d services, got %d", len(ps.Services), len(decoded.Services))
	}
	if len(decoded.Channels) != len(ps.Channels) {
		t.Fatalf("expected %d channels, got %d", len(ps.Channels), len(decoded.Channels))
	}
	if len(decoded.Capabilities) != len(ps.Capabilities) {
		t.Fatalf("expected %d capabilities, got %d", len(ps.Capabilities), len(decoded.Capabilities))
	}

	if decoded.Services[0].ID != ps.Services[0].ID {
		t.Fatalf("expected service ID '%s', got '%s'", ps.Services[0].ID, decoded.Services[0].ID)
	}
	if decoded.Channels[1].SchemaID != ps.Channels[1].SchemaID {
		t.Fatalf("expected channel schemaID '%s', got '%s'", ps.Channels[1].SchemaID, decoded.Channels[1].SchemaID)
	}
	if decoded.Capabilities[2] != ps.Capabilities[2] {
		t.Fatalf("expected capability '%s', got '%s'", ps.Capabilities[2], decoded.Capabilities[2])
	}
}

func TestServicesDuplicateValidation(t *testing.T) {
	services := []ServiceDescriptor{
		{ID: "svc-1", Kind: ServiceKindProcess},
		{ID: "svc-2", Kind: ServiceKindExternal},
		{ID: "svc-1", Kind: ServiceKindProcess},
	}
	if err := ValidateServices(services); err == nil {
		t.Fatal("expected error for duplicate service ID, got nil")
	}
}

func TestChannelsDuplicateValidation(t *testing.T) {
	channels := []ChannelDescriptor{
		{ID: "ch-1", Kind: ChannelKindEvent},
		{ID: "ch-2", Kind: ChannelKindState},
		{ID: "ch-1", Kind: ChannelKindLog},
	}
	if err := ValidateChannels(channels); err == nil {
		t.Fatal("expected error for duplicate channel ID, got nil")
	}
}

func TestCapabilitiesDuplicateValidation(t *testing.T) {
	caps := []Capability{
		CapabilityCustomRPC,
		CapabilityStateStreaming,
		CapabilityCustomRPC,
	}
	if err := ValidateCapabilities(caps); err == nil {
		t.Fatal("expected error for duplicate capability, got nil")
	}
}

func TestPluginSchemaValidate(t *testing.T) {
	ps := PluginSchema{
		Services: []ServiceDescriptor{
			{ID: "svc-1", Kind: ServiceKindProcess},
		},
		Channels: []ChannelDescriptor{
			{ID: "ch-1", Kind: ChannelKindEvent},
		},
		Capabilities: []Capability{CapabilityCustomRPC},
	}
	if err := ps.Validate(); err != nil {
		t.Fatalf("PluginSchema.Validate() failed: %v", err)
	}

	invalidPs := PluginSchema{
		Services: []ServiceDescriptor{
			{ID: "", Kind: ServiceKindProcess},
		},
	}
	if err := invalidPs.Validate(); err == nil {
		t.Fatal("expected error for invalid plugin schema, got nil")
	}
}

func TestPluginSchemaFindService(t *testing.T) {
	ps := PluginSchema{
		Services: []ServiceDescriptor{
			{ID: "svc-1", Kind: ServiceKindProcess},
			{ID: "svc-2", Kind: ServiceKindExternal},
		},
	}

	svc, found := ps.FindService("svc-1")
	if !found {
		t.Fatal("expected to find service 'svc-1'")
	}
	if svc.ID != "svc-1" {
		t.Fatalf("expected service ID 'svc-1', got '%s'", svc.ID)
	}

	_, found = ps.FindService("svc-nonexistent")
	if found {
		t.Fatal("should not find nonexistent service")
	}
}

func TestPluginSchemaFindChannel(t *testing.T) {
	ps := PluginSchema{
		Channels: []ChannelDescriptor{
			{ID: "ch-1", Kind: ChannelKindEvent},
			{ID: "ch-2", Kind: ChannelKindState},
		},
	}

	ch, found := ps.FindChannel("ch-1")
	if !found {
		t.Fatal("expected to find channel 'ch-1'")
	}
	if ch.ID != "ch-1" {
		t.Fatalf("expected channel ID 'ch-1', got '%s'", ch.ID)
	}

	_, found = ps.FindChannel("ch-nonexistent")
	if found {
		t.Fatal("should not find nonexistent channel")
	}
}

func TestPluginSchemaHasCapability(t *testing.T) {
	ps := PluginSchema{
		Capabilities: []Capability{CapabilityCustomRPC, CapabilityStateStreaming, "minecraft.pathfinding"},
	}

	if !ps.HasCapability(CapabilityCustomRPC) {
		t.Fatal("should have capability 'custom_rpc'")
	}
	if !ps.HasCapability("minecraft.pathfinding") {
		t.Fatal("should have capability 'minecraft.pathfinding'")
	}
	if ps.HasCapability("vendor.unknown") {
		t.Fatal("should not have capability 'vendor.unknown'")
	}
}

func TestDefaultRetryable(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected bool
	}{
		{ErrorRuntimeUnavailable, true},
		{ErrorServiceUnavailable, true},
		{ErrorResourceExhausted, true},
		{ErrorTimeout, true},
		{ErrorInvalidRequest, false},
		{ErrorPermissionDenied, false},
		{ErrorInternal, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := DefaultRetryable(tt.code)
			if result != tt.expected {
				t.Fatalf("DefaultRetryable(%q) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}

func TestNewErrorEnvelopeAutoRetry(t *testing.T) {
	env := NewErrorEnvelopeAutoRetry("msg-001", "msg-000", ErrorRuntimeUnavailable, "runtime unavailable")
	if env.Error == nil {
		t.Fatal("expected Error to be non-nil")
	}
	if !env.Error.Retryable {
		t.Fatal("expected Error.Retryable to be true for ErrorRuntimeUnavailable")
	}

	env2 := NewErrorEnvelopeAutoRetry("msg-002", "msg-000", ErrorInvalidRequest, "invalid request")
	if env2.Error.Retryable {
		t.Fatal("expected Error.Retryable to be false for ErrorInvalidRequest")
	}
}

func TestServiceIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      ServiceID
		wantErr bool
	}{
		{"valid simple", "agent-service", false},
		{"valid underscore", "service_1", false},
		{"valid dot", "python.worker", false},
		{"valid hyphen", "game-bridge", false},
		{"empty", "", true},
		{"with space", "my service", true},
		{"too long", ServiceID(make([]byte, 300)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateServiceID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestChannelIDValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      ChannelID
		wantErr bool
	}{
		{"valid simple", "my-channel", false},
		{"valid underscore", "channel_1", false},
		{"valid dot", "state.events", false},
		{"empty", "", true},
		{"too long", ChannelID(make([]byte, 300)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChannelID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateChannelID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}
