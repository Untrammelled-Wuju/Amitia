package homekit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockHomeKitBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockHomeKitBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockHomeKitBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockHomeKitBridge(resp nativebridge.Response, err error) *mockHomeKitBridge {
	return &mockHomeKitBridge{response: resp, err: err}
}

func baseHomeKitRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewHomeKitHandler(t *testing.T) {
	bridge := newMockHomeKitBridge(nativebridge.Response{}, nil)
	h := NewHomeKitHandler(bridge)
	if h == nil {
		t.Fatal("NewHomeKitHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest("homekit.unknown")
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
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"state": "authorized"},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
	if bridge.calls[0].Operation != OperationStatus {
		t.Errorf("expected operation %s", OperationStatus)
	}
}

func TestHandler_HomesList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"count": 2},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationHomesList)
	req.Payload["limit"] = float64(10)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["limit"] != 10 {
		t.Errorf("expected limit=10, got %v", bridge.calls[0].Payload["limit"])
	}
}

func TestHandler_HomesGet_MissingID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationHomesGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrHomeNotFound {
		t.Errorf("expected ErrHomeNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_HomesGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"homeId": "h-001"},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationHomesGet)
	req.Payload["homeId"] = "h-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["homeId"] != "h-001" {
		t.Errorf("expected homeId=h-001")
	}
}

func TestHandler_AccessoriesList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"count": 5},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationAccessoriesList)
	req.Payload["homeId"] = "h-001"
	req.Payload["category"] = "light"
	req.Payload["reachableOnly"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["homeId"] != "h-001" {
		t.Errorf("expected homeId=h-001")
	}
	if sent["category"] != "light" {
		t.Errorf("expected category=light")
	}
	if sent["reachableOnly"] != true {
		t.Errorf("expected reachableOnly=true")
	}
}

func TestHandler_AccessoriesGet_MissingID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationAccessoriesGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAccessoryNotFound {
		t.Errorf("expected ErrAccessoryNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsRead_MissingServiceID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationCharacteristicsRead)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsRead_MissingCharID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationCharacteristicsRead)
	req.Payload["serviceId"] = "svc-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsRead_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"value": "on"},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationCharacteristicsRead)
	req.Payload["serviceId"] = "svc-001"
	req.Payload["characteristicId"] = "char-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CharacteristicsWrite_MissingValue(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationCharacteristicsWrite)
	req.Payload["serviceId"] = "svc-001"
	req.Payload["characteristicId"] = "char-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrValueTypeInvalid {
		t.Errorf("expected ErrValueTypeInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsWrite_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"accepted": true},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationCharacteristicsWrite)
	req.Payload["serviceId"] = "svc-001"
	req.Payload["characteristicId"] = "char-001"
	req.Payload["value"] = map[string]any{"type": "bool", "bool": true}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ScenesList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"count": 3},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationScenesList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ScenesGet_MissingID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationScenesGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrSceneNotFound {
		t.Errorf("expected ErrSceneNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_ScenesExecute_MissingID(t *testing.T) {
	h := NewHomeKitHandler(newMockHomeKitBridge(nativebridge.Response{}, nil))
	req := baseHomeKitRequest(OperationScenesExecute)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrSceneNotFound {
		t.Errorf("expected ErrSceneNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_AutomationsList(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-hk-001",
		Status:          "ok",
		Result:          map[string]any{"count": 2},
	}
	bridge := newMockHomeKitBridge(expected, nil)
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationAutomationsList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockHomeKitBridge{
		delay: 5 * time.Second,
	}
	h := NewHomeKitHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseHomeKitRequest(OperationHomesList)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockHomeKitBridge(nativebridge.Response{}, errors.New("bridge connection failed"))
	h := NewHomeKitHandler(bridge)

	req := baseHomeKitRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewHomeKitHandler(nil)
	req := baseHomeKitRequest(OperationHomesList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestAuthorizationStatusFromNative(t *testing.T) {
	tests := []struct {
		bits     int
		expected AuthorizationStatus
	}{
		{0, AuthNotDetermined},
		{1, AuthDenied},
		{2, AuthAuthorized},
		{3, AuthAuthorized},
		{4, AuthRestricted},
		{5, AuthRestricted},
		{6, AuthRestricted},
		{7, AuthRestricted},
	}
	for _, tt := range tests {
		t.Run(string(rune('0'+tt.bits)), func(t *testing.T) {
			result := AuthorizationStatusFromNative(tt.bits)
			if result != tt.expected {
				t.Errorf("bits=%d: expected %v, got %v", tt.bits, tt.expected, result)
			}
		})
	}
}

func TestMapCharacteristicType(t *testing.T) {
	tests := []struct {
		appleType string
		expected  string
	}{
		{"HMCharacteristicTypePowerState", CharacteristicTypePowerState},
		{"HMCharacteristicTypeBrightness", CharacteristicTypeBrightness},
		{"HMCharacteristicTypeLockTargetState", CharacteristicTypeLockTargetState},
		{"UnknownType", CharacteristicTypeUnknown + ":UnknownType"},
	}
	for _, tt := range tests {
		t.Run(tt.appleType, func(t *testing.T) {
			result := MapCharacteristicType(tt.appleType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapAccessoryCategory(t *testing.T) {
	tests := []struct {
		appleCategory string
		expected      string
	}{
		{"HMAccessoryCategoryLightbulb", "light"},
		{"HMAccessoryCategoryDoorLock", "lock"},
		{"HMAccessoryCategoryThermostat", "thermostat"},
		{"HMAccessoryCategoryGarageDoorOpener", "garage_door"},
		{"HMAccessoryCategoryCamera", "camera"},
		{"HMAccessoryCategorySensor", "sensor"},
		{"HMAccessoryCategoryOutlet", "outlet"},
		{"HMAccessoryCategorySwitch", "switch"},
		{"UnknownCategory", "accessory"},
	}
	for _, tt := range tests {
		t.Run(tt.appleCategory, func(t *testing.T) {
			result := MapAccessoryCategory(tt.appleCategory)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRiskFromCharacteristicType(t *testing.T) {
	if RiskFromCharacteristicType("lock_target_state") != RiskLevelHigh {
		t.Error("expected high risk for lock_target_state")
	}
	if RiskFromCharacteristicType("target_temperature") != RiskLevelMedium {
		t.Error("expected medium risk for target_temperature")
	}
	if RiskFromCharacteristicType("brightness") != RiskLevelLow {
		t.Error("expected low risk for brightness")
	}
}

func TestMaxRisk(t *testing.T) {
	if MaxRisk([]string{RiskLevelLow, RiskLevelMedium}) != RiskLevelMedium {
		t.Error("expected medium")
	}
	if MaxRisk([]string{RiskLevelLow, RiskLevelHigh, RiskLevelMedium}) != RiskLevelHigh {
		t.Error("expected high")
	}
	if MaxRisk([]string{RiskLevelLow}) != RiskLevelLow {
		t.Error("expected low")
	}
}

func TestClampLimit(t *testing.T) {
	if ClampLimit(0, 100, 500) != 100 {
		t.Error("expected default")
	}
	if ClampLimit(999, 100, 500) != 500 {
		t.Error("expected max")
	}
	if ClampLimit(250, 100, 500) != 250 {
		t.Error("expected 250")
	}
}

func TestValidateCreateSceneInput(t *testing.T) {
	err := ValidateCreateSceneInput(CreateSceneInput{
		HomeID: "h-001",
		Name:   "Test",
		Actions: []SceneActionInput{
			{
				AccessoryID:      "a-001",
				ServiceID:        "s-001",
				CharacteristicID: "c-001",
			},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateCreateSceneInput(CreateSceneInput{
		HomeID: "h-001",
		Name:   "Test",
	})
	if err == nil {
		t.Error("expected error for empty actions")
	}
}

func TestValidateAutomationInput(t *testing.T) {
	err := ValidateAutomationInput(CreateAutomationInput{
		HomeID: "h-001",
		Name:   "Test",
		Type:   AutomationTypeCalendar,
		CalendarEvent: &CalendarEventAutomationInput{
			FireAt: "19:00",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateAutomationInput(CreateAutomationInput{
		HomeID: "h-001",
		Name:   "Test",
		Type:   "unsupported",
	})
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestSetupSession(t *testing.T) {
	s := NewSetupSession("h-001", "r-001")
	if s.Status != SetupStatusPending {
		t.Errorf("expected pending, got %s", s.Status)
	}
	if s.IsComplete() {
		t.Error("expected not complete")
	}

	s.MarkSuccess("a-001")
	if s.Status != SetupStatusSuccess {
		t.Errorf("expected success, got %s", s.Status)
	}
	if s.PairedAccessoryID != "a-001" {
		t.Errorf("expected a-001")
	}
	if !s.IsComplete() {
		t.Error("expected complete")
	}

	s2 := NewSetupSession("h-001", "r-001")
	s2.MarkFailed("pairing error")
	if s2.Status != SetupStatusFailed {
		t.Errorf("expected failed, got %s", s2.Status)
	}
	if s2.Error != "pairing error" {
		t.Errorf("expected pairing error")
	}

	s3 := NewSetupSession("h-001", "r-001")
	s3.MarkCancelled()
	if s3.Status != SetupStatusCancelled {
		t.Errorf("expected cancelled, got %s", s3.Status)
	}
}

func TestIsBuiltinActionSet(t *testing.T) {
	if !IsBuiltinActionSet(BuiltinActionSetGoodNight) {
		t.Error("expected GoodNight to be builtin")
	}
	if IsBuiltinActionSet("MyCustomScene") {
		t.Error("expected custom scene to not be builtin")
	}
}
