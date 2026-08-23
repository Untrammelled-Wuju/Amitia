package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/integration"
)

type gameHostInstalledGenerationResolver struct {
	store *PackageGenerationStore
}

func newGameHostInstalledGenerationResolver(store *PackageGenerationStore) integration.InstalledGenerationResolver {
	if store == nil {
		return nil
	}
	return &gameHostInstalledGenerationResolver{store: store}
}

func (r *gameHostInstalledGenerationResolver) ResolveInstalledGeneration(ctx context.Context, extensionID string) (integration.InstalledGeneration, error) {
	if r == nil || r.store == nil {
		return integration.InstalledGeneration{}, fmt.Errorf("package generation store is unavailable")
	}
	current, path, err := r.store.ResolveCurrentGeneration(ctx, extensionID)
	if err != nil {
		return integration.InstalledGeneration{}, err
	}
	return integration.InstalledGeneration{
		Path: path, GenerationID: current.GenerationID, TreeHash: current.TreeHash, Version: current.Version,
	}, nil
}
