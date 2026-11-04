package kernel

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
)

func TestPluginMarketContributionsUseGenericHostAdapters(t *testing.T) {
	definition := builtin.BuildPluginMarketExtension("1.0.0")
	container := &Container{
		ToolRegistry:       capability.NewToolRegistry(),
		UIHost:             ui_contribution.NewUIHost(),
		UIProviderRegistry: ui_provider.NewRegistry(),
		SchemaValidator:    schema_ui.NewValidator(),
		SchemaRegistry:     schema_ui.NewSchemaRegistry(schema_ui.NewValidator(), nil),
	}
	installer := NewTypedContributionInstaller(container)
	for _, contribution := range definition.Extension.AllContributions() {
		if contribution.Kind == domain.ContributionKindUIProvider {
			if builtinProvider, _ := contribution.Definition["builtin"].(bool); builtinProvider {
				t.Fatalf("plugin market provider %s must remain an ordinary provider", contribution.ID)
			}
		}
		if _, err := installer.buildInstallOp(context.Background(), contribution, 1); err != nil {
			t.Fatalf("build contribution %s: %v", contribution.ID, err)
		}
	}
}
