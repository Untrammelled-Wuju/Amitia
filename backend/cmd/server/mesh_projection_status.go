package main

import (
	"context"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (p *meshLocalDataPort) ProjectionState(ctx context.Context, scope coordination.ExecutionScope, rebuild bool) (coordination.ProjectionStatus, error) {
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return coordination.ProjectionStatus{}, coordination.ErrWrongOwner
	}
	unlock := character.LockRuntimeRole(p.services.DB)
	defer unlock()
	roles, err := p.rolesLocked(ctx, scope)
	if err != nil {
		return coordination.ProjectionStatus{}, err
	}
	role, err := coordination.ResolveRole(scope.RoleID, roles)
	if err != nil || role.Revision != scope.RoleRevision {
		return coordination.ProjectionStatus{}, coordination.ErrScopeExpired
	}
	if rebuild {
		if err := coordination.CommitCurrent(ctx, func() error { return p.store.RebuildProjections(ctx, scope.RoleID) }); err != nil {
			return coordination.ProjectionStatus{}, err
		}
	} else if err := coordination.ValidateCurrent(ctx); err != nil {
		return coordination.ProjectionStatus{}, err
	}
	return p.store.ProjectionStatus(ctx, scope.RoleID)
}
