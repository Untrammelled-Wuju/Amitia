package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

type PackageMigrationGuard struct {
	core *migration.ReversibleMigrationCore
}

func NewPackageMigrationGuard(repo *migration.MigrationRepository) *PackageMigrationGuard {
	return &PackageMigrationGuard{core: migration.NewReversibleMigrationCore(repo)}
}

func (g *PackageMigrationGuard) PreflightManifest(ctx context.Context, manifest manifest_v2.Manifest, fromVersion string) (*migration.ReversiblePreflight, error) {
	if g == nil || g.core == nil {
		return nil, fmt.Errorf("migration: package guard unavailable")
	}
	definitions, err := manifestMigrationDefinitions(manifest)
	if err != nil {
		return nil, err
	}
	return g.core.Preflight(ctx, migration.ReversiblePreflightInput{
		ExtensionID: manifest.Extension.ID,
		FromVersion: fromVersion,
		ToVersion:   manifest.Extension.Version,
		Definitions: definitions,
	})
}

func (g *PackageMigrationGuard) ExecuteForward(ctx context.Context, request migration.ReversibleExecutionRequest, handler migration.ReversibleStepHandler) (*migration.MigrationOperation, error) {
	if g == nil || g.core == nil {
		return nil, fmt.Errorf("migration: package guard unavailable")
	}
	return g.core.ExecuteForward(ctx, request, handler)
}

func (g *PackageMigrationGuard) CompensateReverse(ctx context.Context, request migration.ReversibleExecutionRequest, handler migration.ReversibleStepHandler) error {
	if g == nil || g.core == nil {
		return fmt.Errorf("migration: package guard unavailable")
	}
	return g.core.CompensateReverse(ctx, request, handler)
}

func manifestMigrationDefinitions(manifest manifest_v2.Manifest) ([]migration.MigrationDefinition, error) {
	if len(manifest.Extension.Metadata) == 0 {
		return nil, fmt.Errorf("migration: manifest migrations missing")
	}
	raw, ok := manifest.Extension.Metadata["migrations"]
	if !ok {
		raw, ok = manifest.Extension.Metadata["migration"]
	}
	if !ok {
		return nil, fmt.Errorf("migration: manifest migrations missing")
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("migration: marshal manifest migrations: %w", err)
	}
	var definitions []migration.MigrationDefinition
	if err := json.Unmarshal(payload, &definitions); err == nil && len(definitions) > 0 {
		return definitions, nil
	}
	var wrapper struct {
		Definitions []migration.MigrationDefinition `json:"definitions"`
		Migrations  []migration.MigrationDefinition `json:"migrations"`
	}
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		return nil, fmt.Errorf("migration: decode manifest migrations: %w", err)
	}
	definitions = wrapper.Definitions
	if len(definitions) == 0 {
		definitions = wrapper.Migrations
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("migration: manifest migrations empty")
	}
	return definitions, nil
}
