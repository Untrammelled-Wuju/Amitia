package repair_baseline

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

func requireRoute(t *testing.T, g *host_api.DefaultGateway, method host_api.Method) host_api.Route {
	t.Helper()
	route := host_api.Route{
		Method:      method,
		Version:     1,
		Permission:  []host_api.PermissionRequirement{{Name: "storage.state.read", Resource: "state"}},
		ScopePolicy: host_api.ScopePolicy{RequireRoles: []string{"assistant"}},
		Handler: func(ctx context.Context, req host_api.CallRequest) (host_api.CallResult, error) {
			return host_api.CallResult{Status: host_api.StatusSuccess}, nil
		},
	}
	if err := g.RegisterRoute(route); err != nil {
		t.Fatalf("register route %s: %v", method, err)
	}
	return route
}

func identity() runtime_supervisor.RuntimeIdentity {
	return runtime_supervisor.RuntimeIdentity{
		InstanceID:  "inst-baseline-1",
		ExtensionID: "com.amitia.baseline/repair",
		ModuleID:    "main",
		Generation:  1,
	}
}

func TestBaseline_HostAPI_NoPermissionCheckerMustReject(t *testing.T) {
	gw := host_api.NewDefaultGateway()
	requireRoute(t, gw, host_api.MethodStateGet)

	res := gw.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-baseline-1",
		RuntimeIdentity: identity(),
		Method:          host_api.MethodStateGet,
		Version:         1,
	})

	if res.Status != host_api.StatusRejected {
		t.Fatalf("expected status=%s when no permission checker is wired, got status=%s err=%+v", host_api.StatusRejected, res.Status, res.Error)
	}
	if res.Error == nil || res.Error.Code != host_api.ErrorCodePermissionDenied {
		t.Fatalf("expected error code=%s, got %+v", host_api.ErrorCodePermissionDenied, res.Error)
	}
}

func TestBaseline_HostAPI_PermissionDeniedMustReject(t *testing.T) {
	gw := host_api.NewDefaultGateway()
	requireRoute(t, gw, host_api.MethodStateGet)

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, reqs []host_api.PermissionRequirement) error {
		return host_api.ErrPermissionDenied
	}))

	res := gw.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-baseline-2",
		RuntimeIdentity: identity(),
		Method:          host_api.MethodStateGet,
		Version:         1,
	})

	if res.Status != host_api.StatusRejected || res.Error == nil || res.Error.Code != host_api.ErrorCodePermissionDenied {
		t.Fatalf("expected permission_denied rejection, got status=%s err=%+v", res.Status, res.Error)
	}
}

func TestBaseline_HostAPI_ScopeDeniedMustReject(t *testing.T) {
	gw := host_api.NewDefaultGateway()
	requireRoute(t, gw, host_api.MethodStateGet)

	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, reqs []host_api.PermissionRequirement) error {
		return nil
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, pol host_api.ScopePolicy) error {
		return host_api.ErrScopeDenied
	}))

	res := gw.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-baseline-3",
		RuntimeIdentity: identity(),
		Method:          host_api.MethodStateGet,
		Version:         1,
		ScopeSnapshotID: "snap-missing",
	})

	if res.Status != host_api.StatusRejected || res.Error == nil || res.Error.Code != host_api.ErrorCodeScopeDenied {
		t.Fatalf("expected scope_denied rejection, got status=%s err=%+v", res.Status, res.Error)
	}
}

func TestBaseline_HostAPI_EmptyScopeSnapshotMustReject(t *testing.T) {
	gw := host_api.NewDefaultGateway()
	requireRoute(t, gw, host_api.MethodStateGet)

	called := false
	gw.SetPermissionChecker(host_api.PermissionCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, reqs []host_api.PermissionRequirement) error {
		return nil
	}))
	gw.SetScopeChecker(host_api.ScopeCheckerFunc(func(ctx context.Context, id runtime_supervisor.RuntimeIdentity, sid string, pol host_api.ScopePolicy) error {
		called = true
		if sid == "" {
			return host_api.ErrScopeDenied
		}
		return nil
	}))

	res := gw.Call(context.Background(), host_api.CallRequest{
		CallID:          "call-baseline-4",
		RuntimeIdentity: identity(),
		Method:          host_api.MethodStateGet,
		Version:         1,
	})

	if !called {
		t.Fatalf("scope checker must be invoked even when ScopeSnapshotID is empty")
	}
	if res.Status != host_api.StatusRejected || res.Error == nil || res.Error.Code != host_api.ErrorCodeScopeDenied {
		t.Fatalf("expected scope_denied for empty snapshot, got status=%s err=%+v", res.Status, res.Error)
	}
}
