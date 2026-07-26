package host_api

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func makeIdentity() runtime_supervisor.RuntimeIdentity {
	return runtime_supervisor.RuntimeIdentity{
		InstanceID:         "inst-1",
		RuntimeDefinitionID: "rt-1",
		ExtensionID:        "com.example/test",
		ModuleID:           "main",
		Generation:         1,
	}
}

func makeRequest(method Method, version int, input any) CallRequest {
	body, _ := json.Marshal(input)
	return CallRequest{
		CallID:          "call-1",
		RuntimeIdentity: makeIdentity(),
		Method:          method,
		Version:         version,
		Input:           body,
		TraceID:         "trace-1",
		InvocationID:    "inv-1",
		Deadline:        time.Now().Add(5 * time.Second),
	}
}

func TestRegisterAndCall(t *testing.T) {
	g := NewDefaultGateway()
	handler := func(_ context.Context, req CallRequest) (CallResult, error) {
		var in map[string]any
		_ = json.Unmarshal(req.Input, &in)
		out, _ := json.Marshal(map[string]any{"echo": in["value"]})
		return CallResult{Status: StatusSuccess, Output: out}, nil
	}
	if err := g.RegisterRoute(Route{
		Method:  MethodStateGet,
		Version: 1,
		Handler: handler,
	}); err != nil {
		t.Fatalf("RegisterRoute: %v", err)
	}
	result := g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{"value": "hello"}))
	if result.Status != StatusSuccess {
		t.Errorf("expected success, got %s: %v", result.Status, result.Error)
	}
	var out map[string]any
	_ = json.Unmarshal(result.Output, &out)
	if out["echo"] != "hello" {
		t.Errorf("expected echo=hello, got %v", out["echo"])
	}
}

func TestMethodNotFound(t *testing.T) {
	g := NewDefaultGateway()
	result := g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
	if result.Status != StatusFailed {
		t.Errorf("expected failed, got %s", result.Status)
	}
	if result.Error.Code != ErrorCodeMethodNotFound {
		t.Errorf("expected method_not_found, got %s", result.Error.Code)
	}
}

func TestRouteExists(t *testing.T) {
	g := NewDefaultGateway()
	route := Route{Method: MethodStateGet, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}}
	if err := g.RegisterRoute(route); err != nil {
		t.Fatal(err)
	}
	if err := g.RegisterRoute(route); err == nil {
		t.Errorf("expected route exists error")
	}
}

func TestSessionLifecycle(t *testing.T) {
	g := NewDefaultGateway()
	session, err := g.OpenSession(context.Background(), makeIdentity(), map[Method]int{MethodStateGet: 1})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	if session.SessionID == "" {
		t.Errorf("expected session id")
	}
	if err := g.CloseSession(context.Background(), session.SessionID); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if err := g.CloseSession(context.Background(), session.SessionID); err == nil {
		t.Errorf("expected session not found")
	}
}

func TestOpenSessionInvalidIdentity(t *testing.T) {
	g := NewDefaultGateway()
	_, err := g.OpenSession(context.Background(), runtime_supervisor.RuntimeIdentity{}, nil)
	if !errors.Is(err, ErrIdentityInvalid) {
		t.Errorf("expected identity invalid, got %v", err)
	}
}

func TestPermissionDenied(t *testing.T) {
	g := NewDefaultGateway()
	g.SetPermissionChecker(PermissionCheckerFunc(func(_ context.Context, _ runtime_supervisor.RuntimeIdentity, _ []PermissionRequirement) error {
		return ErrPermissionDenied
	}))
	_ = g.RegisterRoute(Route{
		Method:     MethodStateGet,
		Version:    1,
		Permission: []PermissionRequirement{{Name: "extension.storage.read"}},
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{Status: StatusSuccess}, nil
		},
	})
	result := g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
	if result.Status != StatusRejected {
		t.Errorf("expected rejected, got %s", result.Status)
	}
	if result.Error.Code != ErrorCodePermissionDenied {
		t.Errorf("expected permission_denied, got %s", result.Error.Code)
	}
}

func TestScopeDenied(t *testing.T) {
	g := NewDefaultGateway()
	g.SetScopeChecker(ScopeCheckerFunc(func(_ context.Context, _ runtime_supervisor.RuntimeIdentity, _ string, _ ScopePolicy) error {
		return ErrScopeDenied
	}))
	_ = g.RegisterRoute(Route{
		Method:      MethodStateGet,
		Version:     1,
		ScopePolicy: ScopePolicy{AllowNarrowing: true},
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{Status: StatusSuccess}, nil
		},
	})
	result := g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
	if result.Status != StatusRejected {
		t.Errorf("expected rejected, got %s", result.Status)
	}
	if result.Error.Code != ErrorCodeScopeDenied {
		t.Errorf("expected scope_denied, got %s", result.Error.Code)
	}
}

func TestTimeout(t *testing.T) {
	g := NewDefaultGateway()
	_ = g.RegisterRoute(Route{
		Method:  MethodStateGet,
		Version: 1,
		Handler: func(ctx context.Context, _ CallRequest) (CallResult, error) {
			select {
			case <-time.After(200 * time.Millisecond):
				return CallResult{Status: StatusSuccess}, nil
			case <-ctx.Done():
				return CallResult{}, ErrTimeout
			}
		},
		Timeout: 50 * time.Millisecond,
	})
	result := g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
	if result.Status != StatusTimeout {
		t.Errorf("expected timeout, got %s", result.Status)
	}
}

func TestDeadlineExceeded(t *testing.T) {
	g := NewDefaultGateway()
	_ = g.RegisterRoute(Route{
		Method:  MethodStateGet,
		Version: 1,
		Handler: func(context.Context, CallRequest) (CallResult, error) {
			return CallResult{Status: StatusSuccess}, nil
		},
	})
	req := makeRequest(MethodStateGet, 1, map[string]any{})
	req.Deadline = time.Now().Add(-1 * time.Second)
	result := g.Call(context.Background(), req)
	if result.Status != StatusTimeout {
		t.Errorf("expected timeout, got %s", result.Status)
	}
}

func TestQueryCapability(t *testing.T) {
	g := NewDefaultGateway()
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 2, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	route, ok := g.QueryCapability(context.Background(), MethodStateGet)
	if !ok {
		t.Fatalf("expected found")
	}
	if route.Version != 2 {
		t.Errorf("expected v2, got v%d", route.Version)
	}
	_, ok = g.QueryCapability(context.Background(), MethodSecretGet)
	if ok {
		t.Errorf("expected not found")
	}
}

func TestListMethods(t *testing.T) {
	g := NewDefaultGateway()
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	_ = g.RegisterRoute(Route{Method: MethodStateCAS, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	methods := g.ListMethods(context.Background())
	if len(methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(methods))
	}
}

func TestVersionFallback(t *testing.T) {
	g := NewDefaultGateway()
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 2, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	req := makeRequest(MethodStateGet, 0, map[string]any{})
	result := g.Call(context.Background(), req)
	if result.Status != StatusSuccess {
		t.Errorf("expected success with version fallback, got %s", result.Status)
	}
}

func TestAuditWriter(t *testing.T) {
	g := NewDefaultGateway()
	aw := &fakeAudit{}
	g.SetAuditWriter(aw)
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess}, nil
	}})
	g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
	if aw.calls != 1 {
		t.Errorf("expected 1 audit call, got %d", aw.calls)
	}
}

func TestConcurrentCalls(t *testing.T) {
	g := NewDefaultGateway()
	var counter int32
	var mu sync.Mutex
	_ = g.RegisterRoute(Route{Method: MethodStateGet, Version: 1, Handler: func(context.Context, CallRequest) (CallResult, error) {
		mu.Lock()
		counter++
		mu.Unlock()
		return CallResult{Status: StatusSuccess}, nil
	}})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Call(context.Background(), makeRequest(MethodStateGet, 1, map[string]any{}))
		}()
	}
	wg.Wait()
	if counter != 20 {
		t.Errorf("expected 20 calls, got %d", counter)
	}
}

type fakeAudit struct {
	calls int
	mu    sync.Mutex
}

func (a *fakeAudit) RecordCall(_ context.Context, _ CallRequest, _ CallResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
}
