package domain

import "testing"

func TestValidateCapability(t *testing.T) {
	validCases := []Capability{
		"realtime_control",
		"state_streaming",
		"minecraft.pathfinding",
		"vendor.visual-agent",
		"custom.gameplay",
		"a",
	}

	for _, cap := range validCases {
		t.Run("valid_"+string(cap), func(t *testing.T) {
			if err := ValidateCapability(cap); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateCapabilityRejectsEmpty(t *testing.T) {
	err := ValidateCapability("")
	if err == nil {
		t.Fatal("expected error for empty capability")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestValidateCapabilityRejectsTooLong(t *testing.T) {
	longCap := Capability(make([]byte, 200))
	err := ValidateCapability(longCap)
	if err == nil {
		t.Fatal("expected error for too long capability")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestValidateCapabilityRejectsControlCharacters(t *testing.T) {
	err := ValidateCapability("bad\x00cap")
	if err == nil {
		t.Fatal("expected error for control character in capability")
	}
	if !IsHostError(err, ErrInvalidArgument) {
		t.Errorf("expected invalid_argument, got %v", err)
	}
}

func TestIsKnownCapability(t *testing.T) {
	knownCases := []Capability{
		CapabilityRealtimeControl,
		CapabilityStateStreaming,
		CapabilityEventStreaming,
		CapabilityBinaryStreaming,
		CapabilityCustomRPC,
		CapabilityHostAPI,
		CapabilitySharedControl,
		CapabilityCustomUI,
		CapabilityMultiService,
	}

	for _, cap := range knownCases {
		t.Run("known_"+string(cap), func(t *testing.T) {
			if !IsKnownCapability(cap) {
				t.Errorf("expected %s to be a known capability", cap)
			}
		})
	}

	unknownCases := []Capability{
		"minecraft.pathfinding",
		"minecraft.building",
		"vendor.custom-agent",
		"custom.gameplay",
	}

	for _, cap := range unknownCases {
		t.Run("unknown_"+string(cap), func(t *testing.T) {
			if IsKnownCapability(cap) {
				t.Errorf("expected %s to not be a known capability", cap)
			}
			if err := ValidateCapability(cap); err != nil {
				t.Errorf("unknown but valid capability %s should pass validation: %v", cap, err)
			}
		})
	}
}

func TestAllKnownCapabilities(t *testing.T) {
	all := AllKnownCapabilities()
	if len(all) != 9 {
		t.Errorf("expected 9 known capabilities, got %d", len(all))
	}

	seen := make(map[Capability]struct{})
	for _, cap := range all {
		if _, exists := seen[cap]; exists {
			t.Errorf("duplicate capability in AllKnownCapabilities: %s", cap)
		}
		seen[cap] = struct{}{}
		if !IsKnownCapability(cap) {
			t.Errorf("capability %s returned by AllKnownCapabilities but IsKnownCapability returns false", cap)
		}
	}
}
