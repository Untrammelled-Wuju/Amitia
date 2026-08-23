package protocol

import "testing"

func TestParseGamePluginSpecDefaultsGameProtocolV2(t *testing.T) {
	spec, err := ParseGamePluginSpec(map[string]any{
		"protocolVersion": ProtocolVersion,
		"runtimeModuleId": "runtime",
		"gameId":          "minecraft-java",
	})
	if err != nil {
		t.Fatalf("ParseGamePluginSpec() error = %v", err)
	}
	if spec.GameProtocolVersion != GameProtocolVersion {
		t.Fatalf("GameProtocolVersion = %q, want %q", spec.GameProtocolVersion, GameProtocolVersion)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestGamePluginSpecRejectsUnsupportedGameProtocol(t *testing.T) {
	spec, err := ParseGamePluginSpec(map[string]any{
		"protocolVersion":     ProtocolVersion,
		"gameProtocolVersion": "amitia-game/999",
		"runtimeModuleId":     "runtime",
	})
	if err != nil {
		t.Fatalf("ParseGamePluginSpec() error = %v", err)
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsupported game protocol error")
	}
}

func TestGamePluginSpecValidatesCompanionArtifacts(t *testing.T) {
	spec, err := ParseGamePluginSpec(map[string]any{
		"protocolVersion": ProtocolVersion,
		"runtimeModuleId": "runtime",
		"companionArtifacts": []any{
			map[string]any{
				"id":     "bridge",
				"type":   "fabric_mod",
				"source": "companion/bridge.jar",
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseGamePluginSpec() error = %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	spec.CompanionArtifacts[0].Source = ""
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want companion artifact validation error")
	}
}
