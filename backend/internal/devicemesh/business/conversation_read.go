package business

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ErrConversationUnavailable = errors.New("当前执行 Core 下没有可访问的会话记录")

func (e *Engine) OwnsConversationTurn(ctx context.Context, scope coordination.ExecutionScope, conversationID, requestID string) (bool, error) {
	if requestID == "" {
		return false, nil
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return false, ErrConversationUnavailable
	}
	proof, err := port.Resource(ctx, scope, "checkpoint", "turn/"+requestID)
	if err != nil {
		return false, err
	}
	if proof == nil || proof.Deleted {
		return false, nil
	}
	var document checkpoint
	if err := json.Unmarshal(proof.Body, &document); err != nil {
		return false, err
	}
	storedScope := document.Scope
	if document.Response != nil {
		storedScope = &document.Response.Scope
	}
	return proof.OwnerID == scope.ResourceOwnerID && proof.RoleID == scope.RoleID && document.ConversationID == conversationID && storedScope != nil && storedScope.CoreID == scope.CoreID && storedScope.SpaceID == scope.SpaceID && storedScope.ResourceOwnerID == scope.ResourceOwnerID && storedScope.RoleID == scope.RoleID, nil
}

func (e *Engine) WithConversationRead(ctx context.Context, request Request, read func(context.Context, coordination.ExecutionScope, string) error) error {
	if request.ConversationID == "" || request.ConversationOrigin == nil || request.RoleID == "" {
		return coordination.ErrWrongOwner
	}
	ctx, scope, finish, err := e.coordination.Begin(ctx, request.SpaceID, request.DeviceID, request.TargetDeviceID, request.CoreID, request.RoleID, uuid.NewString())
	if err != nil {
		return err
	}
	defer finish()
	if err := e.coordination.RequireCapability(ctx, scope.SpaceID, scope.InitiatorDeviceID, scope.TargetDeviceID, "ai.chat"); err != nil {
		return err
	}
	roles, err := e.data.Roles(ctx, scope)
	if err != nil {
		return err
	}
	role, err := coordination.ResolveRole(request.RoleID, roles)
	if err != nil {
		return err
	}
	scope.RoleID, scope.RoleRevision = role.ID, role.Revision
	ctx = coordination.WithScope(ctx, scope)
	route, err := e.resolveStoredConversationRoute(ctx, scope, request.ConversationID, request.ConversationOrigin)
	if err != nil {
		return err
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return ErrConversationUnavailable
	}
	resource, err := port.Resource(ctx, scope, "conversation", route.CurrentID)
	if err != nil {
		return err
	}
	if resource == nil || resource.Deleted {
		return ErrConversationUnavailable
	}
	if resource.ID != route.CurrentID || resource.Kind != "conversation" || resource.OwnerID != scope.ResourceOwnerID || resource.RoleID != scope.RoleID {
		return coordination.ErrWrongOwner
	}
	if err := read(ctx, scope, route.CurrentID); err != nil {
		return err
	}
	if err := e.coordination.Validate(ctx, scope); err != nil {
		return err
	}
	if err := coordination.ValidateRoleRevision(ctx, e.data, scope); err != nil {
		return err
	}
	current, err := port.Resource(ctx, scope, "conversation", route.CurrentID)
	if err != nil {
		return err
	}
	if current == nil || current.Deleted || current.Revision != resource.Revision || current.OwnerID != resource.OwnerID || current.RoleID != resource.RoleID {
		return coordination.ErrResourceVersion
	}
	return nil
}
