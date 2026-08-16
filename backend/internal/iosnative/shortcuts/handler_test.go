package shortcuts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockShortcutsBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockShortcutsBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	m.calls = append(m.calls, req)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nativebridge.Response{}, ctx.Err()
		}
	}
	return m.response, m.err
}

func (m *mockShortcutsBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockShortcutsBridge(resp nativebridge.Response, err error) *mockShortcutsBridge {
	return &mockShortcutsBridge{response: resp, err: err}
}

func baseShortcutsRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewShortcutsHandler(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewShortcutsHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest("shortcuts.unknown")
	resp := h.Execute(context.Background(), req)
	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil {
		t.Fatal("expected error object")
	}
	if resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Errorf("expected ErrOperationNotSupported, got %s", resp.Error.Code)
	}
}

func TestHandler_Status(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_EntitiesCharacters(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"characters": []any{}},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationEntitiesCharacters)
	req.Payload["limit"] = float64(10)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["limit"] != 10 {
		t.Errorf("expected limit 10, got %v", bridge.calls[0].Payload["limit"])
	}
}

func TestHandler_EntitiesCharacters_DefaultLimit(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationEntitiesCharacters)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["limit"] != DefaultListLimit {
		t.Errorf("expected default limit %d, got %v", DefaultListLimit, bridge.calls[0].Payload["limit"])
	}
}

func TestHandler_EntitiesConversations_WithCharacterFilter(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"conversations": []any{}},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationEntitiesConversations)
	req.Payload["characterId"] = "char-001"
	req.Payload["limit"] = float64(20)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["characterId"] != "char-001" {
		t.Error("expected characterId filter")
	}
}

func TestHandler_EntityResolve_MissingID(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationEntityResolve)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsEntityNotFound {
		t.Errorf("expected ErrShortcutsEntityNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_EntityResolve_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"found": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationEntityResolve)
	req.Payload["entityId"] = "char-001"
	req.Payload["entityType"] = "character"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_EntitySuggestions(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"suggestions": []any{}},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationEntitySuggestions)
	req.Payload["entityType"] = "character"
	req.Payload["limit"] = float64(5)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ActionsCatalog(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"actions": []any{}},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationActionsCatalog)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ActionDescribe_MissingID(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionDescribe)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsParameterRequired {
		t.Errorf("expected ErrShortcutsParameterRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionDescribe_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"id": "create_alarm"},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationActionDescribe)
	req.Payload["actionId"] = "create_alarm"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ActionExecute_MissingID(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionExecute)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsParameterRequired {
		t.Errorf("expected ErrShortcutsParameterRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionExecute_InvalidID(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionExecute)
	req.Payload["actionId"] = "invalid action id with spaces"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsParameterInvalid {
		t.Errorf("expected ErrShortcutsParameterInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionExecute_ShortIdempotencyKey(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionExecute)
	req.Payload["actionId"] = "create_alarm"
	req.Payload["idempotencyKey"] = "short"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsIdempotencyInvalid {
		t.Errorf("expected ErrShortcutsIdempotencyInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionExecute_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"status": "completed"},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationActionExecute)
	req.Payload["actionId"] = "create_alarm"
	req.Payload["idempotencyKey"] = "unique-key-1234567890"
	req.Payload["parameters"] = map[string]any{"title": "Morning"}
	req.Payload["invocationId"] = "inv-001"
	req.Payload["executionMode"] = "foreground_immediate"
	req.Payload["userId"] = "user-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["actionId"] != "create_alarm" {
		t.Error("expected actionId")
	}
	if sent["idempotencyKey"] != "unique-key-1234567890" {
		t.Error("expected idempotencyKey")
	}
}

func TestHandler_ActionConfirm_MissingActionID(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionConfirm)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsParameterRequired {
		t.Errorf("expected ErrShortcutsParameterRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionConfirm_MissingTitle(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationActionConfirm)
	req.Payload["actionId"] = "cancel_alarm"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsConfirmationRequired {
		t.Errorf("expected ErrShortcutsConfirmationRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_ActionConfirm_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"confirmed": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationActionConfirm)
	req.Payload["actionId"] = "cancel_alarm"
	req.Payload["title"] = "Cancel Alarm"
	req.Payload["message"] = "Are you sure you want to cancel this alarm?"
	req.Payload["objectName"] = "Morning Alarm"
	req.Payload["consequence"] = "It won't ring again"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["title"] != "Cancel Alarm" {
		t.Error("expected title")
	}
}

func TestHandler_RuntimeReadiness(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"ready": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationRuntimeReadiness)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_RuntimeEnsure_DefaultRequirement(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"ready": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationRuntimeEnsure)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["requirement"] != string(ShortcutRuntimeRequirementNativeOnly) {
		t.Errorf("expected default requirement native_only, got %v", bridge.calls[0].Payload["requirement"])
	}
}

func TestHandler_RuntimeEnsure_WithTimeout(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationRuntimeEnsure)
	req.Payload["requirement"] = "backend_interaction"
	req.Payload["timeoutMs"] = float64(5000)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["requirement"] != "backend_interaction" {
		t.Error("expected requirement")
	}
	if sent["timeoutMs"] != 5000 {
		t.Error("expected timeoutMs")
	}
}

func TestHandler_SnapshotGet(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"version": 1},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationSnapshotGet)
	req.Payload["entityType"] = "character"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_SnapshotRefresh(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"refreshed": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationSnapshotRefresh)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ShortcutsProvider(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"shortcuts": []any{}},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationShortcutsProvider)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ShortcutsPhrase_MissingPhrase(t *testing.T) {
	h := NewShortcutsHandler(newMockShortcutsBridge(nativebridge.Response{}, nil))
	req := baseShortcutsRequest(OperationShortcutsPhrase)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsPhraseInvalid {
		t.Errorf("expected ErrShortcutsPhraseInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_ShortcutsPhrase_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationShortcutsPhrase)
	req.Payload["phrase"] = "Ask Amitia"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_SettingsGet(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"enabled": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationSettingsGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_SettingsUpdate(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-shortcuts-001",
		Status:          "ok",
		Result:          map[string]any{"updated": true},
	}
	bridge := newMockShortcutsBridge(expected, nil)
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationSettingsUpdate)
	req.Payload["enabled"] = true
	req.Payload["askAmitiaEnabled"] = true
	req.Payload["exposeConversationTitles"] = false
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["enabled"] != true {
		t.Error("expected enabled=true")
	}
	if sent["exposeConversationTitles"] != false {
		t.Error("expected exposeConversationTitles=false")
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockShortcutsBridge{
		delay: 5 * time.Second,
	}
	h := NewShortcutsHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseShortcutsRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockShortcutsBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewShortcutsHandler(bridge)

	req := baseShortcutsRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewShortcutsHandler(nil)
	req := baseShortcutsRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShortcutsNativeBridgeUnavailable {
		t.Errorf("expected ErrShortcutsNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestValidateActionID(t *testing.T) {
	if err := ValidateActionID("create_alarm"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateActionID(""); err == nil {
		t.Error("expected error for empty actionId")
	}
	if err := ValidateActionID("action with spaces"); err == nil {
		t.Error("expected error for actionId with spaces")
	}
	longID := make([]byte, MaxActionIDLength+1)
	if err := ValidateActionID(string(longID)); err == nil {
		t.Error("expected error for too long actionId")
	}
}

func TestValidateEntityID(t *testing.T) {
	if err := ValidateEntityID("char-001"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateEntityID(""); err == nil {
		t.Error("expected error for empty entityId")
	}
}

func TestValidateParameters(t *testing.T) {
	if err := ValidateParameters(nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateParameters(map[string]any{"key": "value"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateParameters(map[string]any{"": "value"}); err == nil {
		t.Error("expected error for empty key")
	}
}

func TestValidateMessage(t *testing.T) {
	if err := ValidateMessage(""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateMessage("Hello Amitia"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	longMsg := make([]byte, MaxMessageBytes+1)
	if err := ValidateMessage(string(longMsg)); err == nil {
		t.Error("expected error for too long message")
	}
}

func TestValidateActionRequest(t *testing.T) {
	err := ValidateActionRequest(ShortcutActionRequest{
		ActionID:   "create_alarm",
		Parameters: map[string]any{"title": "Morning"},
		Invocation: ShortcutInvocationMetadata{IdempotencyKey: "unique-key-1234567890"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateActionRequest(ShortcutActionRequest{})
	if err == nil {
		t.Error("expected error for empty actionId")
	}
}

func TestValidateConfirmationRequest(t *testing.T) {
	err := ValidateConfirmationRequest(ConfirmationRequest{
		ActionID: "cancel_alarm",
		Title:    "Cancel Alarm",
		Message:  "Are you sure?",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateConfirmationRequest(ConfirmationRequest{ActionID: "cancel_alarm"})
	if err == nil {
		t.Error("expected error for missing title")
	}

	err = ValidateConfirmationRequest(ConfirmationRequest{Title: "Title"})
	if err == nil {
		t.Error("expected error for missing actionId")
	}
}

func TestValidateShortcutPhrase(t *testing.T) {
	if err := ValidateShortcutPhrase("Ask Amitia"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateShortcutPhrase(""); err == nil {
		t.Error("expected error for empty phrase")
	}
}

func TestValidateContribution(t *testing.T) {
	err := ValidateContribution(ShortcutContribution{
		ActionID: "custom_action",
		Title:    "Custom Action",
		Risk:     ShortcutRiskLevelReadOnly,
		Exposure: ShortcutExposureShortcuts,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateContribution(ShortcutContribution{
		ActionID: "custom_action",
		Title:    "Custom Action",
		Risk:     "invalid_risk",
		Exposure: ShortcutExposureShortcuts,
	})
	if err == nil {
		t.Error("expected error for invalid risk level")
	}
}

func TestClampLimit(t *testing.T) {
	if ClampLimit(0) != DefaultListLimit {
		t.Errorf("expected default %d", DefaultListLimit)
	}
	if ClampLimit(-1) != DefaultListLimit {
		t.Errorf("expected default %d", DefaultListLimit)
	}
	if ClampLimit(50) != 50 {
		t.Error("expected 50")
	}
	if ClampLimit(MaxListLimit+1) != MaxListLimit {
		t.Errorf("expected max %d", MaxListLimit)
	}
}

func TestClampLimitWithMax(t *testing.T) {
	if ClampLimitWithMax(0, 10) != DefaultListLimit {
		t.Errorf("expected default %d", DefaultListLimit)
	}
	if ClampLimitWithMax(15, 10) != 10 {
		t.Error("expected 10")
	}
	if ClampLimitWithMax(5, 10) != 5 {
		t.Error("expected 5")
	}
}

func TestIsValidCanonicalTarget(t *testing.T) {
	for _, target := range AllowedCanonicalTargets {
		if !IsValidCanonicalTarget(target) {
			t.Errorf("expected %s to be valid", target)
		}
	}
	if IsValidCanonicalTarget("invalid_target") {
		t.Error("expected invalid target")
	}
}

func TestIsValidExecutionMode(t *testing.T) {
	for _, mode := range AllowedExecutionModes {
		if !IsValidExecutionMode(mode) {
			t.Errorf("expected %s to be valid", mode)
		}
	}
	if IsValidExecutionMode("invalid_mode") {
		t.Error("expected invalid mode")
	}
}

func TestIsValidRiskLevel(t *testing.T) {
	for _, risk := range AllowedRiskLevels {
		if !IsValidRiskLevel(risk) {
			t.Errorf("expected %s to be valid", risk)
		}
	}
	if IsValidRiskLevel("invalid_risk") {
		t.Error("expected invalid risk")
	}
}

func TestIsValidExposure(t *testing.T) {
	for _, exp := range []ShortcutExposure{
		ShortcutExposureNone, ShortcutExposureSiri, ShortcutExposureShortcuts,
		ShortcutExposureSpotlight, ShortcutExposureAppShortcut, ShortcutExposureAll,
	} {
		if !IsValidExposure(exp) {
			t.Errorf("expected %s to be valid", exp)
		}
	}
	if IsValidExposure("invalid_exposure") {
		t.Error("expected invalid exposure")
	}
}

func TestIsHighRiskAction(t *testing.T) {
	if !IsHighRiskAction("cancel_alarm") {
		t.Error("expected cancel_alarm to be high risk")
	}
	if !IsHighRiskAction("homekit_write") {
		t.Error("expected homekit_write to be high risk")
	}
	if IsHighRiskAction("create_alarm") {
		t.Error("expected create_alarm to not be high risk")
	}
}

func TestRiskRequiresConfirmation(t *testing.T) {
	if !RiskRequiresConfirmation(ShortcutRiskLevelHigh) {
		t.Error("expected high risk to require confirmation")
	}
	if !RiskRequiresConfirmation(ShortcutRiskLevelUIMediated) {
		t.Error("expected ui mediated to require confirmation")
	}
	if RiskRequiresConfirmation(ShortcutRiskLevelReadOnly) {
		t.Error("expected read only to not require confirmation")
	}
}

func TestRiskAllowsBackground(t *testing.T) {
	if !RiskAllowsBackground(ShortcutRiskLevelReadOnly) {
		t.Error("expected read only to allow background")
	}
	if RiskAllowsBackground(ShortcutRiskLevelHigh) {
		t.Error("expected high risk to not allow background")
	}
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrShortcutsUnavailable, "shortcuts are not available on this device"},
		{ErrShortcutsEntityNotFound, "the specified entity was not found"},
		{ErrShortcutsPermissionDenied, "permission denied for this action"},
		{ErrShortcutsForegroundRequired, "this action requires the app to be in the foreground"},
		{ErrShortcutsConfirmationRequired, "user confirmation is required before executing this action"},
		{"UNKNOWN_CODE", "UNKNOWN_CODE"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := MapCodeToMessage(tt.code)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDisplaySafeResult(t *testing.T) {
	short := "Hello"
	if DisplaySafeResult(short) != short {
		t.Error("expected same string for short result")
	}
	long := make([]byte, MaxResultBytes+1)
	result := DisplaySafeResult(string(long))
	if len(result) != MaxResultBytes {
		t.Errorf("expected truncated to %d, got %d", MaxResultBytes, len(result))
	}
}

func TestSanitizeForDisplay(t *testing.T) {
	input := "Hello\x00World"
	expected := "HelloWorld"
	if SanitizeForDisplay(input) != expected {
		t.Errorf("expected %q, got %q", expected, SanitizeForDisplay(input))
	}
}
