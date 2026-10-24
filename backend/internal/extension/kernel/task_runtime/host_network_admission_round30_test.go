package task_runtime

import (
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestRound30CloudTaskNetworkDefaultsToIsolation(t *testing.T) {
	for _, policy := range []string{"", "strict", " strict "} {
		got, err := cloudHostNetworkMode(runtimeprofile.ProfileCloudCore, false, policy)
		if err != nil || got != cloudTaskNetworkIsolated {
			t.Fatalf("Cloud Core network was not isolated for policy %q: %q, %v", policy, got, err)
		}
	}
}

func TestRound30CloudTaskNetworkExplicitDegradation(t *testing.T) {
	got, err := cloudHostNetworkMode(runtimeprofile.ProfileCloudCore, false, "allow-degraded")
	if err != nil || got != cloudTaskNetworkDegraded {
		t.Fatalf("explicit operator degradation rejected: %q, %v", got, err)
	}
	for _, value := range []string{"off", "false", "none", "unrestricted", "0", "ALLOW-DEGRADED", "allow-degraded extra"} {
		if got, err := cloudHostNetworkMode(runtimeprofile.ProfileCloudCore, false, value); err == nil || got != "" {
			t.Fatalf("unsafe or unknown cloud network policy was accepted: %q -> %q, %v", value, got, err)
		}
	}
}

func TestRound30CloudNetworkPreservesOtherProfilesAndFullSandbox(t *testing.T) {
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileDeviceAgent} {
		got, err := cloudHostNetworkMode(profile, false, "strict")
		if err != nil || got != cloudTaskNetworkUnrestricted {
			t.Fatalf("existing %s execution profile changed: %s, %v", profile, got, err)
		}
	}
	got, err := cloudHostNetworkMode(runtimeprofile.ProfileCloudCore, true, "strict")
	if err != nil || got != cloudTaskNetworkUnrestricted {
		t.Fatalf("existing independently enforced sandbox was replaced: %s, %v", got, err)
	}
}

func TestRound30CloudNetworkPolicyCannotBeDisabledByAliases(t *testing.T) {
	for _, value := range []string{"disabled", "no-network", "allow-all", "host", "allow_degraded"} {
		_, err := cloudHostNetworkMode(runtimeprofile.ProfileCloudCore, false, value)
		if err == nil || !strings.Contains(err.Error(), "策略") {
			t.Fatalf("unknown cloud mode %q was not rejected: %v", value, err)
		}
	}
}
