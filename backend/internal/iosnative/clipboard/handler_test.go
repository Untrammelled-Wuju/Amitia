package clipboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockClipboardBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockClipboardBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockClipboardBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockClipboardBridge(resp nativebridge.Response, err error) *mockClipboardBridge {
	return &mockClipboardBridge{response: resp, err: err}
}

func baseClipboardRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewClipboardHandler(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewClipboardHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	req := baseClipboardRequest("clipboard.unknown")
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
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_Detect_MissingPatterns(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	req := baseClipboardRequest(OperationDetect)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrDetectionUnsupported {
		t.Errorf("expected ErrDetectionUnsupported, got %s", resp.Error.Code)
	}
}

func TestHandler_Detect_InvalidPatterns(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	req := baseClipboardRequest(OperationDetect)
	req.Payload["patterns"] = []any{"invalid_pattern"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrDetectionUnsupported {
		t.Errorf("expected ErrDetectionUnsupported, got %s", resp.Error.Code)
	}
}

func TestHandler_Detect_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"matches": []any{}},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationDetect)
	req.Payload["patterns"] = []any{"probableWebURL", "probableWebSearch", "number"}
	req.Payload["includeValues"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	patterns, ok := sent["patterns"].([]string)
	if !ok {
		t.Fatal("expected patterns to be []string")
	}
	if len(patterns) != 3 {
		t.Errorf("expected 3 patterns, got %d", len(patterns))
	}
}

func TestHandler_Read(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"items": []any{}},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationRead)
	req.Payload["preferredTypes"] = []any{"text/plain", "text/html"}
	req.Payload["maxItems"] = float64(5)
	req.Payload["maxBytes"] = float64(32768)
	req.Payload["materializeBinary"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["maxItems"] != 5 {
		t.Errorf("expected maxItems=5, got %v", sent["maxItems"])
	}
	if sent["maxBytes"] != int64(32768) {
		t.Errorf("expected maxBytes=32768")
	}
	if sent["materializeBinary"] != true {
		t.Error("expected materializeBinary=true")
	}
}

func TestHandler_Read_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationRead)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["maxItems"] != DefaultMaxItems {
		t.Errorf("expected default maxItems=%d, got %v", DefaultMaxItems, sent["maxItems"])
	}
	if sent["maxBytes"] != MaxClipboardReadBytes {
		t.Errorf("expected default maxBytes=%d, got %v", MaxClipboardReadBytes, sent["maxBytes"])
	}
}

func TestHandler_Write_MissingItems(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	req := baseClipboardRequest(OperationWrite)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrWriteValueRequired {
		t.Errorf("expected ErrWriteValueRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_Write_InvalidType(t *testing.T) {
	h := NewClipboardHandler(newMockClipboardBridge(nativebridge.Response{}, nil))
	req := baseClipboardRequest(OperationWrite)
	req.Payload["items"] = []any{
		map[string]any{"type": "invalid/type", "text": "hello"},
	}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrWriteValueInvalid {
		t.Errorf("expected ErrWriteValueInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_Write_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"written": true},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationWrite)
	req.Payload["items"] = []any{
		map[string]any{"type": "text/plain", "text": "hello world"},
	}
	req.Payload["localOnly"] = true
	req.Payload["expirationSeconds"] = float64(120)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["localOnly"] != true {
		t.Error("expected localOnly=true")
	}
	if sent["expirationSeconds"] != 120 {
		t.Errorf("expected expirationSeconds=120, got %v", sent["expirationSeconds"])
	}
}

func TestHandler_Write_WithURL(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"written": true},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationWrite)
	req.Payload["items"] = []any{
		map[string]any{"type": "text/uri-list", "url": "https://example.com"},
	}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Write_WithResourceURI(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"written": true},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationWrite)
	req.Payload["items"] = []any{
		map[string]any{"type": "image/png", "resourceUri": "resource://abc123"},
	}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Clear(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-cb-001",
		Status:          "ok",
		Result:          map[string]any{"cleared": true},
	}
	bridge := newMockClipboardBridge(expected, nil)
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationClear)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockClipboardBridge{
		delay: 5 * time.Second,
	}
	h := NewClipboardHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseClipboardRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockClipboardBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewClipboardHandler(bridge)

	req := baseClipboardRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewClipboardHandler(nil)
	req := baseClipboardRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestIsValidPatternKind(t *testing.T) {
	valid := []PatternKind{
		PatternProbableWebURL,
		PatternProbableWebSearch,
		PatternNumber,
		PatternEmailAddress,
	}
	for _, p := range valid {
		if !IsValidPatternKind(p) {
			t.Errorf("expected %s to be valid", p)
		}
	}

	invalid := []PatternKind{"invalid_pattern", ""}
	for _, p := range invalid {
		if IsValidPatternKind(p) {
			t.Errorf("expected %s to be invalid", p)
		}
	}
}

func TestIsValidReadType(t *testing.T) {
	valid := []ContentType{
		ContentTypeTextPlain,
		ContentTypeTextHTML,
		ContentTypeTextURI,
		ContentTypeImagePNG,
		ContentTypeFileURL,
	}
	for _, ct := range valid {
		if !IsValidReadType(ct) {
			t.Errorf("expected %s to be valid", ct)
		}
	}

	invalid := []ContentType{"invalid/type", "application/octet-stream"}
	for _, ct := range invalid {
		if IsValidReadType(ct) {
			t.Errorf("expected %s to be invalid", ct)
		}
	}
}

func TestClampMaxItems(t *testing.T) {
	if ClampMaxItems(0) != DefaultMaxItems {
		t.Errorf("expected default %d", DefaultMaxItems)
	}
	if ClampMaxItems(999) != MaxItemsLimit {
		t.Errorf("expected max %d", MaxItemsLimit)
	}
	if ClampMaxItems(5) != 5 {
		t.Error("expected 5")
	}
}

func TestClampMaxBytes(t *testing.T) {
	if ClampMaxBytes(0) != MaxClipboardReadBytes {
		t.Errorf("expected default %d", MaxClipboardReadBytes)
	}
	if ClampMaxBytes(999999999) != MaxMaterializedClipboardBytes {
		t.Errorf("expected max %d", MaxMaterializedClipboardBytes)
	}
	if ClampMaxBytes(32768) != 32768 {
		t.Error("expected 32768")
	}
}

func TestClampExpirationSeconds(t *testing.T) {
	result := ClampExpirationSeconds(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}

	negative := -1
	result = ClampExpirationSeconds(&negative)
	if result != nil {
		t.Error("expected nil for negative value")
	}

	zero := 0
	result = ClampExpirationSeconds(&zero)
	if result != nil {
		t.Error("expected nil for zero value")
	}

	normal := 300
	result = ClampExpirationSeconds(&normal)
	if *result != 300 {
		t.Error("expected 300")
	}

	excessive := 99999
	result = ClampExpirationSeconds(&excessive)
	if *result != MaxExpirationSeconds {
		t.Errorf("expected max %d", MaxExpirationSeconds)
	}
}

func TestValidateReadRequest(t *testing.T) {
	err := ValidateReadRequest(ClipboardReadRequest{
		PreferredTypes: []string{"text/plain", "text/html"},
		MaxItems:       5,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateReadRequest(ClipboardReadRequest{
		PreferredTypes: []string{"invalid/type"},
	})
	if err == nil {
		t.Error("expected error for invalid type")
	}

	err = ValidateReadRequest(ClipboardReadRequest{
		ItemIndexes: []int{-1},
	})
	if err == nil {
		t.Error("expected error for negative index")
	}
}

func TestValidateWriteRequest(t *testing.T) {
	err := ValidateWriteRequest(ClipboardWriteRequest{
		Items: []ClipboardWriteItem{
			{Type: "text/plain", Text: "hello"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateWriteRequest(ClipboardWriteRequest{
		Items: []ClipboardWriteItem{},
	})
	if err == nil {
		t.Error("expected error for empty items")
	}

	err = ValidateWriteRequest(ClipboardWriteRequest{
		Items: []ClipboardWriteItem{
			{Type: "invalid/type"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidatePatterns(t *testing.T) {
	err := ValidatePatterns([]PatternKind{
		PatternProbableWebURL,
		PatternNumber,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidatePatterns([]PatternKind{"invalid"})
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestMapSensitivityToLocalOnly(t *testing.T) {
	result := MapSensitivityToLocalOnly(SensitivityNormal)
	if result != nil {
		t.Error("expected nil for normal sensitivity")
	}

	result = MapSensitivityToLocalOnly(SensitivitySensitive)
	if result == nil || *result != true {
		t.Error("expected true for sensitive")
	}

	result = MapSensitivityToLocalOnly(SensitivitySecret)
	if result == nil || *result != true {
		t.Error("expected true for secret")
	}
}

func TestMapSensitivityToExpiration(t *testing.T) {
	result := MapSensitivityToExpiration(SensitivityNormal)
	if result != nil {
		t.Error("expected nil for normal sensitivity")
	}

	result = MapSensitivityToExpiration(SensitivitySensitive)
	if result == nil || *result != DefaultExpirationSensitive {
		t.Errorf("expected %d for sensitive", DefaultExpirationSensitive)
	}

	result = MapSensitivityToExpiration(SensitivitySecret)
	if result == nil || *result != DefaultExpirationSecret {
		t.Errorf("expected %d for secret", DefaultExpirationSecret)
	}
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrClipboardUnsupported, "clipboard is not supported on this device"},
		{ErrClipboardEmpty, "clipboard is empty"},
		{ErrReadUserIntentRequired, "user intent is required to read clipboard"},
		{ErrWriteFailed, "clipboard write failed"},
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
