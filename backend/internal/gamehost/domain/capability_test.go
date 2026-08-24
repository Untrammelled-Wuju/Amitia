package domain

import "testing"

func TestValidateCapability(t *testing.T) {
	validCases := []Capability{
		CapabilityRealtimeControl, CapabilityStateStreaming, CapabilityEventStreaming, CapabilityCustomRPC, CapabilityHostAPI,
	}
	for _, cap := range validCases {
		if err := ValidateCapability(cap); err != nil {
			t.Errorf("expected known host feature %s to be valid: %v", cap, err)
		}
	}
}

func TestValidateCapabilityRejectsUnknown(t *testing.T) {
	for _, cap := range []Capability{"example.navigation", "vendor.visual-agent", "custom.gameplay", ""} {
		if err := ValidateCapability(cap); err == nil {
			t.Errorf("expected unknown host feature %q to be rejected", cap)
		}
	}
}

func TestIsKnownCapability(t *testing.T) {
	for _, cap := range []Capability{
		CapabilityRealtimeControl, CapabilityStateStreaming, CapabilityEventStreaming, CapabilityCustomRPC,
		CapabilityHostAPI, CapabilitySharedControl, CapabilityMultiService,
	} {
		if !IsKnownCapability(cap) {
			t.Errorf("expected %s to be known", cap)
		}
	}
	for _, cap := range []Capability{"example.navigation", "example.build", "vendor.custom-agent", "custom_ui"} {
		if IsKnownCapability(cap) {
			t.Errorf("game/tool/non-GameHost capability %s must not be a GameHost feature", cap)
		}
	}
}

func TestAllKnownCapabilities(t *testing.T) {
	all := AllKnownCapabilities()
	if len(all) != 7 {
		t.Fatalf("expected 7 known host features, got %d", len(all))
	}
	seen := make(map[Capability]struct{}, len(all))
	for _, cap := range all {
		if _, ok := seen[cap]; ok {
			t.Fatalf("duplicate host feature: %s", cap)
		}
		seen[cap] = struct{}{}
		if !IsKnownCapability(cap) {
			t.Fatalf("unknown feature returned: %s", cap)
		}
	}
}
