package kernel

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
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
		moduleIDs, err := gameHostContributionRuntimeModules(contribution)
		if err != nil {
			return nil, err
		}
		for _, moduleID := range moduleIDs {
			owned[moduleID] = struct{}{}
		}
	}
	return owned, nil
}

func gameHostContributionRuntimeModules(contribution domain.ContributionDefinition) ([]domain.ModuleID, error) {
	spec, err := gameprotocol.ParsePluginHostSpec(contribution.Definition)
	if err != nil {
		return nil, fmt.Errorf("game_plugin %s has invalid host spec: %w", contribution.ID, err)
	}
	if err := spec.Validate(); err != nil {
		return nil, fmt.Errorf("game_plugin %s has invalid host spec: %w", contribution.ID, err)
	}
	seen := make(map[domain.ModuleID]struct{})
	if runtimeModuleID := strings.TrimSpace(spec.RuntimeModuleID); runtimeModuleID != "" {
		seen[domain.ModuleID(runtimeModuleID)] = struct{}{}
	}
	for _, service := range spec.Services {
		moduleID := strings.TrimSpace(service.ModuleID)
		if moduleID != "" {
			seen[domain.ModuleID(moduleID)] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("game_plugin %s does not declare any runtime module", contribution.ID)
	}
	result := make([]domain.ModuleID, 0, len(seen))
	for moduleID := range seen {
		result = append(result, moduleID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
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

func (r *Runtime) finalizeGameHostExtensionUninstall(ctx context.Context, extensionID string) error {
	if r == nil || r.container == nil || r.container.GameHost == nil {
		return nil
	}
	return r.container.GameHost.FinalizeExtensionUninstall(ctx, extensionID)
}

func (r *Runtime) prepareGameHostExtensionAfterPackageGenerationChange(ctx context.Context, extensionID string) error {
	if r == nil || r.container == nil || r.container.GameHost == nil {
		return nil
	}
	return r.container.GameHost.PrepareExtensionAfterPackageGenerationChange(ctx, extensionID)
}
