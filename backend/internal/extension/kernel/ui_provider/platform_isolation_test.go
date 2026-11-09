package ui_provider

import "testing"

func TestProviderRejectsSharedCrossPlatformContribution(t *testing.T) {
	definition := ProviderDefinition{
		ProviderID:  "page-provider",
		ExtensionID: "com.example/ui",
		Capability:  CapabilityPageProvider,
		Mode:        ModeReplace,
		Placement:   PlacementCloud,
		TrustLevel:  "official",
		Enabled:     true,
		Entries: map[string]Entry{
			"electron_windows": {Type: EntrySchemaRenderer, ContributionID: "shared-page"},
			"mobile":           {Type: EntrySchemaRenderer, ContributionID: "shared-page"},
		},
	}
	if err := definition.Validate(); err == nil {
		t.Fatal("expected shared desktop/mobile contribution to be rejected")
	}
}

func TestProviderAllowsSinglePlatform(t *testing.T) {
	definition := ProviderDefinition{
		ProviderID:  "page-provider",
		ExtensionID: "com.example/ui",
		Capability:  CapabilityPageProvider,
		Mode:        ModeReplace,
		Placement:   PlacementCloud,
		TrustLevel:  "official",
		Enabled:     true,
		Entries: map[string]Entry{
			"mobile": {Type: EntrySchemaRenderer, ContributionID: "mobile-page"},
		},
	}
	if err := definition.Validate(); err != nil {
		t.Fatalf("single-platform provider should pass: %v", err)
	}
}

func TestAmitiaNamespaceDoesNotBypassPublisherTrust(t *testing.T) {
	definition := ProviderDefinition{
		ProviderID:  "plugin-market-page-provider",
		ExtensionID: "com.amitia/plugin-market",
		Capability:  CapabilityPageProvider,
		Mode:        ModeReplace,
		Placement:   PlacementCloud,
		TrustLevel:  "unknown",
		Enabled:     true,
		Entries: map[string]Entry{
			"mobile": {Type: EntrySchemaRenderer, ContributionID: "plugin-market-mobile-page"},
		},
	}
	if err := definition.Validate(); err == nil {
		t.Fatal("com.amitia namespace must not bypass explicit provider trust")
	}
}
