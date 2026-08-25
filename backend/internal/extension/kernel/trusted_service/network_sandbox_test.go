package trusted_service

import (
	"errors"
	"reflect"
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
