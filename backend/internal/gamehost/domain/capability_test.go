package domain

import "testing"

func TestValidateHostFeature(t *testing.T) {
	validCases := []HostFeature{
		HostFeatureRealtimeControl, HostFeatureStateStreaming, HostFeatureEventStreaming, HostFeatureCustomRPC, HostFeatureHostAPI, HostFeatureBinaryStreaming,
	}
	for _, cap := range validCases {
		if err := ValidateHostFeature(cap); err != nil {
			t.Errorf("expected known host feature %s to be valid: %v", cap, err)
		}
	}
}

func TestValidateHostFeatureRejectsUnknown(t *testing.T) {
	for _, cap := range []HostFeature{"example.navigation", "vendor.visual-agent", "custom.gameplay", ""} {
		if err := ValidateHostFeature(cap); err == nil {
			t.Errorf("expected unknown host feature %q to be rejected", cap)
		}
	}
}

func TestIsKnownHostFeature(t *testing.T) {
	for _, cap := range []HostFeature{
		HostFeatureRealtimeControl, HostFeatureStateStreaming, HostFeatureEventStreaming, HostFeatureCustomRPC,
		HostFeatureHostAPI, HostFeatureSharedControl, HostFeatureMultiService, HostFeatureBinaryStreaming,
	} {
		if !IsKnownHostFeature(cap) {
			t.Errorf("expected %s to be known", cap)
		}
	}
	for _, cap := range []HostFeature{"example.navigation", "example.build", "vendor.custom-agent", "custom_ui"} {
		if IsKnownHostFeature(cap) {
			t.Errorf("game/tool/non-GameHost capability %s must not be a GameHost feature", cap)
		}
	}
}

func TestAllKnownHostFeatures(t *testing.T) {
	all := AllKnownHostFeatures()
	if len(all) != 8 {
		t.Fatalf("expected 8 known host features, got %d", len(all))
	}
	seen := make(map[HostFeature]struct{}, len(all))
	for _, cap := range all {
		if _, ok := seen[cap]; ok {
			t.Fatalf("duplicate host feature: %s", cap)
		}
		seen[cap] = struct{}{}
		if !IsKnownHostFeature(cap) {
			t.Fatalf("unknown feature returned: %s", cap)
		}
	}
}
