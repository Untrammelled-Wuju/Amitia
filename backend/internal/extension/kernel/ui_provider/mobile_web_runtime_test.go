package ui_provider

import (
	"strings"
	"testing"
)

func mobileWebRuntimeDefinition() ProviderDefinition {
	return ProviderDefinition{
		ProviderID:  "dsh-host-mobile",
		ExtensionID: "com.amitia/dsh-compatible-host",
		ModuleID:    "dsh-compatible-ui",
		Capability:  CapabilityExtensionPage,
		Mode:        ModeReplace,
		Placement:   PlacementAny,
		TrustLevel:  "official",
		Enabled:     true,
		Entries: map[string]Entry{
			"android": {
				Type:        EntryMobileWebRuntime,
				Path:        "modules/dsh-compatible-ui/mobile/index.mjs",
				ContentHash: "sha256:fixture",
			},
			"ios": {
				Type:        EntryMobileWebRuntime,
				Path:        "modules/dsh-compatible-ui/mobile/index.mjs",
				ContentHash: "sha256:fixture",
			},
		},
	}
}

func TestMobileWebRuntimeProviderValidation(t *testing.T) {
	definition := mobileWebRuntimeDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileWebRuntimeRejectsDesktopPlatform(t *testing.T) {
	definition := mobileWebRuntimeDefinition()
	definition.Entries = map[string]Entry{
		"windows": {
			Type:        EntryMobileWebRuntime,
			Path:        "modules/page/mobile/index.mjs",
			ContentHash: "sha256:fixture",
		},
	}
	err := definition.Validate()
	if err == nil || !strings.Contains(err.Error(), "Android/iOS/mobile") {
		t.Fatalf("expected mobile platform rejection, got %v", err)
	}
}

func TestMobileWebRuntimeRequiresContentHash(t *testing.T) {
	definition := mobileWebRuntimeDefinition()
	definition.Entries = map[string]Entry{
		"android": {
			Type: EntryMobileWebRuntime,
			Path: "modules/page/mobile/index.mjs",
		},
	}
	err := definition.Validate()
	if err == nil || !strings.Contains(err.Error(), "contentHash required") {
		t.Fatalf("expected content hash rejection, got %v", err)
	}
}
