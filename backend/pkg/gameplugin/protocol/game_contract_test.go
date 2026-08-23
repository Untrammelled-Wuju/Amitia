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
				"id":            "bridge",
				"type":          "fabric_mod",
				"source":        "companion/bridge.jar",
				"installTarget": "mods/amitia-bridge.jar",
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

func TestGamePluginSpecValidatesNetworkPolicy(t *testing.T) {
	spec, err := ParseGamePluginSpec(map[string]any{
		"protocolVersion": ProtocolVersion,
		"runtimeModuleId": "runtime",
		"network": map[string]any{
			"mode":           "restricted",
			"allowedDomains": []any{"example.com"},
			"allowedPorts":   []any{25565.0},
		},
	})
	if err != nil {
		t.Fatalf("ParseGamePluginSpec() error = %v", err)
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	spec.Network.Mode = "unrestricted"
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unrestricted policy to reject granular allowlists")
	}
}

func TestGameSessionOpenRequestAutoInstallDefaultsTrue(t *testing.T) {
	var req GameSessionOpenRequest
	if !req.ShouldAutoInstallCompanions() {
		t.Fatal("auto install should default to true")
	}
	v := false
	req.AutoInstallCompanions = &v
	if req.ShouldAutoInstallCompanions() {
		t.Fatal("explicit false should disable automatic companion installation")
	}
}

func TestGamePluginSpecRejectsDuplicateCompanionArtifactIDs(t *testing.T) {
	spec := GamePluginSpec{
		ProtocolVersion: ProtocolVersion,
		RuntimeModuleID: "runtime",
		CompanionArtifacts: []GameCompanionArtifact{
			{ID: "bridge", Type: "fabric_mod", Source: "companion/a.jar", InstallTarget: "mods/a.jar"},
			{ID: "bridge", Type: "fabric_mod", Source: "companion/b.jar", InstallTarget: "mods/b.jar"},
		},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want duplicate companion artifact id error")
	}
}

func TestGamePluginSpecRejectsUnsafeCompanionPathsCrossPlatform(t *testing.T) {
	cases := []GameCompanionArtifact{
		{ID: "a", Type: "fabric_mod", Source: "../escape.jar", InstallTarget: "mods/a.jar"},
		{ID: "b", Type: "fabric_mod", Source: "companion/b.jar", InstallTarget: "../escape.jar"},
		{ID: "c", Type: "fabric_mod", Source: `C:\\evil.jar`, InstallTarget: "mods/c.jar"},
	}
	for _, artifact := range cases {
		spec := GamePluginSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", CompanionArtifacts: []GameCompanionArtifact{artifact}}
		if err := spec.Validate(); err == nil {
			t.Fatalf("Validate() accepted unsafe artifact %+v", artifact)
		}
	}
}

func TestGamePluginSpecRejectsEmptyRestrictedNetworkPolicy(t *testing.T) {
	spec := GamePluginSpec{
		ProtocolVersion: ProtocolVersion,
		RuntimeModuleID: "runtime",
		Network:         &GameNetworkPolicy{Mode: "restricted"},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want restricted policy without allowlist/proxy to fail")
	}
}

func TestGamePluginSpecRejectsProxyOutsideRestrictedMode(t *testing.T) {
	spec := GamePluginSpec{
		ProtocolVersion: ProtocolVersion,
		RuntimeModuleID: "runtime",
		Network:         &GameNetworkPolicy{Mode: "unrestricted", RequireProxy: true},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want requireProxy outside restricted mode to fail")
	}
}
