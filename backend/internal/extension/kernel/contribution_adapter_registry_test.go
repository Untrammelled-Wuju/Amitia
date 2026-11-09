package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestContributionInstallerUsesRegisteredPluginAdapter(t *testing.T) {
	registry, err := plugincontract.NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatalf("create adapter registry: %v", err)
	}
	installer := NewTypedContributionInstaller(&Container{PluginAdapters: registry})
	contribution := domain.ContributionDefinition{
		ID:          "tool",
		ModuleID:    "main",
		ExtensionID: "com.example/plugin",
		Kind:        domain.ContributionKindTool,
		Adapter:     domain.ContributionAdapterTool,
	}
	if err := installer.validateContributionAdapter(contribution); err != nil {
		t.Fatalf("registered adapter rejected: %v", err)
	}
}

func TestContributionInstallerRejectsUnsupportedContributionKind(t *testing.T) {
	registry, err := plugincontract.NewBuiltinAdapterRegistry()
	if err != nil {
		t.Fatalf("create adapter registry: %v", err)
	}
	installer := NewTypedContributionInstaller(&Container{PluginAdapters: registry})
	contribution := domain.ContributionDefinition{
		ID:          "tool",
		ModuleID:    "main",
		ExtensionID: "com.example/plugin",
		Kind:        domain.ContributionKindUIPage,
		Adapter:     domain.ContributionAdapterTool,
	}
	if err := installer.validateContributionAdapter(contribution); err == nil {
		t.Fatal("adapter accepted an unsupported contribution kind")
	}
}
