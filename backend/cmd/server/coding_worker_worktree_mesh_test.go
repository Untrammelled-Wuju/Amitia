package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/gitworktree"
	"github.com/u-ai/backend/internal/interaction"
)

func codingMeshFixture() interaction.WorkerRunRequest {
	scope := coordination.ExecutionScope{SpaceID: "core-a", CoreID: "core-a", AuthorizationRealm: "core-a", InitiatorDeviceID: "device-a", TargetDeviceID: "device-a", ResourceOwnerID: "device-a", RoleOwnerID: "device-a", RoleID: "role-a", RequestID: "request-a", ExecutionID: "execution-a", TurnID: "turn-a", PermissionRevision: 1, TargetPermissionRevision: 1, ProviderEpoch: 1, TargetProviderEpoch: 1, ModeRevision: 1, RoleRevision: 1}
	return interaction.WorkerRunRequest{OwnedScope: &scope, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: "assignment-a", ConversationID: "conversation-a", WorkspaceID: "parent", WorkspaceRootURI: "amitia://workspace/@parent/", WorkspaceDeviceID: "device-a"}
}

func codingMeshFixtureInvocation(request codingWorkerMeshRequest) protocol.RuntimeInvokePayload {
	input, _ := json.Marshal(request)
	scope, _ := json.Marshal(request.Scope)
	return protocol.RuntimeInvokePayload{InvocationID: "invoke-a", AuthorityCallID: "authority-a", SpaceID: "core-a", DeviceID: "device-a", RuntimeID: "runtime-a", RuntimeType: "internal", Handler: codingWorktreeMeshHandler, Input: input, OwnedExecutionScope: scope}
}

func codingMeshCreator() *fakeCodingWorktreeCreator {
	return &fakeCodingWorktreeCreator{entry: &gitworktree.Entry{WorkspaceURI: "amitia://workspace/@child/", TaskBinding: &gitworktree.TaskBinding{TaskID: "assignment-a", ConversationID: "conversation-a"}}}
}

func TestCodingMeshUsesDeviceAllocatorAndReusesBinding(t *testing.T) {
	req := codingMeshFixture()
	creator := codingMeshCreator()
	ctx := coordination.WithScope(t.Context(), *req.OwnedScope)
	remoteCalls := 0
	mesh := &codingWorkerMeshAllocator{
		validate: func(context.Context, coordination.ExecutionScope) error { return nil },
		invoke: func(current context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
			remoteCalls++
			var request codingWorkerMeshRequest
			if err := json.Unmarshal(input, &request); err != nil {
				return nil, err
			}
			reply, err := executeDeviceCodingWorktree(current, codingMeshFixtureInvocation(request), creator, func(context.Context) error { return nil })
			if err != nil {
				return nil, err
			}
			return reply.Result, nil
		},
	}
	local := &fakeCodingWorktreeCreator{}
	allocator := codingWorkerWorktreeAllocator{git: local, localDeviceID: "core-device", mesh: mesh}
	first, err := allocator.EnsureCodingWorkerWorktree(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := allocator.EnsureCodingWorkerWorktree(ctx, req)
	if err != nil || first != second || remoteCalls != 2 || creator.calls != 2 || local.calls != 0 {
		t.Fatalf("device allocation or stable binding failed: first=%+v second=%+v remote=%d local=%d err=%v", first, second, remoteCalls, local.calls, err)
	}
	if creator.request.TaskID != req.AssignmentID || creator.request.ConversationID != req.ConversationID || creator.request.WorkspaceURI != req.WorkspaceRootURI {
		t.Fatal("existing task worktree contract changed")
	}
}

func TestCodingMeshRejectsScopeTamperingBeforeDispatch(t *testing.T) {
	for _, change := range []func(*interaction.WorkerRunRequest){
		func(req *interaction.WorkerRunRequest) { req.OwnedScope.ResourceOwnerID = "other" },
		func(req *interaction.WorkerRunRequest) { req.OwnedScope.CoreID = "other" },
		func(req *interaction.WorkerRunRequest) { req.SpaceID = "other" },
		func(req *interaction.WorkerRunRequest) { req.WorkspaceDeviceID = "other" },
		func(req *interaction.WorkerRunRequest) { req.OwnedScope.TargetPermissionRevision = 0 },
		func(req *interaction.WorkerRunRequest) { req.OwnedScope.RoleRevision = 0 },
		func(req *interaction.WorkerRunRequest) { req.WorkspaceRootURI = "file:///private" },
	} {
		req := codingMeshFixture()
		change(&req)
		calls := 0
		mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error { return nil }, invoke: func(context.Context, coordination.ExecutionScope, []byte) ([]byte, error) { calls++; return nil, nil }}
		_, err := mesh.EnsureCodingWorkerWorktree(coordination.WithScope(t.Context(), *req.OwnedScope), req)
		if err == nil || calls != 0 {
			t.Fatal("tampered request reached device")
		}
	}
}

func TestCodingMeshRejectsRevocationAndOfflineWithoutLocalFallback(t *testing.T) {
	req := codingMeshFixture()
	ctx := coordination.WithScope(t.Context(), *req.OwnedScope)
	for _, revoked := range []bool{true, false} {
		calls := 0
		issue := errors.New("device offline")
		mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error {
			if revoked {
				return coordination.ErrScopeExpired
			}
			return nil
		}, invoke: func(context.Context, coordination.ExecutionScope, []byte) ([]byte, error) { calls++; return nil, issue }}
		local := &fakeCodingWorktreeCreator{}
		_, err := (codingWorkerWorktreeAllocator{git: local, localDeviceID: "core-device", mesh: mesh}).EnsureCodingWorkerWorktree(ctx, req)
		if err == nil || local.calls != 0 || revoked && calls != 0 || !revoked && !errors.Is(err, issue) {
			t.Fatalf("unsafe fallback or lost error: %v", err)
		}
	}
}

func TestDeviceCodingMeshRejectsUntrustedAndRevokedCalls(t *testing.T) {
	req := codingMeshFixture()
	request := codingWorkerMeshRequest{Scope: *req.OwnedScope, AssignmentID: req.AssignmentID, ConversationID: req.ConversationID, WorkspaceID: req.WorkspaceID, WorkspaceRootURI: req.WorkspaceRootURI}
	for _, change := range []func(*protocol.RuntimeInvokePayload){
		func(invoke *protocol.RuntimeInvokePayload) { invoke.AuthorityCallID = "" },
		func(invoke *protocol.RuntimeInvokePayload) { invoke.DeviceID = "other" },
		func(invoke *protocol.RuntimeInvokePayload) { invoke.SpaceID = "other" },
		func(invoke *protocol.RuntimeInvokePayload) { invoke.OwnedExecutionScope = nil },
	} {
		invocation := codingMeshFixtureInvocation(request)
		change(&invocation)
		creator := codingMeshCreator()
		_, err := executeDeviceCodingWorktree(coordination.WithScope(t.Context(), request.Scope), invocation, creator, func(context.Context) error { return nil })
		if err == nil || creator.calls != 0 {
			t.Fatal("untrusted invocation allocated a workspace")
		}
	}
	creator := codingMeshCreator()
	_, err := executeDeviceCodingWorktree(coordination.WithScope(t.Context(), request.Scope), codingMeshFixtureInvocation(request), creator, func(context.Context) error { return coordination.ErrScopeExpired })
	if !errors.Is(err, coordination.ErrScopeExpired) || creator.calls != 0 {
		t.Fatal("revoked authority allocated workspace")
	}
}

func TestCodingMeshRejectsAuthorityChangeAfterAllocation(t *testing.T) {
	req := codingMeshFixture()
	checks := 0
	mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error {
		checks++
		if checks > 1 {
			return coordination.ErrScopeExpired
		}
		return nil
	}, invoke: func(_ context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
		var request codingWorkerMeshRequest
		_ = json.Unmarshal(input, &request)
		return json.Marshal(codingWorkerMeshReply{Request: request, Workspace: interaction.CodingWorkerWorkspace{WorkspaceID: "child", WorkspaceRootURI: "amitia://workspace/@child/", WorkspaceKind: "local"}})
	}}
	_, err := mesh.EnsureCodingWorkerWorktree(coordination.WithScope(t.Context(), *req.OwnedScope), req)
	if !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatalf("stale response accepted: %v", err)
	}
}

func TestDeviceCodingMeshRejectsAuthorityChangeBeforeReply(t *testing.T) {
	req := codingMeshFixture()
	request := codingWorkerMeshRequest{Scope: *req.OwnedScope, AssignmentID: req.AssignmentID, ConversationID: req.ConversationID, WorkspaceID: req.WorkspaceID, WorkspaceRootURI: req.WorkspaceRootURI}
	checks := 0
	creator := codingMeshCreator()
	result, err := executeDeviceCodingWorktree(coordination.WithScope(t.Context(), request.Scope), codingMeshFixtureInvocation(request), creator, func(context.Context) error {
		checks++
		if checks > 1 {
			return coordination.ErrScopeExpired
		}
		return nil
	})
	if !errors.Is(err, coordination.ErrScopeExpired) || result != nil || creator.calls != 1 {
		t.Fatalf("revoked allocation acknowledged: calls=%d result=%+v err=%v", creator.calls, result, err)
	}
}

func TestCodingMeshRejectsMismatchedDeviceResult(t *testing.T) {
	req := codingMeshFixture()
	for _, change := range []func(*codingWorkerMeshReply){
		func(reply *codingWorkerMeshReply) { reply.Request.Scope.ResourceOwnerID = "other" },
		func(reply *codingWorkerMeshReply) { reply.Request.AssignmentID = "other" },
		func(reply *codingWorkerMeshReply) { reply.Request.ConversationID = "other" },
		func(reply *codingWorkerMeshReply) {
			reply.Workspace.WorkspaceID = "parent"
			reply.Workspace.WorkspaceRootURI = "amitia://workspace/@parent/"
		},
		func(reply *codingWorkerMeshReply) { reply.Workspace.WorkspaceRootURI = "file:///private" },
	} {
		mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error { return nil }, invoke: func(_ context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
			var request codingWorkerMeshRequest
			_ = json.Unmarshal(input, &request)
			reply := codingWorkerMeshReply{Request: request, Workspace: interaction.CodingWorkerWorkspace{WorkspaceID: "child", WorkspaceRootURI: "amitia://workspace/@child/", WorkspaceKind: "local"}}
			change(&reply)
			return json.Marshal(reply)
		}}
		if _, err := mesh.EnsureCodingWorkerWorktree(coordination.WithScope(t.Context(), *req.OwnedScope), req); err == nil {
			t.Fatal("foreign or non-isolated device workspace accepted")
		}
	}
}

type codingMeshReviewGit struct {
	*fakeCodingWorktreeCreator
	*fakeReviewGit
}

func TestCodingMeshReviewUsesDeviceEvidenceWithoutLocalFallback(t *testing.T) {
	req := codingMeshFixture()
	ctx := coordination.WithScope(t.Context(), *req.OwnedScope)
	remoteGit := &codingMeshReviewGit{fakeCodingWorktreeCreator: codingMeshCreator(), fakeReviewGit: sampleReviewGit()}
	localGit := sampleReviewGit()
	mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error { return nil }, invoke: func(current context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
		var request codingWorkerMeshRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return nil, err
		}
		if request.Operation != "review" {
			t.Fatal("review mutated a workspace")
		}
		reply, err := executeDeviceCodingWorktree(current, codingMeshFixtureInvocation(request), remoteGit, func(context.Context) error { return nil })
		if err != nil {
			return nil, err
		}
		return reply.Result, nil
	}}
	reviewer := codingWorktreeReviewerService{git: localGit, localDeviceID: "core-device", mesh: mesh}
	result, err := reviewer.ReviewCodingTask(ctx, "amitia://workspace/@main/", "task-a", "conv-a")
	if err != nil || result.State != "ready" || result.FilesChanged != 3 || !result.RequiresApproval || result.TestState != "not_run" || result.ApprovalState != "pending" || result.Terminal == nil || result.Terminal.State != "unavailable" || localGit.calls != 0 || remoteGit.fakeReviewGit.calls != 1 || remoteGit.fakeCodingWorktreeCreator.calls != 0 {
		t.Fatalf("invalid remote review: result=%+v local=%d remote=%d mutation=%d err=%v", result, localGit.calls, remoteGit.fakeReviewGit.calls, remoteGit.fakeCodingWorktreeCreator.calls, err)
	}
}

func TestCodingMeshReviewRejectsWrongTaskAndOffline(t *testing.T) {
	req := codingMeshFixture()
	ctx := coordination.WithScope(t.Context(), *req.OwnedScope)
	remoteGit := &codingMeshReviewGit{fakeCodingWorktreeCreator: codingMeshCreator(), fakeReviewGit: sampleReviewGit()}
	mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error { return nil }, invoke: func(current context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
		var request codingWorkerMeshRequest
		_ = json.Unmarshal(input, &request)
		reply, err := executeDeviceCodingWorktree(current, codingMeshFixtureInvocation(request), remoteGit, func(context.Context) error { return nil })
		if err != nil {
			return nil, err
		}
		return reply.Result, nil
	}}
	if _, err := mesh.ReviewCodingTask(ctx, "amitia://workspace/@main/", "another-task", "conv-a"); err == nil || remoteGit.fakeReviewGit.calls != 0 {
		t.Fatal("foreign task review disclosed")
	}
	issue := errors.New("device offline")
	mesh.invoke = func(context.Context, coordination.ExecutionScope, []byte) ([]byte, error) { return nil, issue }
	localGit := sampleReviewGit()
	_, err := (codingWorktreeReviewerService{git: localGit, localDeviceID: "core-device", mesh: mesh}).ReviewCodingTask(ctx, "amitia://workspace/@main/", "task-a", "conv-a")
	if !errors.Is(err, issue) || localGit.calls != 0 {
		t.Fatal("remote review fell back to core git")
	}
	mesh.validate = func(context.Context, coordination.ExecutionScope) error { return coordination.ErrScopeExpired }
	if _, err := mesh.ReviewCodingTask(ctx, "amitia://workspace/@main/", "task-a", "conv-a"); !errors.Is(err, coordination.ErrScopeExpired) {
		t.Fatal("revoked review accepted")
	}
}

func TestCodingMeshReviewRejectsForgedResponse(t *testing.T) {
	req := codingMeshFixture()
	ctx := coordination.WithScope(t.Context(), *req.OwnedScope)
	mesh := &codingWorkerMeshAllocator{validate: func(context.Context, coordination.ExecutionScope) error { return nil }, invoke: func(_ context.Context, _ coordination.ExecutionScope, input []byte) ([]byte, error) {
		var request codingWorkerMeshRequest
		_ = json.Unmarshal(input, &request)
		request.ConversationID = "other"
		return json.Marshal(codingWorkerMeshReply{Request: request, Review: &interaction.CodingWorktreeReview{State: "ready", WorkspaceURI: "amitia://workspace/@child/", WorktreeID: "worktree-a"}})
	}}
	if _, err := mesh.ReviewCodingTask(ctx, "amitia://workspace/@main/", "task-a", "conv-a"); err == nil {
		t.Fatal("foreign review response accepted")
	}
}
