package trusted_service

import (
	"errors"
	"reflect"
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
	for _, want := range []string{"(deny default)", "(allow network*)", `(subpath "/opt/plugin")`, `(subpath "/tmp/work")`} {
		if !containsLiteral(profile, want) {
			t.Fatalf("profile missing %q:\n%s", want, profile)
		}
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
