package trusted_service

import (
	"errors"
	"reflect"
	"runtime"
	"testing"
)

func TestPrepareNetworkLaunchWithoutEnforcementReturnsDirectPlan(t *testing.T) {
	plan, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:          "unrestricted",
		Enforce:       false,
		AllowOutbound: true,
	}, "/tmp/runtime", []string{"--x"}, "/tmp/work", "/tmp/temp")
	if err != nil {
		t.Fatalf("prepareNetworkLaunch() error = %v", err)
	}
	if plan.Path != "/tmp/runtime" || len(plan.Args) != 1 || plan.Args[0] != "--x" || plan.WorkingDir != "/tmp/work" {
		t.Fatalf("unexpected launch plan: %+v", plan)
	}
}

func TestPrepareNetworkLaunchRejectsGranularPolicyWithoutBackend(t *testing.T) {
	_, err := prepareNetworkLaunch(ServiceNetworkPolicy{
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
	_, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Enforce:       true,
		AllowInbound:  true,
		AllowOutbound: true,
	}, "/tmp/runtime", nil, "", "")
	if !errors.Is(err, ErrUnauthorizedNetwork) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrUnauthorizedNetwork", err)
	}
}

func TestPrepareNetworkLaunchRejectsAuditWithoutAuditBackend(t *testing.T) {
	_, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Enforce:  true,
		AuditAll: true,
	}, "/tmp/runtime", nil, "", "")
	if !errors.Is(err, ErrNetworkSandboxUnavailable) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrNetworkSandboxUnavailable", err)
	}
}

func TestAppendSandboxParentDirsCreatesParentsInOrderWithoutDuplicates(t *testing.T) {
	created := map[string]struct{}{`/`: {}}
	args := appendSandboxParentDirs(nil, `/opt/amitia/plugins/game/index.js`, created)
	want := []string{`--dir`, `/opt`, `--dir`, `/opt/amitia`, `--dir`, `/opt/amitia/plugins`, `--dir`, `/opt/amitia/plugins/game`}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("appendSandboxParentDirs() = %v, want %v", args, want)
	}
	again := appendSandboxParentDirs(args, `/opt/amitia/plugins/game/node_modules/x.js`, created)
	want = append(want, `--dir`, `/opt/amitia/plugins/game/node_modules`)
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("second appendSandboxParentDirs() = %v, want %v", again, want)
	}
}

func TestPathWithinRejectsSiblingPrefix(t *testing.T) {
	if !pathWithin(`/opt/plugin`, `/opt/plugin/dist/index.js`) {
		t.Fatal("expected descendant to be within root")
	}
	if pathWithin(`/opt/plugin`, `/opt/plugin-evil/index.js`) {
		t.Fatal("sibling prefix must not be treated as within root")
	}
}

func TestBuildDarwinSandboxProfileUnrestrictedStillRestrictsFilesystem(t *testing.T) {
	profile, err := buildDarwinSandboxProfile("unrestricted", "/opt/runtime", "/tmp/work", "/tmp/temp", "/opt/plugin")
	if err != nil {
		t.Fatalf("buildDarwinSandboxProfile() error = %v", err)
	}
	for _, want := range []string{"(deny default)", "(allow network-outbound)", `(subpath "/opt/plugin")`, `(subpath "/tmp/work")`} {
		if !containsLiteral(profile, want) {
			t.Fatalf("profile missing %q:\n%s", want, profile)
		}
	}
}

func TestBuildDarwinSandboxProfileUnrestrictedDoesNotAllowInbound(t *testing.T) {
	profile, err := buildDarwinSandboxProfile("unrestricted", "/opt/runtime", "/tmp/work", "/tmp/temp", "/opt/plugin")
	if err != nil {
		t.Fatalf("buildDarwinSandboxProfile() error = %v", err)
	}
	if containsLiteral(profile, "(allow network*)") || containsLiteral(profile, "network-inbound") {
		t.Fatalf("unrestricted outbound profile unexpectedly allows inbound networking:\n%s", profile)
	}
	if !containsLiteral(profile, "(allow network-outbound)") {
		t.Fatalf("unrestricted outbound profile is missing network-outbound rule:\n%s", profile)
	}
}

func TestValidateNetworkPolicySupportRejectsLinuxUnrestrictedBeforeLaunch(t *testing.T) {
	policy := ServiceNetworkPolicy{Mode: "unrestricted", Enforce: true, AllowOutbound: true}
	if err := validateNetworkPolicySupportForOS(policy, "linux"); !errors.Is(err, ErrGranularNetworkPolicyUnsupported) {
		t.Fatalf("validateNetworkPolicySupportForOS() error = %v, want ErrGranularNetworkPolicyUnsupported", err)
	}
	if err := validateNetworkPolicySupportForOS(policy, "darwin"); err != nil {
		t.Fatalf("darwin unrestricted policy should pass static capability validation: %v", err)
	}
	if err := validateNetworkPolicySupportForOS(policy, "windows"); err != nil {
		t.Fatalf("windows unrestricted policy should pass static capability validation: %v", err)
	}
}

func TestValidateNetworkPolicySupportAcceptsLinuxNoneAndLoopback(t *testing.T) {
	for _, policy := range []ServiceNetworkPolicy{
		{Mode: "none", Enforce: true},
		{Mode: "loopback", Enforce: true, AllowOutbound: true, LoopbackOnly: true},
	} {
		if err := validateNetworkPolicySupportForOS(policy, "linux"); err != nil {
			t.Fatalf("linux mode %q should pass static capability validation: %v", policy.Mode, err)
		}
	}
}

func TestPrepareNetworkLaunchLinuxUnrestrictedFailsClosedWithoutOutboundOnlyBackend(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific enforcement behavior")
	}
	_, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:          "unrestricted",
		Enforce:       true,
		AllowOutbound: true,
	}, "/tmp/runtime", nil, "/tmp/work", "/tmp/temp")
	if !errors.Is(err, ErrGranularNetworkPolicyUnsupported) {
		t.Fatalf("prepareNetworkLaunch() error = %v, want ErrGranularNetworkPolicyUnsupported", err)
	}
}

func containsLiteral(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
