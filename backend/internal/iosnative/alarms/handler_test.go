package alarms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockAlarmBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockAlarmBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockAlarmBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockAlarmBridge(resp nativebridge.Response, err error) *mockAlarmBridge {
	return &mockAlarmBridge{response: resp, err: err}
}

func baseAlarmRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewAlarmHandler(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	if h == nil {
		t.Fatal("NewAlarmHandler returned nil")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest("alarms.unknown")
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
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"supported": true},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
}

func TestHandler_AuthorizationStatus(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"authorized": true},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationAuthorizationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_AuthorizationRequest(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"authorized": true},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationAuthorizationRequest)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_List(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"alarms": []any{}},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Get_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsNotFound {
		t.Errorf("expected ErrAlarmsNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_Get_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"alarmId": "alarm-001"},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationGet)
	req.Payload["alarmId"] = "alarm-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["alarmId"] != "alarm-001" {
		t.Error("expected alarmId=alarm-001")
	}
}

func TestHandler_Schedule_MissingKind(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationSchedule)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsScheduleInvalid {
		t.Errorf("expected ErrAlarmsScheduleInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_Schedule_MissingTitle(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "alarm"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsScheduleInvalid {
		t.Errorf("expected ErrAlarmsScheduleInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_Schedule_MissingPresentation(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "timer"
	req.Payload["title"] = "My Timer"
	req.Payload["countdown"] = map[string]any{"preAlertSeconds": float64(60)}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsPresentationInvalid {
		t.Errorf("expected ErrAlarmsPresentationInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_Schedule_InvalidFlashMode(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "alarm"
	req.Payload["title"] = "Wake Up"
	req.Payload["schedule"] = map[string]any{
		"hour":       float64(7),
		"minute":     float64(0),
		"recurrence": "never",
	}
	req.Payload["presentation"] = map[string]any{"alertTitle": "Wake Up"}
	req.Payload["sound"] = map[string]any{"kind": "invalid"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsSoundInvalid {
		t.Errorf("expected ErrAlarmsSoundInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_Schedule_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"alarmId": "alarm-001"},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "alarm"
	req.Payload["title"] = "Wake Up"
	req.Payload["schedule"] = map[string]any{
		"hour":       float64(7),
		"minute":     float64(0),
		"recurrence": "weekly",
		"weekdays":   []any{"monday", "tuesday"},
	}
	req.Payload["presentation"] = map[string]any{
		"alertTitle": "Wake Up",
		"tintColor":  "system",
	}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	sent := bridge.calls[0].Payload
	if sent["kind"] != "alarm" {
		t.Error("expected kind=alarm")
	}
}

func TestHandler_Schedule_Timer(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"alarmId": "timer-001"},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "timer"
	req.Payload["title"] = "Cooking Timer"
	req.Payload["countdown"] = map[string]any{"preAlertSeconds": float64(300)}
	req.Payload["presentation"] = map[string]any{"alertTitle": "Timer Done"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Schedule_Fixed(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"alarmId": "fixed-001"},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationSchedule)
	req.Payload["kind"] = "alarm"
	req.Payload["title"] = "Fixed Event"
	req.Payload["schedule"] = map[string]any{
		"fireAt":     "2026-08-14T07:00:00+08:00",
		"recurrence": "never",
	}
	req.Payload["presentation"] = map[string]any{"alertTitle": "Event Time"}
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Stop_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationStop)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsNotFound {
		t.Errorf("expected ErrAlarmsNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_Stop_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"state": "stopped"},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationStop)
	req.Payload["alarmId"] = "alarm-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Cancel_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationCancel)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrAlarmsNotFound {
		t.Errorf("expected ErrAlarmsNotFound, got %s", resp.Error.Code)
	}
}

func TestHandler_Cancel_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestId:       "test-alarm-001",
		Status:          "ok",
		Result:          map[string]any{"cancelled": true},
	}
	bridge := newMockAlarmBridge(expected, nil)
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationCancel)
	req.Payload["alarmId"] = "alarm-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Countdown_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationCountdown)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_Pause_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationPause)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_Resume_MissingID(t *testing.T) {
	h := NewAlarmHandler(newMockAlarmBridge(nativebridge.Response{}, nil))
	req := baseAlarmRequest(OperationResume)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockAlarmBridge{
		delay: 5 * time.Second,
	}
	h := NewAlarmHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseAlarmRequest(OperationStatus)
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockAlarmBridge(nativebridge.Response{}, errors.New("bridge failed"))
	h := NewAlarmHandler(bridge)

	req := baseAlarmRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestHandler_BridgeUnavailable(t *testing.T) {
	h := NewAlarmHandler(nil)
	req := baseAlarmRequest(OperationStatus)
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
		{"authorized", AuthAuthorized},
		{"denied", AuthDenied},
		{"notDetermined", AuthNotDetermined},
		{"invalid", AuthNotDetermined},
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

func TestIsValidWeekday(t *testing.T) {
	for _, w := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
		if !IsValidWeekday(w) {
			t.Errorf("expected %s to be valid", w)
		}
	}
	if IsValidWeekday("invalid") {
		t.Error("expected invalid weekday")
	}
}

func TestIsValidAction(t *testing.T) {
	for _, a := range []string{"dismiss", "repeat", "open", "pause", "resume"} {
		if !IsValidAction(a) {
			t.Errorf("expected %s to be valid", a)
		}
	}
	if IsValidAction("invalid") {
		t.Error("expected invalid action")
	}
}

func TestIsValidAlarmIntentAction(t *testing.T) {
	for _, a := range []string{"open_alarm_details", "mark_alarm_acknowledged", "open_chat"} {
		if !IsValidAlarmIntentAction(a) {
			t.Errorf("expected %s to be valid", a)
		}
	}
	if IsValidAlarmIntentAction("invalid") {
		t.Error("expected invalid intent action")
	}
}

func TestIsValidKind(t *testing.T) {
	for _, k := range []string{"alarm", "timer", "countdown_alarm"} {
		if !IsValidKind(k) {
			t.Errorf("expected %s to be valid", k)
		}
	}
	if IsValidKind("invalid") {
		t.Error("expected invalid kind")
	}
}

func TestValidateSchedule(t *testing.T) {
	err := ValidateSchedule(&IOSAlarmSchedule{
		Hour:       intPtr(7),
		Minute:     intPtr(0),
		Recurrence: string(RecurrenceWeekly),
		Weekdays:   []string{"monday", "tuesday"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateSchedule(&IOSAlarmSchedule{
		Recurrence: string(RecurrenceWeekly),
	})
	if err == nil {
		t.Error("expected error for weekly without weekdays")
	}

	err = ValidateSchedule(&IOSAlarmSchedule{
		Recurrence: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid recurrence")
	}
}

func TestValidateCountdown(t *testing.T) {
	pre := int64(60)
	err := ValidateCountdown(&IOSAlarmCountdown{PreAlertSeconds: &pre})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	zero := int64(0)
	err = ValidateCountdown(&IOSAlarmCountdown{PreAlertSeconds: &zero})
	if err == nil {
		t.Error("expected error for zero preAlertSeconds")
	}

	negative := int64(-10)
	err = ValidateCountdown(&IOSAlarmCountdown{PostAlertSeconds: &negative})
	if err == nil {
		t.Error("expected error for negative postAlertSeconds")
	}
}

func TestValidatePresentation(t *testing.T) {
	err := ValidatePresentation(IOSAlarmPresentation{AlertTitle: "Wake Up"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidatePresentation(IOSAlarmPresentation{})
	if err == nil {
		t.Error("expected error for empty alertTitle")
	}

	err = ValidatePresentation(IOSAlarmPresentation{AlertTitle: "Test", TintColor: "invalid"})
	if err == nil {
		t.Error("expected error for invalid tint color")
	}

	err = ValidatePresentation(IOSAlarmPresentation{AlertTitle: "Test", SecondaryAction: "invalid"})
	if err == nil {
		t.Error("expected error for invalid secondary action")
	}
}

func TestValidateSound(t *testing.T) {
	err := ValidateSound(IOSAlarmSound{Kind: "default"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateSound(IOSAlarmSound{Kind: "named", SoundID: "chime"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateSound(IOSAlarmSound{Kind: "named"})
	if err == nil {
		t.Error("expected error for named without soundId")
	}

	err = ValidateSound(IOSAlarmSound{Kind: "invalid"})
	if err == nil {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateScheduleRequest(t *testing.T) {
	err := ValidateScheduleRequest(IOSAlarmScheduleRequest{
		Kind:  "alarm",
		Title: "Wake Up",
		Schedule: &IOSAlarmSchedule{
			Hour:       intPtr(7),
			Minute:     intPtr(0),
			Recurrence: string(RecurrenceNever),
		},
		Presentation: IOSAlarmPresentation{AlertTitle: "Wake Up"},
		Sound:        IOSAlarmSound{Kind: "default"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidateScheduleRequest(IOSAlarmScheduleRequest{
		Kind:         "invalid",
		Title:        "Test",
		Presentation: IOSAlarmPresentation{AlertTitle: "Test"},
	})
	if err == nil {
		t.Error("expected error for invalid kind")
	}

	err = ValidateScheduleRequest(IOSAlarmScheduleRequest{
		Kind:  "timer",
		Title: "Test",
		Countdown: &IOSAlarmCountdown{
			PreAlertSeconds: int64Ptr(60),
		},
		Presentation: IOSAlarmPresentation{AlertTitle: "Test"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func intPtr(v int) *int {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestMapCodeToMessage(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{ErrAlarmsUnsupported, "alarms are not supported on this device"},
		{ErrAlarmsAuthDenied, "alarm access denied"},
		{ErrAlarmsNotFound, "alarm not found"},
		{ErrAlarmsScheduleInPast, "alarm schedule is in the past"},
		{ErrAlarmsMaximumLimitReached, "maximum alarm limit reached"},
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
