package ui_provider

import "testing"

func TestBundledTrustAllowsRootProviders(t *testing.T) {
	definition := ProviderDefinition{
		ProviderID:  "bundled-page",
		ExtensionID: "com.example/bundled",
		Capability:  CapabilityPageProvider,
		Mode:        ModeReplace,
		Placement:   PlacementDevice,
		TrustLevel:  "bundled",
		Entries: map[string]Entry{
			"web": {Type: EntrySchemaRenderer, ContributionID: "bundled-page"},
		},
	}
	if err := definition.Validate(); err != nil {
		t.Fatalf("bundled root provider rejected: %v", err)
	}
}
