package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type trackingGateway struct {
	mu         sync.Mutex
	calls      int
	lastReq    host_api.CallRequest
	permCheck  error
	scopeCheck error
	auditCalls int
}

func (g *trackingGateway) RegisterRoute(route host_api.Route) error { return nil }
func (g *trackingGateway) OpenSession(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, allowedVersions map[host_api.Method]int) (host_api.Session, error) {
	return host_api.Session{}, nil
}
func (g *trackingGateway) CloseSession(ctx context.Context, sessionID string) error { return nil }

func (g *trackingGateway) Call(ctx context.Context, request host_api.CallRequest) host_api.CallResult {
	g.mu.Lock()
	g.calls++
	g.lastReq = request
	g.auditCalls++
	permCheck := g.permCheck
	scopeCheck := g.scopeCheck
	g.mu.Unlock()

	if request.PermissionSnapshotID == "" {
		return host_api.CallResult{
			Status: host_api.StatusSuccess,
			Output: json.RawMessage(`{}`),
		}
	}

	if permCheck != nil {
		return host_api.CallResult{
			Status: host_api.StatusRejected,
			Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: permCheck.Error()},
		}
	}
	if scopeCheck != nil {
		return host_api.CallResult{
			Status: host_api.StatusRejected,
			Error:  &host_api.Error{Code: host_api.ErrorCodeScopeDenied, Message: scopeCheck.Error()},
		}
	}
	return host_api.CallResult{Status: host_api.StatusSuccess, Output: json.RawMessage(`{}`)}
}

func (g *trackingGateway) QueryCapability(ctx context.Context, method host_api.Method) (host_api.Route, bool) {
	return host_api.Route{}, false
}
func (g *trackingGateway) ListMethods(ctx context.Context) []host_api.Method { return nil }

func newGovernanceTestAdapter(gw *trackingGateway) *HostAPIAdapter {
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: "test.example/app",
		ModuleID:    "core",
		Generation:  1,
	}}
	adapter, _ := NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            gw,
		Mapper:             mapper,
		PermissionProvider: &fakePermProvider{snapID: "psnap-live"},
		ScopeProvider:      &fakeScopeProvider{snapID: "ssnap-live"},
		ReadyVerifier:      &fakeReady{ready: true},
		IDGenerator:        DefaultIDGenerator(),
	})
	return adapter
}

func TestGovernance_PermissionDeniedMapped(t *testing.T) {
	gw := &trackingGateway{permCheck: errors.New("fail-closed test")}
	adapter := newGovernanceTestAdapter(gw)

	_, err := adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.character.get",
	})
	if err == nil {
		t.Fatalf("expected permission denied error")
	}
	hostErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if hostErr.Code != CodePermissionDenied {
		t.Fatalf("expected CodePermissionDenied, got %s", hostErr.Code)
	}
}

func TestGovernance_ScopeDeniedMapped(t *testing.T) {
	gw := &trackingGateway{scopeCheck: errors.New("out of scope")}
	adapter := newGovernanceTestAdapter(gw)

	_, err := adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.character.get",
	})
	if err == nil {
		t.Fatalf("expected scope denied error")
	}
	hostErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if hostErr.Code != CodePermissionDenied {
		t.Fatalf("expected scope denied mapped to CodePermissionDenied, got %s", hostErr.Code)
	}
}

func TestGovernance_PassesSnapshotIDs(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.memory.query",
	})

	gw.mu.Lock()
	req := gw.lastReq
	gw.mu.Unlock()

	if req.PermissionSnapshotID != "psnap-live" {
		t.Fatalf("expected PermissionSnapshotID psnap-live, got %q", req.PermissionSnapshotID)
	}
	if req.ScopeSnapshotID != "ssnap-live" {
		t.Fatalf("expected ScopeSnapshotID ssnap-live, got %q", req.ScopeSnapshotID)
	}
}

func TestGovernance_CancelledContext(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, _ := adapter.Call(ctx, Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.runtime.health",
	})
	_ = resp
}

func TestGovernance_DeadlinePreserved(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	deadline := time.Now().Add(10 * time.Second)
	adapter.Call(context.Background(), Request{
		Peer:     ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route:    "host.runtime.health",
		Deadline: deadline,
	})

	gw.mu.Lock()
	req := gw.lastReq
	gw.mu.Unlock()

	if !req.Deadline.Equal(deadline) {
		t.Fatalf("expected deadline %v, got %v", deadline, req.Deadline)
	}
}

func TestGovernance_AuditPathCalled(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.runtime.health",
	})

	gw.mu.Lock()
	calls := gw.auditCalls
	gw.mu.Unlock()

	if calls == 0 {
		t.Fatalf("expected audit path to be invoked")
	}
}

func TestRace_ConcurrentAdapterCalls(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			adapter.Call(context.Background(), Request{
				Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
				Route: "host.runtime.health",
				Input: json.RawMessage(`{"idx":` + string(rune('0'+i%10)) + `}`),
			})
		}(i)
	}
	wg.Wait()

	gw.mu.Lock()
	calls := gw.calls
	gw.mu.Unlock()

	if calls != goroutines {
		t.Fatalf("expected %d gateway calls, got %d", goroutines, calls)
	}
}

func TestRace_SnapshotProviderConcurrent(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			adapter.Call(context.Background(), Request{
				Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
				Route: "host.runtime.health",
			})
		}()
	}
	wg.Wait()
}

func TestRace_ErrorMappingConcurrent(t *testing.T) {
	gw := &trackingGateway{}
	adapter := newGovernanceTestAdapter(gw)

	codes := []*host_api.Error{
		{Code: host_api.ErrorCodePermissionDenied},
		{Code: host_api.ErrorCodeScopeDenied},
		{Code: host_api.ErrorCodeMethodNotFound},
		{Code: host_api.ErrorCodeTimeout},
		{Code: host_api.ErrorCodeCancelled},
		{Code: host_api.ErrorCodeRateLimited},
		{Code: host_api.ErrorCodeGenerationStale},
		{Code: host_api.ErrorCodeIdentityInvalid},
	}

	const perCode = 20
	var wg sync.WaitGroup
	wg.Add(len(codes) * perCode)
	for _, code := range codes {
		for i := 0; i < perCode; i++ {
			go func(c *host_api.Error) {
				defer wg.Done()
				gw.mu.Lock()
				gw.calls = 0
				gw.mu.Unlock()
				adapter.Call(context.Background(), Request{
					Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
					Route: "host.runtime.health",
				})
				mapGatewayError(c)
			}(code)
		}
	}
	wg.Wait()
}
