package main

import (
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

func TestOwnedToolScopeCarriesSandboxConstraint(t *testing.T) {
	inference := business.Inference{SandboxRoot: "/tmp/channel-jail", SandboxAllowNetwork: false}
	scope := ownedToolScope(inference, "call-1")
	if scope.ExecContext == nil {
		t.Fatal("execution context missing")
	}
	if scope.ExecContext.Sandbox == nil {
		t.Fatal("sandbox constraint was not propagated to the tool execution context")
	}
	if scope.ExecContext.Sandbox.Root != "/tmp/channel-jail" {
		t.Fatalf("unexpected sandbox root %q", scope.ExecContext.Sandbox.Root)
	}
	if scope.ExecContext.Sandbox.AllowNetwork {
		t.Fatal("sandbox network flag not propagated")
	}
}

func TestOwnedToolScopeWithoutSandboxStaysUnrestricted(t *testing.T) {
	scope := ownedToolScope(business.Inference{}, "call-2")
	if scope.ExecContext == nil {
		t.Fatal("execution context missing")
	}
	if scope.ExecContext.Sandbox != nil {
		t.Fatal("regular conversations must not receive a sandbox constraint")
	}
}
