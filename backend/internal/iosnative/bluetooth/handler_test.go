package bluetooth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockBluetoothBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockBluetoothBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockBluetoothBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockBluetoothBridge(resp nativebridge.Response, err error) *mockBluetoothBridge {
	return &mockBluetoothBridge{response: resp, err: err}
}

func baseBluetoothRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewBluetoothHandler(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewBluetoothHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest("bluetooth.unknown")
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
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"poweredOn": true},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationStatus)
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

func TestHandler_ScanStart(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"scanId": "scan-001"},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationScanStart)
	req.Payload["durationMs"] = float64(5000)
	req.Payload["maxResults"] = float64(20)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["durationMs"] != 5000 {
		t.Errorf("expected durationMs=5000, got %v", sent["durationMs"])
	}
	if sent["maxResults"] != 20 {
		t.Errorf("expected maxResults=20, got %v", sent["maxResults"])
	}
}

func TestHandler_ScanStart_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationScanStart)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["durationMs"] != DefaultScanDurationMs {
		t.Errorf("expected default durationMs=%d, got %v", DefaultScanDurationMs, sent["durationMs"])
	}
	if sent["maxResults"] != DefaultMaxResults {
		t.Errorf("expected default maxResults=%d, got %v", DefaultMaxResults, sent["maxResults"])
	}
}

func TestHandler_ScanStart_WithServiceUUIDs(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationScanStart)
	req.Payload["serviceUuids"] = []any{"180D", "180F"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	uuids, ok := sent["serviceUuids"].([]string)
	if !ok {
		t.Fatal("expected serviceUuids to be []string")
	}
	if len(uuids) != 2 {
		t.Errorf("expected 2 UUIDs, got %d", len(uuids))
	}
}

func TestHandler_ScanStop(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationScanStop)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_PeripheralGet_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationPeripheralGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_PeripheralGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"peripheralId": "dev-001"},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationPeripheralGet)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["peripheralId"] != "dev-001" {
		t.Errorf("expected peripheralId=dev-001")
	}
}

func TestHandler_PeripheralConnected(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"count": 2},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationPeripheralConnected)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Connect_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationConnect)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_Connect_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"state": "connected"},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationConnect)
	req.Payload["peripheralId"] = "dev-001"
	req.Payload["timeoutMs"] = float64(20000)
	req.Payload["autoReconnect"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["peripheralId"] != "dev-001" {
		t.Errorf("expected peripheralId=dev-001")
	}
	if sent["timeoutMs"] != 20000 {
		t.Errorf("expected timeoutMs=20000")
	}
	if sent["autoReconnect"] != true {
		t.Error("expected autoReconnect=true")
	}
}

func TestHandler_Disconnect_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationDisconnect)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_Disconnect_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"state": "disconnected"},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationDisconnect)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ServicesDiscover_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationServicesDiscover)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_ServicesDiscover_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"count": 3},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationServicesDiscover)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CharacteristicsDiscover_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicsDiscover)
	req.Payload["serviceRef"] = "svc-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsDiscover_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicsDiscover)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicsDiscover_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"count": 5},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationCharacteristicsDiscover)
	req.Payload["peripheralId"] = "dev-001"
	req.Payload["serviceRef"] = "svc-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_DescriptorsDiscover_MissingCharRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationDescriptorsDiscover)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicRead_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicRead)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicRead_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"value": "AQID"},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationCharacteristicRead)
	req.Payload["peripheralId"] = "dev-001"
	req.Payload["characteristicRef"] = "chr-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CharacteristicWrite_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicWrite)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicWrite_MissingValue(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicWrite)
	req.Payload["characteristicRef"] = "chr-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrWriteValueInvalid {
		t.Errorf("expected ErrWriteValueInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicWrite_InvalidEncoding(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicWrite)
	req.Payload["characteristicRef"] = "chr-001"
	req.Payload["value"] = map[string]any{"encoding": "utf8"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrValueEncodingInvalid {
		t.Errorf("expected ErrValueEncodingInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicWrite_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"accepted": true},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationCharacteristicWrite)
	req.Payload["characteristicRef"] = "chr-001"
	req.Payload["value"] = map[string]any{"encoding": "base64", "base64": "AQID"}
	req.Payload["mode"] = "with_response"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_CharacteristicSubscribe_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicSubscribe)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_CharacteristicUnsubscribe_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationCharacteristicUnsubscribe)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrCharacteristicNotFound {
		t.Errorf("expected ErrCharacteristicNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_DescriptorRead_MissingRef(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationDescriptorRead)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrDescriptorNotFound {
		t.Errorf("expected ErrDescriptorNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_DescriptorWrite_MissingValue(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationDescriptorWrite)
	req.Payload["peripheralId"] = "dev-001"
	req.Payload["descriptorRef"] = "desc-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrWriteValueInvalid {
		t.Errorf("expected ErrWriteValueInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_RSSIRead_MissingID(t *testing.T) {
	h := NewBluetoothHandler(newMockBluetoothBridge(nativebridge.Response{}, nil))
	req := baseBluetoothRequest(OperationRSSIRead)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidPeripheralID {
		t.Errorf("expected ErrInvalidPeripheralID, got %s", resp.Error.Code)
	}
}

func TestHandler_RSSIRead_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"rssi": -65},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationRSSIRead)
	req.Payload["peripheralId"] = "dev-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_PeripheralRoleStart(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"started": true},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationPeripheralRoleStart)
	req.Payload["localName"] = "Amitia"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["localName"] != "Amitia" {
		t.Errorf("expected localName=Amitia")
	}
}

func TestHandler_PeripheralRoleStop(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-bt-001",
		Status:          "ok",
		Result:          map[string]any{"stopped": true},
	}
	bridge := newMockBluetoothBridge(expected, nil)
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationPeripheralRoleStop)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockBluetoothBridge{
		delay: 5 * time.Second,
	}
	h := NewBluetoothHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseBluetoothRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockBluetoothBridge(nativebridge.Response{}, errors.New("bridge connection failed"))
	h := NewBluetoothHandler(bridge)

	req := baseBluetoothRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewBluetoothHandler(nil)
	req := baseBluetoothRequest(OperationStatus)
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
		native   string
		expected AuthorizationStatus
	}{
		{"allowedAlways", AuthAllowed},
		{"denied", AuthDenied},
		{"restricted", AuthRestricted},
		{"notDetermined", AuthNotDetermined},
		{"unknown", AuthNotDetermined},
	}
	for _, tt := range tests {
		t.Run(tt.native, func(t *testing.T) {
			result := AuthorizationStatusFromNative(tt.native)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCentralStateFromNative(t *testing.T) {
	tests := []struct {
		native   string
		expected CentralState
	}{
		{"unknown", StateUnknown},
		{"resetting", StateResetting},
		{"unsupported", StateUnsupported},
		{"unauthorized", StateUnauthorized},
		{"poweredOff", StatePoweredOff},
		{"poweredOn", StatePoweredOn},
		{"invalid", StateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.native, func(t *testing.T) {
			result := CentralStateFromNative(tt.native)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapCharacteristicProperty(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"read", "read"},
		{"write", "write"},
		{"writeWithoutResponse", "write_without_response"},
		{"notify", "notify"},
		{"indicate", "indicate"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapCharacteristicProperty(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasProperty(t *testing.T) {
	props := []string{"read", "write", "notify"}
	if !HasProperty(props, "read") {
		t.Error("expected read to be found")
	}
	if !HasProperty(props, "write") {
		t.Error("expected write to be found")
	}
	if HasProperty(props, "indicate") {
		t.Error("expected indicate not to be found")
	}
}

func TestIsValidUUID(t *testing.T) {
	valid := []string{
		"180D",
		"0000180D-0000-1000-8000-00805f9b34fb",
		"180F",
		"0000180f",
	}
	for _, uuid := range valid {
		if !IsValidUUID(uuid) {
			t.Errorf("expected %s to be valid", uuid)
		}
	}

	invalid := []string{
		"",
		"not-a-uuid",
		"GGGG",
		"180",
	}
	for _, uuid := range invalid {
		if IsValidUUID(uuid) {
			t.Errorf("expected %s to be invalid", uuid)
		}
	}
}

func TestNormalizeUUID(t *testing.T) {
	if NormalizeUUID("180D") != "0000180D-0000-1000-8000-00805f9b34fb" {
		t.Error("expected 16-bit UUID to be expanded")
	}
	if NormalizeUUID("0000180D-0000-1000-8000-00805f9b34fb") != "0000180D-0000-1000-8000-00805f9b34fb" {
		t.Error("expected 128-bit UUID to remain unchanged")
	}
	if NormalizeUUID("invalid") != "" {
		t.Error("expected invalid UUID to return empty string")
	}
}

func TestClampScanDuration(t *testing.T) {
	if ClampScanDuration(0) != DefaultScanDurationMs {
		t.Errorf("expected default %d", DefaultScanDurationMs)
	}
	if ClampScanDuration(99999) != MaxScanDurationMs {
		t.Errorf("expected max %d", MaxScanDurationMs)
	}
	if ClampScanDuration(15000) != 15000 {
		t.Error("expected 15000")
	}
}

func TestClampMaxResults(t *testing.T) {
	if ClampMaxResults(0) != DefaultMaxResults {
		t.Errorf("expected default %d", DefaultMaxResults)
	}
	if ClampMaxResults(999) != MaxResultsLimit {
		t.Errorf("expected max %d", MaxResultsLimit)
	}
	if ClampMaxResults(100) != 100 {
		t.Error("expected 100")
	}
}

func TestClampConnectTimeout(t *testing.T) {
	if ClampConnectTimeout(0) != DefaultConnectTimeoutMs {
		t.Errorf("expected default %d", DefaultConnectTimeoutMs)
	}
	if ClampConnectTimeout(99999) != MaxConnectTimeoutMs {
		t.Errorf("expected max %d", MaxConnectTimeoutMs)
	}
	if ClampConnectTimeout(500) != MinConnectTimeoutMs {
		t.Errorf("expected min %d", MinConnectTimeoutMs)
	}
	if ClampConnectTimeout(20000) != 20000 {
		t.Error("expected 20000")
	}
}

func TestValidateScanRequest(t *testing.T) {
	err := ValidateScanRequest(BluetoothScanRequest{
		DurationMs: 5000,
		MaxResults: 10,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateScanRequest(BluetoothScanRequest{
		DurationMs: 99999,
	})
	if err == nil {
		t.Error("expected error for excessive duration")
	}

	err = ValidateScanRequest(BluetoothScanRequest{
		DurationMs: 5000,
		ServiceUUIDs: []string{"invalid-uuid"},
	})
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestValidateConnectRequest(t *testing.T) {
	err := ValidateConnectRequest(BluetoothConnectRequest{
		PeripheralID: "dev-001",
		TimeoutMs:    5000,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateConnectRequest(BluetoothConnectRequest{
		PeripheralID: "",
	})
	if err == nil {
		t.Error("expected error for missing peripheralId")
	}
}

func TestValidateWriteRequest(t *testing.T) {
	err := ValidateWriteRequest(BluetoothWriteRequest{
		CharacteristicRef: "chr-001",
		Value:            BluetoothValueInput{Encoding: "base64", Base64: "AQID"},
		Mode:             "with_response",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateWriteRequest(BluetoothWriteRequest{
		CharacteristicRef: "chr-001",
		Mode:             "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid mode")
	}

	err = ValidateWriteRequest(BluetoothWriteRequest{
		CharacteristicRef: "chr-001",
		Value:            BluetoothValueInput{Encoding: "base64"},
		Mode:             "with_response",
	})
	if err == nil {
		t.Error("expected error for empty value")
	}
}

func TestParseBluetoothValue(t *testing.T) {
	v := parseBluetoothValue(map[string]any{
		"encoding": "base64",
		"base64":   "AQID",
	})
	if v.Encoding != "base64" {
		t.Errorf("expected encoding=base64, got %s", v.Encoding)
	}
	if v.Base64 != "AQID" {
		t.Errorf("expected base64=AQID, got %s", v.Base64)
	}

	v2 := parseBluetoothValue(map[string]any{
		"encoding": "hex",
		"hex":      "010203",
	})
	if v2.Encoding != "hex" {
		t.Errorf("expected encoding=hex, got %s", v2.Encoding)
	}
	if v2.Hex != "010203" {
		t.Errorf("expected hex=010203, got %s", v2.Hex)
	}
}
