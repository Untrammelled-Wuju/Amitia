//go:build legacy_migration

package package_legacy_migration

import (
	"context"
	"testing"
)

type migrationStateReader struct {
	allowed bool
}

func (r migrationStateReader) LegacyMigrationReadAllowed(ctx context.Context, extensionID string) bool {
	return r.allowed && ctx != nil && extensionID == "com.example/manual"
}

func TestAdapterRequiresExplicitReaderAndScopesContext(t *testing.T) {
	if ReadAllowed(context.Background(), nil, "com.example/manual") {
		t.Fatal("nil migration reader allowed legacy access")
	}
	if !ReadAllowed(context.Background(), migrationStateReader{allowed: true}, "com.example/manual") {
		t.Fatal("explicit migration adapter rejected scoped access")
	}
}
