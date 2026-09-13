package runtimegate

import "testing"

func TestFeatureGateDefaultsAndOverrides(t *testing.T) {
	if IsEnabled(EmotionExtensionID) {
		t.Fatal("emotion plugin must default to disabled")
	}
	if IsEnabled(ProactiveExtensionID) {
		t.Fatal("proactive plugin must default to disabled")
	}
	if IsEnabled(LifestyleExtensionID) {
		t.Fatal("lifestyle plugin must default to disabled")
	}
	Set(EmotionExtensionID, true)
	t.Cleanup(func() { Set(EmotionExtensionID, false) })
	if !IsEnabled(EmotionExtensionID) {
		t.Fatal("emotion plugin override must enable the gate")
	}
}

func TestHostRuntimeAllowlist(t *testing.T) {
	if !HostRuntimeAllowed(EmotionExtensionID, "host.character.psyche") {
		t.Fatal("emotion host runtime must be allowed")
	}
	if !HostRuntimeAllowed(ProactiveExtensionID, "host.character.proactive") {
		t.Fatal("proactive host runtime must be allowed")
	}
	if !HostRuntimeAllowed(LifestyleExtensionID, "host.character.lifestyle") {
		t.Fatal("lifestyle host runtime must be allowed")
	}
	if HostRuntimeAllowed(ProactiveExtensionID, "host.character.psyche") {
		t.Fatal("cross-plugin host runtime must be rejected")
	}
}
