package main

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	amitialog "log"
)

func (p *meshLocalDataPort) MemoryDeletionComplete(ctx context.Context, scope coordination.ExecutionScope, id string) (bool, error) {
	if scope.ResourceOwnerID != p.ownerID {
		return false, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return false, err
	}
	return p.store.MemoryDeletionComplete(ctx, coordination.StorageRole(scope, "memory"), id)
}

func (p *meshLocalDataPort) CleanupMemoryDeletions(ctx context.Context, scope coordination.ExecutionScope) (int, error) {
	if scope.ResourceOwnerID != p.ownerID {
		return 0, fmt.Errorf("清理目标Owner不匹配: %w", coordination.ErrWrongOwner)
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return 0, fmt.Errorf("清理授权校验失败: %w", err)
	}
	jobs, err := p.store.PendingDeletionProjections(ctx)
	if err != nil {
		return 0, err
	}
	writer := &meshProjectionWriter{port: p, collections: map[string]bool{}}
	count := 0
	for _, job := range jobs {
		if job.Source != nil && !job.Source.Deleted && !job.Resource.Deleted {
			continue
		}
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return count, err
		}
		var location string
		if job.Resource.Kind == "vector" {
			location, err = writer.vector(ctx, job, false)
		} else {
			location, err = writer.graph(ctx, job, false)
		}
		if err != nil {
			if err := p.store.FinishProjection(ctx, job, location, false); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return count, fmt.Errorf("清理%s索引失败: %w", job.Resource.Kind, err)
		}
		if err := p.store.FinishProjection(ctx, job, location, true); err != nil {
			return count, fmt.Errorf("索引清理回执保存失败: %w", err)
		}
		count++
	}
	return count, coordination.ValidateCurrent(ctx)
}

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
		if err := coordination.CommitCurrent(ctx, func() error { return p.store.RebuildProjections(ctx, coordination.StorageRole(scope, "memory")) }); err != nil {
			return coordination.ProjectionStatus{}, err
		}
	} else if err := coordination.ValidateCurrent(ctx); err != nil {
		return coordination.ProjectionStatus{}, err
	}
	return p.store.ProjectionStatus(ctx, coordination.StorageRole(scope, "memory"))
}
