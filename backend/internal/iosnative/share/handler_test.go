package share

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockShareBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockShareBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockShareBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockShareBridge(resp nativebridge.Response, err error) *mockShareBridge {
	return &mockShareBridge{response: resp, err: err}
}

func baseShareRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewShareHandler(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewShareHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest("share.unknown")
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
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_Send_MissingFields(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationSend)
	req.Payload["text"] = "Hello World"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Send_WithURL(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationSend)
	req.Payload["url"] = "https://example.com"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["url"] != "https://example.com" {
		t.Error("expected url")
	}
}

func TestHandler_Send_InvalidURL(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationSend)
	req.Payload["url"] = "javascript:alert(1)"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_Send_WithResources(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationSend)
	req.Payload["resources"] = []any{"amitia://res/1", "amitia://res/2"}
	req.Payload["subject"] = "Test Subject"
	req.Payload["shareTitle"] = "Share Title"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	resources, ok := sent["resources"].([]string)
	if !ok || len(resources) != 2 {
		t.Error("expected 2 resources")
	}
	if sent["subject"] != "Test Subject" {
		t.Error("expected subject")
	}
}

func TestHandler_Send_WithPreview(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationSend)
	req.Payload["text"] = "Test"
	req.Payload["preview"] = map[string]any{
		"title":           "Preview Title",
		"subtitle":        "Preview Subtitle",
		"imageResourceUri": "amitia://res/preview",
	}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	preview, ok := sent["preview"].(map[string]any)
	if !ok {
		t.Fatal("expected preview in payload")
	}
	if preview["title"] != "Preview Title" {
		t.Error("expected preview title")
	}
}

func TestHandler_Send_TextTooLong(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationSend)
	longText := make([]byte, MaxShareTextBytes+1)
	req.Payload["text"] = string(longText)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShareTextTooLong {
		t.Errorf("expected ErrShareTextTooLong, got %s", resp.Error.Code)
	}
}

func TestHandler_Send_TooManyResources(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationSend)
	resources := make([]any, MaxResourcesCount+1)
	for i := range resources {
		resources[i] = "amitia://res/" + string(rune('a'+i))
	}
	req.Payload["resources"] = resources
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShareTooManyResources {
		t.Errorf("expected ErrShareTooManyResources, got %s", resp.Error.Code)
	}
}

func TestHandler_PreviewSupported(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationPreviewSupported)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ReceivePending(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"shares": []any{}},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationReceivePending)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ReceiveConsume_MissingID(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationReceiveConsume)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShareReceivedNotFound {
		t.Errorf("expected ErrShareReceivedNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_ReceiveConsume_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"consumed": true},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationReceiveConsume)
	req.Payload["shareId"] = "share-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["shareId"] != "share-001" {
		t.Error("expected shareId")
	}
}

func TestHandler_ReceivePeek_MissingID(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationReceivePeek)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_ReceiveDismiss_MissingID(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationReceiveDismiss)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_StagingCleanup(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"removed": 0},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationStagingCleanup)
	req.Payload["removeStale"] = true
	req.Payload["maxStaleAgeHours"] = float64(48)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["removeStale"] != true {
		t.Error("expected removeStale=true")
	}
	if sent["maxStaleAgeHours"] != 48 {
		t.Error("expected maxStaleAgeHours=48")
	}
}

func TestHandler_StagingCleanup_Defaults(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationStagingCleanup)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["maxStaleAgeHours"] != StagingMaxStaleAgeHours {
		t.Errorf("expected default maxStaleAgeHours=%d", StagingMaxStaleAgeHours)
	}
}

func TestHandler_LimitedDelete_MissingPhotoIDs(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationLimitedDelete)
	req.Payload["confirm"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShareResourceInvalid {
		t.Errorf("expected ErrShareResourceInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_LimitedDelete_NotConfirmed(t *testing.T) {
	h := NewShareHandler(newMockShareBridge(nativebridge.Response{}, nil))
	req := baseShareRequest(OperationLimitedDelete)
	req.Payload["photoIds"] = []any{"photo-001"}
	req.Payload["confirm"] = false
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrShareUserIntentRequired {
		t.Errorf("expected ErrShareUserIntentRequired, got %s", resp.Error.Code)
	}
}

func TestHandler_LimitedDelete_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-share-001",
		Status:          "ok",
		Result:          map[string]any{"confirmed": true},
	}
	bridge := newMockShareBridge(expected, nil)
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationLimitedDelete)
	req.Payload["photoIds"] = []any{"photo-001", "photo-002"}
	req.Payload["confirm"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockShareBridge{
		delay: 5 * time.Second,
	}
	h := NewShareHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseShareRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockShareBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewShareHandler(bridge)

	req := baseShareRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewShareHandler(nil)
	req := baseShareRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestIsValidURLScheme(t *testing.T) {
	for _, s := range []string{"http", "https"} {
		if !IsValidURLScheme(s) {
			t.Errorf("expected %s to be valid", s)
		}
	}
	for _, s := range []string{"ftp", "javascript", "mailto", "tel"} {
		if IsValidURLScheme(s) {
			t.Errorf("expected %s to be invalid", s)
		}
	}
}

func TestValidateURL(t *testing.T) {
	if err := ValidateURL("https://example.com"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateURL(""); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateURL("javascript:alert(1)"); err == nil {
		t.Error("expected error for javascript scheme")
	}
	if err := ValidateURL("ftp://example.com"); err == nil {
		t.Error("expected error for ftp scheme")
	}
}

func TestValidateSendRequest(t *testing.T) {
	err := ValidateSendRequest(IOSShareSendRequest{
		Text: "Hello",
		URL:  "https://example.com",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	longText := make([]byte, MaxShareTextBytes+1)
	err = ValidateSendRequest(IOSShareSendRequest{Text: string(longText)})
	if err == nil {
		t.Error("expected error for text too long")
	}

	err = ValidateSendRequest(IOSShareSendRequest{URL: "javascript:alert(1)"})
	if err == nil {
		t.Error("expected error for invalid URL scheme")
	}
}

func TestValidateIncomingItem(t *testing.T) {
	err := ValidateIncomingItem(IOSIncomingShareItem{
		ItemID:       "item-001",
		RelativePath: "items/0001",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateIncomingItem(IOSIncomingShareItem{RelativePath: "../etc/passwd"})
	if err == nil {
		t.Error("expected error for path escape")
	}

	err = ValidateIncomingItem(IOSIncomingShareItem{RelativePath: "/absolute/path"})
	if err == nil {
		t.Error("expected error for absolute path")
	}

	err = ValidateIncomingItem(IOSIncomingShareItem{})
	if err == nil {
		t.Error("expected error for missing itemId")
	}
}

func TestValidateIncomingManifest(t *testing.T) {
	err := ValidateIncomingManifest(IOSIncomingShareManifest{
		ShareID:  "share-001",
		Complete: true,
		Items: []IOSIncomingShareItem{
			{ItemID: "item-001", RelativePath: "items/0001"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateIncomingManifest(IOSIncomingShareManifest{
		ShareID:  "share-001",
		Complete: false,
	})
	if err == nil {
		t.Error("expected error for incomplete manifest")
	}

	err = ValidateIncomingManifest(IOSIncomingShareManifest{
		ShareID:  "",
		Complete: true,
	})
	if err == nil {
		t.Error("expected error for missing shareId")
	}
}

func TestClampMaxStaleAgeHours(t *testing.T) {
	if ClampMaxStaleAgeHours(0) != StagingMaxStaleAgeHours {
		t.Errorf("expected default %d", StagingMaxStaleAgeHours)
	}
	if ClampMaxStaleAgeHours(-1) != StagingMaxStaleAgeHours {
		t.Errorf("expected default %d", StagingMaxStaleAgeHours)
	}
	if ClampMaxStaleAgeHours(48) != 48 {
		t.Error("expected 48")
	}
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrShareUnsupported, "share is not supported on this device"},
		{ErrShareTooManyResources, "too many resources to share"},
		{ErrShareUserCancelled, "share was cancelled by user"},
		{ErrShareStagingPathEscape, "staging path contains escape sequence"},
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
