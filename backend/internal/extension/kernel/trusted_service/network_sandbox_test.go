package trusted_service

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
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

func TestValidateNetworkPolicySupportAcceptsHostMediatedRestricted(t *testing.T) {
	policy := ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedDomains: []string{"api.example.com", "*.cdn.example.com"},
		AllowedPorts:   []int{443, 8443},
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if err := validateNetworkPolicySupportForOS(policy, goos); err != nil {
			t.Fatalf("%s restricted policy rejected: %v", goos, err)
		}
	}
}

func TestValidateNetworkPolicySupportAcceptsExactIPRestricted(t *testing.T) {
	policy := ServiceNetworkPolicy{
		Mode: "restricted", Enforce: true, RequireProxy: true,
		AllowedIPs: []string{"127.0.0.1", "2001:db8::10"}, AllowedPorts: []int{18080},
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if err := validateNetworkPolicySupportForOS(policy, goos); err != nil {
			t.Fatalf("%s exact-IP restricted policy rejected: %v", goos, err)
		}
	}
}

func TestValidateNetworkPolicySupportRejectsRestrictedAmbientOutbound(t *testing.T) {
	policy := ServiceNetworkPolicy{Mode: "restricted", Enforce: true, RequireProxy: true, AllowOutbound: true, AllowedDomains: []string{"example.com"}, AllowedPorts: []int{443}}
	if err := validateNetworkPolicySupportForOS(policy, "linux"); !errors.Is(err, ErrUnauthorizedNetwork) {
		t.Fatalf("restricted ambient outbound error = %v, want ErrUnauthorizedNetwork", err)
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

func TestBuildDarwinSandboxProfileRestrictedHasNoAmbientNetwork(t *testing.T) {
	profile, err := buildDarwinSandboxProfile("restricted", "/opt/runtime", "/tmp/work", "/tmp/temp", "/opt/plugin")
	if err != nil {
		t.Fatal(err)
	}
	if containsLiteral(profile, "network-outbound") || containsLiteral(profile, "network-inbound") || containsLiteral(profile, "network*") {
		t.Fatalf("restricted profile grants ambient networking:\n%s", profile)
	}
}

func TestValidateNetworkPolicySupportRejectsUnsupportedPlatform(t *testing.T) {
	policy := ServiceNetworkPolicy{Mode: "restricted", Enforce: true, RequireProxy: true, AllowedIPs: []string{"127.0.0.1"}, AllowedPorts: []int{443}}
	for _, goos := range []string{"freebsd", "android", "ios", ""} {
		if err := validateNetworkPolicySupportForOS(policy, goos); !errors.Is(err, ErrNetworkSandboxUnavailable) {
			t.Fatalf("%q unsupported platform error = %v, want ErrNetworkSandboxUnavailable", goos, err)
		}
	}
}

func TestValidateNetworkPolicySupportAcceptsUnrestrictedOnSupportedDesktopOSes(t *testing.T) {
	policy := ServiceNetworkPolicy{Mode: "unrestricted", Enforce: true, AllowOutbound: true}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if err := validateNetworkPolicySupportForOS(policy, goos); err != nil {
			t.Fatalf("%s unrestricted policy should pass static validation: %v", goos, err)
		}
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

func TestPrepareNetworkLaunchLinuxUnrestrictedRequiresConcreteSandboxBackends(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific enforcement behavior")
	}
	bwrap := firstTrustedLauncher("/usr/bin/bwrap", "/bin/bwrap")
	slirp := firstTrustedLauncher("/usr/bin/slirp4netns", "/bin/slirp4netns")
	plan, err := prepareNetworkLaunch(ServiceNetworkPolicy{
		Mode:          "unrestricted",
		Enforce:       true,
		AllowOutbound: true,
	}, "/bin/true", nil, "/tmp", "/tmp")
	if bwrap == "" || slirp == "" {
		if !errors.Is(err, ErrNetworkSandboxUnavailable) {
			t.Fatalf("missing linux backend error = %v, want ErrNetworkSandboxUnavailable", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("prepareNetworkLaunch() error = %v", err)
	}
	defer plan.cleanup()
	if plan.Path != bwrap || !plan.FilesystemIsolated || !plan.NetworkPolicyEnforced || plan.AfterStart == nil || len(plan.ExtraFiles) != 2 {
		t.Fatalf("unexpected unrestricted linux launch plan: %+v", plan)
	}
}

func TestPrepareLinuxSlirpLaunchUsesBlockAndInfoFDs(t *testing.T) {
	plan, err := prepareLinuxSlirpLaunch("/usr/bin/bwrap", []string{"--unshare-net"}, "/bin/true", []string{"--flag"}, "/tmp", "/usr/bin/slirp4netns")
	if err != nil {
		t.Fatal(err)
	}
	defer plan.cleanup()
	if len(plan.ExtraFiles) != 2 || plan.AfterStart == nil {
		t.Fatalf("slirp launch coordination missing: %+v", plan)
	}
	if len(plan.Args) < 11 {
		t.Fatalf("slirp launch args are incomplete: %v", plan.Args)
	}
	if !containsSequence(plan.Args, []string{"--ro-bind", plan.Args[2], "/etc/resolv.conf"}) || !strings.Contains(plan.Args[2], "amitia-gamehost-resolv-") {
		t.Fatalf("slirp launch is missing host-owned resolver config: %v", plan.Args)
	}
	if !containsSequence(plan.Args, []string{"--block-fd", "3", "--info-fd", "4", "--", "/bin/true", "--flag"}) {
		t.Fatalf("slirp launch is missing block/info fd coordination: %v", plan.Args)
	}
}

func containsSequence(values, sequence []string) bool {
	if len(sequence) == 0 || len(values) < len(sequence) {
		return false
	}
	for i := 0; i+len(sequence) <= len(values); i++ {
		if reflect.DeepEqual(values[i:i+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func containsLiteral(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
