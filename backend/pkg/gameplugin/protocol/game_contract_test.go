package protocol

import "testing"

func TestPluginHostSpecValidateGenericContract(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		HostFeatures:    []HostFeature{HostFeatureCustomRPC, HostFeatureEventStreaming, HostFeatureMultiService},
		Services: []PluginServiceSpec{
			{ID: "control", ModuleID: "control-runtime", Kind: "process", Required: true},
			{ID: "events", ModuleID: "event-runtime", Kind: "process", Required: true, DependsOn: []string{"control"}},
		},
		Channels:           []PluginChannelSpec{{ID: "events", ServiceID: "events", Kind: "event"}},
		ControlEffectSinks: []PluginControlEffectSinkSpec{{ID: "effect", ServiceID: "control"}},
		Network:            &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestParseGamePluginSpecRejectsGameSpecificFields(t *testing.T) {
	_, err := ParseGamePluginSpec(map[string]any{
		"protocolVersion": ProtocolVersion,
		"runtimeModuleId": "runtime",
		"gameId":          "minecraft-java",
	})
	if err == nil {
		t.Fatal("ParseGamePluginSpec() accepted game-specific gameId")
	}
}

func TestPluginHostSpecRequiresExplicitProtocolVersion(t *testing.T) {
	spec, err := ParseGamePluginSpec(map[string]any{"runtimeModuleId": "runtime"})
	if err != nil {
		t.Fatal(err)
	}
	if spec.ProtocolVersion != "" {
		t.Fatalf("protocol defaulted unexpectedly: %q", spec.ProtocolVersion)
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted missing protocolVersion")
	}
}

func TestPluginHostSpecRejectsUnknownHostFeature(t *testing.T) {
	spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", HostFeatures: []HostFeature{"event_publishing"}}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted unknown host feature")
	}
}

func TestPluginNetworkPolicyOnlyExposesEnforceableModes(t *testing.T) {
	for _, mode := range []string{"none", "loopback", "unrestricted"} {
		spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", Network: &PluginNetworkPolicy{Mode: mode}}
		if err := spec.Validate(); err != nil {
			t.Fatalf("mode %s rejected: %v", mode, err)
		}
	}
	for _, mode := range []string{"restricted", "audit"} {
		spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", Network: &PluginNetworkPolicy{Mode: mode}}
		if err := spec.Validate(); err == nil {
			t.Fatalf("mode %s unexpectedly accepted", mode)
		}
	}
}

func TestPluginHostSpecRejectsUnwiredBinaryChannel(t *testing.T) {
	spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", Channels: []PluginChannelSpec{{ID: "frames", Kind: "binary"}}}
	if err := spec.Validate(); err == nil {
		t.Fatal("binary channel must not be exposed until production binary channel routing is wired")
	}
}
