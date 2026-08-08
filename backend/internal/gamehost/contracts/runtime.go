package contracts

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeManager interface {
	Create(
		ctx context.Context,
		pluginID domain.PluginID,
	) (*domain.RuntimeInstance, error)

	Get(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) (*domain.RuntimeInstance, error)

	List(
		ctx context.Context,
	) ([]domain.RuntimeInstance, error)
}
