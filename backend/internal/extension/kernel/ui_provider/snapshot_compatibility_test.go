package ui_provider

import (
	"context"
	"testing"
)

func TestSnapshotPublishesEveryProviderCompatibilityReason(t *testing.T) {
	registry := newCloudProfileTestRegistry(t)
	compatible := ProviderDefinition{
		ProviderID:  "test.snapshot.compatible",
		ExtensionID: "test.snapshot",
		Capability:  CapabilityPageProvider,
		Mode:        ModeReplace,
		Priority:    10,
		TrustLevel:  "trusted",
		Entries: map[string]Entry{
			"windows": {Type: EntrySchemaRenderer, ContributionID: "compatible"},
		},
		Enabled: true,
	}
	wrongPlatform := compatible
	wrongPlatform.ProviderID = "test.snapshot.wrong-platform"
	wrongPlatform.DeviceRequirements = &DeviceRequirements{Platforms: []string{"macos"}}
	disabled := compatible
	disabled.ProviderID = "test.snapshot.disabled"
	disabled.Enabled = false
	for _, definition := range []ProviderDefinition{compatible, wrongPlatform, disabled} {
		if err := registry.Register(definition); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := registry.SnapshotWithContext(context.Background(), ResolveContext{Platform: "windows", LocalRuntime: true})
	if got := snapshot.ProviderResolutions[compatible.ProviderID]; !got.Compatible || got.Reason != "" {
		t.Fatalf("compatible provider resolution missing: %+v", got)
	}
	if got := snapshot.ProviderResolutions[wrongPlatform.ProviderID]; got.Compatible || got.Reason != "device_platform_mismatch:windows" {
		t.Fatalf("wrong-platform reason missing: %+v", got)
	}
	if got := snapshot.ProviderResolutions[disabled.ProviderID]; got.Compatible || got.Reason != "provider_disabled" {
		t.Fatalf("disabled reason missing: %+v", got)
	}
}
