package contacts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/nativebridge"
)

type mockContactsBridge struct {
	response nativebridge.Response
	err      error
	calls    []nativebridge.Request
	delay    time.Duration
}

func (m *mockContactsBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
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

func (m *mockContactsBridge) Health(context.Context) nativebridge.Health {
	return ""
}

func newMockBridge(resp nativebridge.Response, err error) *mockContactsBridge {
	return &mockContactsBridge{response: resp, err: err}
}

func baseRequest(operation string) nativebridge.Request {
	return nativebridge.Request{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Platform:        "ios",
		Operation:       operation,
		Payload:         map[string]any{},
	}
}

func TestNewContactsHandler(t *testing.T) {
	bridge := newMockBridge(nativebridge.Response{}, nil)
	h := NewContactsHandler(bridge)
	if h == nil {
		t.Fatal("NewContactsHandler returned nil")
	}
	if h.bridge != bridge {
		t.Error("bridge not set correctly")
	}
}

func TestHandler_Execute_UnknownOperation(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest("contacts.unknown")
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

func TestHandler_Status_BridgeUnavailable(t *testing.T) {
	h := NewContactsHandler(nil)
	req := baseRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)
	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrNativeBridgeUnavailable {
		t.Errorf("expected ErrNativeBridgeUnavailable, got %s", resp.Error.Code)
	}
}

func TestHandler_AuthorizationStatus(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"level": "authorized"},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationAuthorizationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
	if bridge.calls[0].Operation != OperationAuthorizationStatus {
		t.Errorf("expected operation %s, got %s", OperationAuthorizationStatus, bridge.calls[0].Operation)
	}
}

func TestHandler_AuthorizationRequest(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"granted": true},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationAuthorizationRequest)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if resp.Result["granted"] != true {
		t.Errorf("expected granted=true")
	}
}

func TestHandler_Search_MissingQuery(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationSearch)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %s", resp.Error.Code)
	}
}

func TestHandler_Search_InvalidField(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationSearch)
	req.Payload["query"] = "test"
	req.Payload["field"] = "invalid_field"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidSearchField {
		t.Errorf("expected ErrInvalidSearchField, got %s", resp.Error.Code)
	}
}

func TestHandler_Search_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"count": 5},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationSearch)
	req.Payload["query"] = "John"
	req.Payload["limit"] = float64(10)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if len(bridge.calls) != 1 {
		t.Errorf("expected 1 bridge call, got %d", len(bridge.calls))
	}
	sentReq := bridge.calls[0]
	if sentReq.Payload["query"] != "John" {
		t.Errorf("expected query=John, got %v", sentReq.Payload["query"])
	}
	if sentReq.Payload["limit"] != 10 {
		t.Errorf("expected limit=10, got %v", sentReq.Payload["limit"])
	}
}

func TestHandler_List_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"count": 50},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationList)
	req.Payload["limit"] = float64(30)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["limit"] != 30 {
		t.Errorf("expected limit=30, got %v", bridge.calls[0].Payload["limit"])
	}
}

func TestHandler_Get_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_Get_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"contactId": "c-001"},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationGet)
	req.Payload["contactId"] = "c-001"
	req.Payload["includePhoto"] = true
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["contactId"] != "c-001" {
		t.Errorf("expected contactId=c-001")
	}
	if bridge.calls[0].Payload["includePhoto"] != true {
		t.Errorf("expected includePhoto=true")
	}
}

func TestHandler_Create_MissingIdentity(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationCreate)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %s", resp.Error.Code)
	}
}

func TestHandler_Create_WithPhone(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"contactId": "new-001"},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationCreate)
	req.Payload["organization"] = "Acme Inc."
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Update_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationUpdate)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_Update_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"contactId": "c-001"},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationUpdate)
	req.Payload["contactId"] = "c-001"
	req.Payload["organization"] = "New Corp"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_Delete_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationDelete)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_Delete_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"deleted": true},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationDelete)
	req.Payload["contactId"] = "c-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContainersList_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"count": 3},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationContainersList)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Operation != OperationContainersList {
		t.Errorf("expected operation %s", OperationContainersList)
	}
}

func TestHandler_GroupsList_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"count": 5},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationGroupsList)
	req.Payload["containerId"] = "cont-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["containerId"] != "cont-001" {
		t.Errorf("expected containerId=cont-001")
	}
}

func TestHandler_PhotoGet_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationPhotoGet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotoGet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"hasImage": true},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationPhotoGet)
	req.Payload["contactId"] = "c-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_PhotoSet_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationPhotoSet)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotoSet_MissingResourceURI(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationPhotoSet)
	req.Payload["contactId"] = "c-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrPhotoInvalid {
		t.Errorf("expected ErrPhotoInvalid, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotoSet_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"hasImage": true},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationPhotoSet)
	req.Payload["contactId"] = "c-001"
	req.Payload["resourceUri"] = "file:///tmp/photo.jpg"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
	if bridge.calls[0].Payload["resourceUri"] != "file:///tmp/photo.jpg" {
		t.Errorf("expected resourceUri")
	}
}

func TestHandler_PhotoRemove_MissingContactID(t *testing.T) {
	h := NewContactsHandler(newMockBridge(nativebridge.Response{}, nil))
	req := baseRequest(OperationPhotoRemove)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != ErrInvalidContactID {
		t.Errorf("expected ErrInvalidContactID, got %s", resp.Error.Code)
	}
}

func TestHandler_PhotoRemove_Success(t *testing.T) {
	expected := nativebridge.Response{
		ProtocolVersion: 1,
		RequestID:       "test-req-001",
		Status:          "ok",
		Result:          map[string]any{"hasImage": false},
	}
	bridge := newMockBridge(expected, nil)
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationPhotoRemove)
	req.Payload["contactId"] = "c-001"
	resp := h.Execute(context.Background(), req)

	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %s", resp.Status)
	}
}

func TestHandler_ContextCancel(t *testing.T) {
	bridge := &mockContactsBridge{
		delay: 5 * time.Second,
	}
	h := NewContactsHandler(bridge)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := baseRequest(OperationSearch)
	req.Payload["query"] = "test"
	resp := h.Execute(ctx, req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
	if resp.Error.Code != nativebridge.ErrBridgeTimeout {
		t.Errorf("expected ErrBridgeTimeout, got %s", resp.Error.Code)
	}
}

func TestHandler_BridgeError(t *testing.T) {
	bridge := newMockBridge(nativebridge.Response{}, errors.New("bridge connection failed"))
	h := NewContactsHandler(bridge)

	req := baseRequest(OperationStatus)
	resp := h.Execute(context.Background(), req)

	if resp.Status != "error" {
		t.Errorf("expected error status, got %s", resp.Status)
	}
}

func TestExtractContactNameInput(t *testing.T) {
	payload := map[string]any{
		"name": map[string]any{
			"given":  "John",
			"family": "Doe",
		},
	}
	name := extractContactNameInput(payload)
	if name.Given != "John" {
		t.Errorf("expected given=John, got %s", name.Given)
	}
	if name.Family != "Doe" {
		t.Errorf("expected family=Doe, got %s", name.Family)
	}
}

func TestHasAnyIdentity(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected bool
	}{
		{
			name:     "empty",
			payload:  map[string]any{},
			expected: false,
		},
		{
			name:     "with organization",
			payload:  map[string]any{"organization": "Acme"},
			expected: true,
		},
		{
			name:     "with phones",
			payload:  map[string]any{"phoneNumbers": []any{map[string]any{"value": "123"}}},
			expected: true,
		},
		{
			name:     "with emails",
			payload:  map[string]any{"emailAddresses": []any{map[string]any{"value": "a@b.c"}}},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := extractContactNameInput(tt.payload)
			result := hasAnyIdentity(tt.payload, name)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTruncateLabeledStrings(t *testing.T) {
	items := []any{1, 2, 3, 4, 5}
	result := truncateLabeledStrings(items, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}

	result2 := truncateLabeledStrings(items, 10)
	if len(result2) != 5 {
		t.Errorf("expected 5 items, got %d", len(result2))
	}
}

func TestAuthorizationLevelFromNative(t *testing.T) {
	tests := []struct {
		native   string
		expected AuthorizationLevel
	}{
		{"not_determined", AuthorizationNotDetermined},
		{"notDetermined", AuthorizationNotDetermined},
		{"restricted", AuthorizationRestricted},
		{"denied", AuthorizationDenied},
		{"authorized", AuthorizationAuthorized},
		{"limited", AuthorizationLimited},
		{"unknown", AuthorizationNotDetermined},
	}
	for _, tt := range tests {
		t.Run(tt.native, func(t *testing.T) {
			result := AuthorizationLevelFromNative(tt.native)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMapCapabilityState(t *testing.T) {
	state := MapCapabilityState(AuthorizationAuthorized, true)
	if !state.Supported {
		t.Error("expected supported=true")
	}
	if !state.CanRead || !state.CanCreate || !state.CanUpdate || !state.CanDelete {
		t.Error("expected all capabilities true for authorized")
	}
	if !state.CanReadNotes {
		t.Error("expected canReadNotes=true for authorized with entitlement")
	}
	if state.State != "authorized" {
		t.Errorf("expected state=authorized, got %s", state.State)
	}

	limited := MapCapabilityState(AuthorizationLimited, false)
	if !limited.Limited {
		t.Error("expected limited=true")
	}
	if limited.CanReadNotes {
		t.Error("expected canReadNotes=false for limited")
	}

	denied := MapCapabilityState(AuthorizationDenied, false)
	if denied.CanRead || denied.CanCreate {
		t.Error("expected no capabilities for denied")
	}
	if denied.State != "denied" {
		t.Errorf("expected state=denied, got %s", denied.State)
	}
}

func TestClampLimits(t *testing.T) {
	if ClampListLimit(0) != DefaultLimitList {
		t.Errorf("expected default list limit")
	}
	if ClampListLimit(999) != MaxLimitList {
		t.Errorf("expected max list limit")
	}
	if ClampListLimit(75) != 75 {
		t.Errorf("expected 75")
	}
	if ClampSearchLimit(0) != DefaultLimitSearch {
		t.Errorf("expected default search limit")
	}
	if ClampSearchLimit(999) != MaxLimitSearch {
		t.Errorf("expected max search limit")
	}
}

func TestContactNameInputIsEmpty(t *testing.T) {
	name := ContactNameInput{Given: "John"}
	if name.IsEmpty() {
		t.Error("expected non-empty")
	}
	empty := ContactNameInput{}
	if !empty.IsEmpty() {
		t.Error("expected empty")
	}
}

func TestCreateContactRequestHasMinimalIdentity(t *testing.T) {
	req := CreateContactRequest{
		Name: ContactNameInput{Given: "John"},
	}
	if !req.HasMinimalIdentity() {
		t.Error("expected minimal identity")
	}

	empty := CreateContactRequest{}
	if empty.HasMinimalIdentity() {
		t.Error("expected no minimal identity")
	}
}

func TestValidatePredicate(t *testing.T) {
	err := ValidatePredicate(ContactPredicate{
		Type:  PredicateTypeEqual,
		Field: "name",
		Value: "John",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = ValidatePredicate(ContactPredicate{
		Type:  PredicateTypeEqual,
		Value: "John",
	})
	if err == nil {
		t.Error("expected error for missing field")
	}

	err = ValidatePredicate(ContactPredicate{
		Type: PredicateTypeAnd,
		Value: []any{
			ContactPredicate{Type: PredicateTypeEqual, Field: "name", Value: "John"},
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
