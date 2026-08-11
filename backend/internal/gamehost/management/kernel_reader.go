package management

import (
	"context"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
)

type KernelReader struct {
	DefinitionRepo   kerneldomain.DefinitionRepository
	InstallationRepo kerneldomain.InstallationRepository
}

func NewKernelReader(defRepo kerneldomain.DefinitionRepository, instRepo kerneldomain.InstallationRepository) *KernelReader {
	return &KernelReader{
		DefinitionRepo:   defRepo,
		InstallationRepo: instRepo,
	}
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
