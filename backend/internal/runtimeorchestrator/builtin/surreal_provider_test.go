// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package builtin

import (
	"context"
	"testing"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

func TestSurrealProviderExists(t *testing.T) {
	factory := NewSurrealDBProviderFactory()
	if factory == nil {
		t.Fatal("expected NewSurrealDBProviderFactory to return non-nil")
	}

	ctx := runtimeorchestrator.ProviderBuildContext{
		Config: &config.Config{
			Providers: config.ProvidersConfig{
				GraphStore: config.GraphStoreProviderConfig{
					Enabled: true,
					SurrealDB: config.SurrealConfig{
						Port: 18000,
					},
				},
			},
		},
	}

	inst, err := factory.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if inst == nil {
		t.Fatal("expected Build to return non-nil instance")
	}
	_ = context.Background()
}
