package business

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"strings"
)

type workerContextKey struct{}
type workerContext struct {
	Workspaces []ProjectWorkspace
	PrimaryID  string
}

func (e *Engine) WithExecutionScope(ctx context.Context, expected coordination.ExecutionScope, run func(context.Context) error) error {
	ctx, scope, finish, err := e.coordination.Begin(ctx, expected.SpaceID, expected.InitiatorDeviceID, expected.TargetDeviceID, expected.CoreID, expected.RoleID, expected.RequestID)
	if err != nil {
		return err
	}
	defer finish()
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(expected.RoleID, roles)
	if err != nil {
		return err
	}
	scope.RoleRevision = role.Revision
	if err := coordination.ResolveMemoryScope(&scope, ""); err != nil {
		return err
	}
	if expected.ContinuityRoleID != "" {
		if coordination.CommitLease(ctx) == nil {
			return coordination.ErrWrongOwner
		}
		scope.ContinuityRoleID = expected.ContinuityRoleID
	}
	scope.TurnID, scope.ExecutionID = expected.TurnID, expected.ExecutionID
	if scope != expected {
		return coordination.ErrScopeExpired
	}
	ctx = coordination.WithScope(ctx, scope)
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return err
	}
	if err := run(ctx); err != nil {
		return err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return err
	}
	return coordination.ValidateRoleRevision(ctx, e.data, scope)
}

func (e *Engine) RunWorker(ctx context.Context, parent coordination.ExecutionScope, request Request, workspaces []ProjectWorkspace, primary string) (Response, error) {
	if request.RoleID != parent.RoleID || request.SpaceID != parent.SpaceID || request.DeviceID != parent.InitiatorDeviceID || request.TargetDeviceID != parent.TargetDeviceID || request.CoreID != parent.CoreID || request.ProjectID != "" || request.ConversationOrigin != nil || primary == "" || len(workspaces) != 1 {
		return Response{}, coordination.ErrWrongOwner
	}
	workspace := workspaces[0]
	if workspace.ID != primary || workspace.DeviceID != parent.TargetDeviceID || workspace.ReadOnly || workspace.Kind != "local" || workspace.RootURI != "amitia://workspace/@"+primary+"/" && workspace.RootURI != "amitia://workspace/@"+primary || strings.ContainsAny(primary, "/?#\\") {
		return Response{}, coordination.ErrWrongOwner
	}
	var response Response
	err := e.WithExecutionScope(ctx, parent, func(ctx context.Context) error {
		request.ExpectedScope = &parent
		ctx = context.WithValue(ctx, workerContextKey{}, workerContext{Workspaces: append([]ProjectWorkspace(nil), workspaces...), PrimaryID: primary})
		var err error
		response, err = e.RunEvents(ctx, request, nil)
		return err
	})
	return response, err
}
