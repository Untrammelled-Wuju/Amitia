package kernel

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestBuildModuleInstanceSpecUsesRuntimeTimeout(t *testing.T) {
	spec := buildModuleInstanceSpec(
		domain.ExtensionID("com.example/extension"),
		domain.ModuleID("runtime"),
		&domain.RuntimeDefinition{
			Type:       domain.RuntimeType("javascript"),
			EntryPoint: "index.js",
			Timeout:    10 * time.Minute,
		},
		1,
	)
	if spec.Limits.MaxExecutionTime != 10*time.Minute {
		t.Fatalf("max execution time = %s, want 10m", spec.Limits.MaxExecutionTime)
	}
}
