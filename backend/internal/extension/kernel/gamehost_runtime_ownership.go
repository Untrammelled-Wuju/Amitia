package kernel

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

// gameHostOwnedRuntimeModules returns module IDs whose process lifecycle is
// exclusively owned by GameHost. Those modules must never be reconciled by the
// generic Kernel RuntimeSupervisor as that would create a second host for the
// same game runtime.
func (r *Runtime) gameHostOwnedRuntimeModules(ctx context.Context, extensionID domain.ExtensionID) (map[domain.ModuleID]struct{}, error) {
	owned := make(map[domain.ModuleID]struct{})
	if r == nil || r.container == nil || r.container.ContributionRepository == nil {
		return owned, nil
	}
	contributions, err := r.container.ContributionRepository.ListContributions(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("list contributions for game runtime ownership: %w", err)
	}
	for _, contribution := range contributions {
		if contribution.Kind != domain.ContributionKindGamePlugin {
			continue
		}
		raw, _ := contribution.Definition["runtimeModuleId"].(string)
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("game_plugin %s is missing runtimeModuleId", contribution.ID)
		}
		owned[domain.ModuleID(raw)] = struct{}{}
	}
	return owned, nil
}

func isGameHostOwnedRuntimeModule(owned map[domain.ModuleID]struct{}, moduleID domain.ModuleID) bool {
	_, ok := owned[moduleID]
	return ok
}

func (r *Runtime) reconcileGameHostExtension(ctx context.Context, extensionID string) error {
	if r == nil || r.container == nil || r.container.GameHost == nil {
		return nil
	}
	return r.container.GameHost.ReconcileExtension(ctx, extensionID)
}
