package management

import (
	"context"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type KernelReader struct {
	DefinitionRepo   kerneldomain.DefinitionRepository
	InstallationRepo kerneldomain.InstallationRepository
	ContributionRepo sqlite.ContributionRepository
}

func NewKernelReader(defRepo kerneldomain.DefinitionRepository, instRepo kerneldomain.InstallationRepository) *KernelReader {
	return &KernelReader{
		DefinitionRepo:   defRepo,
		InstallationRepo: instRepo,
	}
}

func NewKernelReaderWithContributions(defRepo kerneldomain.DefinitionRepository, instRepo kerneldomain.InstallationRepository, contribRepo sqlite.ContributionRepository) *KernelReader {
	return &KernelReader{
		DefinitionRepo:   defRepo,
		InstallationRepo: instRepo,
		ContributionRepo: contribRepo,
	}
}

func (r *KernelReader) ListGameCenterContributions(ctx context.Context, extensionID string) ([]kerneldomain.ContributionDefinition, error) {
	if r.ContributionRepo == nil {
		return nil, nil
	}
	contribs, err := r.ContributionRepo.ListContributions(ctx, kerneldomain.ExtensionID(extensionID))
	if err != nil {
		return nil, err
	}
	result := make([]kerneldomain.ContributionDefinition, 0, len(contribs))
	for _, c := range contribs {
		if c.Kind == kerneldomain.ContributionKindGamePlugin {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *KernelReader) ListGameCenterExtensions(ctx context.Context) ([]kerneldomain.ExtensionDefinition, []kerneldomain.ExtensionInstallation, error) {
	if r.DefinitionRepo == nil || r.InstallationRepo == nil {
		return nil, nil, nil
	}

	defs, err := r.DefinitionRepo.ListExtensions(ctx)
	if err != nil {
		return nil, nil, err
	}

	filtered := kerneldomain.FilterGameCenter(defs)

	insts, err := r.InstallationRepo.ListInstallations(ctx)
	if err != nil {
		return nil, nil, err
	}

	return filtered, insts, nil
}

func (r *KernelReader) GetGameCenterExtension(ctx context.Context, extensionID string) (*kerneldomain.ExtensionDefinition, *kerneldomain.ExtensionInstallation, error) {
	if r.DefinitionRepo == nil {
		return nil, nil, nil
	}

	defs, err := r.DefinitionRepo.ListExtensions(ctx)
	if err != nil {
		return nil, nil, err
	}

	var target *kerneldomain.ExtensionDefinition
	for i := range defs {
		if string(defs[i].ID) == extensionID && defs[i].Domain == kerneldomain.ExtensionDomainGame {
			target = &defs[i]
			break
		}
	}
	if target == nil {
		return nil, nil, nil
	}

	if r.InstallationRepo != nil {
		inst, err := r.InstallationRepo.GetInstallation(ctx, kerneldomain.ExtensionID(extensionID))
		if err == nil {
			return target, &inst, nil
		}
	}

	return target, nil, nil
}
