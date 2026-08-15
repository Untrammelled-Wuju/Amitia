package builtin

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type Bootstrapper struct {
	catalog            *Catalog
	definitionRepo     domain.DefinitionRepository
	installationRepo   domain.InstallationRepository
	providerReconciler *capability.ExtensionProviderReconciler
}

func NewBootstrapper(
	catalog *Catalog,
	definitionRepo domain.DefinitionRepository,
	installationRepo domain.InstallationRepository,
) *Bootstrapper {
	return &Bootstrapper{
		catalog:          catalog,
		definitionRepo:   definitionRepo,
		installationRepo: installationRepo,
	}
}

func (b *Bootstrapper) SetProviderReconciler(reconciler *capability.ExtensionProviderReconciler) {
	b.providerReconciler = reconciler
}

func (b *Bootstrapper) Reconcile(ctx context.Context) error {
	if b.catalog == nil {
		return fmt.Errorf("builtin bootstrapper: catalog is nil")
	}
	if b.definitionRepo == nil {
		return fmt.Errorf("builtin bootstrapper: definition repository is nil")
	}

	defs := b.catalog.List()
	for _, def := range defs {
		if err := b.reconcileDefinition(ctx, def); err != nil {
			log.Printf("[builtin-bootstrap] reconcile %s failed: %v", def.Extension.ID, err)
			return fmt.Errorf("reconcile builtin %s: %w", def.Extension.ID, err)
		}
	}
	return nil
}

func (b *Bootstrapper) reconcileDefinition(ctx context.Context, def Definition) error {
	extID := def.Extension.ID
	extDef := def.WithMetadata()

	existing, err := b.definitionRepo.GetExtension(ctx, extID, extDef.Version)
	if err == nil && existing.Version.String() == extDef.Version.String() {
		return nil
	}
	if err == nil && existing.Version.String() != extDef.Version.String() {
		log.Printf("[builtin-bootstrap] upgrading %s from %s to %s", extID, existing.Version.String(), extDef.Version.String())
	}

	if err := b.definitionRepo.PutExtension(ctx, extDef); err != nil {
		return fmt.Errorf("persist definition: %w", err)
	}

	instID := domain.ExtensionID(fmt.Sprintf("builtin:%s", extID))
	_, instErr := b.installationRepo.GetInstallation(ctx, instID)
	if instErr != nil {
		inst := domain.ExtensionInstallation{
			InstallationID:   string(instID),
			ExtensionID:      extID,
			InstalledVersion: extDef.Version,
			EnablementState:  domain.EnablementEnabled,
			InstalledAt:      time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
			Metadata:         map[string]any{"source": "builtin", "immutablePackage": true},
		}
		if err := b.installationRepo.PutInstallation(ctx, inst); err != nil {
			return fmt.Errorf("persist installation: %w", err)
		}
	}

	return nil
}
