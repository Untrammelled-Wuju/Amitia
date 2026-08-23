package trusted_service

import (
	"errors"
	"testing"
)

func TestPrepareNetworkLaunchUnrestrictedExplicitPolicyDoesNotWrap(t *testing.T) {
	path, args, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:          "unrestricted",
		Enforce:       true,
		AllowOutbound: true,
	}, "/tmp/runtime", []string{"--x"}, "/tmp/work", "/tmp/temp")
	if err != nil {
		t.Fatalf("prepareNetworkLaunch() error = %v", err)
	}
	if path != "/tmp/runtime" || len(args) != 1 || args[0] != "--x" {
		t.Fatalf("unexpected launch plan: path=%q args=%v", path, args)
	}
}

func TestPrepareNetworkLaunchRejectsGranularPolicyWithoutBackend(t *testing.T) {
	_, _, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:           "restricted",
		Enforce:        true,
		AllowOutbound:  true,
		AllowedDomains: []string{"example.com"},
		AllowedPorts:   []int{25565},
	}, "/tmp/runtime", nil, "", "")
	if !errors.Is(err, ErrGranularNetworkPolicyUnsupported) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrGranularNetworkPolicyUnsupported", err)
	}
}

func TestPrepareNetworkLaunchRejectsNonLoopbackInbound(t *testing.T) {
	_, _, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Enforce:       true,
		AllowInbound:  true,
		AllowOutbound: true,
	}, "/tmp/runtime", nil, "", "")
	if !errors.Is(err, ErrUnauthorizedNetwork) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrUnauthorizedNetwork", err)
	}
}

func TestPrepareNetworkLaunchRejectsAuditWithoutAuditBackend(t *testing.T) {
	_, _, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Enforce:  true,
		AuditAll: true,
	}, "/tmp/runtime", nil, "", "")
	if !errors.Is(err, ErrNetworkSandboxUnavailable) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrNetworkSandboxUnavailable", err)
	}
}
