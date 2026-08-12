package reminders

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
)

type fakeRemindersBridge struct {
	executeFunc func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error)
	healthFunc  func(ctx context.Context) nativebridge.Health
}

func (f *fakeRemindersBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	if f.executeFunc != nil {
		return f.executeFunc(ctx, req)
	}
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (f *fakeRemindersBridge) Health(ctx context.Context) nativebridge.Health {
	if f.healthFunc != nil {
		return f.healthFunc(ctx)
	}
	return nativebridge.HealthReady
}

func TestRemindersHandler_UnknownOperation(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       "reminders.unknown",
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != nativebridge.ErrOperationNotSupported {
		t.Fatalf("expected OPERATION_NOT_SUPPORTED, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Status_NilBridge(t *testing.T) {
	h := NewRemindersHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
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

func TestRemindersHandler_AuthorizationStatus_NilBridge(t *testing.T) {
	h := NewRemindersHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
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

func TestRemindersHandler_AuthorizationRequest_NilBridge(t *testing.T) {
	h := NewRemindersHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationAuthorizationRequest,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Fatalf("expected ErrNativeBridgeUnavailable, got %+v", resp.Error)
	}
}

func TestRemindersHandler_ListsList_NilBridge(t *testing.T) {
	h := NewRemindersHandler(nil)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationListsList,
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Fatalf("expected ErrNativeBridgeUnavailable, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Query_InvalidStatus(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationQuery,
		Payload:         map[string]any{"status": "invalid"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidResponse {
		t.Fatalf("expected ErrInvalidResponse, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Query_InvalidDueDateFormat(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationQuery,
		Payload:         map[string]any{"dueStart": "invalid-date"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidDateRange {
		t.Fatalf("expected ErrInvalidDateRange, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Get_MissingReminderID(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationGet,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrReminderNotFound {
		t.Fatalf("expected ErrReminderNotFound, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Create_EmptyTitle(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationCreate,
		Payload:         map[string]any{"title": ""},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrTitleEmpty {
		t.Fatalf("expected ErrTitleEmpty, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Create_InvalidPriority(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationCreate,
		Payload:         map[string]any{"title": "Test", "priority": "invalid"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidPriority {
		t.Fatalf("expected ErrInvalidPriority, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Create_InvalidTimezone(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationCreate,
		Payload:         map[string]any{"title": "Test", "timeZone": "Invalid/Timezone"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidTimezone {
		t.Fatalf("expected ErrInvalidTimezone, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Create_InvalidURL(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationCreate,
		Payload:         map[string]any{"title": "Test", "url": "javascript:alert(1)"},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrInvalidURL {
		t.Fatalf("expected ErrInvalidURL, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Update_MissingReminderID(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationUpdate,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrReminderNotFound {
		t.Fatalf("expected ErrReminderNotFound, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Complete_MissingReminderID(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationComplete,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrReminderNotFound {
		t.Fatalf("expected ErrReminderNotFound, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Uncomplete_MissingReminderID(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationUncomplete,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrReminderNotFound {
		t.Fatalf("expected ErrReminderNotFound, got %+v", resp.Error)
	}
}

func TestRemindersHandler_Delete_MissingReminderID(t *testing.T) {
	bridge := &fakeRemindersBridge{}
	h := NewRemindersHandler(bridge)

	resp := h.Execute(context.Background(), nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-1",
		Platform:        "ios",
		Operation:       OperationDelete,
		Payload:         map[string]any{},
	})

	if resp.Status != "error" {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
	if resp.Error == nil || resp.Error.Code != ErrReminderNotFound {
		t.Fatalf("expected ErrReminderNotFound, got %+v", resp.Error)
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

func TestCanReadReminders(t *testing.T) {
	tests := []struct {
		level    AuthorizationLevel
		expected bool
	}{
		{AuthorizationFullAccess, true},
		{AuthorizationLegacyAuthorized, true},
		{AuthorizationNotDetermined, false},
		{AuthorizationDenied, false},
		{AuthorizationRestricted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := CanReadReminders(tt.level)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCanCreateReminders(t *testing.T) {
	tests := []struct {
		level    AuthorizationLevel
		expected bool
	}{
		{AuthorizationFullAccess, true},
		{AuthorizationLegacyAuthorized, true},
		{AuthorizationNotDetermined, false},
		{AuthorizationDenied, false},
		{AuthorizationRestricted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := CanCreateReminders(tt.level)
			if result != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPriorityFromNative(t *testing.T) {
	tests := []struct {
		native   Priority
		expected Priority
		ok       bool
	}{
		{PriorityNone, PriorityNone, true},
		{PriorityLow, PriorityLow, true},
		{PriorityMedium, PriorityMedium, true},
		{PriorityHigh, PriorityHigh, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.native), func(t *testing.T) {
			result, ok := PriorityFromNative(tt.native)
			if ok != tt.ok {
				t.Fatalf("expected ok=%v, got %v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestPriorityToNative(t *testing.T) {
	tests := []struct {
		priority Priority
		expected int
	}{
		{PriorityNone, 0},
		{PriorityLow, 1},
		{PriorityMedium, 5},
		{PriorityHigh, 9},
	}

	for _, tt := range tests {
		t.Run(string(tt.priority), func(t *testing.T) {
			result := PriorityToNative(tt.priority)
			if result != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIsValidAuthorizationLevelForReminders(t *testing.T) {
	if IsValidAuthorizationLevelForReminders(AuthorizationNotDetermined) {
		t.Fatal("not_determined should not be valid for reminders")
	}
	if IsValidAuthorizationLevelForReminders(AuthorizationDenied) {
		t.Fatal("denied should not be valid for reminders")
	}
	if !IsValidAuthorizationLevelForReminders(AuthorizationFullAccess) {
		t.Fatal("full_access should be valid for reminders")
	}
	if !IsValidAuthorizationLevelForReminders(AuthorizationLegacyAuthorized) {
		t.Fatal("legacy_authorized should be valid for reminders")
	}
}

func TestIsValidQueryStatus(t *testing.T) {
	validStatuses := []string{"all", "incomplete", "completed"}
	for _, status := range validStatuses {
		if !IsValidQueryStatus(status) {
			t.Fatalf("expected %s to be valid", status)
		}
	}
	if IsValidQueryStatus("invalid") {
		t.Fatal("invalid status should not be valid")
	}
}

func TestIsValidPriority(t *testing.T) {
	validPriorities := []string{"none", "low", "medium", "high"}
	for _, p := range validPriorities {
		if !IsValidPriority(p) {
			t.Fatalf("expected %s to be valid", p)
		}
	}
	if IsValidPriority("urgent") {
		t.Fatal("urgent should not be valid")
	}
}

func TestGetDefaultSort(t *testing.T) {
	incompleteSort := GetDefaultSort(QueryStatusIncomplete)
	if len(incompleteSort) == 0 {
		t.Fatal("expected incomplete sort orders")
	}
	if incompleteSort[0].Field != "dueDate" {
		t.Fatalf("expected first sort field to be dueDate, got %s", incompleteSort[0].Field)
	}

	completedSort := GetDefaultSort(QueryStatusCompleted)
	if len(completedSort) == 0 {
		t.Fatal("expected completed sort orders")
	}
	if completedSort[0].Field != "completionDate" {
		t.Fatalf("expected first sort field to be completionDate, got %s", completedSort[0].Field)
	}
}

func TestPatchFieldSet(t *testing.T) {
	value := "test"
	patch := PatchField[string]{
		Set:   true,
		Value: &value,
	}
	if !patch.Set {
		t.Fatal("expected Set to be true")
	}
	if patch.Value == nil || *patch.Value != "test" {
		t.Fatal("expected Value to be 'test'")
	}
}

func TestPatchFieldClear(t *testing.T) {
	patch := PatchField[string]{
		Set:   true,
		Value: nil,
	}
	if !patch.Set {
		t.Fatal("expected Set to be true")
	}
	if patch.Value != nil {
		t.Fatal("expected Value to be nil (clear)")
	}
}

func TestPatchFieldUnmodified(t *testing.T) {
	patch := PatchField[string]{
		Set: false,
	}
	if patch.Set {
		t.Fatal("expected Set to be false (unmodified)")
	}
}
