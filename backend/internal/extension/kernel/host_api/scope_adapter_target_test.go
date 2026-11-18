package host_api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type stubScopeManager struct{}

func (stubScopeManager) Bind(context.Context, scope.ScopeBindRequest) (scope.ScopeBinding, error) {
	return scope.ScopeBinding{}, nil
}

func (stubScopeManager) Unbind(context.Context, string) error { return nil }

func (stubScopeManager) Evaluate(context.Context, scope.ScopeEvaluationRequest) scope.ScopeDecision {
	return scope.ScopeDecision{Allowed: true}
}

func (stubScopeManager) Snapshot(context.Context, scope.ScopeResolveRequest) (scope.ScopeSnapshot, error) {
	return scope.ScopeSnapshot{}, nil
}

func (stubScopeManager) Invalidate(context.Context, scope.ScopeInvalidationFilter) error { return nil }

func (stubScopeManager) ListBindings(context.Context, scope.ScopeBindingFilter) ([]scope.ScopeBinding, error) {
	return nil, nil
}

type stubScopeSnapshotStore struct {
	snapshots map[string]*scope.ScopeSnapshot
	err       error
}

func (s stubScopeSnapshotStore) Get(_ context.Context, snapshotID string) (*scope.ScopeSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.snapshots[snapshotID], nil
}

func targetSnapshot() *scope.ScopeSnapshot {
	return &scope.ScopeSnapshot{
		SnapshotID:     "conversation-target-1",
		SpaceID:        "space-1",
		CharacterID:    "character-1",
		ConversationID: "conversation-1",
		ExtensionID:    "com.example/test",
		ModuleID:       "main",
		Generation:     1,
		ResolvedScopes: []scope.ScopeRef{
			scope.NewExtensionScope("com.example/test"),
			scope.NewModuleScope("com.example/test", "main"),
			scope.NewCharacterScope("character-1"),
			scope.NewConversationScope("conversation-1"),
		},
	}
}

func targetChecker(snapshots ...*scope.ScopeSnapshot) *ManagerScopeChecker {
	store := map[string]*scope.ScopeSnapshot{}
	for _, snapshot := range snapshots {
		store[snapshot.SnapshotID] = snapshot
	}
	return NewManagerScopeChecker(stubScopeManager{}, stubScopeSnapshotStore{snapshots: store})
}

func targetRequest() CallRequest {
	request := makeRequest(MethodConversationRead, 1, map[string]any{"conversationId": "conversation-1"})
	request.ScopeSnapshotID = ""
	request.TargetScopeSnapshotID = "conversation-target-1"
	request.SpaceID = "space-1"
	request.CharacterID = "character-1"
	request.ConversationID = "conversation-1"
	return request
}

func TestCheckTargetAllowsConversationRole(t *testing.T) {
	checker := targetChecker(targetSnapshot())
	policy := ScopePolicy{RequireRoles: []string{"conversation"}}
	if err := checker.CheckTarget(context.Background(), makeIdentity(), targetRequest(), policy); err != nil {
		t.Fatalf("expected target snapshot to satisfy conversation role, got %v", err)
	}
}

func TestCheckTargetRejectsMissingSnapshot(t *testing.T) {
	checker := targetChecker()
	err := checker.CheckTarget(context.Background(), makeIdentity(), targetRequest(), ScopePolicy{RequireRoles: []string{"conversation"}})
	if !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("expected ErrScopeDenied, got %v", err)
	}
}

func TestCheckTargetRejectsExpiredSnapshot(t *testing.T) {
	snapshot := targetSnapshot()
	expired := time.Now().Add(-time.Minute)
	snapshot.ExpiresAt = &expired
	checker := targetChecker(snapshot)
	err := checker.CheckTarget(context.Background(), makeIdentity(), targetRequest(), ScopePolicy{RequireRoles: []string{"conversation"}})
	if err == nil {
		t.Fatal("expected expired target snapshot to be rejected")
	}
}

func TestCheckTargetRejectsForeignExtension(t *testing.T) {
	snapshot := targetSnapshot()
	snapshot.ExtensionID = "com.example/other"
	checker := targetChecker(snapshot)
	if err := checker.CheckTarget(context.Background(), makeIdentity(), targetRequest(), ScopePolicy{RequireRoles: []string{"conversation"}}); err == nil {
		t.Fatal("expected foreign extension snapshot to be rejected")
	}
}

func TestCheckTargetRejectsConversationMismatch(t *testing.T) {
	request := targetRequest()
	request.ConversationID = "conversation-2"
	checker := targetChecker(targetSnapshot())
	if err := checker.CheckTarget(context.Background(), makeIdentity(), request, ScopePolicy{RequireRoles: []string{"conversation"}}); err == nil {
		t.Fatal("expected conversation mismatch to be rejected")
	}
}

func TestCheckTargetRejectsInvalidCallerScope(t *testing.T) {
	checker := NewManagerScopeChecker(stubScopeManager{}, stubScopeSnapshotStore{
		snapshots: map[string]*scope.ScopeSnapshot{"conversation-target-1": targetSnapshot()},
		err:       errors.New("store offline"),
	})
	err := checker.CheckTarget(context.Background(), makeIdentity(), targetRequest(), ScopePolicy{RequireRoles: []string{"conversation"}})
	if err == nil {
		t.Fatal("expected target snapshot load failure to be rejected")
	}
}

func TestGatewayFailsClosedWithoutTargetChecker(t *testing.T) {
	g := makeTestGateway()
	handler := func(_ context.Context, _ CallRequest) (CallResult, error) {
		return CallResult{Status: StatusSuccess, Output: []byte(`{}`)}, nil
	}
	if err := g.RegisterRoute(Route{
		Method:      MethodConversationRead,
		Version:     1,
		ScopePolicy: ScopePolicy{RequireRoles: []string{"conversation"}},
		Handler:     handler,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	request := targetRequest()
	result := g.Call(context.Background(), request)
	if result.Status != StatusRejected {
		t.Fatalf("expected rejected status, got %s (%v)", result.Status, result.Error)
	}
	if result.Error == nil || result.Error.Code != ErrorCodeScopeDenied {
		t.Fatalf("expected scope denied error, got %+v", result.Error)
	}
}

func TestGatewayDelegatesTargetChecker(t *testing.T) {
	g := NewDefaultGateway()
	g.SetPermissionChecker(PermissionCheckerFunc(func(_ context.Context, _ runtime_supervisor.RuntimeIdentity, _ []PermissionRequirement) error {
		return nil
	}))
	g.SetScopeChecker(targetChecker(targetSnapshot()))
	var seen string
	handler := func(_ context.Context, request CallRequest) (CallResult, error) {
		seen = request.TargetScopeSnapshotID
		return CallResult{Status: StatusSuccess, Output: []byte(`{}`)}, nil
	}
	if err := g.RegisterRoute(Route{
		Method:      MethodConversationRead,
		Version:     1,
		ScopePolicy: ScopePolicy{RequireRoles: []string{"conversation"}},
		Handler:     handler,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	result := g.Call(context.Background(), targetRequest())
	if result.Status != StatusSuccess {
		t.Fatalf("expected success, got %s (%+v)", result.Status, result.Error)
	}
	if seen != "conversation-target-1" {
		t.Fatalf("expected handler to receive target snapshot, got %q", seen)
	}
}
