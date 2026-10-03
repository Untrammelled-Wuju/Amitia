package security

import "testing"

func TestLocalActorsCarryPersistentDeviceIdentity(t *testing.T) {
	cfg := AuthConfig{SpaceID: "space-a", LocalDeviceID: "device-a", LocalRuntimeID: "runtime-a"}
	local := buildLocalActor(cfg, AuthMethodLocalToken)
	desktop := buildDesktopSessionActor(&DesktopSession{ID: "session-a", SpaceID: "space-a"}, cfg)
	if local.DeviceID != cfg.LocalDeviceID || local.RuntimeID != cfg.LocalRuntimeID || !local.IsLocalTrusted {
		t.Fatal("local token actor lost device identity")
	}
	if desktop.DeviceID != cfg.LocalDeviceID || desktop.RuntimeID != cfg.LocalRuntimeID || desktop.SessionID != "session-a" {
		t.Fatal("desktop session actor lost device identity")
	}
}
