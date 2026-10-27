package business

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownedWorkerFixture(t *testing.T) (*Engine, *coordination.Service, *testModel, coordination.ExecutionScope, Request, []ProjectWorkspace) {
	t.Helper()
	engine, _, service, model := engineHarness(t)
	parent, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "parent", RequestID: "parent-request", Message: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	request := Request{SpaceID: "core", DeviceID: "a", TargetDeviceID: "a", CoreID: "core", RoleID: "role", ConversationID: "worker-conversation", RequestID: "worker-request", Message: "implement isolated task"}
	workspaces := []ProjectWorkspace{{ID: "worker-tree", DeviceID: "a", Kind: "local", RootURI: "amitia://workspace/@worker-tree/"}}
	return engine, service, model, parent.Scope, request, workspaces
}

func TestOwnedWorkerTrustedWorkspacePersistsAndReplaysWithoutModel(t *testing.T) {
	engine, _, model, parent, request, workspaces := ownedWorkerFixture(t)
	model.generate = func(ctx context.Context, inference Inference) (Generation, error) {
		if !inference.InternalWorker || len(inference.Workspaces) != 1 || inference.Workspaces[0] != workspaces[0] || inference.PrimaryWorkspaceID != "worker-tree" {
			t.Fatalf("worker context lost: %+v", inference)
		}
		if inference.Scope.RequestID != request.RequestID || inference.Scope.ResourceOwnerID != parent.ResourceOwnerID || inference.Scope.RoleID != parent.RoleID || inference.Scope.CoreID != parent.CoreID || inference.Scope.TurnID == parent.TurnID {
			t.Fatalf("worker execution identity invalid: %+v", inference.Scope)
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			t.Fatal(err)
		}
		return Generation{Text: "isolated worker result"}, nil
	}
	response, err := engine.RunWorker(t.Context(), parent, request, workspaces, "worker-tree")
	if err != nil || !response.Saved || response.Text != "isolated worker result" || response.ConversationID != request.ConversationID || response.RequestID != request.RequestID {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	replay, err := engine.RunWorker(t.Context(), parent, request, workspaces, "worker-tree")
	if err != nil || replay.Text != response.Text || model.calls.Load() != 2 {
		t.Fatalf("worker reran: response=%+v calls=%d err=%v", replay, model.calls.Load(), err)
	}
	resources, err := coordination.NewOwnershipStore(engine.data.(testDataPort).db, "a").List(t.Context(), "message", "role", false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, resource := range resources {
		var body map[string]any
		if json.Unmarshal(resource.Body, &body) == nil && body["content"] == request.Message {
			found = true
		}
	}
	if !found {
		t.Fatal("worker user request was not committed verbatim")
	}
}

func TestOwnedWorkerRejectsUntrustedIdentityAndWorkspace(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*coordination.ExecutionScope, *Request, *[]ProjectWorkspace, *string)
	}{
		{"core", func(_ *coordination.ExecutionScope, r *Request, _ *[]ProjectWorkspace, _ *string) {
			r.CoreID = "foreign"
		}},
		{"role", func(_ *coordination.ExecutionScope, r *Request, _ *[]ProjectWorkspace, _ *string) {
			r.RoleID = "foreign"
		}},
		{"owner", func(s *coordination.ExecutionScope, _ *Request, _ *[]ProjectWorkspace, _ *string) {
			s.ResourceOwnerID = "foreign"
		}},
		{"role-revision", func(s *coordination.ExecutionScope, _ *Request, _ *[]ProjectWorkspace, _ *string) {
			s.RoleRevision++
		}},
		{"project", func(_ *coordination.ExecutionScope, r *Request, _ *[]ProjectWorkspace, _ *string) {
			r.ProjectID = "project"
		}},
		{"missing", func(_ *coordination.ExecutionScope, _ *Request, w *[]ProjectWorkspace, _ *string) { *w = nil }},
		{"foreign-device", func(_ *coordination.ExecutionScope, _ *Request, w *[]ProjectWorkspace, _ *string) {
			(*w)[0].DeviceID = "foreign"
		}},
		{"filesystem-path", func(_ *coordination.ExecutionScope, _ *Request, w *[]ProjectWorkspace, _ *string) {
			(*w)[0].RootURI = "file:///private"
		}},
		{"readonly", func(_ *coordination.ExecutionScope, _ *Request, w *[]ProjectWorkspace, _ *string) {
			(*w)[0].ReadOnly = true
		}},
		{"path-traversal", func(_ *coordination.ExecutionScope, _ *Request, w *[]ProjectWorkspace, p *string) {
			*p = "../private"
			(*w)[0].ID = *p
			(*w)[0].RootURI = "amitia://workspace/@" + *p
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, _, model, parent, request, workspaces := ownedWorkerFixture(t)
			primary := "worker-tree"
			test.mutate(&parent, &request, &workspaces, &primary)
			if _, err := engine.RunWorker(t.Context(), parent, request, workspaces, primary); err == nil || model.calls.Load() != 1 {
				t.Fatalf("untrusted worker accepted: calls=%d err=%v", model.calls.Load(), err)
			}
		})
	}
}

func TestOwnedWorkerOrdinaryRequestCannotInjectWorkerContext(t *testing.T) {
	engine, _, model, _, request, workspaces := ownedWorkerFixture(t)
	request.ProjectWorkspaces, request.PrimaryWorkspaceID = workspaces, "worker-tree"
	model.generate = func(_ context.Context, inference Inference) (Generation, error) {
		if inference.InternalWorker || len(inference.Workspaces) != 0 || inference.PrimaryWorkspaceID != "" {
			t.Fatalf("ordinary request injected worker workspace: %+v", inference)
		}
		return Generation{Text: "ordinary"}, nil
	}
	if _, err := engine.Run(t.Context(), request); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerExecutionRejectsRevocationAndCancellation(t *testing.T) {
	t.Run("revoked-before-start", func(t *testing.T) {
		engine, service, model, parent, request, workspaces := ownedWorkerFixture(t)
		if err := service.RevokeDevice(t.Context(), "core", "a", func(tx *sql.Tx) error {
			_, err := tx.ExecContext(t.Context(), `UPDATE kernel_devices SET trust_state='revoked' WHERE device_id='a'`)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.RunWorker(t.Context(), parent, request, workspaces, "worker-tree"); err == nil || model.calls.Load() != 1 {
			t.Fatalf("revoked worker accepted: calls=%d err=%v", model.calls.Load(), err)
		}
	})
	t.Run("permission-changed-during-execution", func(t *testing.T) {
		engine, service, _, parent, _, _ := ownedWorkerFixture(t)
		err := engine.WithExecutionScope(t.Context(), parent, func(ctx context.Context) error {
			_, err := service.InitializeCoordinated(ctx, "core", "a", 1, "role")
			return err
		})
		if !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatalf("expired scope accepted: %v", err)
		}
	})
	t.Run("cancelled-model", func(t *testing.T) {
		engine, _, model, parent, request, workspaces := ownedWorkerFixture(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		model.generate = func(ctx context.Context, _ Inference) (Generation, error) {
			cancel()
			return Generation{}, ctx.Err()
		}
		if response, err := engine.RunWorker(ctx, parent, request, workspaces, "worker-tree"); !errors.Is(err, context.Canceled) || !response.Interrupted || response.Text != "" {
			t.Fatalf("cancelled worker persisted success: %+v %v", response, err)
		}
	})
}

func TestWorkerExecutionContinuityScopeRequiresLeaseContext(t *testing.T) {
	engine, _, _, parent, _, _ := ownedWorkerFixture(t)
	parent.ContinuityRoleID = "continuity-partition"
	called := false
	if err := engine.WithExecutionScope(t.Context(), parent, func(context.Context) error {
		called = true
		return nil
	}); !errors.Is(err, coordination.ErrWrongOwner) || called {
		t.Fatalf("continuity scope without lease accepted: called=%v err=%v", called, err)
	}
	proof := coordination.CommitLeaseProof{ContinuityID: "thread", LeaseID: "lease", RequestID: parent.RequestID}
	ctx := coordination.WithCommitLease(t.Context(), proof)
	if err := engine.WithExecutionScope(ctx, parent, func(ctx context.Context) error {
		actual, ok := coordination.FromContext(ctx)
		if !ok || actual != parent || coordination.CommitLease(ctx) == nil || *coordination.CommitLease(ctx) != proof {
			t.Fatalf("trusted synchronous scope changed: scope=%+v lease=%+v", actual, coordination.CommitLease(ctx))
		}
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("trusted synchronous continuity scope rejected: called=%v err=%v", called, err)
	}
}

func TestExecutionScopePreservesTrustedGroupRole(t *testing.T) {
	engine, service, _, scope, _, _ := ownedWorkerFixture(t)
	scope.ConversationRoleID = "group-owner-role"
	called := false
	if err := engine.WithExecutionScope(t.Context(), scope, func(context.Context) error {
		called = true
		return nil
	}); !errors.Is(err, coordination.ErrScopeExpired) || called {
		t.Fatalf("unbound group scope accepted: called=%v err=%v", called, err)
	}
	if err := engine.WithExecutionScope(coordination.WithScope(t.Context(), scope), scope, func(context.Context) error { return nil }); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("group scope without authority accepted: %v", err)
	}
	ctx, _, finish, err := service.Begin(t.Context(), scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, scope.CoreID, scope.RoleID, scope.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	ctx = coordination.WithScope(ctx, scope)
	if err := engine.WithExecutionScope(ctx, scope, func(current context.Context) error {
		actual, ok := coordination.FromContext(current)
		if !ok || actual != scope {
			t.Fatalf("group scope changed: %+v", actual)
		}
		called = true
		return nil
	}); err != nil || !called {
		t.Fatalf("trusted group scope rejected: called=%v err=%v", called, err)
	}
}

type groupScopeTestPort struct {
	coordination.DataPort
	state coordination.GroupState
}

func (p groupScopeTestPort) GroupState(_ context.Context, _ coordination.ExecutionScope, conversation string) (coordination.GroupState, error) {
	if conversation != p.state.ConversationID {
		return coordination.GroupState{}, coordination.ErrWrongOwner
	}
	return p.state, nil
}

func TestConversationExecutionScopeChecksGroupMembership(t *testing.T) {
	engine, _, _, scope, _, _ := ownedWorkerFixture(t)
	scope.ConversationRoleID = "group-owner-role"
	port := groupScopeTestPort{DataPort: engine.data, state: coordination.GroupState{ConversationID: "chat", ConversationType: "group", ConversationRoleID: scope.ConversationRoleID, RoleOwnerID: scope.RoleOwnerID, Members: []coordination.GroupMember{{CharacterID: scope.RoleID, Status: coordination.GroupMemberStatusActive}}}}
	engine.data = port
	if err := engine.WithConversationExecutionScope(t.Context(), scope, "chat", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("valid group scope rejected: %v", err)
	}
	port.state.Members[0].Status = coordination.GroupMemberStatusLeft
	engine.data = port
	if err := engine.WithConversationExecutionScope(t.Context(), scope, "chat", func(context.Context) error { t.Fatal("inactive member accepted"); return nil }); !errors.Is(err, coordination.ErrWrongOwner) {
		t.Fatalf("inactive member not rejected: %v", err)
	}
}
