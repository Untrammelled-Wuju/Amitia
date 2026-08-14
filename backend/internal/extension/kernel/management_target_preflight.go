package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
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
	contributions := preview.Manifest.AllContributions()
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

func resolveManagementTarget(contributions []manifest_v2.ContributionMeta) (domain.ManagementTarget, error) {
	kinds := make([]domain.ContributionKind, 0, len(contributions))
	for _, c := range contributions {
		kinds = append(kinds, domain.ContributionKind(c.Kind))
	}
	domain, err := domain.ResolveDomainFromKinds(kinds)
	if err != nil {
		return "", err
	}
	target, err := domain.ManagementTargetForDomain(domain)
	if err != nil {
		return "", err
	}
	return target, nil
}

func (r *Runtime) PreviewArchiveTarget(ctx context.Context, archivePath string) (InstallPreview, error) {
	return r.PreviewInstall(ctx, archivePath)
}
