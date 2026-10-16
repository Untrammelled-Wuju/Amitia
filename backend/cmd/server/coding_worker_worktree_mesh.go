package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	devicemeshagent "github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const codingWorktreeMeshHandler = "workbench.coding-worktree.ensure"

type codingWorkerMeshRequest struct {
	Operation        string                      `json:"operation,omitempty"`
	Scope            coordination.ExecutionScope `json:"scope"`
	AssignmentID     string                      `json:"assignmentId"`
	ConversationID   string                      `json:"conversationId"`
	WorkspaceID      string                      `json:"workspaceId"`
	WorkspaceRootURI string                      `json:"workspaceRootUri"`
}

type codingWorkerMeshReply struct {
	Request   codingWorkerMeshRequest           `json:"request"`
	Workspace interaction.CodingWorkerWorkspace `json:"workspace"`
	Review    *interaction.CodingWorktreeReview `json:"review,omitempty"`
}

type codingWorkerMeshAllocator struct {
	validate func(context.Context, coordination.ExecutionScope) error
	invoke   func(context.Context, coordination.ExecutionScope, []byte) ([]byte, error)
}

func (a *codingWorkerMeshAllocator) ReviewCodingTask(ctx context.Context, sourceWorkspaceURI, taskID, conversationID string) (interaction.CodingWorktreeReview, error) {
	scope, ok := coordination.FromContext(ctx)
	if !ok || a == nil || a.validate == nil || a.invoke == nil {
		return interaction.CodingWorktreeReview{}, coordination.ErrWrongOwner
	}
	mount := strings.TrimSuffix(strings.TrimPrefix(sourceWorkspaceURI, "amitia://workspace/@"), "/")
	req := interaction.WorkerRunRequest{OwnedScope: &scope, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: taskID, ConversationID: conversationID, WorkspaceID: mount, WorkspaceRootURI: sourceWorkspaceURI, WorkspaceDeviceID: scope.TargetDeviceID}
	if _, err := codingWorkerMeshScope(req); err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	if err := a.validate(ctx, scope); err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	request := codingWorkerMeshRequest{Operation: "review", Scope: scope, AssignmentID: taskID, ConversationID: conversationID, WorkspaceID: mount, WorkspaceRootURI: sourceWorkspaceURI}
	input, err := json.Marshal(request)
	if err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	output, err := a.invoke(ctx, scope, input)
	if err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	if err := a.validate(ctx, scope); err != nil {
		return interaction.CodingWorktreeReview{}, err
	}
	var reply codingWorkerMeshReply
	if len(output) > 64<<10 || json.Unmarshal(output, &reply) != nil || reply.Request != request || reply.Review == nil {
		return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
	}
	review := *reply.Review
	childMount := strings.TrimSuffix(strings.TrimPrefix(review.WorkspaceURI, "amitia://workspace/@"), "/")
	if review.WorktreeID == "" || childMount == mount || !validCodingMeshMount(childMount, review.WorkspaceURI) ||
		(review.State != "ready" && review.State != "incomplete") {
		return interaction.CodingWorktreeReview{}, errCodingReviewUnavailable
	}
	review.RequiresApproval = true
	review.VerificationGate = nil
	review.Terminal = &interaction.CodingTerminalDiagnostics{State: "unavailable", VerificationKind: "terminal-lifecycle-metadata-not-test-proof", ErrorCode: "TERMINAL_DIAGNOSTICS_UNAVAILABLE"}
	return review, nil
}

func codingWorkerMeshScope(req interaction.WorkerRunRequest) (coordination.ExecutionScope, error) {
	if req.OwnedScope == nil {
		return coordination.ExecutionScope{}, coordination.ErrWrongOwner
	}
	scope := *req.OwnedScope
	owner := scope.TargetDeviceID
	if scope.Coordinated {
		owner = scope.CoreID
	}
	if scope.CoreID == "" || scope.CoreID != scope.SpaceID || scope.AuthorizationRealm != scope.CoreID ||
		scope.SpaceID != req.SpaceID || scope.RoleID != req.CharacterID || scope.RoleID == "" ||
		scope.ResourceOwnerID != owner || scope.RoleOwnerID != owner || owner == "" ||
		scope.TargetDeviceID != req.WorkspaceDeviceID || scope.TargetDeviceID == "" ||
		scope.InitiatorDeviceID == "" || scope.RequestID == "" || scope.ExecutionID == "" || scope.TurnID == "" ||
		scope.PermissionRevision < 1 || scope.TargetPermissionRevision < 1 || scope.ProviderEpoch < 1 ||
		scope.TargetProviderEpoch < 1 || scope.ModeRevision < 1 || scope.RoleRevision < 1 ||
		req.Source != "multi_agent" || req.WorkerRole != "coding_worker" ||
		req.AssignmentID == "" || len(req.AssignmentID) > 128 || req.ConversationID == "" || len(req.ConversationID) > 512 ||
		!validCodingMeshMount(req.WorkspaceID, req.WorkspaceRootURI) {
		return coordination.ExecutionScope{}, coordination.ErrWrongOwner
	}
	return scope, nil
}

func validCodingMeshMount(id, uri string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, value := range id {
		if !(value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_' || value == '-') {
			return false
		}
	}
	return uri == "amitia://workspace/@"+id || uri == "amitia://workspace/@"+id+"/"
}

func (a *codingWorkerMeshAllocator) EnsureCodingWorkerWorktree(ctx context.Context, req interaction.WorkerRunRequest) (interaction.CodingWorkerWorkspace, error) {
	scope, err := codingWorkerMeshScope(req)
	if err != nil || a == nil || a.validate == nil || a.invoke == nil {
		return interaction.CodingWorkerWorkspace{}, coordination.ErrWrongOwner
	}
	current, ok := coordination.FromContext(ctx)
	if !ok || current != scope {
		return interaction.CodingWorkerWorkspace{}, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	if err := a.validate(ctx, scope); err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	request := codingWorkerMeshRequest{Scope: scope, AssignmentID: req.AssignmentID, ConversationID: req.ConversationID, WorkspaceID: req.WorkspaceID, WorkspaceRootURI: req.WorkspaceRootURI}
	input, err := json.Marshal(request)
	if err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	result, err := a.invoke(ctx, scope, input)
	if err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	if err := a.validate(ctx, scope); err != nil {
		return interaction.CodingWorkerWorkspace{}, err
	}
	var reply codingWorkerMeshReply
	if len(result) > 16<<10 || json.Unmarshal(result, &reply) != nil || reply.Request != request ||
		reply.Workspace.WorkspaceID == req.WorkspaceID || reply.Workspace.WorkspaceKind != "local" ||
		!validCodingMeshMount(reply.Workspace.WorkspaceID, reply.Workspace.WorkspaceRootURI) {
		return interaction.CodingWorkerWorkspace{}, errors.New("设备返回的任务工作区无效")
	}
	return reply.Workspace, nil
}

func newCodingWorkerWorktreeAllocator(services func() *AppServices, git codingTaskWorktreeCreator, localDeviceID string) codingWorkerWorktreeAllocator {
	allocator := codingWorkerWorktreeAllocator{git: git, localDeviceID: localDeviceID}
	allocator.mesh = &codingWorkerMeshAllocator{
		validate: func(ctx context.Context, scope coordination.ExecutionScope) error {
			if services == nil || services() == nil || services().DeviceMesh == nil || services().DeviceMesh.Coordination == nil {
				return errors.New("设备工作区调度不可用")
			}
			mesh := services().DeviceMesh
			if err := mesh.Coordination.Validate(ctx, scope); err != nil {
				return err
			}
			if err := coordination.ValidateRoleRevision(ctx, mesh, scope); err != nil {
				return err
			}
			return mesh.Coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat")
		},
		invoke: func(ctx context.Context, scope coordination.ExecutionScope, input []byte) ([]byte, error) {
			invokeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			result, err := services().DeviceMesh.InvokeDeviceHandlerWithRuntimeType(invokeCtx, runtimeidentity.SpaceID(scope.SpaceID), runtimeidentity.DeviceID(scope.TargetDeviceID), capability.RuntimeTypeInternal, codingWorktreeMeshHandler, input, 30*time.Second)
			return result.Structured, err
		},
	}
	return allocator
}

func registerDeviceCodingWorktreeHandler(dispatcher interface {
	RegisterCancellable(string, devicemeshagent.CancellableRuntimeInvokeHandler)
}, services *AppServices) {
	dispatcher.RegisterCancellable(codingWorktreeMeshHandler, func(ctx context.Context, invocation protocol.RuntimeInvokePayload) (*protocol.RuntimeResultPayload, error) {
		if services == nil || services.DeviceMesh == nil || services.DeviceMesh.LocalHandler == nil || services.WorkspaceGitHandler == nil {
			return nil, errors.New("设备任务工作区不可用")
		}
		validate := func(ctx context.Context) error {
			identity, err := services.DeviceMesh.LocalHandler.LoadIdentity()
			if err != nil || identity == nil || identity.DeviceID != invocation.DeviceID || identity.RuntimeID != invocation.RuntimeID {
				return coordination.ErrWrongOwner
			}
			credential, err := services.DeviceMesh.LocalHandler.LoadCredential()
			if err != nil || credential == nil || credential.SpaceID != invocation.SpaceID || credential.DeviceID != invocation.DeviceID || credential.RuntimeID != invocation.RuntimeID || credential.CredentialID == "" || credential.Credential == "" || !time.Now().Before(credential.ExpiresAt) {
				return coordination.ErrWrongOwner
			}
			return coordination.ValidateCurrent(ctx)
		}
		return executeDeviceCodingWorktree(ctx, invocation, services.WorkspaceGitHandler, validate)
	})
}

func executeDeviceCodingWorktree(ctx context.Context, invocation protocol.RuntimeInvokePayload, git codingTaskWorktreeCreator, validate func(context.Context) error) (*protocol.RuntimeResultPayload, error) {
	var request codingWorkerMeshRequest
	scope, ok := coordination.FromContext(ctx)
	var advertised coordination.ExecutionScope
	if !ok || invocation.AuthorityCallID == "" || invocation.InvocationID == "" || invocation.Handler != codingWorktreeMeshHandler || invocation.RuntimeType != string(capability.RuntimeTypeInternal) ||
		len(invocation.Input) > 16<<10 || json.Unmarshal(invocation.Input, &request) != nil ||
		json.Unmarshal(invocation.OwnedExecutionScope, &advertised) != nil || advertised != scope || request.Scope != scope ||
		scope.SpaceID != invocation.SpaceID.String() || scope.TargetDeviceID != invocation.DeviceID.String() || git == nil || validate == nil {
		return nil, coordination.ErrWrongOwner
	}
	req := interaction.WorkerRunRequest{OwnedScope: &scope, SpaceID: scope.SpaceID, CharacterID: scope.RoleID, WorkerRole: "coding_worker", Source: "multi_agent", AssignmentID: request.AssignmentID, ConversationID: request.ConversationID, WorkspaceID: request.WorkspaceID, WorkspaceRootURI: request.WorkspaceRootURI, WorkspaceDeviceID: scope.TargetDeviceID}
	if _, err := codingWorkerMeshScope(req); err != nil {
		return nil, err
	}
	if err := validate(ctx); err != nil {
		return nil, err
	}
	reply := codingWorkerMeshReply{Request: request}
	switch request.Operation {
	case "", "ensure":
		workspace, err := (codingWorkerWorktreeAllocator{git: git, localDeviceID: scope.TargetDeviceID}).EnsureCodingWorkerWorktree(ctx, req)
		if err != nil {
			return nil, err
		}
		reply.Workspace = workspace
	case "review":
		reviewGit, ok := git.(codingWorktreeReviewGit)
		if !ok {
			return nil, errCodingReviewUnavailable
		}
		review, err := (codingWorktreeReviewerService{git: reviewGit, localDeviceID: scope.TargetDeviceID}).ReviewCodingTask(ctx, request.WorkspaceRootURI, request.AssignmentID, request.ConversationID)
		if err != nil {
			return nil, err
		}
		review.Terminal = &interaction.CodingTerminalDiagnostics{State: "unavailable", VerificationKind: "terminal-lifecycle-metadata-not-test-proof", ErrorCode: "TERMINAL_DIAGNOSTICS_UNAVAILABLE"}
		reply.Review = &review
	default:
		return nil, errors.New("设备任务工作区操作无效")
	}
	if err := validate(ctx); err != nil {
		return nil, err
	}
	output, err := json.Marshal(reply)
	if err != nil {
		return nil, err
	}
	return &protocol.RuntimeResultPayload{InvocationID: invocation.InvocationID, Status: "success", Result: output}, nil
}
