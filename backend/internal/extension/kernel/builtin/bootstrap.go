package builtin

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type EnableExtensionFunc func(ctx context.Context, extensionID domain.ExtensionID) error

type Bootstrapper struct {
	catalog            *Catalog
	definitions        domain.DefinitionRepository
	modules            sqlite.ModuleRepository
	contributions      sqlite.ContributionRepository
	installations      domain.InstallationRepository
	providerReconciler *capability.ExtensionProviderReconciler
	enableFunc         EnableExtensionFunc
}

func NewBootstrapper(
	catalog *Catalog,
	definitions domain.DefinitionRepository,
	installations domain.InstallationRepository,
) *Bootstrapper {
	return &Bootstrapper{
		catalog:       catalog,
		definitions:    definitions,
		installations: installations,
	}
}

func (b *Bootstrapper) SetModuleRepository(repo sqlite.ModuleRepository) {
	b.modules = repo
}

func (b *Bootstrapper) SetContributionRepository(repo sqlite.ContributionRepository) {
	b.contributions = repo
}

func (b *Bootstrapper) SetProviderReconciler(reconciler *capability.ExtensionProviderReconciler) {
	b.providerReconciler = reconciler
}

func (b *Bootstrapper) SetEnableFunc(fn EnableExtensionFunc) {
	b.enableFunc = fn
}

func (b *Bootstrapper) Reconcile(ctx context.Context) error {
	if b.catalog == nil {
		return fmt.Errorf("builtin bootstrapper: catalog is nil")
	}
	if b.definitions == nil {
		return fmt.Errorf("builtin bootstrapper: definition repository is nil")
	}
	if b.installations == nil {
		return fmt.Errorf("builtin bootstrapper: installation repository is nil")
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

	existing, err := b.definitions.GetExtension(ctx, extID, extDef.Version)
	if err == nil {
		if existing.Version.String() != extDef.Version.String() {
			log.Printf("[builtin-bootstrap] upgrading %s from %s to %s", extID, existing.Version.String(), extDef.Version.String())
		}
	}

	if err := b.definitions.PutExtension(ctx, extDef); err != nil {
		return fmt.Errorf("persist definition: %w", err)
	}

	for _, mod := range extDef.Modules {
		if b.modules != nil {
			if err := b.modules.PutModule(ctx, mod); err != nil {
				return fmt.Errorf("persist module %s: %w", mod.ID, err)
			}
		}
	}

	for _, contrib := range extDef.AllContributions() {
		if b.contributions != nil {
			if err := b.contributions.PutContribution(ctx, contrib); err != nil {
				return fmt.Errorf("persist contribution %s: %w", contrib.ID, err)
			}
		}
	}

	inst, instErr := b.installations.GetInstallation(ctx, extID)
	desiredEnabled := def.Required || def.ShouldEnable()

	if instErr != nil {
		inst = domain.ExtensionInstallation{
			InstallationID:   string(extID),
			ExtensionID:      extID,
			InstalledVersion: extDef.Version,
			EnablementState:  domain.EnablementDisabled,
			InstalledAt:      time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
			Metadata:         map[string]any{"source": "builtin", "immutablePackage": true},
		}
		if err := b.installations.PutInstallation(ctx, inst); err != nil {
			return fmt.Errorf("persist installation: %w", err)
		}
	} else if inst.InstalledVersion.String() != extDef.Version.String() {
		inst.InstalledVersion = extDef.Version
		inst.UpdatedAt = time.Now().UTC()
		if err := b.installations.PutInstallation(ctx, inst); err != nil {
			return fmt.Errorf("update installation version: %w", err)
		}
	}

	if inst.IsUserDisabled() {
		desiredEnabled = false
	}

	if b.providerReconciler != nil {
		if err := b.providerReconciler.ReconcileDefinitions(extDef); err != nil {
			log.Printf("[builtin-bootstrap] provider reconcile %s failed: %v", extID, err)
		}
	}

	if desiredEnabled && inst.AllowsEnable() && b.enableFunc != nil {
		log.Printf("[builtin-bootstrap] enabling %s", extID)
		if err := b.enableFunc(ctx, extID); err != nil {
			log.Printf("[builtin-bootstrap] enable %s failed: %v", extID, err)
			return fmt.Errorf("enable builtin %s: %w", extID, err)
		}
	}

	return nil
}
