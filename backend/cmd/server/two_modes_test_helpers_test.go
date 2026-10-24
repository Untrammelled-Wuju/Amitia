package main

import "testing"

func ensureCoordinatedForTest(t *testing.T, provider, source *threeCoreFixture, revision int64) {
	t.Helper()
	policy, err := provider.services.DeviceMesh.Coordination.Get(t.Context(), provider.core, source.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	if policy.Coordinated {
		return
	}
	if _, err := provider.services.DeviceMesh.Coordination.InitializeCoordinated(t.Context(), provider.core, source.device.DeviceID.String(), revision, "one"); err != nil {
		t.Fatal(err)
	}
}
