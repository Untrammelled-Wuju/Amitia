package main

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestDeviceAgentProviderCoreFollowsRuntimeMode(t *testing.T) {
	credential := &agent.StoredCredential{SpaceID: "remote-core"}
	for _, test := range []struct {
		name       string
		profile    runtimeprofile.Profile
		credential *agent.StoredCredential
		want       string
	}{
		{name: "local center", profile: runtimeprofile.ProfileLocal, credential: credential, want: "local-core"},
		{name: "cloud center", profile: runtimeprofile.ProfileCloudCore, credential: credential, want: "local-core"},
		{name: "device agent", profile: runtimeprofile.ProfileDeviceAgent, credential: credential, want: "remote-core"},
		{name: "device agent without credential", profile: runtimeprofile.ProfileDeviceAgent, want: "local-core"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := deviceAgentProviderCore(test.profile, "local-core", test.credential); got != test.want {
				t.Fatalf("provider core = %q, want %q", got, test.want)
			}
		})
	}
}
