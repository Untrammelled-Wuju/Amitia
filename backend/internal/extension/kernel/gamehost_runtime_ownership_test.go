package kernel

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestGameHostContributionRuntimeModulesSupportsServicesOnly(t *testing.T) {
	contribution := domain.ContributionDefinition{
		ID:   "game",
		Kind: domain.ContributionKindGamePlugin,
		Definition: map[string]any{
			"protocolVersion": "amitia-game-host/1",
			"hostFeatures":    []any{"multi_service"},
			"services": []any{
				map[string]any{"id": "control", "moduleId": "module-control"},
				map[string]any{"id": "events", "moduleId": "module-events"},
			},
			"network": map[string]any{"mode": "none"},
		},
	}
	got, err := gameHostContributionRuntimeModules(contribution)
	if err != nil {
		t.Fatalf("gameHostContributionRuntimeModules: %v", err)
	}
	want := []domain.ModuleID{"module-control", "module-events"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("modules=%v want %v", got, want)
	}
}

func TestGameHostContributionRuntimeModulesSupportsCompatibleLegacyRuntimeModuleID(t *testing.T) {
	contribution := domain.ContributionDefinition{
		ID:   "game",
		Kind: domain.ContributionKindGamePlugin,
		Definition: map[string]any{
			"protocolVersion": "amitia-game-host/1",
			"runtimeModuleId": "module-control",
			"hostFeatures":    []any{"multi_service"},
			"services": []any{
				map[string]any{"id": "control", "moduleId": "module-control"},
				map[string]any{"id": "events", "moduleId": "module-events"},
			},
			"network": map[string]any{"mode": "none"},
		},
	}
	got, err := gameHostContributionRuntimeModules(contribution)
	if err != nil {
		t.Fatalf("gameHostContributionRuntimeModules: %v", err)
	}
	want := []domain.ModuleID{"module-control", "module-events"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("modules=%v want %v", got, want)
	}
}
