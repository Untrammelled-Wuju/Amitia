package contracts

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Supervisor interface {
	Health(
		ctx context.Context,
		runtimeID domain.RuntimeInstanceID,
	) (domain.HealthState, error)
}
