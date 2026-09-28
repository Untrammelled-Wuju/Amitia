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
