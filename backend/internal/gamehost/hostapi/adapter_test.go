package hostapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type fakeIdentityMapper struct {
	identity runtime_supervisor.RuntimeIdentity
	err      error
}

func (f *fakeIdentityMapper) MapIdentity(ctx context.Context, peer Peer) (runtime_supervisor.RuntimeIdentity, error) {
	return f.identity, f.err
}

type fakeGateway struct {
	calls  int
	result host_api.CallResult
}

func (f *fakeGateway) RegisterRoute(route host_api.Route) error { return nil }
func (f *fakeGateway) OpenSession(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, allowedVersions map[host_api.Method]int) (host_api.Session, error) {
	return host_api.Session{}, nil
}
func (f *fakeGateway) CloseSession(ctx context.Context, sessionID string) error { return nil }
func (f *fakeGateway) Call(ctx context.Context, request host_api.CallRequest) host_api.CallResult {
	f.calls++
	return f.result
}
func (f *fakeGateway) QueryCapability(ctx context.Context, method host_api.Method) (host_api.Route, bool) {
	return host_api.Route{}, false
}
func (f *fakeGateway) ListMethods(ctx context.Context) []host_api.Method { return nil }

type fakePermProvider struct{ snapID string }
type fakeScopeProvider struct{ snapID string }
type fakeReady struct{ ready bool }

func (f *fakeReady) IsReady(connKey string) bool { return f.ready }

func (f *fakePermProvider) CurrentSnapshotID(ctx context.Context, extensionID string, moduleID string, generation int64) (string, bool, error) {
	return f.snapID, f.snapID != "", nil
}

func (f *fakeScopeProvider) CurrentSnapshotID(ctx context.Context, extensionID string, moduleID string, generation int64) (string, bool, error) {
	return f.snapID, f.snapID != "", nil
}

func newTestAdapter(gw *fakeGateway, mapper *fakeIdentityMapper) *HostAPIAdapter {
	adapter, _ := NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            gw,
		Mapper:             mapper,
		PermissionProvider: &fakePermProvider{snapID: "psnap-test"},
		ScopeProvider:      &fakeScopeProvider{snapID: "ssnap-test"},
		ReadyVerifier:      &fakeReady{ready: true},
		IDGenerator:        DefaultIDGenerator(),
	})
	return adapter
}

func TestAdapter_NormalCall_Success(t *testing.T) {
	gw := &fakeGateway{result: host_api.CallResult{
		Status: host_api.StatusSuccess,
		Output: json.RawMessage(`{"ok":true}`),
	}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: "test.example/app",
		ModuleID:    "core",
		RuntimeType: "service",
	}}
	adapter := newTestAdapter(gw, mapper)

	resp, err := adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.memory.query",
		Input: json.RawMessage(`{"query":"hello"}`),
	})

	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp.Status != host_api.StatusSuccess {
		t.Fatalf("expected status=success, got %s", resp.Status)
	}
	if gw.calls != 1 {
		t.Fatalf("expected gateway called once, got %d", gw.calls)
	}
}

func TestAdapter_RejectsEmptyRoute(t *testing.T) {
	adapter := newTestAdapter(&fakeGateway{}, &fakeIdentityMapper{})
	_, err := adapter.Call(context.Background(), Request{Route: ""})
	if err == nil {
		t.Fatalf("expected error for empty route")
	}
}

func TestAdapter_RejectsNotReady(t *testing.T) {
	gw := &fakeGateway{}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{}}
	adapter, _ := NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            gw,
		Mapper:             mapper,
		PermissionProvider: &fakePermProvider{},
		ScopeProvider:      &fakeScopeProvider{},
		ReadyVerifier:      &fakeReady{ready: false},
		IDGenerator:        DefaultIDGenerator(),
	})
	_, err := adapter.Call(context.Background(), Request{
		Peer:    ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route:   "host.runtime.health",
		ConnKey: "conn-1",
	})
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
	if gw.calls != 0 {
		t.Fatalf("gateway should not be called when not ready, got %d", gw.calls)
	}
}

func TestAdapter_RejectsInvalidPeer(t *testing.T) {
	gw := &fakeGateway{}
	mapper := &fakeIdentityMapper{err: errors.New("peer not found")}
	adapter := newTestAdapter(gw, mapper)

	_, err := adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "unknown", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.runtime.health",
	})
	if err == nil {
		t.Fatalf("expected error for invalid peer")
	}
	if gw.calls != 0 {
		t.Fatalf("gateway should not be called when identity mapping fails, got %d", gw.calls)
	}
}

func TestAdapter_GatewayErrorMapping(t *testing.T) {
	gw := &fakeGateway{result: host_api.CallResult{
		Status: host_api.StatusRejected,
		Error:  &host_api.Error{Code: host_api.ErrorCodePermissionDenied, Message: "no permission"},
	}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{}}
	adapter := newTestAdapter(gw, mapper)

	resp, err := adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.secret.get",
	})
	if err == nil {
		t.Fatalf("expected error from gateway permission denied")
	}
	if resp.Status != host_api.StatusRejected {
		t.Fatalf("expected rejected status, got %s", resp.Status)
	}
	hostErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error type, got %T", err)
	}
	if hostErr.Code != CodePermissionDenied {
		t.Fatalf("expected permission_denied code, got %s", hostErr.Code)
	}
}

func TestAdapter_SetsSnapshotIDs(t *testing.T) {
	gw := &fakeGateway{result: host_api.CallResult{
		Status: host_api.StatusSuccess,
		Output: json.RawMessage(`{}`),
	}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{
		InstanceID:  "runtime-1",
		ExtensionID: "test.example/app",
		ModuleID:    "core",
	}}
	snapGateway := &snapshotCapturingGateway{fakeGateway: gw}
	adapter, _ := NewHostAPIAdapter(HostAPIAdapterConfig{
		Gateway:            snapGateway,
		Mapper:             mapper,
		PermissionProvider: &fakePermProvider{snapID: "psnap-x"},
		ScopeProvider:      &fakeScopeProvider{snapID: "ssnap-x"},
		ReadyVerifier:      &fakeReady{ready: true},
		IDGenerator:        DefaultIDGenerator(),
	})

	adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.runtime.health",
	})

	if snapGateway.lastReq.PermissionSnapshotID != "psnap-x" {
		t.Fatalf("expected permission snapshot psnap-x, got %s", snapGateway.lastReq.PermissionSnapshotID)
	}
	if snapGateway.lastReq.ScopeSnapshotID != "ssnap-x" {
		t.Fatalf("expected scope snapshot ssnap-x, got %s", snapGateway.lastReq.ScopeSnapshotID)
	}
}

type snapshotCapturingGateway struct {
	*fakeGateway
	lastReq host_api.CallRequest
}

func (s *snapshotCapturingGateway) Call(ctx context.Context, request host_api.CallRequest) host_api.CallResult {
	s.lastReq = request
	return s.fakeGateway.Call(ctx, request)
}

func TestAdapter_PayloadOmitted(t *testing.T) {
	original := json.RawMessage(`{"nested":{"value":9007199254740993}}`)
	gw := &fakeGateway{result: host_api.CallResult{
		Status: host_api.StatusSuccess,
		Output: json.RawMessage(`{}`),
	}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{}}
	adapter := newTestAdapter(gw, mapper)

	adapter.Call(context.Background(), Request{
		Peer:  ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route: "host.memory.query",
		Input: original,
	})

	original[2] = 'X'
	if gw.calls == 0 {
		t.Fatalf("expected gateway call")
	}
}

func TestAdapter_PreservesDeadline(t *testing.T) {
	gw := &fakeGateway{result: host_api.CallResult{Status: host_api.StatusSuccess, Output: json.RawMessage(`{}`)}}
	mapper := &fakeIdentityMapper{identity: runtime_supervisor.RuntimeIdentity{}}
	adapter := newTestAdapter(gw, mapper)

	deadline := time.Now().Add(5 * time.Second)
	adapter.Call(context.Background(), Request{
		Peer:     ipc.Peer{PluginID: "p1", RuntimeID: "r1", ServiceID: "s1"},
		Route:    "host.runtime.health",
		Deadline: deadline,
	})

	if gw.calls == 0 {
		t.Fatalf("expected gateway call")
	}
}
