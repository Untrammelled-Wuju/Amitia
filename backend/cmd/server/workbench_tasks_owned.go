package main

import (
	"context"

	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/workbenchtasks"
)

func workbenchConversationAccess(services *AppServices, coreID string) workbenchtasks.ConversationAccess {
	return func(ctx context.Context, actor *auth.ActorContext, query workbenchtasks.ConversationQuery, read func(context.Context, workbenchtasks.QueryContext) error) error {
		if services.OwnedBusiness == nil {
			return business.ErrConversationUnavailable
		}
		if query.ExpectedCoreID != coreID || actor.SpaceID.String() != coreID {
			return coordination.ErrScopeExpired
		}
		origin := business.ConversationOrigin{OwnerID: query.OwnerID, ID: query.ConversationID}
		request := business.Request{SpaceID: actor.SpaceID.String(), DeviceID: actor.DeviceID.String(), TargetDeviceID: query.TargetDeviceID, CoreID: coreID, RoleID: query.CharacterID, ConversationID: query.ConversationID, ConversationOrigin: &origin}
		return services.OwnedBusiness.WithConversationRead(ctx, request, func(ctx context.Context, scope coordination.ExecutionScope, id string) error {
			return read(ctx, workbenchtasks.QueryContext{ExecutionScope: scope, ConversationOrigin: origin, ConversationID: id, CanReadRecord: func(ctx context.Context, record *interaction.InteractionRecord) (bool, error) {
				if record.Scope.SpaceID != scope.SpaceID || record.Scope.ConversationID != id || record.Scope.CharacterID != scope.RoleID {
					return false, nil
				}
				if record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
					return false, nil
				}
				binding := record.RecoveryDescriptor.MultiAgent
				if binding.ExecutionCoreID != scope.CoreID || binding.ResourceOwnerID != scope.ResourceOwnerID {
					return false, nil
				}
				requestID := record.Scope.RequestID
				if turn := record.RecoveryDescriptor.OwnedTurn; turn != nil {
					if turn.Scope.CoreID != scope.CoreID || turn.Scope.ResourceOwnerID != scope.ResourceOwnerID || turn.Scope.RoleID != scope.RoleID {
						return false, nil
					}
					requestID = turn.RequestID
				}
				return services.OwnedBusiness.OwnsConversationTurn(ctx, scope, id, requestID)
			}})
		})
	}
}
