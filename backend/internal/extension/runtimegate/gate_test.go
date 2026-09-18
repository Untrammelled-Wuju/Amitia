package runtimegate

import "testing"

func TestFeatureGateDefaultsAndOverrides(t *testing.T) {
	if IsEnabled(EmotionExtensionID) {
		t.Fatal("emotion plugin must default to disabled")
	}
	Set(EmotionExtensionID, true)
	t.Cleanup(func() { Set(EmotionExtensionID, false) })
	if !IsEnabled(EmotionExtensionID) {
		t.Fatal("emotion plugin override must enable the gate")
	}
}

func TestHostRuntimeAllowlist(t *testing.T) {
	if !HostRuntimeAllowed(HostRuntimeCharacterPsyche) {
		t.Fatal("psyche host runtime must be allowed")
	}
	if HostRuntimeAllowed("host.character.unknown") {
		t.Fatal("unknown host runtime must be rejected")
	}
}
