package gamehost

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/integration"
	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
)

// kernelContributionSource 将 kernel Container 适配到 integration.KernelContributionSource。
// 启用条件 = InstallationState == installed && EnablementState == enabled。
type kernelContributionSource struct {
	instRepo    installationLister
	defRepo     definitionLister
	contribRepo contributionLister
}

type installationLister interface {
	ListInstallations(ctx context.Context) ([]kerneldomain.ExtensionInstallation, error)
}

type definitionLister interface {
	GetExtension(ctx context.Context, id kerneldomain.ExtensionID, version kerneldomain.SemanticVersion) (kerneldomain.ExtensionDefinition, error)
}

type contributionLister interface {
	ListContributions(ctx context.Context, extensionID kerneldomain.ExtensionID) ([]kerneldomain.ContributionDefinition, error)
}

func newKernelContributionSource(
	instRepo installationLister,
	defRepo definitionLister,
	contribRepo contributionLister,
) *kernelContributionSource {
	return &kernelContributionSource{
		instRepo:    instRepo,
		defRepo:     defRepo,
		contribRepo: contribRepo,
	}
}

func (s *kernelContributionSource) ListEnabledGamePlugins(ctx context.Context) ([]integration.KernelGamePlugin, error) {
	installations, err := s.instRepo.ListInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list extension installations: %w", err)
	}

	var result []integration.KernelGamePlugin
	for _, inst := range installations {
		if inst.InstallationState != kerneldomain.InstallationStateInstalled {
			continue
		}
		if inst.EnablementState != kerneldomain.EnablementEnabled {
			continue
		}

		def, err := s.defRepo.GetExtension(ctx, inst.ExtensionID, inst.InstalledVersion)
		if err != nil {
			return nil, fmt.Errorf("load enabled game plugin definition %q: %w", inst.ExtensionID, err)
		}

		contribs, err := s.contribRepo.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
			return nil, fmt.Errorf("load enabled game plugin contributions %q: %w", inst.ExtensionID, err)
		}

		for _, c := range contribs {
			if c.Kind != kerneldomain.ContributionKindGamePlugin {
				continue
			}
			result = append(result, integration.KernelGamePlugin{
				Extension:    def,
				Contribution: c,
			})
		}
	}
	return result, nil
}
