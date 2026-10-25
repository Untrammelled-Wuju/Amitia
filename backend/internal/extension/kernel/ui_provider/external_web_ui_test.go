package ui_provider

import (
	"strings"
	"testing"
)

func externalWebUITabDefinition() ProviderDefinition {
	return ProviderDefinition{
		ProviderID:  "service-webui",
		ExtensionID: "com.example/service",
		ModuleID:    "service-ui",
		Capability:  CapabilityWorkspaceTab,
		Mode:        ModeReplace,
		Placement:   PlacementDevice,
		TrustLevel:  "official",
		Enabled:     true,
		Entries: map[string]Entry{
			"electron_macos": {
				Type:           EntryExternalWebUI,
				URL:            "http://127.0.0.1:6099/webui/",
				AllowedOrigins: []string{"http://127.0.0.1:6099"},
			},
		},
		Metadata: map[string]any{
			"workspace":    "channel.workspace",
			"tabId":        "service-webui",
			"title":        "Service WebUI",
			"routePrefix":  "/channels/example",
			"contextMatch": map[string]any{"channelId": []any{"example"}},
			"keepMounted":  true,
			"storage":      "persistent",
		},
	}
}

func TestExternalWebUIWorkspaceTabValidation(t *testing.T) {
	definition := externalWebUITabDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestExternalWebUIRejectsUnlistedTargetOrigin(t *testing.T) {
	definition := externalWebUITabDefinition()
	entry := definition.Entries["electron_macos"]
	entry.AllowedOrigins = []string{"https://example.com"}
	definition.Entries["electron_macos"] = entry
	err := definition.Validate()
	if err == nil || !strings.Contains(err.Error(), "origin must be listed") {
		t.Fatalf("expected target origin rejection, got %v", err)
	}
}

func TestExternalWebUIRejectsUnsafeMetadata(t *testing.T) {
	definition := externalWebUITabDefinition()
	definition.Metadata["tabId"] = "../unsafe"
	err := definition.Validate()
	if err == nil || !strings.Contains(err.Error(), "safe lowercase") {
		t.Fatalf("expected tab id rejection, got %v", err)
	}
}
