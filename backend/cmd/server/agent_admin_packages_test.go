package main

import (
	"context"
	"testing"

	extensionkernel "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestListKernelPackagesUsesKernelInstallationRepository(t *testing.T) {
	ctx := context.Background()
	version := domain.SemanticVersion{Major: 1, Minor: 2, Patch: 3}
	definitions := domain.NewInMemoryDefinitionRepository()
	installations := domain.NewInMemoryInstallationRepository()
	if err := definitions.PutExtension(ctx, domain.ExtensionDefinition{
		ID:      domain.ExtensionID("com.example/demo"),
		Name:    domain.LocalizedText{Default: "Demo", Translations: map[string]string{"zh-CN": "示例"}},
		Version: version,
		Publisher: domain.PublisherReference{
			PublisherID: "example",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := installations.PutInstallation(ctx, domain.ExtensionInstallation{
		ExtensionID:       domain.ExtensionID("com.example/demo"),
		InstalledVersion:  version,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementDisabled,
	}); err != nil {
		t.Fatal(err)
	}

	items, err := listKernelPackages(ctx, &extensionkernel.Container{
		DefinitionRepository:   definitions,
		InstallationRepository: installations,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0]["packageName"] != "com.example/demo" || items[0]["name"] != "示例" || items[0]["version"] != "1.2.3" || items[0]["publisher"] != "example" || items[0]["enabled"] != false {
		t.Fatalf("unexpected package: %#v", items[0])
	}
}
