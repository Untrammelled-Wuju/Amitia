package integration

import (
	"testing"

	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestBuildPluginNetworkPolicyRestrictedIsHostMediated(t *testing.T) {
	policy, err := buildPluginNetworkPolicy(&gameprotocol.PluginNetworkPolicy{
		Mode: "restricted", AllowedDomains: []string{"api.example.com", "*.cdn.example.com"}, AllowedIPs: []string{"127.0.0.1"}, AllowedPorts: []int{443, 8443},
	}, []string{"service.network.request"})
	if err != nil {
		t.Fatal(err)
	}
	if policy.AllowOutbound || policy.AllowInbound || policy.LoopbackOnly || !policy.RequireProxy || !policy.Enforce {
		t.Fatalf("restricted policy grants ambient network access: %+v", policy)
	}
	if len(policy.AllowedDomains) != 2 || len(policy.AllowedIPs) != 1 || len(policy.AllowedPorts) != 2 {
		t.Fatalf("restricted allowlist lost: %+v", policy)
	}
}

func TestBuildPluginNetworkPolicyRestrictedRequiresNetworkPermission(t *testing.T) {
	_, err := buildPluginNetworkPolicy(&gameprotocol.PluginNetworkPolicy{Mode: "restricted", AllowedDomains: []string{"api.example.com"}, AllowedPorts: []int{443}}, nil)
	if err == nil {
		t.Fatal("restricted policy without service.network.request was accepted")
	}
}
