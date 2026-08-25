package protocol

import "testing"

func TestPluginHostSpecValidateGenericContract(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		HostFeatures:    []HostFeature{HostFeatureCustomRPC, HostFeatureEventStreaming, HostFeatureMultiService, HostFeatureRealtimeControl},
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

func TestPluginHostSpecAllowsServiceScopedChannelIDsAndBidirectionalFlow(t *testing.T) {
	frequency := FrequencyHintRealtime
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		HostFeatures:    []HostFeature{HostFeatureMultiService, HostFeatureEventStreaming},
		Services: []PluginServiceSpec{
			{ID: "one", ModuleID: "module-one"},
			{ID: "two", ModuleID: "module-two"},
		},
		Channels: []PluginChannelSpec{
			{ID: "events", ServiceID: "one", Kind: "event", Direction: ChannelDirectionBidirectional, FrequencyHint: &frequency},
			{ID: "events", ServiceID: "two", Kind: "event", Direction: ChannelDirectionHostToPlugin},
		},
		Network: &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() rejected service-scoped channel ids: %v", err)
	}
}

func TestPluginHostSpecRejectsInvalidChannelContractFields(t *testing.T) {
	tooLongSchema := string(make([]byte, 1025))
	cases := []PluginChannelSpec{
		{ID: "bad\x00id", Kind: "event"},
		{ID: "events", Kind: "event", SchemaID: tooLongSchema},
		{ID: "events", Kind: "event", Direction: ChannelDirection("sideways")},
	}
	for i, channel := range cases {
		spec := PluginHostSpec{
			ProtocolVersion: ProtocolVersion,
			RuntimeModuleID: "runtime",
			HostFeatures:    []HostFeature{HostFeatureEventStreaming},
			Channels:        []PluginChannelSpec{channel},
			Network:         &PluginNetworkPolicy{Mode: "none"},
		}
		if err := spec.Validate(); err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
	}
}

func TestParsePluginHostSpecRejectsGameSpecificFields(t *testing.T) {
	_, err := ParsePluginHostSpec(map[string]any{
		"protocolVersion": ProtocolVersion,
		"runtimeModuleId": "runtime",
		"gameId":          "examplegame-java",
		"network":         map[string]any{"mode": "none"},
	})
	if err == nil {
		t.Fatal("ParsePluginHostSpec() accepted game-specific gameId")
	}
}

func TestPluginHostSpecRequiresExplicitProtocolVersion(t *testing.T) {
	spec, err := ParsePluginHostSpec(map[string]any{"runtimeModuleId": "runtime", "network": map[string]any{"mode": "none"}})
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
	spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", HostFeatures: []HostFeature{"event_publishing"}, Network: &PluginNetworkPolicy{Mode: "none"}}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted unknown host feature")
	}
}

func TestPluginNetworkPolicyExposesProtocolV1Modes(t *testing.T) {
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

func TestPluginHostSpecBinaryChannelRequiresNegotiatedFeature(t *testing.T) {
	spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime", Channels: []PluginChannelSpec{{ID: "frames", Kind: "binary"}}, Network: &PluginNetworkPolicy{Mode: "none"}}
	if err := spec.Validate(); err == nil {
		t.Fatal("binary channel without binary_streaming host feature must be rejected")
	}
	spec.HostFeatures = []HostFeature{HostFeatureBinaryStreaming}
	if err := spec.Validate(); err != nil {
		t.Fatalf("binary channel with binary_streaming feature rejected: %v", err)
	}
}

func TestHostFeatureVersionPolicyIsProtocolMajorBound(t *testing.T) {
	for feature := range knownHostFeatures {
		major, ok := HostFeatureIntroducedInMajor(feature)
		if !ok || major != 1 {
			t.Fatalf("feature %q version policy = (%d,%v), want (1,true)", feature, major, ok)
		}
		if !HostFeatureSupportedByCurrentProtocol(feature) {
			t.Fatalf("current protocol does not support known feature %q", feature)
		}
	}
	if _, ok := HostFeatureIntroducedInMajor(HostFeature("vendor.experimental")); ok {
		t.Fatal("unknown namespaced feature must require a protocol-major update")
	}
}

func TestPluginHostSpecRequiresExplicitNetworkPolicy(t *testing.T) {
	spec := PluginHostSpec{ProtocolVersion: ProtocolVersion, RuntimeModuleID: "runtime"}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted missing explicit network policy")
	}
}

func TestPluginHostSpecRejectsDuplicateServiceDependency(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		HostFeatures:    []HostFeature{HostFeatureMultiService},
		Services: []PluginServiceSpec{
			{ID: "a", ModuleID: "module-a", DependsOn: []string{"b", "b"}},
			{ID: "b", ModuleID: "module-b"},
		},
		Network: &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted duplicate service dependency")
	}
}

func TestPluginHostSpecRejectsInvalidServiceIDsAcrossReferences(t *testing.T) {
	tooLong := string(make([]byte, 257))
	cases := []PluginHostSpec{
		{
			ProtocolVersion: ProtocolVersion,
			Services:        []PluginServiceSpec{{ID: "bad id", ModuleID: "module-a"}},
			Network:         &PluginNetworkPolicy{Mode: "none"},
		},
		{
			ProtocolVersion: ProtocolVersion,
			Services:        []PluginServiceSpec{{ID: tooLong, ModuleID: "module-a"}},
			Network:         &PluginNetworkPolicy{Mode: "none"},
		},
		{
			ProtocolVersion: ProtocolVersion,
			HostFeatures:    []HostFeature{HostFeatureMultiService},
			Services: []PluginServiceSpec{
				{ID: "a", ModuleID: "module-a", DependsOn: []string{"bad id"}},
				{ID: "b", ModuleID: "module-b"},
			},
			Network: &PluginNetworkPolicy{Mode: "none"},
		},
		{
			ProtocolVersion: ProtocolVersion,
			RuntimeModuleID: "module-a",
			HostFeatures:    []HostFeature{HostFeatureEventStreaming},
			Channels:        []PluginChannelSpec{{ID: "events", ServiceID: "bad id", Kind: "event"}},
			Network:         &PluginNetworkPolicy{Mode: "none"},
		},
		{
			ProtocolVersion: ProtocolVersion,
			RuntimeModuleID: "module-a",
			HostFeatures:    []HostFeature{HostFeatureRealtimeControl},
			ControlEffectSinks: []PluginControlEffectSinkSpec{
				{ID: "control", ServiceID: "bad id"},
			},
			Network: &PluginNetworkPolicy{Mode: "none"},
		},
	}

	for i, spec := range cases {
		if err := spec.Validate(); err == nil {
			t.Fatalf("case %d unexpectedly accepted invalid service id", i)
		}
	}
}

func TestPluginHostSpecRejectsInvalidServiceName(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		Services: []PluginServiceSpec{
			{ID: "service-a", ModuleID: "module-a", Name: string(make([]byte, maxServiceNameLength+1))},
		},
		Network: &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted oversized service name")
	}
}

func TestPluginHostSpecRejectsServiceDependencyCycle(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		HostFeatures:    []HostFeature{HostFeatureMultiService},
		Services: []PluginServiceSpec{
			{ID: "a", ModuleID: "module-a", DependsOn: []string{"b"}},
			{ID: "b", ModuleID: "module-b", DependsOn: []string{"c"}},
			{ID: "c", ModuleID: "module-c", DependsOn: []string{"a"}},
		},
		Network: &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted cyclic service dependency graph")
	}
}

func TestPluginHostSpecRuntimeModuleIDMustBelongToDeclaredServices(t *testing.T) {
	spec := PluginHostSpec{
		ProtocolVersion: ProtocolVersion,
		RuntimeModuleID: "legacy-runtime",
		HostFeatures:    []HostFeature{HostFeatureMultiService},
		Services: []PluginServiceSpec{
			{ID: "a", ModuleID: "module-a"},
			{ID: "b", ModuleID: "module-b"},
		},
		Network: &PluginNetworkPolicy{Mode: "none"},
	}
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() accepted runtimeModuleId outside declared service modules")
	}
	spec.RuntimeModuleID = "module-a"
	if err := spec.Validate(); err != nil {
		t.Fatalf("Validate() rejected compatible legacy runtimeModuleId: %v", err)
	}
}

func TestPluginArtifactCompatibilityConstraintValidation(t *testing.T) {
	valid := []string{"1.21.4", "build-2026-a", "*", "1.21.x", ">=1.20.1 <1.22", "^1.20.1", "~1.20.2", "1.20.1 - 1.21.4", ">=1.20 <1.21 || >=1.21.4 <1.22"}
	for _, constraint := range valid {
		if err := ValidateCompatibilityConstraint(constraint); err != nil {
			t.Fatalf("constraint %q rejected: %v", constraint, err)
		}
	}
	invalid := []string{"", "1.x.3", ">=", "1.2 ||", "1.2 - broken"}
	for _, constraint := range invalid {
		if err := ValidateCompatibilityConstraint(constraint); err == nil {
			t.Fatalf("constraint %q unexpectedly accepted", constraint)
		}
	}
}

func TestCompatibilityVersionMatchesRangesAndRequiresExplicitVersion(t *testing.T) {
	if CompatibilityVersionMatches([]string{"1.21.x"}, "") {
		t.Fatal("declared compatibility constraint must not match a missing version")
	}
	if !CompatibilityVersionMatches([]string{">=1.20.1 <1.22"}, "1.21.4") {
		t.Fatal("expected range to match 1.21.4")
	}
	if CompatibilityVersionMatches([]string{">=1.20.1 <1.22"}, "1.22.0") {
		t.Fatal("range unexpectedly matched upper bound")
	}
	if !CompatibilityVersionMatches([]string{"build-2026-a"}, "BUILD-2026-A") {
		t.Fatal("opaque exact identifiers should remain case-insensitive")
	}
}
