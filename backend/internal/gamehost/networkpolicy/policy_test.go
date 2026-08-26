package networkpolicy

import (
	"errors"
	"testing"

	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestBuildRestrictedRequiresPermission(t *testing.T) {
	_, err := Build(&gameprotocol.PluginNetworkPolicy{
		Mode:           "restricted",
		AllowedDomains: []string{"example.com"},
		AllowedPorts:   []int{443},
	}, nil)
	if !errors.Is(err, ErrPermissionRequired) {
		t.Fatalf("Build() error = %v, want ErrPermissionRequired", err)
	}
}

func TestBuildNoneProducesEnforcedDenyAllPolicy(t *testing.T) {
	policy, err := Build(&gameprotocol.PluginNetworkPolicy{Mode: "none"}, nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if policy.Mode != "none" || !policy.Enforce || policy.AllowOutbound || policy.AllowInbound {
		t.Fatalf("unexpected deny-all policy: %+v", policy)
	}
}
