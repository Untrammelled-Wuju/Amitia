package binary

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type CleanupManager struct {
	resolver *Resolver
}

func NewCleanupManager(resolver *Resolver) *CleanupManager {
	return &CleanupManager{resolver: resolver}
}

func (m *CleanupManager) ReleaseObject(
	ctx context.Context,
	owner BinaryOwner,
	id BinaryObjectID,
) error {
	return m.resolver.Release(ctx, owner, id)
}

func (m *CleanupManager) ReleaseByService(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
) error {
	return m.resolver.ReleaseByService(ctx, runtimeID, serviceID)
}

func (m *CleanupManager) ReleaseByRuntime(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
) error {
	return m.resolver.ReleaseByRuntime(ctx, runtimeID)
}

func (m *CleanupManager) Shutdown(ctx context.Context) error {
	return m.resolver.Shutdown(ctx)
}
