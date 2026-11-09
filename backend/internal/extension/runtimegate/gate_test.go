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
	if !HostRuntimeAllowed(HostRuntimeCharacterActiveTask) {
		t.Fatal("active task host runtime must be allowed")
	}
	for _, runtimeID := range []string{
		HostRuntimeKernelWasm,
		HostRuntimeKernelHooks,
		HostRuntimeKernelTrusted,
		HostRuntimeKernelTasks,
		HostRuntimeKernelEvents,
		HostRuntimeKernelSchedules,
		HostRuntimeKernelDesktop,
		HostRuntimeKernelUpdates,
		HostRuntimeKernelDevConsole,
		HostRuntimeKernelDevMode,
	} {
		if !HostRuntimeAllowed(runtimeID) {
			t.Fatalf("%s host runtime must be allowed", runtimeID)
		}
	}
	if HostRuntimeAllowed("host.character.unknown") {
		t.Fatal("unknown host runtime must be rejected")
	}
}
