package workbenchtasks

import (
	"context"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/mindruntime"
)

func authorizeBudgetScope(ctx context.Context, getter budgetParentReader, query QueryContext, parentID, coordinationID string) (context.Context, mindruntime.OwnedBudgetReconciliationScope, error) {
	parent, found, err := getter.Get(ctx, parentID)
	if err != nil {
		return nil, mindruntime.OwnedBudgetReconciliationScope{}, err
	}
	if !found || parent == nil || parent.ID != parentID || parent.Scope.Source == "multi_agent" || parent.Scope.SpaceID != query.ExecutionScope.SpaceID || parent.Scope.CharacterID != query.ExecutionScope.RoleID || parent.Scope.ConversationID != query.ConversationID || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil || parent.RecoveryDescriptor.MultiAgent.CoordinationID != coordinationID || parent.RecoveryDescriptor.MultiAgent.ResourceOwnerID != query.ExecutionScope.ResourceOwnerID || parent.RecoveryDescriptor.MultiAgent.ExecutionCoreID != query.ExecutionScope.CoreID || !validateCheckpoint(parent.RecoveryDescriptor.MultiAgent) {
		return nil, mindruntime.OwnedBudgetReconciliationScope{}, business.ErrConversationUnavailable
	}
	if parent.RecoveryDescriptor.Fingerprint != "" {
		copyOfDescriptor := *parent.RecoveryDescriptor
		copyOfDescriptor.ComputeFingerprint()
		if copyOfDescriptor.Fingerprint != parent.RecoveryDescriptor.Fingerprint {
			return nil, mindruntime.OwnedBudgetReconciliationScope{}, coordination.ErrScopeExpired
		}
	}
	if query.CanReadRecord == nil {
		return nil, mindruntime.OwnedBudgetReconciliationScope{}, coordination.ErrWrongOwner
	}
	allowed, err := query.CanReadRecord(ctx, parent)
	if err != nil {
		return nil, mindruntime.OwnedBudgetReconciliationScope{}, err
	}
	if !allowed {
		return nil, mindruntime.OwnedBudgetReconciliationScope{}, coordination.ErrWrongOwner
	}
	version := parent.StatusVersion
	fingerprint := parent.RecoveryDescriptor.Fingerprint
	parentStatus := parent.Status
	authorized := coordination.WithScope(ctx, query.ExecutionScope)
	authorized = coordination.WithAdditionalGuard(authorized, func(current context.Context) error {
		latest, exists, err := getter.Get(current, parentID)
		if err != nil {
			return err
		}
		if !exists || latest == nil || latest.ID != parentID || latest.StatusVersion != version || latest.Status != parentStatus || latest.Scope.Source == "multi_agent" || latest.Scope.SpaceID != query.ExecutionScope.SpaceID || latest.Scope.CharacterID != query.ExecutionScope.RoleID || latest.Scope.ConversationID != query.ConversationID || latest.RecoveryDescriptor == nil || latest.RecoveryDescriptor.Fingerprint != fingerprint || latest.RecoveryDescriptor.MultiAgent == nil || latest.RecoveryDescriptor.MultiAgent.CoordinationID != coordinationID || latest.RecoveryDescriptor.MultiAgent.ResourceOwnerID != query.ExecutionScope.ResourceOwnerID || latest.RecoveryDescriptor.MultiAgent.ExecutionCoreID != query.ExecutionScope.CoreID || !validateCheckpoint(latest.RecoveryDescriptor.MultiAgent) {
			return coordination.ErrScopeExpired
		}
		if latest.RecoveryDescriptor.Fingerprint != "" {
			copyOfDescriptor := *latest.RecoveryDescriptor
			copyOfDescriptor.ComputeFingerprint()
			if copyOfDescriptor.Fingerprint != latest.RecoveryDescriptor.Fingerprint {
				return coordination.ErrScopeExpired
			}
		}
		stillAllowed, err := query.CanReadRecord(current, latest)
		if err != nil {
			return err
		}
		if !stillAllowed {
			return coordination.ErrWrongOwner
		}
		return nil
	})
	scope := mindruntime.OwnedBudgetReconciliationScope{SpaceID: query.ExecutionScope.SpaceID, RoleID: query.ExecutionScope.RoleID, OwnerID: query.ExecutionScope.ResourceOwnerID, ParentInteractionID: parentID, CoordinationID: coordinationID}
	return authorized, scope, nil
}
