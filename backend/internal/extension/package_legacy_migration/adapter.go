//go:build legacy_migration

package package_legacy_migration

import (
	"context"

	"github.com/u-ai/backend/internal/extension"
)

type StateReader interface {
	LegacyMigrationReadAllowed(context.Context, string) bool
}

func Context(ctx context.Context) context.Context {
	return extension.WithLegacyMigrationToolContext(ctx)
}

func ReadAllowed(ctx context.Context, reader StateReader, extensionID string) bool {
	if reader == nil {
		return false
	}
	return reader.LegacyMigrationReadAllowed(Context(ctx), extensionID)
}
