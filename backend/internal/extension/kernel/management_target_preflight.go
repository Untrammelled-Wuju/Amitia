package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

type PackageTargetPreview struct {
	ExtensionID      string
	ManagementTarget domain.ManagementTarget
	Installable      bool
}

type PackageTargetPreflight interface {
	ValidateArchiveTarget(ctx context.Context, archivePath string, expected domain.ManagementTarget) (*PackageTargetPreview, error)
}

type TargetMutationGuard struct {
	previewFn func(ctx context.Context, archivePath string) (InstallPreview, error)
}

func NewTargetMutationGuard(previewFn func(ctx context.Context, archivePath string) (InstallPreview, error)) *TargetMutationGuard {
	return &TargetMutationGuard{previewFn: previewFn}
}

func (g *TargetMutationGuard) ValidateArchiveTarget(ctx context.Context, archivePath string, expected domain.ManagementTarget) (*PackageTargetPreview, error) {
	if g.previewFn == nil {
		return nil, fmt.Errorf("target mutation guard: preview function unavailable")
	}
	preview, err := g.previewFn(ctx, archivePath)
	if err != nil {
		return nil, fmt.Errorf("target mutation guard: archive preview failed: %w", err)
	}
	if !preview.Installable {
		return nil, fmt.Errorf("target mutation guard: archive is not installable")
	}
	contributions := make([]plugincontract.ContributionDescriptor, 0)
	for _, module := range preview.Manifest.Modules {
		contributions = append(contributions, module.Contributions...)
	}
	target, err := resolveManagementTarget(contributions)
	if err != nil {
		return nil, fmt.Errorf("target mutation guard: cannot determine management target: %w", err)
	}
	if target != expected {
		return nil, fmt.Errorf("target mutation guard: expected %s, got %s", expected, target)
	}
	return &PackageTargetPreview{
		ExtensionID:      preview.ExtensionID,
		ManagementTarget: target,
		Installable:      true,
	}, nil
}

func resolveManagementTarget(contributions []plugincontract.ContributionDescriptor) (domain.ManagementTarget, error) {
	adapters := make([]domain.ContributionAdapter, 0, len(contributions))
	for _, c := range contributions {
		adapters = append(adapters, domain.ContributionAdapter(c.Adapter))
	}
	extensionDomain, err := domain.ResolveDomainFromAdapters(adapters)
	if err != nil {
		return "", err
	}
	target, err := domain.ManagementTargetForDomain(extensionDomain)
	if err != nil {
		return "", err
	}
	return target, nil
}

func (r *Runtime) PreviewArchiveTarget(ctx context.Context, archivePath string) (InstallPreview, error) {
	return r.PreviewInstall(ctx, archivePath)
}
