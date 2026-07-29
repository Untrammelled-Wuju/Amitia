package wasm_runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type mockHostGateway struct {
	mu         sync.Mutex
	sessions   map[string]*host_api.Session
	callCount  int
	callOutput json.RawMessage
	callError  *host_api.Error
	routes     map[host_api.Method]host_api.Route
}

func newMockHostGateway() *mockHostGateway {
	return &mockHostGateway{
		sessions:   make(map[string]*host_api.Session),
		callOutput: json.RawMessage(`{"ok":true}`),
		routes:     make(map[host_api.Method]host_api.Route),
	}
}

func (m *mockHostGateway) RegisterRoute(route host_api.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes[route.Method] = route
	return nil
}

func (m *mockHostGateway) OpenSession(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, allowedVersions map[host_api.Method]int) (host_api.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sessionID := "session-" + identity.InstanceID
	session := host_api.Session{
		SessionID:       sessionID,
		RuntimeIdentity: identity,
		Generation:      identity.Generation,
		AllowedVersions: allowedVersions,
		CreatedAt:       time.Now().UTC(),
		Active:          true,
	}
	m.sessions[sessionID] = &session
	return session, nil
}

func (m *mockHostGateway) CloseSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.Active = false
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockHostGateway) Call(ctx context.Context, request host_api.CallRequest) host_api.CallResult {
	m.mu.Lock()
	m.callCount++
	output := m.callOutput
	callErr := m.callError
	m.mu.Unlock()
	if callErr != nil {
		return host_api.CallResult{
			Status: host_api.StatusFailed,
			Error:  callErr,
		}
	}
	return host_api.CallResult{
		Status: host_api.StatusSuccess,
		Output: output,
	}
}

func (m *mockHostGateway) QueryCapability(ctx context.Context, method host_api.Method) (host_api.Route, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.routes[method]
	return r, ok
}

func (m *mockHostGateway) ListMethods(ctx context.Context) []host_api.Method {
	m.mu.Lock()
	defer m.mu.Unlock()
	methods := make([]host_api.Method, 0, len(m.routes))
	for method := range m.routes {
		methods = append(methods, method)
	}
	return methods
}

func TestHostImportRegistry(t *testing.T) {
	r := NewHostImportRegistry()
	called := false
	r.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		called = true
		return json.RawMessage(`{}`), nil
	})
	h, ok := r.Lookup(ImportLog)
	if !ok {
		t.Fatalf("lookup failed for ImportLog")
	}
	_, _ = h(context.Background(), HostCallContext{}, nil)
	if !called {
		t.Fatalf("handler not called")
	}
	_, ok = r.Lookup(ImportTime)
	if ok {
		t.Fatalf("should not find unregistered import")
	}
}

func TestHostImportRegistryAllowed(t *testing.T) {
	r := NewHostImportRegistry()
	allowed := []HostImportName{ImportLog, ImportTime}
	if !r.Allowed(allowed, ImportLog) {
		t.Fatalf("log should be allowed")
	}
	if r.Allowed(allowed, ImportStorageGet) {
		t.Fatalf("storage should not be allowed")
	}
	if !r.Allowed(allowed, ImportTime) {
		t.Fatalf("time should be allowed")
	}
}

func TestHostImportRegistryOverwrite(t *testing.T) {
	r := NewHostImportRegistry()
	callCount := 0
	r.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		callCount++
		return json.RawMessage(`{"v":1}`), nil
	})
	r.Register(ImportLog, func(ctx context.Context, hctx HostCallContext, params json.RawMessage) (json.RawMessage, error) {
		callCount += 10
		return json.RawMessage(`{"v":2}`), nil
	})
	h, ok := r.Lookup(ImportLog)
	if !ok {
		t.Fatalf("lookup failed")
	}
	result, err := h(context.Background(), HostCallContext{}, nil)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if callCount != 10 {
		t.Fatalf("expected 10 (overwritten), got %d", callCount)
	}
	var obj map[string]any
	json.Unmarshal(result, &obj)
	if obj["v"].(float64) != 2 {
		t.Fatalf("expected v=2 from overwritten handler")
	}
}

func TestHostFunctionSet(t *testing.T) {
	logs := make([]string, 0)
	hfs := NewHostFunctionSet(HostFunctionConfig{
		Logger: func(level, msg string, fields map[string]any) {
			logs = append(logs, level+":"+msg)
		},
	})
	registry := hfs.Registry()
	h, ok := registry.Lookup(ImportLog)
	if !ok {
		t.Fatalf("log import not registered")
	}
	_, err := h(context.Background(), HostCallContext{
		ModuleID:    "mod-1",
		ExtensionID: "ext-1",
	}, json.RawMessage(`{"level":"info","message":"hello"}`))
	if err != nil {
		t.Fatalf("log call: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected log to be recorded")
	}
}

func TestHostFunctionSet_Time(t *testing.T) {
	hfs := NewHostFunctionSet(HostFunctionConfig{})
	h, ok := hfs.Registry().Lookup(ImportTime)
	if !ok {
		t.Fatalf("time import not registered")
	}
	result, err := h(context.Background(), HostCallContext{}, nil)
	if err != nil {
		t.Fatalf("time call: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("result not valid json: %v", err)
	}
	if obj["unix"] == nil {
		t.Fatalf("expected unix field")
	}
	if obj["iso8601"] == nil {
		t.Fatalf("expected iso8601 field")
	}
}

func TestHostFunctionSet_Random(t *testing.T) {
	hfs := NewHostFunctionSet(HostFunctionConfig{})
	h, ok := hfs.Registry().Lookup(ImportRandom)
	if !ok {
		t.Fatalf("random import not registered")
	}
	result, err := h(context.Background(), HostCallContext{}, json.RawMessage(`{"min":0,"max":100}`))
	if err != nil {
		t.Fatalf("random call: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("result not valid json: %v", err)
	}
	if obj["value"] == nil {
		t.Fatalf("expected value field")
	}
}

func TestHostFunctionSet_StorageGet_NotConfigured(t *testing.T) {
	hfs := NewHostFunctionSet(HostFunctionConfig{})
	h, ok := hfs.Registry().Lookup(ImportStorageGet)
	if !ok {
		t.Fatalf("storage_get import not registered")
	}
	_, err := h(context.Background(), HostCallContext{}, json.RawMessage(`{"key":"test"}`))
	if err == nil {
		t.Fatalf("expected error for unconfigured storage")
	}
}

func TestHostFunctionSet_StorageGet(t *testing.T) {
	hfs := NewHostFunctionSet(HostFunctionConfig{
		Storage: &mockStorageBroker{
			data: map[string][]byte{"test": []byte("value")},
		},
	})
	h, ok := hfs.Registry().Lookup(ImportStorageGet)
	if !ok {
		t.Fatalf("storage_get import not registered")
	}
	result, err := h(context.Background(), HostCallContext{
		ExtensionID: "ext-1",
	}, json.RawMessage(`{"key":"test"}`))
	if err != nil {
		t.Fatalf("storage get: %v", err)
	}
	var obj map[string]any
	json.Unmarshal(result, &obj)
	if obj["found"] != true {
		t.Fatalf("expected found=true")
	}
}

func TestHostFunctionSet_ResultSetError(t *testing.T) {
	hfs := NewHostFunctionSet(HostFunctionConfig{})
	h, ok := hfs.Registry().Lookup(ImportResultSetError)
	if !ok {
		t.Fatalf("result_set_error import not registered")
	}
	result, err := h(context.Background(), HostCallContext{}, json.RawMessage(`{"code":"test_error","message":"test"}`))
	if err != nil {
		t.Fatalf("set error: %v", err)
	}
	var obj map[string]any
	json.Unmarshal(result, &obj)
	if obj["set"] != true {
		t.Fatalf("expected set=true")
	}
}

func TestHostGateway_NilGateway(t *testing.T) {
	gw := NewHostGateway(nil)
	_, err := gw.OpenSession(context.Background(), runtime_supervisor.RuntimeIdentity{}, 10)
	if err == nil {
		t.Fatalf("expected error for nil gateway open session")
	}
	_, err = gw.Call(context.Background(), "session-1", host_api.MethodStateGet, json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for nil gateway call")
	}
}

func TestHostGateway(t *testing.T) {
	mockGW := newMockHostGateway()
	gw := NewHostGateway(mockGW)
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID:  "inst-1",
		ExtensionID: "ext-1",
		ModuleID:    "mod-1",
		RuntimeType: "wasm",
		Generation:  1,
	}
	sessionID, err := gw.OpenSession(context.Background(), identity, 10)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	if sessionID == "" {
		t.Fatalf("expected non-empty session id")
	}
	output, err := gw.Call(context.Background(), sessionID, host_api.MethodStateGet, json.RawMessage(`{"key":"test"}`))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if string(output) != `{"ok":true}` {
		t.Fatalf("expected {\"ok\":true}, got %s", string(output))
	}
	if mockGW.callCount != 1 {
		t.Fatalf("expected 1 call, got %d", mockGW.callCount)
	}
}

func TestHostGateway_CallLimit(t *testing.T) {
	mockGW := newMockHostGateway()
	gw := NewHostGateway(mockGW)
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID: "inst-limit",
	}
	sessionID, err := gw.OpenSession(context.Background(), identity, 2)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	_, err = gw.Call(context.Background(), sessionID, host_api.MethodStateGet, nil)
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	_, err = gw.Call(context.Background(), sessionID, host_api.MethodStateGet, nil)
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	_, err = gw.Call(context.Background(), sessionID, host_api.MethodStateGet, nil)
	if err == nil {
		t.Fatalf("expected error for exceeding call limit")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeHostCallLimit {
		t.Fatalf("expected %s, got %s", ErrCodeHostCallLimit, werr.Code)
	}
}

func TestHostGateway_CallSessionNotFound(t *testing.T) {
	mockGW := newMockHostGateway()
	gw := NewHostGateway(mockGW)
	_, err := gw.Call(context.Background(), "non-existent-session", host_api.MethodStateGet, nil)
	if err == nil {
		t.Fatalf("expected error for non-existent session")
	}
	werr, ok := err.(*WASMError)
	if !ok {
		t.Fatalf("expected WASMError, got %T", err)
	}
	if werr.Code != ErrCodeHostCallFailed {
		t.Fatalf("expected %s, got %s", ErrCodeHostCallFailed, werr.Code)
	}
}

func TestHostGateway_CallError(t *testing.T) {
	mockGW := newMockHostGateway()
	mockGW.callError = &host_api.Error{Code: "internal", Message: "something went wrong"}
	gw := NewHostGateway(mockGW)
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID: "inst-err",
	}
	sessionID, err := gw.OpenSession(context.Background(), identity, 10)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	_, err = gw.Call(context.Background(), sessionID, host_api.MethodStateGet, nil)
	if err == nil {
		t.Fatalf("expected error for call with error result")
	}
}

func TestHostGateway_CloseSession(t *testing.T) {
	mockGW := newMockHostGateway()
	gw := NewHostGateway(mockGW)
	identity := runtime_supervisor.RuntimeIdentity{
		InstanceID: "inst-close",
	}
	sessionID, err := gw.OpenSession(context.Background(), identity, 10)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	if err := gw.CloseSession(context.Background(), sessionID); err != nil {
		t.Fatalf("close session: %v", err)
	}
	_, err = gw.Call(context.Background(), sessionID, host_api.MethodStateGet, nil)
	if err == nil {
		t.Fatalf("expected error after session closed")
	}
}

func TestHostGateway_CloseSession_NilGateway(t *testing.T) {
	gw := NewHostGateway(nil)
	if err := gw.CloseSession(context.Background(), "session-1"); err != nil {
		t.Fatalf("close session with nil gateway should return nil, got: %v", err)
	}
}

func TestHostGateway_SetHostFunctionSet(t *testing.T) {
	gw := NewHostGateway(newMockHostGateway())
	newSet := NewHostFunctionSet(HostFunctionConfig{})
	gw.SetHostFunctionSet(newSet)
}

type mockStorageBroker struct {
	data map[string][]byte
}

func (m *mockStorageBroker) Get(ctx context.Context, extensionID, key string) ([]byte, error) {
	if v, ok := m.data[key]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *mockStorageBroker) CAS(ctx context.Context, extensionID, key string, oldVal, newVal []byte) (bool, error) {
	current, ok := m.data[key]
	if !ok {
		if len(oldVal) == 0 {
			m.data[key] = newVal
			return true, nil
		}
		return false, nil
	}
	if string(current) == string(oldVal) {
		m.data[key] = newVal
		return true, nil
	}
	return false, nil
}
