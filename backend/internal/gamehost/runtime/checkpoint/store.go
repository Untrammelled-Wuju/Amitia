package checkpoint

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type CheckpointStore interface {
	SaveMetadata(ctx context.Context, metadata RuntimeMetadata) error
	LoadMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeMetadata, error)
	HasMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) (bool, error)

	SaveCheckpoint(ctx context.Context, checkpoint RuntimeCheckpoint) error
	LoadCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) (RuntimeCheckpoint, error)

	DeleteMetadata(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
	DeleteCheckpoint(ctx context.Context, runtimeID domain.RuntimeInstanceID) error
}
