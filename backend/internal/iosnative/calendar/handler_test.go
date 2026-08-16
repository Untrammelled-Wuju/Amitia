package calendar

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
)

type fakeCalendarBridge struct {
	executeFunc func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error)
	healthFunc  func(ctx context.Context) nativebridge.Health
}

func (f *fakeCalendarBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	if f.executeFunc != nil {
		return f.executeFunc(ctx, req)
	}
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestId:       req.RequestId,
		Status:          "success",
	}, nil
}

func (f *fakeCalendarBridge) Health(ctx context.Context) nativebridge.Health {
	if f.healthFunc != nil {
		return f.healthFunc(ctx)
	}
	return nativebridge.HealthReady
}

func TestCalendarHandler_UnknownOperation(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       "calendar.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestCalendarHandler_Status_NilBridge(t *testing.T) {
	h := NewCalendarHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Fatalf("expected ErrNativeBridgeUnavailable, got %+v", resp.Error)
	}
}

func TestCalendarHandler_AuthorizationStatus_NilBridge(t *testing.T) {
	h := NewCalendarHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationAuthorizationStatus,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Fatalf("expected ErrNativeBridgeUnavailable, got %+v", resp.Error)
	}
}

func TestCalendarHandler_AuthorizationRequest_MissingAccess(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationAuthorizationRequest,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidResponse {
		t.Fatalf("expected ErrInvalidResponse, got %+v", resp.Error)
	}
}

func TestCalendarHandler_AuthorizationRequest_InvalidAccess(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationAuthorizationRequest,
		Payload:         map[string]any{"access": "invalid"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidResponse {
		t.Fatalf("expected ErrInvalidResponse, got %+v", resp.Error)
	}
}

func TestCalendarHandler_AuthorizationRequest_ValidAccess(t *testing.T) {
	bridge := &fakeCalendarBridge{
		executeFunc: func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
			return nativebridge.Response{
				ProtocolVersion: req.ProtocolVersion,
				RequestId:       req.RequestId,
				Status:          "success",
				Result: map[string]any{
					"level":   "full_access",
					"granted": true,
				},
			}, nil
		},
	}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationAuthorizationRequest,
		Payload:         map[string]any{"access": "full_access"},
	})

	if resp.Status != "success" {
		t.Fatalf("expected success status, got %s", resp.Status)
	}
}

func TestCalendarHandler_EventsQuery_MissingStartAt(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsQuery,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidDateRange {
		t.Fatalf("expected ErrInvalidDateRange, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsQuery_EndBeforeStart(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsQuery,
		Payload: map[string]any{
			"startAt": "2026-08-13T00:00:00Z",
			"endAt":   "2026-08-12T00:00:00Z",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidDateRange {
		t.Fatalf("expected ErrInvalidDateRange, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsQuery_RangeTooLarge(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsQuery,
		Payload: map[string]any{
			"startAt": "2025-01-01T00:00:00Z",
			"endAt":   "2026-12-31T00:00:00Z",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrQueryRangeTooLarge {
		t.Fatalf("expected ErrQueryRangeTooLarge, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsCreate_EmptyTitle(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsCreate,
		Payload: map[string]any{
			"title":   "",
			"startAt": "2026-08-13T14:00:00Z",
			"endAt":   "2026-08-13T15:00:00Z",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidResponse {
		t.Fatalf("expected ErrInvalidResponse, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsCreate_EndBeforeStart(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsCreate,
		Payload: map[string]any{
			"title":   "Test Event",
			"startAt": "2026-08-13T15:00:00Z",
			"endAt":   "2026-08-13T14:00:00Z",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidDateRange {
		t.Fatalf("expected ErrInvalidDateRange, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsCreate_InvalidTimezone(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsCreate,
		Payload: map[string]any{
			"title":    "Test Event",
			"startAt":  "2026-08-13T14:00:00Z",
			"endAt":    "2026-08-13T15:00:00Z",
			"timeZone": "Invalid/Timezone",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidTimezone {
		t.Fatalf("expected ErrInvalidTimezone, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsCreate_InvalidURL(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsCreate,
		Payload: map[string]any{
			"title":   "Test Event",
			"startAt": "2026-08-13T14:00:00Z",
			"endAt":   "2026-08-13T15:00:00Z",
			"url":     "javascript:alert(1)",
		},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidResponse {
		t.Fatalf("expected ErrInvalidResponse, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsGet_MissingEventID(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsGet,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrEventNotFound {
		t.Fatalf("expected ErrEventNotFound, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsUpdate_MissingEventID(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsUpdate,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrEventNotFound {
		t.Fatalf("expected ErrEventNotFound, got %+v", resp.Error)
	}
}

func TestCalendarHandler_EventsDelete_MissingEventID(t *testing.T) {
	bridge := &fakeCalendarBridge{}
	h := NewCalendarHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-1",
		Platform:        "ios",
		Operation:       OperationEventsDelete,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrEventNotFound {
		t.Fatalf("expected ErrEventNotFound, got %+v", resp.Error)
	}
}

func TestAuthorizationLevelFromNative(t *testing.T) {
	tests := []struct {
		native   string
		expected AuthorizationLevel
	}{
		{"notDetermined", AuthorizationNotDetermined},
		{"restricted", AuthorizationRestricted},
		{"denied", AuthorizationDenied},
		{"writeOnly", AuthorizationWriteOnly},
		{"fullAccess", AuthorizationFullAccess},
		{"authorized", AuthorizationLegacyAuthorized},
		{"", AuthorizationNotDetermined},
	}

	for _, tt := range tests {
		t.Run(tt.native, func(t *testing.T) {
			result := AuthorizationLevelFromNative(tt.native)
			if result != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCanCreateEvent(t *testing.T) {
	tests := []struct {
		level    AuthorizationLevel
		expected bool
	}{
		{AuthorizationWriteOnly, true},
		{AuthorizationFullAccess, true},
		{AuthorizationLegacyAuthorized, true},
		{AuthorizationNotDetermined, false},
		{AuthorizationDenied, false},
		{AuthorizationRestricted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := CanCreateEvent(tt.level)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanReadEvents(t *testing.T) {
	tests := []struct {
		level    AuthorizationLevel
		expected bool
	}{
		{AuthorizationFullAccess, true},
		{AuthorizationLegacyAuthorized, true},
		{AuthorizationWriteOnly, false},
		{AuthorizationNotDetermined, false},
		{AuthorizationDenied, false},
		{AuthorizationRestricted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := CanReadEvents(tt.level)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapEventSpan(t *testing.T) {
	tests := []struct {
		span     string
		expected EventSpan
		ok       bool
	}{
		{"this_event", EventSpanThisEvent, true},
		{"future_events", EventSpanFutureEvents, true},
		{"", EventSpanThisEvent, true},
		{"invalid", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.span, func(t *testing.T) {
			result, ok := MapEventSpan(tt.span)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestIsValidAccessLevel(t *testing.T) {
	if !IsValidAccessLevel("write_only") {
		t.Fatal("expected write_only to be valid")
	}
	if !IsValidAccessLevel("full_access") {
		t.Fatal("expected full_access to be valid")
	}
	if IsValidAccessLevel("invalid") {
		t.Fatal("expected invalid to be invalid")
	}
}

func TestIsValidDayOfWeek(t *testing.T) {
	validDays := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range validDays {
		if !IsValidDayOfWeek(day) {
			t.Fatalf("expected %s to be valid", day)
		}
	}
	if IsValidDayOfWeek("invalid") {
		t.Fatal("expected invalid day to be invalid")
	}
}

func TestIsValidFrequency(t *testing.T) {
	validFreqs := []string{"daily", "weekly", "monthly", "yearly"}
	for _, freq := range validFreqs {
		if !IsValidFrequency(freq) {
			t.Fatalf("expected %s to be valid", freq)
		}
	}
	if IsValidFrequency("hourly") {
		t.Fatal("expected hourly to be invalid")
	}
}
